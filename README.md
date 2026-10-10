# capscan-demo

A working demonstration of **capscan**, a companion to Dependabot that reviews each dependency update for new capabilities a repository could adopt.
It accompanies the blog post [Your Dependencies Shipped Features Too](https://stevehenderson.github.io/Dependencies-Shipped-Features/) and its [Go appendix](https://stevehenderson.github.io/Go-Routing-Appendix/).

Dependabot answers what changed and whether it is safe.
capscan answers what the change unlocks for this codebase, grounded in specific files and lines.

## What is in this repository

| Path | Purpose |
|---|---|
| `go-inventory/` | A small Go API routed by gorilla/mux, with `go 1.21` in `go.mod` and a `golang:1.21-alpine` base image |
| `node-catalog/` | A small Express API that uses `dotenv`, `node-fetch`, `nodemon`, and `jest`, pinned to Node 18 |
| `.github/capscan/RULES.md` | The agent rules that define a capability review |
| `.github/capscan/verify-report.sh` | Checks every cited `path:line` and link before a report is posted |
| `.github/workflows/capscan.yml` | Runs on each Dependabot pull request and comments with the report |
| `.github/workflows/capscan-sweep.yml` | Runs weekly and opens an issue covering version gates and drift that no Dependabot PR surfaces |
| `AGENTS.md` | Points interactive coding agents at the same rules |

## The drift is intentional

Both services are written the way many real services were written a few years ago, so capscan has something to find:

- `go-inventory` declares `go 1.21`, which keeps the 1.21 router behavior and older TLS defaults even when a newer Go toolchain builds it.
  Go 1.22 can replace gorilla/mux with the standard library router, and the `item := item` copy in `reindex.go` is no longer needed.
- `node-catalog` targets Node 18.
  Current Node releases cover what `dotenv`, `node-fetch`, `nodemon`, and `jest` do here with `--env-file`, built-in `fetch`, `--watch`, and `node:test`.
  `npm audit` reports high-severity advisories that reach this project only through `nodemon` and `jest`, so adopting the built-ins would also remove them.

## How the workflows are built

- The agent job has a read-only token and no shell.
  It can read the checkout, query GitHub releases, and fetch documentation from an allowlist of hosts.
- Only the comment and issue jobs can write, and they never run the agent.
- The sweep gathers facts first (govulncheck, a GODEBUG diff, `npm outdated`, `npm audit`, Node and Go versions) and passes them to the agent as files.
- Every report must pass `verify-report.sh` before it is posted.
  The script confirms that cited lines exist and that links use allowed hosts; it does not confirm that the claims in the report are correct, so a person still reviews each finding.
- All actions are pinned to commit SHAs, and Dependabot keeps the pins current.

## Running it in another repository

1. Copy `.github/capscan/`, the two capscan workflows, and the `AGENTS.md` pointer.
2. Adjust the WebFetch domains and `ALLOWED_HOSTS` for the ecosystems in use.
3. Add an Anthropic API key as both a Dependabot secret and an Actions secret.
   Workflows triggered by Dependabot can read only Dependabot secrets.
   I recommend a dedicated key in its own workspace with a spend limit.

   ```console
   gh secret set ANTHROPIC_API_KEY --app dependabot
   gh secret set ANTHROPIC_API_KEY --app actions
   ```

4. Replace the Go and Node fact-gathering steps in the sweep with the equivalents for the stack in use.

## Local development

```console
cd go-inventory && go test -race ./...
cd node-catalog && npm ci --ignore-scripts && npm test
```

## License

MIT
