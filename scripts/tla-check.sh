#!/usr/bin/env bash
# P-TLA (ADR 0034 step 3, docs/protocol/tla/README.md): fetch the pinned TLA+ tools
# (tla2tools.jar SHA-256 checked, never committed) and model-check the two models.
#
# Sound models must finish with no error; every mutant listed in the README must be
# detected (a violation). The task fails if a sound model violates a property OR if a
# mutant passes. TLC runs through scripts/heavy.sh with a bounded heap.
#
#   mise run tla
#   INKWELL_TLA_XMX=1g INKWELL_TLA_WORKERS=2 mise run tla
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="$root/docs/protocol/tla"
cache="${INKWELL_TLA_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/inkwell/tla}"
jar="$cache/tla2tools.jar"
logs="$cache/logs"
heavy="$root/scripts/heavy.sh"

# docs/protocol/tla/README.md, "Résultats".
jar_sha="011267fca25d0752cf18894dd585cd28b96960c112ea8a20b823ea770e7e10cd"
# Immutable distribution of that exact build: TLA+ VS Code extension 2026.10.301552,
# whose extension/tools/tla2tools.jar is the jar above (upstream's v1.8.0 release asset
# is rebuilt on every master commit, so its URL is not reproducible).
vsix_version="2026.10.301552"
vsix_sha="89adb8d05d3136f74a4a33c97ae35b04730cdb3fb551b81d4299ba5b56c3c5a8"
vsix_url="https://open-vsx.org/api/tlaplus/vscode-ide/${vsix_version}/file/tlaplus.vscode-ide-${vsix_version}.vsix"

xmx="${INKWELL_TLA_XMX:-2g}"
workers="${INKWELL_TLA_WORKERS:-2}"

die() { printf 'tla: %s\n' "$*" >&2; exit 1; }

sha256() { sha256sum "$1" | cut -d' ' -f1; }

fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL --retry 3 --max-time 180 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$2" "$1"
  else
    die "curl or wget is required to fetch the pinned TLA+ tools"
  fi
}

extract_jar() {
  if command -v unzip >/dev/null 2>&1; then
    unzip -p "$1" extension/tools/tla2tools.jar
  elif command -v python3 >/dev/null 2>&1; then
    python3 -I -c 'import sys, zipfile; sys.stdout.buffer.write(zipfile.ZipFile(sys.argv[1]).read(sys.argv[2]))' "$1" extension/tools/tla2tools.jar
  else
    die "unzip or python3 is required to extract the pinned TLA+ tools"
  fi
}

command -v java >/dev/null 2>&1 || die "java (11+) is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"

mkdir -p "$cache" "$logs"

if [[ ! -f "$jar" ]] || [[ "$(sha256 "$jar")" != "$jar_sha" ]]; then
  rm -f "$jar"
  vsix="$cache/tlaplus-vscode-ide-${vsix_version}.vsix"
  if [[ ! -f "$vsix" ]] || [[ "$(sha256 "$vsix")" != "$vsix_sha" ]]; then
    rm -f "$vsix"
    printf 'tla: fetching pinned TLA+ tools (%s)…\n' "$vsix_version" >&2
    fetch "$vsix_url" "$vsix.part"
    got="$(sha256 "$vsix.part")"
    if [[ "$got" != "$vsix_sha" ]]; then
      rm -f "$vsix.part"
      die "refusing $vsix_url: sha256 $got, want $vsix_sha"
    fi
    mv "$vsix.part" "$vsix"
  fi
  tmp="$(mktemp "$cache/tla2tools.jar.XXXXXX")"
  extract_jar "$vsix" >"$tmp"
  got="$(sha256 "$tmp")"
  if [[ "$got" != "$jar_sha" ]]; then
    rm -f "$tmp"
    die "refusing extracted tla2tools.jar: sha256 $got, want $jar_sha"
  fi
  mv "$tmp" "$jar"
  printf 'tla: using %s\n' "$jar" >&2
fi

fail=0

# run NAME CONFIG MODULE sound|mutant
run() {
  local name="$1" cfg="$2" module="$3" expect="$4"
  local out="$logs/$name.log" metadir status
  metadir="$(mktemp -d)"
  set +e
  (
    cd "$dir"
    "$heavy" java -Xmx"$xmx" -XX:+UseParallelGC -cp "$jar" tlc2.TLC \
      -workers "$workers" -noGenerateSpecTE -cleanup -metadir "$metadir" \
      -config "$cfg" "$module"
  ) >"$out" 2>&1
  status=$?
  set -e
  rm -rf "$metadir"

  if [[ "$expect" == sound ]]; then
    if ((status != 0)) || ! grep -q 'No error has been found' "$out" || ! grep -q 'complete state space' "$out"; then
      printf '✗ %s: sound model failed (exit %d)\n' "$name" "$status" >&2
      tail -n 20 "$out" >&2
      fail=1
    else
      printf '✓ %s: no error (exhaustive, liveness checked)\n' "$name"
    fi
  else
    if ((status == 12 || status == 13)) && grep -Eq 'Error: (Invariant|Temporal property) .* violated' "$out"; then
      printf '✓ %s: violation detected\n' "$name"
    else
      printf '✗ %s: mutant passed (exit %d) — the model no longer catches it\n' "$name" "$status" >&2
      tail -n 20 "$out" >&2
      fail=1
    fi
  fi
}

# Sound models (README tables): exhaustive, liveness included.
run RepoGenSmall RepoGenSmall.cfg RepoGen sound
run ConnCredit ConnCredit.cfg ConnCredit sound
run ConnCreditHonest ConnCreditHonest.cfg ConnCredit sound
run ConnCreditDishonest ConnCreditDishonest.cfg ConnCredit sound

# Mutants (README "Contre-épreuve" and the ConnCredit table): each must be detected.
run RepoGenLossNoBump mutants/RepoGenLossNoBump.cfg mutants/RepoGenLossNoBump mutant
run RepoGenDeliverNoCheck mutants/RepoGenDeliverNoCheck.cfg mutants/RepoGenDeliverNoCheck mutant
run RepoGenNoRuleB mutants/RepoGenNoRuleB.cfg mutants/RepoGenNoRuleB mutant
run ConnCreditNoResetNoTeardown mutants/ConnCreditNoResetNoTeardown.cfg ConnCredit mutant
run ConnCreditNoPullLarge mutants/ConnCreditNoPullLarge.cfg ConnCredit mutant

exit "$fail"
