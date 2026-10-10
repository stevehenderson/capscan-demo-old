---
description: Weekly review of capabilities no Dependabot pull request surfaces, such as language version gates and drifted dependencies.
on:
  schedule: weekly on monday
  workflow_dispatch:

permissions:
  contents: read

engine: copilot
max-turns: 40
timeout-minutes: 20

network:
  allowed:
    - defaults
    - github.com
    - docs.github.com
    - go.dev
    - pkg.go.dev
    - nodejs.org
    - www.npmjs.com
    - docs.npmjs.com
    - expressjs.com
    - hub.docker.com

tools:
  # gh-proxy avoids starting the Docker-based GitHub MCP server, which this sweep does not need.
  github:
    mode: gh-proxy
    toolsets: [context, repos]
  web-fetch:
  edit:
  bash: [cat, head, grep, find, wc, jq, ".github/capscan/verify-report.sh"]

# Every step that needs a shell or the network runs here, before the agent starts.
steps:
  - run: mkdir -p capscan-input

  # govulncheck reports standard library vulnerabilities for the Go version it runs with,
  # so it uses the version the project builds with.
  - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
    with:
      go-version-file: go-inventory/go.mod
      cache: false

  # govulncheck itself needs a recent Go to build, so it is installed with the latest toolchain
  # and then run under the project's toolchain, which decides the standard library version it checks.
  - name: "Go: reachable vulnerabilities"
    working-directory: go-inventory
    run: |
      latest=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)
      GOTOOLCHAIN=$latest go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
      # Exit code 3 means vulnerabilities were found, which is a result; anything else is a failure.
      rc=0
      "$(go env GOPATH)/bin/govulncheck" ./... > ../capscan-input/govulncheck.txt 2>&1 || rc=$?
      cat ../capscan-input/govulncheck.txt
      [ "$rc" -eq 0 ] || [ "$rc" -eq 3 ] || exit "$rc"

  - name: "Go: GODEBUG defaults at the current and latest go directive"
    working-directory: go-inventory
    run: |
      target=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)
      # Both lists must come from the same toolchain, since DefaultGODEBUG only reports
      # settings that differ from that toolchain's own defaults.
      export GOTOOLCHAIN=$target
      target=${target#go}
      mains() { go list -f '{{if eq .Name "main"}}{{.DefaultGODEBUG}}{{end}}' ./... | tr ',' '\n' | sort; }
      mains > "$RUNNER_TEMP/godebug-now.txt"
      cp go.mod "$RUNNER_TEMP/go.mod.orig"
      go mod edit -go="$target"
      mains > "$RUNNER_TEMP/godebug-target.txt"
      cp "$RUNNER_TEMP/go.mod.orig" go.mod
      {
        echo "go-inventory go directive: $(go mod edit -json | jq -r .Go) -> $target"
        diff "$RUNNER_TEMP/godebug-now.txt" "$RUNNER_TEMP/godebug-target.txt" || true
      } > ../capscan-input/godebug-diff.txt

  - name: "Go: direct dependencies with newer versions"
    working-directory: go-inventory
    run: |
      go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all \
        > ../capscan-input/go-updates.txt

  # These steps use the runner's default Node on purpose. Installing the project's Node 18 here
  # leaves it on PATH for gh-aw's own scripts, and Node 18 fails the MCP gateway health check
  # because it resolves localhost to ::1 without falling back to 127.0.0.1.
  - name: "Node: versions, outdated packages, and audit"
    working-directory: node-catalog
    run: |
      # --ignore-scripts installs packages without running any of their lifecycle scripts.
      npm ci --ignore-scripts
      latest_lts=$(curl -fsSL https://nodejs.org/dist/index.json | jq -r '[.[] | select(.lts)][0].version')
      {
        echo "node-catalog .nvmrc: $(cat .nvmrc)"
        echo "node-catalog engines.node: $(jq -r .engines.node package.json)"
        echo "node-catalog Dockerfile: $(grep -m1 '^FROM' Dockerfile)"
        echo "go-inventory Dockerfile: $(grep -m1 '^FROM' ../go-inventory/Dockerfile)"
        echo "latest Node LTS: $latest_lts"
      } > ../capscan-input/versions.txt
      npm outdated --json > ../capscan-input/npm-outdated.json || true
      npm audit --json > ../capscan-input/npm-audit.json || true

safe-outputs:
  create-issue:
    title-prefix: "capscan: "
    close-older-issues: true
    max: 1
  steps:
    # The safe-outputs job does not check out the repository, and the citation check needs it.
    - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      with:
        persist-credentials: false
    - name: Verify citations and links
      env:
        AGENT_OUTPUT: ${{ steps.setup-agent-output-env.outputs.GH_AW_AGENT_OUTPUT }}
      run: |
        if [ -z "$AGENT_OUTPUT" ]; then echo "No agent output to verify."; exit 0; fi
        jq -r '.items[] | select(.type == "create_issue") | .body' "$AGENT_OUTPUT" > "$RUNNER_TEMP/capscan-report.md"
        .github/capscan/verify-report.sh "$RUNNER_TEMP/capscan-report.md" \
          github.com docs.github.com go.dev pkg.go.dev nodejs.org www.npmjs.com docs.npmjs.com expressjs.com hub.docker.com
---

# capscan: weekly capability sweep

Follow the rules in `.github/capscan/RULES.md` exactly.
This is the weekly sweep, so the "Stay in scope" rule applies to the gap between each version this repository declares and the latest release.

The `capscan-input` directory holds facts gathered before you started:

- `govulncheck.txt`: reachable Go vulnerabilities
- `godebug-diff.txt`: Go defaults that change if the `go` directive is raised to the latest release
- `go-updates.txt`: Go direct dependencies with newer versions
- `versions.txt`: declared Node and Go versions, base images, and the latest Node LTS
- `npm-outdated.json` and `npm-audit.json`: outdated npm packages and advisories

Report reachable vulnerabilities first.
Then review capabilities gated by the `go` directive, the Node version (`.nvmrc`, `engines`, and the Dockerfile), and the outdated direct dependencies.

Before creating the issue, write the report to `/tmp/gh-aw/capscan-draft.md` and check it with
`.github/capscan/verify-report.sh /tmp/gh-aw/capscan-draft.md github.com docs.github.com go.dev pkg.go.dev nodejs.org www.npmjs.com docs.npmjs.com expressjs.com hub.docker.com`.
Fix every error it reports and run it again until it passes.
Paths are relative to the repository root, so write `go-inventory/go.mod:3`, never `go.mod:3`.
The same check runs again before the issue is created, and a report that fails it is not published.

Create one issue titled with today's date, such as `weekly sweep 2026-10-12`, with the verified report as its body.
