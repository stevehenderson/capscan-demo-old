#!/usr/bin/env bash
# Gathers the facts a capscan review starts from and writes them to an output directory.
# The weekly sweep runs it before the agent starts, and the capscan skill runs it on a workstation,
# so both start from the same inputs.
#
# Usage: gather-facts.sh [output-dir]   (default: capscan-input)
#
# Nothing here executes dependency code: Go commands read module metadata, govulncheck analyzes
# source without running it, and npm commands read the lockfile and the registry.
# The working tree is never modified; anything that needs an edited go.mod works on a copy.
set -euo pipefail

out=${1:-capscan-input}
mkdir -p "$out"
out=$(cd "$out" && pwd)
root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$root"
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

latest_go=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -n1)
latest_node_lts=$(curl -fsSL https://nodejs.org/dist/index.json | jq -r '[.[] | select(.lts)][0].version')

go_modules=$(git ls-files '*go.mod' | xargs -r -n1 dirname)
npm_projects=$(git ls-files '*package.json' | grep -v node_modules | xargs -r -n1 dirname)

: > "$out/dependencies.md"
: > "$out/versions.txt"
: > "$out/godebug-diff.txt"
: > "$out/go-updates.txt"
: > "$out/govulncheck.txt"

{
  echo "latest Go: $latest_go"
  echo "latest Node LTS: $latest_node_lts"
} >> "$out/versions.txt"

# Prints each `path:line` in a directory that mentions a pattern, skipping vendored code.
usages() {
  local dir=$1 pattern=$2
  grep -rnE --exclude-dir=node_modules --exclude-dir=vendor --exclude=package-lock.json --exclude=go.sum \
    "$pattern" "$dir" 2>/dev/null | cut -d: -f1,2 | sed 's#^\./##' | head -n 8 | sed 's/^/  - `/; s/$/`/'
}

for dir in $go_modules; do
  echo "== Go module $dir"
  directive=$(cd "$dir" && go mod edit -json | jq -r .Go)
  {
    echo "$dir go directive: $directive"
    [ -f "$dir/Dockerfile" ] && echo "$dir Dockerfile: $(grep -m1 '^FROM' "$dir/Dockerfile")"
  } >> "$out/versions.txt"

  # Go commands below can rewrite go.mod and go.sum, so they run on a copy of the module.
  copy="$scratch/$(echo "$dir" | tr / _)"
  cp -R "$dir" "$copy"

  # Direct dependencies, with the lines that import them.
  {
    echo "## Go module \`$dir\` (go $directive)"
    echo
    (cd "$copy" && GOTOOLCHAIN=$latest_go go list -m -f '{{if not (or .Main .Indirect)}}{{.Path}} {{.Version}}{{end}}' all) |
      while read -r path version; do
        [ -n "$path" ] || continue
        echo "- \`$path\` $version, used at:"
        usages "$dir" "\"$path(/[^\"]*)?\"" || true
      done
    echo
  } >> "$out/dependencies.md"

  # Direct dependencies with newer versions.
  (cd "$copy" && GOTOOLCHAIN=$latest_go go list -m -u -f '{{if and (not .Indirect) .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}' all) |
    sed "s#^#$dir: #" >> "$out/go-updates.txt"

  # GODEBUG defaults at the current and latest go directive. Both lists must come from the same
  # toolchain, since DefaultGODEBUG only reports settings that differ from that toolchain's defaults.
  mains() { (cd "$copy" && GOTOOLCHAIN=$latest_go go list -f '{{if eq .Name "main"}}{{.DefaultGODEBUG}}{{end}}' ./... | tr ',' '\n' | sort); }
  mains > "$scratch/now.txt"
  (cd "$copy" && GOTOOLCHAIN=$latest_go go mod edit -go="${latest_go#go}")
  mains > "$scratch/target.txt"
  {
    echo "$dir go directive: $directive -> ${latest_go#go}"
    diff "$scratch/now.txt" "$scratch/target.txt" || true
    echo
  } >> "$out/godebug-diff.txt"

  # govulncheck needs a recent Go to build, so it is built with the latest toolchain into a scratch
  # directory, then run under the project's toolchain, which decides the standard library it checks.
  if [ ! -x "$scratch/bin/govulncheck" ]; then
    GOTOOLCHAIN=$latest_go GOBIN="$scratch/bin" go install golang.org/x/vuln/cmd/govulncheck@v1.8.0
  fi
  # Exit code 3 means vulnerabilities were found, which is a result; anything else is a failure.
  # A directive like 1.21 names a language version, so use the newest patch release of it.
  project_go=go$directive
  case $directive in
    *.*.*) ;;
    *) project_go=$(curl -fsSL 'https://go.dev/dl/?mode=json&include=all' |
         jq -r --arg v "go$directive" '[.[].version | select(startswith($v + "."))][0]') ;;
  esac
  echo "== $dir, scanned with $project_go" >> "$out/govulncheck.txt"
  rc=0
  (cd "$dir" && GOTOOLCHAIN=$project_go "$scratch/bin/govulncheck" ./...) >> "$out/govulncheck.txt" 2>&1 || rc=$?
  [ "$rc" -eq 0 ] || [ "$rc" -eq 3 ] || { cat "$out/govulncheck.txt"; exit "$rc"; }
done

for dir in $npm_projects; do
  echo "== npm project $dir"
  {
    echo "$dir .nvmrc: $(cat "$dir/.nvmrc" 2>/dev/null || echo none)"
    echo "$dir engines.node: $(jq -r '.engines.node // "none"' "$dir/package.json")"
    [ -f "$dir/Dockerfile" ] && echo "$dir Dockerfile: $(grep -m1 '^FROM' "$dir/Dockerfile")"
  } >> "$out/versions.txt"

  # Direct dependencies, with the lines that load them or run them from package.json scripts.
  {
    echo "## npm project \`$dir\`"
    echo
    jq -r '(.dependencies // {}), (.devDependencies // {}) | to_entries[] | "\(.key) \(.value)"' "$dir/package.json" |
      while read -r name range; do
        echo "- \`$name\` $range, used at:"
        usages "$dir" "(require\\(|from |import\\()['\"]$name(/[^'\"]*)?['\"]|\"[^\"]*\\b$name\\b[^\"]*\"[,]?$" || true
      done
    echo
  } >> "$out/dependencies.md"

  name=$(echo "$dir" | tr / _)
  (cd "$dir" && npm outdated --json) > "$out/npm-outdated-$name.json" 2>/dev/null || true
  (cd "$dir" && npm audit --json) > "$out/npm-audit-$name.json" 2>/dev/null || true
done

echo "Facts written to $out:"
ls -1 "$out"
