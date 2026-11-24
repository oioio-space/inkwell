#!/usr/bin/env bash
# P-TLA (ADR 0034 step 3, docs/protocol/tla/README.md): fetch the pinned TLA+ tools
# (tla2tools.jar SHA-256 checked, never committed) and model-check the models
# (RepoGen, ConnCredit, SyncIndicator, WriteBudget, SendPath, LinkProbe, OISingleInstance, SandboxScope,
# ProjectChange, DirectUpgrade, PeerUpAck, Supersede, CloseScope, SyncOrder, InboundLifecycle).
#
# Sound models must finish with no error; every mutant listed in the README must be
# detected (a violation). The task fails if a sound model violates a property OR if a
# mutant passes. TLC runs through scripts/heavy.sh with a bounded heap.
#
#   mise run tla
#   TLA_SHARD=1/3 mise run tla   # run the same shard as GitHub CI
#   INKWELL_TLA_XMX=1g INKWELL_TLA_WORKERS=2 mise run tla
#   INKWELL_TLA_ONLY='^ProjectChange' ./scripts/tla-check.sh   # inner loop: only the runs whose name matches
#   INKWELL_FORMAL_NO_CACHE=1 mise run tla   # rerun every model: no verdict reused (scripts/gate.py sets it)
#
# Verdict cache (block CI3b, scripts/formal_cache.py): a run whose inputs (model closure, cfg, the code of run()
# below, the jar, Java, its arguments) are unchanged since a green verdict prints that verdict's ✓ line, then
# « reused <key prefix>: <name> », without running TLC.
set -euo pipefail
# Functions inherited from the environment (exported), left out of the verdict cache's runner key (runner_sha, below).
inherited_functions="$(declare -F)"

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

shard="${TLA_SHARD:-}"
case "$shard" in
  ''|1/3|2/3|3/3) ;;
  *) die "invalid TLA_SHARD '$shard' (expected 1/3, 2/3 or 3/3)" ;;
esac

list=0
case "${1:-}" in
  '') ;;
  --list) list=1 ;;
  *) die "usage: tla-check.sh [--list]" ;;
esac

if ((!list)); then
  # TLC's state files (-metadir) go under one directory per run of this script, removed on any exit: a run cut
  # by a timeout or a signal left its states in /tmp, a tmpfs, until it filled (« disk quota exceeded », 2026-11-17).
  metaroot="$(mktemp -d -t inkwell-tla.XXXXXX)"
  trap 'rm -rf "$metaroot"' EXIT INT TERM HUP

  command -v java >/dev/null 2>&1 || die "java (11+) is required"
  command -v sha256sum >/dev/null 2>&1 || die "sha256sum is required"
  java_version="$(java -version 2>&1)"

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
fi

fail=0

# run NAME CONFIG MODULE sound|safety|mutant [PROPERTY]
# PROPERTY (mutants): the violation must be of that invariant or temporal property, named by TLC.
run() {
  local name="$1" cfg="$2" module="$3" expect="$4" prop="${5:-}"
  local out="$logs/$name.log" metadir status
  if [[ -n "${INKWELL_TLA_ONLY:-}" && ! "$name" =~ $INKWELL_TLA_ONLY ]]; then
    return 0
  fi
  if [[ -n "$shard" ]]; then
    local assigned
    # Family totals and the headroom for InboundLifecycle are recorded in the CI2 plan.
    case "$name" in
      RepoGen*|ConnCredit*|WriteBudget*|OISingleInstance*|SyncOrder*|InboundLifecycle*|HP1c[!S]*|RF1*|RFR*) assigned=1/3 ;;
      SendPath*|SP1*|SandboxScope*|ProjectChange*) assigned=2/3 ;;
      SyncIndicator*|LinkProbe*|DirectUpgrade*|HP1r*|HP1cSupersede*|PeerUpAck*|PU1d*) assigned=3/3 ;;
      *) die "no shard assigned to '$name'" ;;
    esac
    [[ "$assigned" == "$shard" ]] || return 0
  fi
  if ((list)); then
    printf '%s\n' "$name"
    return 0
  fi
  # Verdict cache: on a hit the stored ✓ line is printed and TLC does not run; key stays empty when bypassed or when
  # the inputs cannot be named exactly, and then nothing is stored.
  local key="" cached ok=""
  if [[ "${INKWELL_FORMAL_NO_CACHE:-0}" == 0 ]]; then
    local args=(--arg "name=$name" --arg "expect=$expect" --arg "prop=$prop" --arg "workers=$workers")
    # shellcheck disable=SC2154 # sim_seed, sim_depth and sim_traces are set per call as prefixes (below)
    [[ "$expect" == simulate ]] && args+=(--arg "seed=$sim_seed" --arg "depth=$sim_depth" --arg "traces=$sim_traces")
    set +e
    cached="$(python3 -I "$root/scripts/formal_cache.py" tla-lookup --root "$dir" --module "$module" --cfg "$cfg" \
      --jar-sha "$jar_sha" --java "$java_version" --runner-sha "$runner_sha" "${args[@]}")"
    status=$?
    set -e
    if ((status == 0)); then
      key="${cached%%$'\n'*}"
      printf '%s\n' "${cached#*$'\n'}"
      printf 'reused %s: %s\n' "${key:0:12}" "$name"
      printf 'reused verdict %s (scripts/formal_cache.py)\n' "$key" >"$out"
      return 0
    fi
    if ((status == 1)); then key="$cached"; else key=""; fi
  fi
  metadir="$(mktemp -d -p "$metaroot")"
  # simulate: TLC's random simulation, fixed seed and depth, where no exhaustive run fits the budget (a second,
  # weaker check beside the exhaustive runs of smaller bounds; block HP1c2).
  local mode=()
  # shellcheck disable=SC2154 # sim_seed, sim_depth and sim_traces are set per call as prefixes (sim_seed=… run …, below)
  [[ "$expect" == simulate ]] && mode=(-simulate "num=$sim_traces" -depth "$sim_depth" -seed "$sim_seed")
  set +e
  (
    cd "$dir"
    # java.io.tmpdir: Java ignores TMPDIR, and TLC leaves an empty tlc-<n> folder in it on every run (its
    # module scratch, util.FilenameToStream): 14 389 of them piled up in /tmp by 2026-11-23.
    "$heavy" java -Xmx"$xmx" -XX:+UseParallelGC -Djava.io.tmpdir="$metadir" -cp "$jar" tlc2.TLC \
      -workers "$workers" -noGenerateSpecTE -cleanup -metadir "$metadir" "${mode[@]}" \
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
      ok="$(printf '✓ %s: no error (exhaustive, liveness checked)' "$name")"
    fi
  elif [[ "$expect" == simulate ]]; then
    if ((status != 0)) || grep -q '^Error:' "$out" || ! grep -q 'traces generated' "$out"; then
      printf '✗ %s: simulation found an error or did not run (exit %d)\n' "$name" "$status" >&2
      tail -n 20 "$out" >&2
      fail=1
    else
      # shellcheck disable=SC2154 # sim_seed, sim_depth and sim_traces are set per call as prefixes (sim_seed=… run …, below)
      ok="$(printf '✓ %s: no error (simulation, seed %s, depth %s, %s traces per worker)' "$name" "$sim_seed" "$sim_depth" "$sim_traces")"
    fi
  elif [[ "$expect" == safety ]]; then
    # A larger configuration of a model whose liveness a smaller sound configuration checks.
    if ((status != 0)) || ! grep -q 'No error has been found' "$out" || ! grep -q '0 states left on queue' "$out"; then
      printf '✗ %s: sound model failed (exit %d)\n' "$name" "$status" >&2
      tail -n 20 "$out" >&2
      fail=1
    else
      ok="$(printf '✓ %s: no error (exhaustive, invariants)' "$name")"
    fi
  else
    if ((status == 12 || status == 13)) && grep -Eq 'Error: (Invariant|Temporal property) .* violated' "$out" \
       && { [[ -z "$prop" ]] || grep -Eq "Error: (Invariant|Temporal property) $prop (is|was) violated" "$out"; }; then
      ok="$(printf '✓ %s: violation detected%s' "$name" "${prop:+ ($prop)}")"
    else
      printf '✗ %s: mutant passed (exit %d) — the model no longer catches it\n' "$name" "$status" >&2
      tail -n 20 "$out" >&2
      fail=1
    fi
  fi
  # Only a green verdict reaches here with ok set; a failure leaves ok empty and is never stored.
  if [[ -n "$ok" ]]; then
    printf '%s\n' "$ok"
    if [[ -n "$key" ]]; then
      printf '%s\n' "$ok" | python3 -I "$root/scripts/formal_cache.py" put "$key" "$name" \
        || printf 'tla: verdict of %s not stored\n' "$name" >&2
    fi
  fi
}
# The verdict cache keys on every function this script defines (run and each helper it calls; `declare -f` prints
# them in a stable order), not on the list of runs below: adding a model changes no other run's key.
mapfile -t own_functions < <(comm -13 <(printf '%s\n' "$inherited_functions" | sort) <(declare -F | sort) | sed 's/^declare -f //')
runner_sha="$(declare -f "${own_functions[@]}" | sha256sum | cut -d' ' -f1)"

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
         SP1Grown90 SP1GrownKeepalive SP1Hop SP1Blackout SP1Silence SP1Any; do
  run "$c" "$c.cfg" SendPath sound
done

# LinkProbe (README "LinkProbe", spec §8.5, ADR 0036): honest responder with liveness, Byzantine safety.
run LinkProbeHonest LinkProbeHonest.cfg LinkProbe sound
run LinkProbeAny LinkProbeAny.cfg LinkProbe sound
run LinkProbeGreedy LinkProbeGreedy.cfg LinkProbe sound
run LinkProbeAccept LinkProbeAccept.cfg LinkProbe sound

# OISingleInstance (README "OISingleInstance", block OI, ADR 0037): one process per profile and the hand-off.
run OISingleInstance OISingleInstance.cfg OISingleInstance sound
run OISingleInstanceHang OISingleInstanceHang.cfg OISingleInstance sound
run OISingleInstanceReconnect OISingleInstanceReconnect.cfg OISingleInstance safety
run OISingleInstanceReconnectLive OISingleInstanceReconnectLive.cfg OISingleInstance sound

# SandboxScope (README "SandboxScope", block SB1b, ADR 0042 A1-A4 and A8): the sandbox's memory scope, no fault and every fault.
run SandboxScope SandboxScope.cfg SandboxScope sound
run SandboxScopeFaults SandboxScopeFaults.cfg SandboxScope sound

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
         SP1PartialTTLAtIdle SP1NoTotal SP1NoCut SP1FloorKeepaliveBigHop \
         SP1TypingResidual; do
  run "$m" "mutants/$m.cfg" mutants/SendPathMutants mutant
done
# LinkProbe mutants (README "LinkProbe"): definition overrides from LinkProbeMutants.
for m in AcceptLate ZeroPTO NoSentBound StrictSentBound NoMonotone NoClamp NoPendingBound KeepAcrossRestart \
         NoReopen StopAndWait NoStreamCheck FloodLimit NoCredit NoRefusal AcceptSecond LongRefusal NoSeqBound \
         DataOnly NoDropBand; do
  run "LinkProbe$m" "mutants/LinkProbe$m.cfg" mutants/LinkProbeMutants mutant
done
# OISingleInstance mutants and witnesses (README "OISingleInstance"): definition overrides from OISingleInstanceMutants;
# a witness is an invariant the sound model must violate (anti-vacuity).
for m in NoLock EarlyIdentity AckNoQueue FireAndForget LockReleasedEarly CrashKeepsLock GiveUpOnClosing SecondUnlinks \
         NoRetryLock NoDrain AckDropOnClose EarlyDequeue StaleReportAccepted ResetAllOnDisconnect \
         WitnessHandoff WitnessGiveUp WitnessTakeover WitnessHandover WitnessStaleReport; do
  run "OISingleInstance$m" "mutants/OISingleInstance$m.cfg" mutants/OISingleInstanceMutants mutant
done
# SandboxScope mutants and witnesses (README "SandboxScope"): definition overrides from SandboxScopeMutants;
# a witness is an invariant the sound model must violate (anti-vacuity); the Residual witness must
# violate too, so that the README never claims a guarantee the design does not give.
for m in NotBornInside ExecBeforeCheck CleanupBeforeCounters UnreadAsZero VictimAsLimit MixedHistoryAsLimit \
         ContinueAfterPartialSetup SessionUnbounded JobDoneIsConfined RaiseAddressLimit HolderDeathEscapes \
         OrphanAfterAppKill ErrorOnFallback WaitForScope WitnessLocalLimit WitnessAddressLimit WitnessFallback \
         WitnessVictim WitnessMixedHistory WitnessHolderDeath ResidualCollected NoPdeathsig NoPidNamespace; do
  run "SandboxScope$m" "mutants/SandboxScope$m.cfg" mutants/SandboxScopeMutants mutant
done

# ProjectChange (README "ProjectChange", block PC1, ADR 0043 part 2): the project changes by a restart, exec and spawn.
run ProjectChange ProjectChange.cfg ProjectChange sound
run ProjectChangeSpawn ProjectChangeSpawn.cfg ProjectChange sound
run ProjectChangeFaults ProjectChangeFaults.cfg ProjectChange sound
run ProjectChangeSpawnFaults ProjectChangeSpawnFaults.cfg ProjectChange sound
# ProjectChange mutants and witnesses: each must be caught by the property its README row names.
while read -r m prop; do
  run "ProjectChange$m" "mutants/ProjectChange$m.cfg" mutants/ProjectChangeMutants mutant "$prop"
done <<'MUTANTS'
LiveAdopt NoOldStateAfterChange
RestartBeforeDurable WriteBeforeRestart
RestartTwice RestartOnce
RequestBeforeNavigation NoOldStateAfterChange
JoinNoRestart ChangeCompletes
JoinTwice JoinOnce
WriteBeforeWiringStopped NoOldStateAfterChange
WriteBeforeDrain NoLostWrite
NoBarrier NoOldStateAfterChange
SpawnNoWait OneImage
JoinRecordAfterInstall OpensAfterCrash
RecoverPrefersAside NewStateSurvives
CarriedOutcome NoticeMatchesStanding
PageNeverReloads ChangeCompletes
WitnessTakeOver NoTakeOverDone
WitnessJoinAnnounced NoJoinAnnounced
WitnessCrashAfterInstall NoCrashAfterInstall
WitnessFailedWrite NoFailedNotice
WitnessWriteRefused NoRefusedWrite
MUTANTS

# DirectUpgrade (README "DirectUpgrade", block HP1, plan 2026-11-16-hp1-direct-upgrade.md §6): a relayed link
# to a wanted peer keeps trying to become direct. Byzantine is invariants only (its liveness: the honest configs).
run DirectUpgradeHonest DirectUpgradeHonest.cfg DirectUpgrade sound
run DirectUpgradeSilent DirectUpgradeSilent.cfg DirectUpgrade sound
run DirectUpgradeTwoCircuits DirectUpgradeTwoCircuits.cfg DirectUpgrade sound
run DirectUpgradeAnswerer DirectUpgradeAnswerer.cfg DirectUpgrade sound
run DirectUpgradeTwoPeers DirectUpgradeTwoPeers.cfg DirectUpgrade sound
run DirectUpgradeLostPush DirectUpgradeLostPush.cfg DirectUpgrade sound
run DirectUpgradeByzantine DirectUpgradeByzantine.cfg DirectUpgrade safety
# DirectUpgrade mutants (M1-M29) and witnesses (anti-vacuity):
# each must be caught by the property its README row names.
while read -r m prop; do
  run "DirectUpgrade$m" "mutants/DirectUpgrade$m.cfg" mutants/DirectUpgradeMutants mutant "$prop"
done <<'MUTANTS'
NoRetry Upgrades
StaleApplied StaleIgnored
StaleAppliedLive Upgrades
BusyTakesToken BusyFree
AnyPeer NoLaunchUnwanted
OutboundPunch RoleOK
LowerOnly Upgrades
EpochRefills RoundBudget
SchedulerOnly RoundBudget
NoLocalReady NoLaunchUnready
StopAtCap Upgrades
NoCap GlobalCap
WhileDirect NoLaunchDirect
WhileInFlight OneCall
GateAnyPeer NoRoundUnwanted
NoDeadline BusyEnds
EpochRefillsAttempts AttemptBudget
RemoteGuard Upgrades
AnyOrder BusyEnds
PreDialFree DestBudget
GaterAnyPeer NoDialUnwanted
IdleIgnored NoLadderCallIdle
StaySuspended Upgrades
GrantFree AdmissionBudget
H15Off Upgrades
NoClose DupResolves
NoAllowance NoUnallowedDial
GaterNoLease NoUnallowedDial
TicketAny NoTicketUnwanted
PlaceLost Upgrades
WitnessDirect NoDirect
WitnessStale NoStaleResult
WitnessBusy NoBusy
WitnessSlow NoSlowStep
WitnessGateRefusal NoGateRefusal
WitnessWaitBehind NoWaitBehind
WitnessCross NoCross
MUTANTS
# Block HP1r (rule H-19, the retention of a direct connection): its runs are named outside ^DirectUpgrade so that
# block HP1's frozen count of the DirectUpgrade runs (gates/HP1.toml, `tla`: 44) stays exact.
run HP1rLinkLost DirectUpgradeLinkLost.cfg DirectUpgrade sound
run HP1rLateLand mutants/DirectUpgradeLateLand.cfg mutants/DirectUpgradeMutants mutant LinkBound
run HP1rWitnessSurvives mutants/DirectUpgradeWitnessSurvives.cfg mutants/DirectUpgradeMutants mutant NoDirectAfterRelayLost
# Block HP1c (rules H-20 and P-15, plan 2026-11-23-hp1-reformalize.md §2): R crashes, its circuits stay open at L, dead.
# Named outside ^DirectUpgrade, ^HP1r and ^PeerUpAck so that the frozen counts of HP1 (44), HP1r (3) and PU1 (19) stay.
run HP1cCrash DirectUpgradeCrash.cfg DirectUpgrade sound
run HP1cProofTwoCircuits DirectUpgradeProofTwoCircuits.cfg DirectUpgrade sound
while read -r m prop; do
  run "HP1c$m" "mutants/DirectUpgrade$m.cfg" mutants/DirectUpgradeMutants mutant "$prop"
done <<'MUTANTS'
RoundAnyConn RoundOnLive
CloseOnTimer ProofKeepsLive
ResolveNoWait ResolveKeepsLive
WitnessGhost NoRoundBesideGhost
WitnessAnswered NoProofAnswered
WitnessProofClose NoProofClose
MUTANTS
# Supersede (README "Supersede", block HP1c, spec §8.7.4): P-15 on PeerUpAck's connections, PeerUpAck itself unedited
# (gates/PU1c.toml pins it). Honest and AnyTiming are invariants; Live checks P15-5 with no idle timeout assumed.
run HP1cSupersede SupersedeHonest.cfg Supersede safety
run HP1cSupersedeAnyTiming SupersedeAnyTiming.cfg Supersede safety
# Three connections (HP1c2, review 3b): no exhaustive run fits the budget, so a seeded simulation (residual -> HP1).
sim_seed=20771 sim_depth=60 sim_traces=100000 run HP1cSupersedeThree SupersedeThree.cfg Supersede simulate
run HP1cSupersedeLive SupersedeLive.cfg Supersede sound
while read -r m prop; do
  run "HP1cSupersede$m" "mutants/Supersede$m.cfg" mutants/SupersedeMutants mutant "$prop"
done <<'MUTANTS'
NoProof SupersededBounded
CloseOnTimer LiveKept
IgnoreAckWait FallbackKept
ProveSole NeverCloseSole
BackwardAck FallbackKept
WitnessProofClose NoProofClose
WitnessAnswer NoAnswer
WitnessFallback NoFallbackWait
MUTANTS

# PeerUpAck (README "PeerUpAck", block PU1, spec §8.7): PeerUp only after the peer's acceptance ACK of this stream.
# Late, Byzantine and Reuse are invariants under any timing; Honest, LateLive, Silent and Stuck check liveness.
run PeerUpAckHonest PeerUpAckHonest.cfg PeerUpAck sound
run PeerUpAckLate PeerUpAckLate.cfg PeerUpAck safety
run PeerUpAckLateLive PeerUpAckLateLive.cfg PeerUpAck sound
run PeerUpAckSilent PeerUpAckSilent.cfg PeerUpAck sound
run PeerUpAckStuck PeerUpAckStuck.cfg PeerUpAck sound
run PeerUpAckByzantine PeerUpAckByzantine.cfg PeerUpAck safety
run PeerUpAckReuse PeerUpAckReuse.cfg PeerUpAck safety
# PeerUpAck mutants (M1-M8) and witnesses: each must be caught by the property its README row names.
while read -r m prop; do
  run "PeerUpAck$m" "mutants/PeerUpAck$m.cfg" mutants/PeerUpAckMutants mutant "$prop"
done <<'MUTANTS'
UpOnReturn NoUpOnRefused
BeforeAdmit UpAfterAccept
AfterOwnUp BothUp
MoveNoAck SessionOnAccepted
NoRetention UpAfterAccept
TimeoutGuarded OpenResolves
AckByConn SessionOnAccepted
BudgetPerPhase Budget
WitnessLazyLate NotLazyLate
WitnessBothUp NotBothUp
WitnessMove NoMove
WitnessLateFire NoLateFire
MUTANTS

# CloseScope (README "CloseScope", block PU1d, spec §8.7.4, rules P-16 and P-17): a failed session opening closes only
# its named connection; any usable connection starts an opening. PeerUpAck itself unedited (gates/PU1b.toml, gates/PU1c.toml
# pin it); named outside '^PeerUpAck' so that PU1's frozen count (19) stays. Any and Negotiated are invariants; Live checks
# L3 and L2.
run PU1dCloseScopeAny CloseScopeAny.cfg CloseScope safety
run PU1dCloseScopeLive CloseScopeLive.cfg CloseScope sound
run PU1dCloseScopeNegotiated CloseScopeNegotiated.cfg CloseScope safety
while read -r m prop; do
  run "PU1dCloseScope$m" "mutants/CloseScope$m.cfg" mutants/CloseScopeMutants mutant "$prop"
done <<'MUTANTS'
ClosePeer CrossedIndependent
OnEventOnly NeverStranded
WitnessBeside NoFailBeside
UnnamedOpen OpeningNamed
UnnamedCollateral CrossedIndependent
WitnessQuiet NoQuietOpen
WitnessNeg NoUnreturnedFail
MUTANTS

# SyncOrder (README "SyncOrder", block PU1b, spec §8.7.3-§8.7.4): the responder's order of a peer's inbound sync
# streams, two places, removal with wake, capacity refusal without ClosePeer. Names outside PU1's '^PeerUpAck' set.
run SyncOrderHonest SyncOrderHonest.cfg SyncOrder sound
run SyncOrderAnyTiming SyncOrderAnyTiming.cfg SyncOrder safety
run SyncOrderByzantine SyncOrderByzantine.cfg SyncOrder sound
while read -r m prop; do
  run "SyncOrder$m" "mutants/SyncOrder$m.cfg" mutants/SyncOrderMutants mutant "$prop"
done <<'MUTANTS'
NoWake AllDelivered
KeepEmpty EmptyLeaves
ClosePeerOnBusy SessionKept
Bound3 Places
EofKeeps EmptyLeaves
WitnessBusy NoBusy
WitnessRemoveWaiting NoRemovalWhileWaiting
WitnessEmptyEof NoEmptyEof
WitnessBudgetOut NoBudgetOut
MUTANTS

# InboundLifecycle (README "InboundLifecycle", block PU1c, spec §8.1, §8.7.3, §16.1): one reader per inbound stream on
# both ordered lanes at once, the probe while waiting, the budget only from the turn, the shared QUIC credit. Names
# outside PU1's '^PeerUpAck' and PU1b's '^SyncOrder' sets.
run InboundLifecycleHonest InboundLifecycleHonest.cfg InboundLifecycle sound
run InboundLifecycleByzantine InboundLifecycleByzantine.cfg InboundLifecycle sound
run InboundLifecycleByzantineSafety InboundLifecycleByzantineSafety.cfg InboundLifecycle safety
run InboundLifecycleCredit InboundLifecycleCredit.cfg InboundLifecycle sound
while read -r m prop; do
  run "InboundLifecycle$m" "mutants/InboundLifecycle$m.cfg" mutants/InboundLifecycleMutants mutant "$prop"
done <<'MUTANTS'
PlaceSyncLate Order
FailedWaits EmptyLeaves
ProbeOnce NoBlindWait
WaitCharges WaitNoBudget
KeepWholeOnly FinFramesDelivered
NoCancel BlockedFreed
BulkTail Order
SmallWindow HeadProgresses
WitnessKept NoKept
WitnessBlocked NoBlocked
WitnessTruncatedTail NoTruncatedTail
WitnessExhausted NoCreditExhaustedFirst
MUTANTS


# RelayFanout (README "RelayFanout", block RF1, spec §16.1, rules RF-1 to RF-6): members leave the per-origin counts at
# their handshake, a circuit is charged to its relay's PeerID, strangers' pending connections are bounded per origin,
# in all and by a token bucket, they never use members' room, and a Byzantine member never takes another's share.
# Flood, Inbound and Mixed check S1-S9 and liveness L1.
run RF1Flood RelayFanoutFlood.cfg RelayFanout sound
run RF1Inbound RelayFanoutInbound.cfg RelayFanout sound
run RF1Mixed RelayFanoutMixed.cfg RelayFanout sound
# Review round (roster growth, outside the 26 runs gates/RF1.toml froze): a member admitted after the surplus was
# taken; the shed keeps SharesKept (RFRGrowth, with liveness), and without it SharesKept fails (RFRNoShed, MFR1).
run RFRGrowth RelayFanoutGrowth.cfg RelayFanout sound
run RFRNoShed mutants/RelayFanoutNoShed.cfg mutants/RelayFanoutMutants mutant SharesKept
while read -r m prop; do
  run "RF1$m" "mutants/RelayFanout$m.cfg" mutants/RelayFanoutMutants mutant "$prop"
done <<'MUTANTS'
PerIPCount InJustified
PerIPCountLive MembersEventuallyUp
OutboundCharged OutNeverRefused
OutboundStarved MembersEventuallyUp
NoDeadline MembersEventuallyUp
NoOriginQuota OriginBound
NoGlobalQuota GlobalBound
NoToken RateBound
NoRefund MembersSpendNoRate
SharedCap OutNeverRefused
AuthShared AuthNeverRefused
NoShare SharesKept
NoMemberCap MemberCapBound
RefundUnheld RateBound
WindowRefill RateBound
WitnessInRefused NoInRefusal
WitnessRateRefused NoRateRefusal
WitnessQuotaFull NoFullQuota
WitnessPassage NoPassage
WitnessSurplus NoSurplus
WitnessByzRefused NoByzRefusal
WitnessPaidPending NoPaidPending
WitnessResidual MembersEventuallyUp
MUTANTS

exit "$fail"
