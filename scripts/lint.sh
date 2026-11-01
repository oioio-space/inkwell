#!/usr/bin/env bash
# Deterministic Go style-guide gate.
# Runs gofmt, go vet, golangci-lint, build and tests. Used by the git pre-commit
# hook (.githooks/pre-commit) and runnable by hand: `./scripts/lint.sh`.
#
# By default it lints the whole module. Pass --staged to limit gofmt to the files
# staged for commit (used by the pre-commit hook).
set -euo pipefail


for tool in git go; do
 command -v "$tool" >/dev/null 2>&1 || { printf 'Required tool missing: %s\n' "$tool" >&2; exit 1; }
done
repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

# Make Go-installed tools (golangci-lint, gofumpt) reachable (fallback when not
# running under mise, which already puts the pinned tools on PATH).
gopath_bin="$(go env GOPATH)/bin"
export PATH="$PATH:$gopath_bin"
for tool in git go find gofmt; do
 command -v "$tool" >/dev/null 2>&1 || { printf 'Required tool missing: %s\n' "$tool" >&2; exit 1; }
done

staged_only=0
[[ "${1:-}" == "--staged" ]] && staged_only=1

fail() { printf '\n\033[31m✗ %s\033[0m\n' "$1" >&2; exit 1; }
ok()   { printf '\033[32m✓ %s\033[0m\n' "$1"; }

# Collect Go files (staged set for the hook, else all tracked .go files).
if [[ $staged_only -eq 1 ]]; then
  mapfile -t go_files < <(git diff --cached --name-only --diff-filter=ACM -- '*.go')
else
  files="$(find . -type d \( -name .git -o -path './frontend/bindings' -o -path './frontend/node_modules' \) -prune -o -type f -name '*.go' -print)"
  go_files=()
  [[ -z "$files" ]] || mapfile -t go_files <<< "$files"
fi

if [[ ${#go_files[@]} -eq 0 ]]; then
  ok "No Go files to check."
  exit 0
fi

command -v golangci-lint >/dev/null 2>&1 || { printf 'Required tool missing: golangci-lint\n' >&2; exit 1; }

# 1. Formatting — guide: Formatting (gofmt).
unformatted="$(gofmt -l "${go_files[@]}" || true)"
if [[ -n "$unformatted" ]]; then
  printf 'These files are not gofmt-clean:\n%s\n' "$unformatted" >&2
  fail "Formatting (run: gofmt -w . or gofumpt -w .)"
fi
ok "gofmt"

# 2. go vet — best-practices: correctness.
(cd protocol && go vet ./...) || fail "protocol go vet found problems"
go vet ./... || fail "go vet found problems"
ok "go vet"

# 3. golangci-lint — the itemized style-guide linters (.golangci.yml).
(cd protocol && golangci-lint run ./...) || fail "protocol style-guide violations"
golangci-lint run ./... || fail "golangci-lint found style-guide violations"
ok "golangci-lint"

# 4. Build — code must compile.
# -pgo=off: this build-check runs concurrently (under `mise run ci`) with the
# coverage test build. If a package ships a default.pgo profile, -pgo=auto would
# recompile its dependencies WITH PGO here while the test build compiles them
# WITHOUT — two fingerprints race in the shared GOCACHE and linking fails
# (golang/go#61376). A compile-check needs no PGO; a release build can still opt
# into it via goreleaser.
go build -pgo=off ./... || fail "go build failed"
ok "go build"

# NOTE: tests are intentionally NOT run here. The dedicated `test:ci` task is the
# authoritative test + coverage gate (and `mise run ci` runs both lint and
# test:ci in parallel — running tests here too just duplicated the whole suite on
# every CI run and every commit). Run tests via `mise run test` / `mise run ci`.

printf '\n\033[32mAll style-guide checks passed.\033[0m\n'
