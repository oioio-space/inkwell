#!/usr/bin/env bash
# P-TLA (ADR 0034 step 3, docs/protocol/tla/README.md): fetch the pinned TLA+ tools
# (tla2tools.jar SHA-256 checked, never committed) and model-check the models
# (RepoGen, ConnCredit, SyncIndicator, WriteBudget, SendPath).
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

command -v java >/dev/null 2>&1 || die "java (11+) is required"
command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"

mkdir -p "$cache" "$logs"

if [[ ! -f "$jar" ]] || [[ "$(sha256sum "$jar" | cut -d' ' -f1)" != "$jar_sha" ]]; then
  rm -f "$jar"
  vsix="$cache/tlaplus-vscode-ide-${vsix_version}.vsix"
  # fetch_pinned reuses a valid cached copy, else downloads, verifies and renames atomically.
  python3 -I "$root/scripts/fetch_pinned.py" "$vsix_url" "$vsix_sha" "$vsix"
  tmp="$(mktemp "$cache/tla2tools.jar.XXXXXX")"
  if command -v unzip >/dev/null 2>&1; then
    unzip -p "$vsix" extension/tools/tla2tools.jar >"$tmp"
  elif command -v python3 >/dev/null 2>&1; then
    python3 -I -c 'import sys, zipfile; sys.stdout.buffer.write(zipfile.ZipFile(sys.argv[1]).read(sys.argv[2]))' "$vsix" extension/tools/tla2tools.jar >"$tmp"
  else
    die "unzip or python3 is required to extract the pinned TLA+ tools"
  fi
  got="$(sha256sum "$tmp" | cut -d' ' -f1)"
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
run SyncIndicator SyncIndicator.cfg SyncIndicator sound
run SyncIndicatorMesh SyncIndicatorMesh.cfg SyncIndicator sound
run WriteBudget WriteBudget.cfg WriteBudget sound
run WriteBudgetAny WriteBudgetAny.cfg WriteBudget sound
# SendPath (README "SendPath", design W38) and block SP1 (README "SP1"): one module, two rule sets.
for c in SendPathHonest SendPathHop SendPathAny SP1Relay SP1Floor SP1RelayKeepalive SP1FloorKeepalive \
         SP1Grown90 SP1Hop90 SP1Blackout SP1Any; do
  run "$c" "$c.cfg" SendPath sound
done

# Mutants (README "Contre-épreuve" and the ConnCredit table): each must be detected.
run RepoGenLossNoBump mutants/RepoGenLossNoBump.cfg mutants/RepoGenLossNoBump mutant
run RepoGenDeliverNoCheck mutants/RepoGenDeliverNoCheck.cfg mutants/RepoGenDeliverNoCheck mutant
run RepoGenNoRuleB mutants/RepoGenNoRuleB.cfg mutants/RepoGenNoRuleB mutant
run ConnCreditNoResetNoTeardown mutants/ConnCreditNoResetNoTeardown.cfg ConnCredit mutant
run ConnCreditNoPullLarge mutants/ConnCreditNoPullLarge.cfg ConnCredit mutant
# SyncIndicator mutants (README "SyncIndicator"): definition overrides from SyncIndicatorMutants.
for m in PeerCount IgnoreRemote SentIsAcked QueueOnly EveryVacuous AnyPeer NoRuleL; do
  run "SyncIndicator$m" "mutants/SyncIndicator$m.cfg" mutants/SyncIndicatorMutants mutant
done
# WriteBudget mutants (README "WriteBudget", block WB): definition overrides from WriteBudgetMutants.
for m in FixedFloor IdleOnly TotalOnly NoCut; do
  run "WriteBudget$m" "mutants/WriteBudget$m.cfg" mutants/WriteBudgetMutants mutant
done
# SendPath mutants and SP1 mutants (README "SendPath", "SP1"); SP1TypingResidual and SP1FloorKeepaliveBigHop
# are residual witnesses: they must violate, so that the README never claims a bound SP1 does not give.
for m in SendPathWriteReturn SendPathFixedFloor SendPathNoCap SendPathCapAboveWindow \
         SendPathSharedWriter SendPathFromZero SendPathStackedWindows SendPathMuxWriteTimeout \
         SendPathKeepaliveBehindData SendPathSilentAbandon \
         SendPathNoCut SendPathNoCutLive SendPathZeroOnly SendPathTrustCredit \
         SP1Idle20 SP1FixedFloor SP1FloorGrown SP1HopValve10 SP1KeepaliveBehindData SP1PartialTTL30 \
         SP1NoTotal SP1NoCut SP1FloorKeepaliveBigHop SP1GrownKeepalive \
         SP1TypingResidual; do
  run "$m" "mutants/$m.cfg" mutants/SendPathMutants mutant
done

exit "$fail"
