# capscan-demo

A working demonstration of **capscan**, a companion to Dependabot that reviews each dependency update for new capabilities a repository could adopt.
It accompanies the blog post [Your Dependencies Shipped Features Too](https://stevehenderson.github.io/Dependencies-Shipped-Features/) and its [Go appendix](https://stevehenderson.github.io/Dependencies-Shipped-Features/#go-appendix).

Dependabot answers what changed and whether it is safe.
capscan answers what the change unlocks for this codebase, grounded in specific files and lines.

## What is in this repository

| Path | Purpose |
|---|---|
| `go-inventory/` | A small Go API routed by gorilla/mux, with `go 1.21` in `go.mod` and a `golang:1.21-alpine` base image |
| `node-catalog/` | A small Express API that uses `dotenv`, `node-fetch`, `nodemon`, and `jest`, pinned to Node 18 |
| `.github/capscan/RULES.md` | The agent rules that define a capability review |
| `.github/capscan/gather-facts.sh` | Gathers the facts a review starts from: a dependency inventory with the lines that use each dependency, govulncheck, the GODEBUG diff, version declarations, and npm audit |
| `.github/capscan/verify-report.sh` | Checks every cited `path:line` and link before a report is posted |
| `.github/workflows/capscan.md` | A [GitHub Agentic Workflow](https://github.github.com/gh-aw/) that runs on each Dependabot pull request and comments with the report |
| `.github/workflows/capscan-sweep.md` | An agentic workflow that runs weekly and opens an issue covering version gates and drift that no Dependabot PR surfaces |
| `.github/workflows/*.lock.yml` | The GitHub Actions workflows compiled from the Markdown files by `gh aw compile` |
| `examples/claude-code-action/` | The same two workflows built directly on `anthropics/claude-code-action`, for comparison |
| `.claude/skills/capscan/SKILL.md` | The same review as an [Agent Skill](https://agentskills.io), run on a workstation by Claude Code, Copilot, or another agent that reads skills |
| `.claude/settings.json` | Pre-approves the skill's read-only commands and denies pushes, merges, comments, and package installs |
| `AGENTS.md` | Points interactive coding agents at the same rules |

## The drift is intentional

Both services are written the way many real services were written a few years ago, so capscan has something to find:

- `go-inventory` declares `go 1.21`, which keeps the 1.21 router behavior and older TLS defaults even when a newer Go toolchain builds it.
  Go 1.22 can replace gorilla/mux with the standard library router, and the `item := item` copy in `reindex.go` is no longer needed.
- `node-catalog` targets Node 18.
  Current Node releases cover what `dotenv`, `node-fetch`, `nodemon`, and `jest` do here with `--env-file`, built-in `fetch`, `--watch`, and `node:test`.
  `npm audit` reports high-severity advisories that reach this project only through `nodemon` and `jest`, so adopting the built-ins would also remove them.

## How the workflows are built

The workflows use GitHub Agentic Workflows with the Copilot engine.
The framework enforces most of the capscan design:

- The agent job is read-only, and its network egress goes through a firewall that allows only the hosts listed under `network:`.
- The agent's only write path is a declared safe output: one `add-comment` on the triggering pull request, or one `create-issue` for the sweep.
  A separate job applies it after a threat-detection pass.
- The shell available to the agent is limited to the commands listed under `tools.bash`.
- Fixed steps gather facts before the agent starts and pass them to the agent as files: Dependabot metadata, and the output of `gather-facts.sh`.
- Every report must pass `verify-report.sh` in the safe-outputs job before it is published.
  The script confirms that cited lines exist and that links use allowed hosts; it does not confirm that the claims in the report are correct, so a person still reviews each finding.
- Actions and containers in the compiled lock files are pinned to commit SHAs and digests.

After editing a workflow's Markdown, recompile with `gh aw compile` and commit both files.

## Running it out of band with the skill

The skill runs the same review without GitHub Actions.
From a clone, start an agent that reads skills and ask for a review:

```console
claude "/capscan review the open Dependabot pull requests"
claude "/capscan sweep"
```

It runs `gather-facts.sh`, reads the rules, researches each candidate, and writes verified reports to `capscan-reports/`, which git ignores.
It does not post, commit, or push anything.

## Running it in another repository

1. Install the CLI with `gh extension install github/gh-aw`.
2. Copy `.github/capscan/`, the two capscan Markdown workflows, `.claude/`, and the `AGENTS.md` pointer.
3. Adjust `network.allowed` and the host list passed to `verify-report.sh` for the ecosystems in use, and extend `gather-facts.sh` for any ecosystem beyond Go and npm.
4. Run `gh aw compile` and commit the Markdown and lock files.
5. Add a token for the engine.
   On a personal repository, the Copilot engine needs a fine-grained personal access token with the Copilot Requests permission, stored as `COPILOT_GITHUB_TOKEN`.
   Organizations with Copilot billing can use the built-in token with `copilot-requests: write` instead.
   Workflows triggered by Dependabot can read only Dependabot secrets, so the token goes in both stores:

   ```console
   gh secret set COPILOT_GITHUB_TOKEN --app dependabot
   gh secret set COPILOT_GITHUB_TOKEN --app actions
   ```

To use Claude, Codex, or Gemini, change `engine:` and set that engine's secret.

## Local development

```console
cd go-inventory && go test -race ./...
cd node-catalog && npm ci --ignore-scripts && npm test
```

## License

MIT
