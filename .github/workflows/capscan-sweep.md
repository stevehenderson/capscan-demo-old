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
  # Any recent Go works here; gather-facts.sh picks the toolchain each command needs.
  - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
    with:
      go-version: stable
      cache: false

  # This uses the runner's default Node on purpose. Installing the project's Node 18 here
  # leaves it on PATH for gh-aw's own scripts, and Node 18 fails the MCP gateway health check
  # because it resolves localhost to ::1 without falling back to 127.0.0.1.
  # --ignore-scripts installs packages without running any of their lifecycle scripts.
  - name: Install npm packages without running their scripts
    working-directory: node-catalog
    run: npm ci --ignore-scripts

  # The same script runs on a workstation through the capscan skill, so both start from the same facts.
  - name: Gather facts
    run: .github/capscan/gather-facts.sh capscan-input

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

The `capscan-input` directory holds facts that `.github/capscan/gather-facts.sh` gathered before you started:

- `dependencies.md`: every direct dependency, with the lines that use it
- `govulncheck.txt`: reachable Go vulnerabilities
- `godebug-diff.txt`: Go defaults that change if the `go` directive is raised to the latest release
- `go-updates.txt`: Go direct dependencies with newer versions
- `versions.txt`: declared Node and Go versions, base images, and the latest Go and Node LTS releases
- `npm-outdated-*.json` and `npm-audit-*.json`: outdated npm packages and advisories

Report reachable vulnerabilities first.
Then work through every entry in `dependencies.md` as the "Check every direct dependency" rule describes,
and review the capabilities gated by the `go` directive and the Node version (`.nvmrc`, `engines`, and the Dockerfile).

Before creating the issue, write the report to `/tmp/gh-aw/capscan-draft.md` and check it with
`.github/capscan/verify-report.sh /tmp/gh-aw/capscan-draft.md github.com docs.github.com go.dev pkg.go.dev nodejs.org www.npmjs.com docs.npmjs.com expressjs.com hub.docker.com`.
Fix every error it reports and run it again until it passes.
Paths are relative to the repository root, so write `go-inventory/go.mod:3`, never `go.mod:3`.
The same check runs again before the issue is created, and a report that fails it is not published.

Create one issue titled with today's date, such as `weekly sweep 2026-10-12`, with the verified report as its body.
