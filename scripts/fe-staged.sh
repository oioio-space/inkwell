#!/usr/bin/env bash
# Biome check (format + lint) of the staged frontend files, for the git pre-commit hook.
# A badly formatted file used to reach main and turn `fe:check` red for the next block (2026-11-12, native.test.ts).
set -euo pipefail
root="$(git rev-parse --show-toplevel)"
mapfile -d '' files < <(git -C "$root" diff --cached --name-only --diff-filter=ACMR -z -- \
  'frontend/*.ts' 'frontend/*.js' 'frontend/*.mjs' 'frontend/*.cjs' 'frontend/*.svelte' 'frontend/*.json' 'frontend/*.css')
[[ ${#files[@]} -eq 0 ]] && exit 0
rel=()
for f in "${files[@]}"; do rel+=("${f#frontend/}"); done
cd "$root/frontend"
exec ./node_modules/.bin/biome check --no-errors-on-unmatched --files-ignore-unknown=true "${rel[@]}"
