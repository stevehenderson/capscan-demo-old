---
description: Review each Dependabot pull request for new capabilities this repository could adopt.
on:
  pull_request:
    types: [opened, reopened, synchronize]
  bots: ["dependabot[bot]"]
if: github.event.pull_request.user.login == 'dependabot[bot]'

permissions:
  contents: read
  pull-requests: read

engine: copilot
max-turns: 30
timeout-minutes: 15

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
  github:
    mode: gh-proxy
    toolsets: [context, repos, pull_requests]
  web-fetch:
  edit:
  bash: [cat, head, grep, find, wc, jq, "gh release view", "gh release list", "gh pr view", "gh pr diff", ".github/capscan/verify-report.sh"]

steps:
  - name: Read Dependabot metadata
    id: meta
    uses: dependabot/fetch-metadata@25dd0e34f4fe68f24cc83900b1fe3fe149efef98 # v3.1.0
  - name: Hand the metadata to the agent as a file
    env:
      UPDATES: ${{ steps.meta.outputs.updated-dependencies-json }}
      UPDATE_TYPE: ${{ steps.meta.outputs.update-type }}
    run: |
      mkdir -p capscan-input
      jq -n --arg type "$UPDATE_TYPE" --argjson updates "$UPDATES" \
        '{update_type: $type, updates: $updates}' > capscan-input/dependabot.json
  - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
    with:
      go-version: stable
      cache: false
  # The runner's default Node is used on purpose; see the note in capscan-sweep.md.
  - name: Install npm packages without running their scripts
    working-directory: node-catalog
    run: npm ci --ignore-scripts
  - name: Gather facts
    run: .github/capscan/gather-facts.sh capscan-input

safe-outputs:
  add-comment:
    max: 1
    hide-older-comments: true
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
        jq -r '.items[] | select(.type == "add_comment") | .body' "$AGENT_OUTPUT" > "$RUNNER_TEMP/capscan-report.md"
        .github/capscan/verify-report.sh "$RUNNER_TEMP/capscan-report.md" \
          github.com docs.github.com go.dev pkg.go.dev nodejs.org www.npmjs.com docs.npmjs.com expressjs.com hub.docker.com
---

# capscan: capability review for a Dependabot pull request

Review pull request #${{ github.event.pull_request.number }} in ${{ github.repository }}.
Dependabot opened it, and `capscan-input/dependabot.json` lists the updated dependencies with their previous and new versions.
The rest of `capscan-input` holds facts gathered before you started, including `dependencies.md`,
which lists every direct dependency with the lines that use it, and `govulncheck.txt`.

Follow the rules in `.github/capscan/RULES.md` exactly.

- If `update_type` in `capscan-input/dependabot.json` is `version-update:semver-patch`, call `noop`.
  Patch releases rarely add capabilities.
- Read release notes with `gh release view` or `web-fetch`, and read this repository's code with `cat`, `grep`, and `find`.
- Apply the "Check every direct dependency" rule to the dependencies that this update's new version could replace.
  When the update moves a language or runtime, such as a Go or Node base image, that means every entry in `dependencies.md`
  for the affected service.
- Before posting, write the report to `/tmp/gh-aw/capscan-draft.md` and check it with
  `.github/capscan/verify-report.sh /tmp/gh-aw/capscan-draft.md github.com docs.github.com go.dev pkg.go.dev nodejs.org www.npmjs.com docs.npmjs.com expressjs.com hub.docker.com`.
  Fix every error it reports and run it again until it passes.
  Paths are relative to the repository root, so write `go-inventory/go.mod:3`, never `go.mod:3`.
- Post the verified report with a single `add_comment`.
  The same check runs again before the comment is published, and a report that fails it is not posted.
