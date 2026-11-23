# HP1 - re-formalization after HP1m (crash restart, MH6, e2e mutant builds)

Status: **draft, step 1** (2026-11-23), reviewed (§7). Rigour: **complete**. Worktree `../inkwell-hp1m` on `main`
`a4a5a5a`. Complements [the HP1 plan](2026-11-16-hp1-direct-upgrade.md). `L/` stands for
`$(go env GOMODCACHE)/github.com/oioio-space/go-libp2p@v0.51.1-0.20261122194938-8ebee42bc17a`.

## 1. Problem

HP1m failed and the fixes after it no longer converge (`gates/HP1/SIMPLIFY.md`, « W77 stopped call », « HP1
mutants »). There are three causes. After a peer **crashes**, new work lands on a dead connection for about 35 s
(§2). MH6's test path is now doubly guarded (§3). The e2e mutant builds read VCS state above their copy (§4).

## 2. Item 2 first: a dead connection that libp2p still prefers

**Evidence.**
- The relay keeps b's old connection until it times out: quic-go `DefaultIdleTimeout = 30 s` (`quic-go@v0.64.1`
  `internal/protocol/params.go:87`; keepalive 15 s, `L/p2p/transport/quicreuse/config.go:18`). Our yamux keepalive
  is 30 s (`internal/transport/p2p/muxer.go:23`). The measured 33.8-38.9 s is that window.
- Until then a sees two open circuits to b. `Swarm.NewStream` takes `bestConnToPeer` (`L/p2p/net/swarm/swarm.go:492`),
  and `isBetterConn` prefers **more open streams** before the newest (`swarm.go:610-624`). That is the old circuit,
  which carries the session.
- Three of our own selectors also take the oldest connection: `circuitTo` returns the first open circuit
  (`upgrade.go:629-636`), and `cmpConn` keeps the first one seen (`upgrade.go:377`).
- The dead circuit then blocks three things:
  - `autoPunch(c)` is caused by the new circuit `c` (`node.go:566-569`), but it opens its rounds through
    `host.NewStream(p)` (`dcutr.go:161-163`, as upstream does at `L/p2p/protocol/holepunch/holepuncher.go:204-208`).
    The round waits up to `StreamTimeout = 1 min` (`svc.go:30`) while it holds `active[p]`, so every policy call is
    Busy (H-4).
  - a's outbound session stays on the dead circuit, so a→b is lost.
  - b's old inbound stream holds the head of the order. The head read is unbounded (§8.7.3), so b→a is held.
- Upstream: no issue names this case; per-`NewStream` selection is intended (go-libp2p PR #2726); "perceived as
  connected" dead peers are a known class (go-libp2p-pubsub #433); `relay/DCUtR.md` never says which connection
  carries the stream.

**Root cause.** Two independent faults:
- (R1) **Work is not bound to the connection that caused it.** A round caused by circuit `c` runs wherever libp2p
  chooses.
- (R2) **A superseded connection is never asked to prove that it is alive.** An honest node dials only a peer that it
  is not connected to (`dial.go:63`). A new connection therefore means the older one may have died without a close.
  Nothing detects that in under 30 s.

Both rules are needed. R2 alone leaves H-6 broken: the unbound round is reset at the proof deadline, classed Spent,
and the 60 s ladder follows. R1 alone leaves the crossed-direction crash stuck: H-15 forbids initiation while two
circuits exist.

**Rule H-20 (spec §8.6, after « Rôles »), text to add:**
> **Circuit d'une ronde (règle H-20).** Un nœud ouvre chaque ronde DCUtR qu'il initie sur le circuit qui la cause,
> jamais sur la connexion que choisit libp2p. La tentative automatique s'ouvre sur le circuit entrant dont
> l'arrivée la déclenche. Un appel du rythme s'ouvre sur le circuit entrant qui a ouvert l'époque en cours, ou, s'il
> est fermé, sur le circuit entrant ouvert le plus récent. Si ce circuit se ferme pendant l'appel, l'appel finit et se
> classe sur ce qui a été observé. Raison : libp2p préfère le circuit qui porte le plus de flux, donc le plus ancien.
> Après un plantage du pair, ce circuit reste ouvert environ 30 s sans être vivant. Une ronde ouverte sur lui y
> attendrait tout ce temps, en tenant l'unique appel en cours vers le pair (H-4).

**Rule P-15 (spec §8.7.4, new paragraph after the « connexion qui arrive » sentence; one row added to §8.7.5), → HP1
text to add:**
> **Connexion supplantée (règle P-15).** Quand une connexion vers un pair s'enregistre et est gardée (H-19), chaque
> connexion plus ancienne encore ouverte vers ce pair DOIT prouver qu'elle vit. Le nœud ouvre, sur cette connexion
> précise, un flux `/ipfs/ping/1.0.0` et y écrit 32 octets. Tout octet reçu du pair sur cette connexion dans les
> `T_vie` = 5 s qui suivent l'enregistrement est une preuve : un écho, un refus de négociation ou une
> réinitialisation. Une connexion qui prouve qu'elle vit reste ouverte, et rien ne change. Une connexion sans preuve
> est fermée, mais seulement une fois que la nouvelle porte un flux `/inkwell/sync/2.0.0` accusé, dans un sens ou
> dans l'autre. Tant que l'ouverture sur la nouvelle connexion attend son accusé (15 s au plus, §8.7.4), l'ancienne
> reste le chemin de repli. La fermer ferait annoncer à tort la perte du pair (`PeerDown`) si cette ouverture
> échouait. La fermeture a donc lieu à la plus tardive de ces deux échéances : `T_vie`, ou l'accusé sur la nouvelle
> connexion. Si la nouvelle connexion se ferme avant cet accusé, la preuve est abandonnée et l'ancienne connexion
> reste ouverte. Les flux d'une connexion ainsi fermée finissent comme des flux perdus avec leur connexion : la
> session se rouvre ailleurs, sans `ClosePeer` ni événement, et un flux entrant qui n'a rien livré quitte l'ordre
> (§8.7.3). Le nœud NE DOIT PAS fermer ainsi la connexion la plus récente, ni sa seule connexion au pair. Il tient
> au plus une preuve en cours par connexion. Raison : un pair conforme ne compose que vers un pair auquel il n'est
> pas relié, donc une connexion nouvelle signale que l'ancienne a pu mourir sans être fermée. Ni QUIC (30 s
> d'inactivité) ni le multiplexeur (30 s) ne le voient plus tôt. `T_vie` vaut deux fois l'aller-retour de 2,5 s à
> partir duquel un lien est « mauvais » (§8.5.7). Un pair qui ouvre des connexions en boucle ne fait envoyer que
> 32 octets par connexion plus ancienne, et ne ferme que ses propres connexions.
>
> Row for §8.7.5: `Ouvert | connexion du flux fermée faute de preuve (P-15) | comme « flux perdu » : ouverture sur une
> connexion utilisable, sans événement`.

**Code.**
- H-20 (`internal/transport/p2p`):
  - `punchService.connect(ctx, p, via)` opens each round with `newStreamOn(via, …)` (`stream.go:599`).
  - The upgrader records the epoch's circuit on E7.
  - `circuitTo` and `cmpConn` take the newest open connection (`Stat().Opened`).
- P-15 is a pure decision, `syncack.Supersede`, in the reference package of §8.7.
  - Input: each connection's open time, its proof state (pending, answered, silent) and whether it carries an acked
    sync stream.
  - Output: the proofs to start and the connections to close. An answered connection is never closed. A silent one
    is closed only once a newer connection is acked.
  - Vectors `supersede_*` in `vectors/syncack.json`, including `supersede_newer_unacked_keeps_older` (a newer direct
    connection awaiting its ACK and a silent older circuit: keep) and `supersede_answered_keeps_older`. Its decoder
    is fuzzed.
- The proof itself is a 32-byte echo on the chosen connection. `ping.Ping` cannot be used: it picks its own connection
  (`L/p2p/protocol/ping/ping.go:116`). The ping handler is on by default (`L/config/config.go:454`).

**Models (agent `formal-modeler`; run names start with `HP1c`, so HP1's 44 and HP1r's 3 runs keep their counts).**
- `DirectUpgrade.tla`:
  - constant `Crash` and action `RCrash`: live circuits become `ghost` (open for L, dropping every message until
    `GhostTimeout`), and a new circuit in the same direction may coexist with a ghost.
  - operator `RoundConn` (the cause circuit) and S19 `RoundOnLive` (no round starts on a ghost). Mutant M31
    `RoundAnyConn` (the swarm's choice) is caught by S19.
  - P-15 on circuits: a live circuit's proof is answered and it stays open, checked by S20 `ProofKeepsLive`. With
    `TwoCirc`, crossed simultaneous dials must still settle by H-15 (the existing `DupResolves`), not by a premature
    close. Mutant M32 `CloseOnTimer` is caught by S20.
  - witness `HP1cWitnessGhost`: a ghost and a new circuit coexist.
  - H-15 waits for P-15's proofs (`ResolveWaits`), so a ghost is never taken for a duplicate. S21
    `ResolveKeepsLive`: H-15 never closes L's live circuit in favour of a ghost. Mutant M33 `ResolveNoWait` (no wait)
    is caught by S21 on `Crash` (counterexample: Dial(LR), RCrash(RL), Resolve keeps the ghost LR). Added
    2026-11-23: the guard had no mutant.
- `PeerUpAck.tla` already has the half-open state (`c ∉ live`, `c ∉ seen[n]`). It gets:
  - `ProveAnswered(n, c)`, the positive transition: a live c answers within `TA` and stays usable, so every existing
    invariant is checked while proofs run.
  - `ProveClose(n, c)`: a dead c gets no answer by `TA` while a newer connection carries an acked stream; c closes.
  - Invariants `SupersededBounded`, `LiveKept`, `FallbackKept` (no close while the newer opening awaits its ACK) and
    `NeverCloseSole`; liveness `SessionLeavesDead`.
  - Mutants `NoProof`, `CloseOnTimer`, `IgnoreAckWait` and `ProveSole`, each caught by the invariant in the same
    order.
  - The existing configurations set `Crash = FALSE`, so their state spaces do not change.

**Tests.**
- `TestUpgradeAfterCrashRestart` (synctest, simnet): the old endpoint is blacked out, then `crashRestart` runs on
  endpoint 3. Subtests `same-direction` and `crossed-direction`, 20 seeds each. Each assertion fails with its rule's
  message:
  - `H-20 round on a dead circuit`.
  - `H-6 direct after the new circuit`: within 3 s for the same direction, or `T_vie` + 3 s when crossed (H-15 waits
    for P-15).
  - `P-15 dead connection kept`: not closed within `T_vie` + 1 s.
  - `P-15 frames held`: frames are not delivered both ways within `T_vie` + 2 s, or a `PeerDown` appears.
- Statistical acceptance as in HP1r: 40/40 rejects a residual failure rate ≥ 7.2 % at 95 % confidence. → HP1
- `TestSupersedeKeepsLiveOlder` (synctest, simnet): a second circuit arrives while the first is live and answers
  within `T_vie`. The first must stay open, with the session neither reopened nor moved and no event; otherwise the
  test fails with `P-15 live connection closed`. A subtest holds the ACK of a newer direct connection for 10 s while
  the older circuit is silent. The circuit must survive until the ACK, with no `PeerDown` (`P-15 fallback
  closed`).
- Pure tests: `TestSupersede` (vectors), `TestSupersedeKeepsSole`, `TestSupersedeKeepsAnswered`,
  `TestSupersedeWaitsForNewerAck`.
- Go mutants (block tag `HP1c`, so HP1's frozen table is untouched). Each is caught by its test failing with its
  text:
  - MH23: rounds go through `host.NewStream`. `TestUpgradeAfterCrashRestart`, `H-20 round on a dead circuit`. → HP1
    Its anchor is the H-20 code (`punchService.connect` opening each round with `newStreamOn(via, …)`), written at
    step 2. `mutants.py` has no deferred anchors, so the entry is added at step 2; `gates/HP1c.toml` freezes its test
    and text and stays red until then.
  - MP1: no proof is started (`supersede.go`, the `Unproved` case proves nothing). `TestUpgradeAfterCrashRestart`,
    `P-15 dead connection kept`.
  - MP2: a silent sole connection is closed (`older := cs`: the newest is judged like the older ones).
    `TestSupersedeKeepsSole`, `P-15 sole connection closed`.
  - MP3: the timer closes whatever the answer (`supersede.go`, `Heard` ignored). `TestSupersedeKeepsLiveOlder`,
    `P-15 live connection closed`.
  - MP4: the ACK wait is ignored (`closes[i] = true`). `TestSupersedeWaitsForNewerAck`, `P-15 fallback closed`.
  - MP1-MP4 have their entries now: their anchors are in the reference decision `protocol/syncack/supersede.go`,
    which exists at step 1. Step 2 may move an anchor, never a test or its text.

## 3. Item 1: MH6

**Recommendation: (a).** Repoint MH6 to `("protocol/directup", "TestPolicyEpoch", "H-6 direct after the new
circuit")` in a superseding gate.
- The mutated line is in the pure policy, and `TestPolicyEpoch` already kills it with the frozen message.
- On the old integration path MH6 is now equivalent: a stopped call and a stale epoch both discard the result, as in
  the model (`StaleIgnored`, M2).
- This follows the project's rule of asserting at the cheapest layer that keeps the defect class, and it is
  deterministic.

**(b) is rejected.** Once H-20 binds the one-shot to the new circuit, the one-shot goes direct by itself and masks
MH6. Seeing MH6 again would mean choreographing around the one-shot, the reason the `crashed` subtest was dropped.

## 4. Item 3: e2e mutant builds

- `/tmp/.git` exists again: an empty directory created 2026-11-21 19:17, 42 min after `f5d2b55` removed it.
- `run_e2e` builds in `tempfile` copies under `/tmp` (`scripts/netsim/mutants.py:221-223`, `:148`). Go stamps VCS
  information « if the main package, the main module containing it, and the current directory are all in the same
  repository » (`go help build`, `-buildvcs`). It finds `/tmp/.git` and `git` exits 128.
- The copy deliberately has no `.git`, so a stamp there is meaningless. `go test` mutants were unaffected, and the
  perf tools already pass the flag (`scripts/perf/refbuild.py:48`).

**Fix:** `-buildvcs=false` on both `go build` commands of `run_e2e`, plus a unit test in
`scripts/netsim/mutants_test.py` that asserts the flag (like `refbuild_test.py:280`). Smoke: `--check HP1` and one real
MN1 build. This is a light tooling block, committed to `main` first. No frozen gate changes.

## 5. Gate set

| Gate | Status | Contents | Cap sum |
|---|---|---|---|
| **HP1b** | new, supersedes HP1 (kept frozen as history) | the 8 criteria, verbatim except `mutant-tool`'s MH6 tuple (§3); the MH6 entry of `scripts/netsim/mutants.toml` stays in block `HP1`, so HP1m runs the pure version and HP1b's table checks it | 2340 s, as HP1 |
| HP1m | kept verbatim | MH1-MH22 per the table (MH6 now pure) | 1800 s |
| HP1r | kept verbatim | H-19 | 2190 s |
| **HP1c** | new | docs 30 (H-20; P-15 with its exemption) · tla `^HP1c` 1000, exclusive (18 runs: 3 sound, 2 safety, the 7 model mutants M31-M33 and Supersede's four caught, 6 witnesses reached) · pure + vectors 60, fuzz 30 (corpus digest frozen) · sim + 40-run acceptance, race, 600 · MH23, MP1-MP4: table 30, anchors 30, runs 440 · gremlins `protocol/syncack` 300 | 2520 s |
| **HP1c2** | new (2026-11-23), supersedes HP1c (kept frozen as history) | HP1c verbatim except the corpus (15 vectors: `supersede_dead_middle_acked_keeps_older`, review finding 3b) and its digest, and the TLA runs (20 ✓: mutant `HP1cSupersedeBackwardAck` caught by `FallbackKept` on three connections; `HP1cSupersedeThree`, the sound rule on three connections by seeded simulation; MS4 redefined as MP2, since `FallbackKept` now implies `NeverCloseSole`); raw results under `gates/HP1c2/` | 2520 s, as HP1c |
| HP1n, HP1n2, HP1n3 | kept verbatim | MN1/MN2 now build (§4) | as frozen |
| **HP1n4** | new | `e2e:build` 240 · scenario 9 variant `crash` (SIGKILL, new port), seeds 20771-3, strict, from bob's page readiness: C1c direct ≤ 15 s, C2c both ways converge ≤ 15 s, C3c alice never shows bob away (no spurious `PeerDown`), 1300 · MN3 (block `HP1n4`, `Supersede` decides nothing), P-15 off, turns C2c red (≥ 30 s): table 30, anchor 30, run 900. Needs `main`'s `a153829` (§4) | 2500 s |

Every new gate has `max_minutes = 45`. There is no netsim mutant for H-20: MH23 proves it on controlled time. Netsim
is used only for the environment hypothesis, that a real relay keeps the half-open circuit for about 30 s (MN3).

**Order.** (1) The §4 tooling block, committed to `main`. (2) Step 1: spec text, models with mutants, vectors, the
HP1c and HP1n4 tests (red for the right reason), cross-family review of spec and gates, freeze. (3) TDD to green.
(4) `/simplify`. (5) A 20-30 min smoke. (6) Each gate **once** on the same final candidate: HP1b → HP1m → HP1r →
HP1c2 → HP1n → HP1n2 → HP1n3 → HP1n4 (controlled time first, exclusive netsim last; no gate runs another). HP1c2
replaces HP1c in this order (2026-11-23, code review finding 3b, §8 below).

## 6. Out of scope (each with its block)

- HP1n's C1 is counted from page readiness, while its text says from restart → **PC1b**.
- HP1n3 C3a cannot place an outage inside a restart window → **PC1b**.
- H-12: DNS names arriving through identify → **TX1** (RC1).
- Who recreates `/tmp/.git` (2026-11-21 19:17:37) → **CI1b**. The repository does not show the writer: no script,
  hook or test creates a `.git` outside its own temporary directory (`selftest.sh:111` uses its own `mktemp -d`).
  The 19:15-19:16 neighbours are `arch-exec-check.sh`'s `tmp.*` directories and two empty `.tmp??????` directories
  (a Rust `tempfile` pattern), which points to a tool outside the repository. Next step: watch with
  `inotifywait -m /tmp -e create` or `auditctl -w /tmp/.git`. The §4 fix does not depend on it.
- Reporting the `isBetterConn` stream-count tie-break, and `holepuncher.go:208` not being bound to its relayed
  connection, upstream → **LP1**.
- Aligning the scope of `ClosePeer` with `PeerUpAck.tla` → **PU1d**, which reuses P-15's model changes.

Rejected:
- Global QUIC or yamux timeouts: every link would pay.
- Closing the older connection at once: this breaks H-15.
- An incarnation field: a wire change.
- Patching `bestConnToPeer`: the session and the inbound head would stay stuck.

## 7. Cross-family review (Gemini, 2026-11-23)

The verdict was « accept with changes ». R1, R2, the MH6 repoint and the gate plan were confirmed, and three
changes were applied:
1. [Major] P-15 must not close an older relayed connection while a newer one awaits its first sync ACK: that
   removes the fallback and causes a spurious `PeerDown`. Applied in the spec exemption, `Supersede` and its vector,
   `FallbackKept`/`IgnoreAckWait`, and clause C3c.
2. [Major] The model needs the positive liveness transition (an answered proof keeps the connection), so that safety
   is checked on live connections and crossed dials settle by H-15. Applied with `ProveAnswered`, S20 and
   `LiveKept`.
3. [Minor] A test and a Go mutant for an older connection that answers in time. Applied with
   `TestSupersedeKeepsLiveOlder`, MP3 and M32/`CloseOnTimer`.

## 8. Code review of step 2 (Gemini, 2026-11-23)

Verdict « reject ». Findings 1, 2 and 3a were real and are fixed in code and spec (§8.6, §8.7.4). Finding 4 was not a
defect on its own: churn is bounded by the rule, one proof per connection, a pending one never restarted. Table and
evidence: `gates/HP1/SIMPLIFY.md`, « HP1c step 2, cross-family code review ». Finding 3b is a defect of the rule
itself. The exemption was lifted by the ACK of any newer connection, so a dead acknowledged middle connection let the
oldest one close while the newest awaited its first ACK. The fix is in the rule: only a newer connection still judged
alive lifts it (acknowledged, its proof not silent; the newest is never judged). It is written as spec text, the 15th
vector `supersede_dead_middle_acked_keeps_older` (red on the step-1 decision: `close: got [1], want []`), the model
change (`NewerAcked` excludes a silent connection, the close checks it again, `FallbackKept` reads it when the close
runs) and mutant MS5 `HP1cSupersedeBackwardAck`, on AnyTiming's constants with three connections. The sound rule on three
connections is checked by a seeded TLC simulation (`HP1cSupersedeThree`), and MS4 is redefined as MP2. The
exhaustive check on three stays a residual → HP1: no exhaustive bound measured fits the budget. That changes frozen
content (corpus digest, run list), so `gates/HP1c2.toml` supersedes `gates/HP1c.toml` (method v2 rule 9). The pure
decision changes at the code step after the freeze.


## Gate freeze review (Gemini, 2026-11-23)

Gemini reviewed gates/HP1b.toml, HP1c.toml and HP1n4.toml: « freeze ». → HP1


## Second review round and HP1c2 freeze (Gemini, 2026-11-23)

The first code review rejected step 2 (a local proof failure read as peer silence; a proof state never cleared; a close executed after the newer connection was lost; an ACK from a dead middle connection lifting the fallback exemption). The second round verified the first three fixed at the root (Void = no verdict; Proving ends on every exit; the close re-decided against live connections before Close), the 3b spec sentence, the 15th vector (red on the unfixed pure code, as required) and the model (NewerAcked excludes a silent connection; mutant MS5 BackwardAck on three connections). Verdicts: code « accept », HP1c2 « freeze ». HP1c2 supersedes HP1c; MS4 redefined because FallbackKept implies NeverCloseSole under the close re-check; the exhaustive three-connection check is a residual, covered meanwhile by a seeded TLC simulation. → HP1


## HP1r2 freeze (Gemini, 2026-11-23)

MR1 (H-19's late-link guard) became equivalent on HP1r's frozen arms: H-20 cancels a bound initiator call when its circuit closes, so no late link can arise there (MR1 caught 0 times now, 5 with H-20's binding reverted, 0 with each other change reverted). The answerer's DCUtR dial runs on the service context, unbound, and can still land after a Drop. HP1r2 supersedes HP1r with one new closed arm, closed_answerer_dial, where MR1 must be caught with the same text. Gemini: equivalence correct, the arm exercises a real product path, nothing weakened, G-1 holds; « freeze ». Run order: HP1b → HP1m → HP1r2 → HP1c2 → HP1n → HP1n2 → HP1n3 → HP1n4. → HP1


## HP1n5 freeze (Gemini, 2026-11-23)

HP1n4 passed its netsim criterion but missed MN3 (P-15 disabled): C1c/C2c started their clock at tReady, taken after bob's page shows « Synchronisé » (a wait of up to 60 s), which absorbed the ~30 s spent on the dead direct connection (tReady − tRestart = 31.2 s under MN3 against 6.1–6.6 s unmutated). P-15 is needed in the real stack on this path; the clause was blind. HP1n5 supersedes HP1n4 with C1d/C2d counted from tRestart, « Synchronisé » inside the measure, 15 s each (unmutated 6.7–7.3 s, MN3 31.3–31.9 s); MN3 expected on C1d. Gemini: defect real, tRestart the right origin, margin ample, nothing weakened, G-1 holds; « freeze ». The same clock defect in HP1n's C1 is filed under PC1b. Run order ends … → HP1n3 → HP1n5. → HP1
