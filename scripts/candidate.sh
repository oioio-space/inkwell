#!/usr/bin/env bash
# Push one commit as N candidate branches and dispatch one GitHub CI run on each (2026-11-24).
# Usage: scripts/candidate.sh <name> [commit] [runs]   (commit: HEAD; runs: 1)
# Branches are candidate/<name> for one run, candidate/<name>-1 .. -N for several. .github/workflows/ci.yml has
# `concurrency: group: <workflow>-<ref>, cancel-in-progress: true`: several runs dispatched on ONE branch cancel each
# other (four of five were cancelled on candidate/sb2b1g-1), so each run needs its own branch. All refs go in one
# `git push`, so the pre-push CI runs once. Prints one « run: <id> » line per dispatched run (the format of the
# gates' GITHUB.md files) on stdout; progress goes to stderr.
set -euo pipefail
name=${1:-}
commit=${2:-HEAD}
runs=${3:-1}
[[ $# -le 3 && $name =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ && $runs =~ ^[1-9][0-9]?$ ]] ||
  { echo "usage: $0 <name> [commit] [runs 1-99]" >&2; exit 2; }
sha=$(git rev-parse --verify --quiet "$commit^{commit}") || { echo "candidate: unknown commit $commit" >&2; exit 1; }
workflow=.github/workflows/ci.yml
# shellcheck disable=SC2016 # the literal GitHub expression is what is searched for
grep -q 'cancel-in-progress: true' "$workflow" && grep -qF 'group: ${{ github.workflow }}-${{ github.ref }}' "$workflow" ||
  echo "candidate: $workflow concurrency changed; one branch per run is kept anyway" >&2

branches=()
if [[ $runs -eq 1 ]]; then branches=("candidate/$name"); else
  for i in $(seq 1 "$runs"); do branches+=("candidate/$name-$i"); done
fi
refspecs=()
for b in "${branches[@]}"; do
  ! git ls-remote --exit-code --heads origin "$b" >/dev/null 2>&1 || { echo "candidate: $b already exists on origin" >&2; exit 1; }
  refspecs+=("$sha:refs/heads/$b")
done
echo "candidate: pushing ${#branches[@]} branch(es) at ${sha:0:12} (one pre-push CI)" >&2
"${CANDIDATE_PUSH:-scripts/qpush.sh}" origin "${refspecs[@]}" >&2

for b in "${branches[@]}"; do
  before=$(gh run list --workflow ci.yml --branch "$b" --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId // 0')
  gh workflow run ci.yml --ref "$b" >&2
  id=0
  for _ in $(seq 1 30); do
    id=$(gh run list --workflow ci.yml --branch "$b" --event workflow_dispatch --limit 1 --json databaseId --jq '.[0].databaseId // 0')
    [[ $id != "$before" && $id != 0 ]] && break
    sleep 2
  done
  [[ $id != "$before" && $id != 0 ]] || { echo "candidate: no run appeared for $b" >&2; exit 1; }
  echo "run: $id"
done
