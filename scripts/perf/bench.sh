#!/usr/bin/env bash
# Fresh repeated HOT_PATHS measurements; never substitute an old run.
set -euo pipefail
out=docs/perf/baseline/2026-11-02
mkdir -p "$out"
read -ra packages <<< "$(sed -n 's/^HOT_PATHS: //p' CLAUDE.md)"
args=()
for package in "${packages[@]}"; do args+=("./$package"); done
go test -run '^$' -bench . -benchmem -count 10 "${args[@]}" > "$out/go-bench.txt" 2>&1
benchstat "$out/go-bench.txt" > "$out/benchstat.txt" 2>&1
