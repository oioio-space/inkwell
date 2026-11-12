# Modèles TLA+ (spec.md §5.1, §7.3, §10.7)

Deux modèles, chacun avec ce qu'il couvre **et ce qu'il ne couvre pas** (spec §7.3) :
`RepoGen` (générations, deux pairs, une session) et `ConnCredit` (un sens d'une connexion
QUIC, fenêtre de connexion partagée).

## RepoGen

`RepoGen.tla` modélise deux pairs, un flux REPO sortant par génération (génération dans le
préambule), des flux **non ordonnés entre eux**, des files d'entrée bornées, une livraison
asynchrone à la webview avec contrôle de génération (règle T), des pertes locales (règle L),
des demandes de la webview, des suspensions (Restreint), la perte du flux entrant
(`InboundLost`) ou sortant (`StreamLost`), et les règles I, B, A, R. Non couverts : plus de
deux pairs, le passage entre deux sessions (époques distinctes, argumenté), les crédits du pont
(la livraison n'attend que la place dans la file).

| Propriété | Sens |
|---|---|
| `NoViolation` | deux pairs honnêtes ne déclenchent jamais la règle A (h > gen + 1) ni un trou dans une génération |
| `GenGap` | `|gen_A − gen_B| ≤ 1` |
| `SyncedAgree` | deux côtés en Synchro ont la même génération |
| `QueueBound` | files bornées |
| `Prefix` | ce qui est livré dans une génération est un préfixe de ce qui y a été envoyé |
| `NoSilentLoss` | à l'arrêt en Synchro, tout ce qui a été envoyé a été livré |
| `Converges` (vivacité, équité faible) | avec un nombre fini de pertes et de suspensions, les deux côtés finissent en Synchro dans la même génération (ou à la borne explorée) |

## Résultats (TLC 1.8.0, `tla2tools.jar` SHA-256 `011267fca25d0752cf18894dd585cd28b96960c112ea8a20b823ea770e7e10cd`)

| Constantes | Résultat |
|---|---|
| `RepoGenSmall.cfg` : MaxGen 3, MaxMsg 1, QMax 1, MaxLoss 1, MaxRestrict 1 | **exhaustif, aucune erreur** : 123 905 états, 47 830 distincts, profondeur 32, vivacité comprise (révision 3, avec `InboundLost`) |
| `RepoGen.cfg` : MaxGen 4, MaxMsg 2, QMax 1, MaxLoss 2, MaxRestrict 1 | **non exhaustif** : plus de 39 millions d'états distincts (104 millions engendrés, profondeur ≥ 35) explorés en 25 min sur 4 cœurs sans aucune violation de sûreté ni de vivacité sur la partie explorée ; à finir sur la CI nocturne (porte P-TLA) |

Le modèle a trouvé deux défauts de la première rédaction, corrigés dans la spec :
1. réactiver plusieurs fois pendant que l'autre pair est Bas faisait monter `gen` sans
   confirmation (`GenGap` violé) → **règle B** : seule une génération confirmée est dépassée ;
2. un pair qui rouvre une génération non confirmée était pris pour un doublon (violation)
   → **règle A, ligne 3** : le flux rouvert remplace l'entrant, qui n'a porté aucune donnée.

Contre-épreuve (mutations, chacune détectée sur la petite configuration) : perte sans
changement de génération → `NoSilentLoss` violé ; livraison sans contrôle de génération →
`NoViolation` (trou) ; règle B retirée → `GenGap`.

## ConnCredit

Un sens d'une connexion QUIC : flux CONTROL, REPO (peut rester non lu), LARGE ; crédit de
connexion partagé `Wc = 3`, fenêtre de flux `Ws = 2` ; la webview peut s'arrêter **pour
toujours** ; deux changements de génération. **L'annulation d'un flux ne rend pas son crédit
tout de suite** (révision 4, revues 05-codex et 06-fable) : le crédit de l'ancien flux reste
pris (`pending`) jusqu'au RESET_STREAM du pair (`ResetArrives`, équité faible pour un pair
honnête) ; un seul flux annulé en attente ; un pair qui ne répond pas déclenche la fermeture
de la session (`Teardown`, 10 s). Seul le flux REPO est annulé : un flux LARGE entrant de
l'ancienne génération est lu jusqu'au bout et jeté (spec §5.1 c), il ne retient pas de crédit ;
`Switch` ne touche donc que REPO, par construction. Propriété : `ControlLive` (toute trame de contrôle voulue
finit par être lue, ou la session est fermée). Résultats (TLC exhaustif, 2026-11-01) :

| Variante | Résultat |
|---|---|
| règles de la spec (`PullLarge`, `Honest`, `TeardownRule` vrais) | aucune erreur (36 683 états, 10 882 distincts) |
| pair honnête **sans** règle de fermeture (`ConnCreditHonest.cfg`) | aucune erreur : le contrôle attend au plus le RESET_STREAM, jamais de fermeture nécessaire |
| pair qui n'envoie jamais RESET_STREAM, avec la règle de fermeture | aucune erreur : la session est fermée |
| pair qui n'envoie jamais RESET_STREAM, **sans** règle de fermeture | `ControlLive` violée : le crédit reste pris pour toujours |
| LARGE ouvert sans LARGE_REPLY (`PullLarge = FALSE`) | `ControlLive` violée : REPO et LARGE non lus prennent tout le crédit |

Non couverts : les valeurs réelles des fenêtres (règle « maximum de connexion ≥ maximum de flux
+ fenêtre initiale de flux + 1 Mio », vérifiée par un test au démarrage contre les versions
épinglées de go-libp2p et quic-go : 15 ≥ 10 + 0,5 + 1 Mio), la croissance automatique des
fenêtres, yamux (sans fenêtre de connexion, le cas facile).

## Lancer

```
java -cp tla2tools.jar tlc2.TLC -workers 4 -config RepoGenSmall.cfg RepoGen   # exhaustif, ~5 s
java -cp tla2tools.jar tlc2.TLC -workers 4 RepoGen                             # RepoGen.cfg, long
java -cp tla2tools.jar tlc2.TLC -workers 2 ConnCredit                          # exhaustif, ~1 s
```

`mise run tla` (ADR 0034, étape 3) récupère `tla2tools.jar` épinglé par SHA-256 et lance les
deux configurations ; c'est une porte avant la bascule.

## SyncIndicator (W33, honest sync indicator)

Rule: `docs/plans/2026-11-08-netsim-impaired-plan.md`, section "Honest sync indicator: executable
specification". When may the header of this device (`Me`) say « Synchronisé », per peer and
globally? `SyncIndicator.tla` models `Me` and its peers running the automerge-repo sync protocol
over one ordered stream per direction: a document is a causally closed set of changes; a sync
message carries the sender's heads and the changes the sender believes the receiver lacks, minus
those already sent (Automerge `sentHashes`, so a lost change is never resent until the sync state is
reset, plan R1); a change is applied only once its dependencies are present. The environment edits
(online or offline), loses messages (each loss owes spec §5.5 rule L: PeerDown then PeerUp, both
repos reset their sync state), drops and restores connections, and publishes the coalesced Links
notification (`pendingMsgs`, stale between samples). The indicator is a pure function of what the
webview observes: `up[p]` (peer-candidate / peer-disconnected), the heads of the last sync message
received from `p`, my own heads, and `snap[p]` (Links `pendingMsgs`). The properties are checked
against the ground truth (the peer's real document), which the indicator never reads.

Not covered: several documents (the rule is applied per document and joined per peer, see the plan),
peers that sync with each other directly (`Me` is the hub; a change of `p1` reaches `p2` through
`Me`), Automerge's bloom-filter `have`/`need` rounds (changes are sent at once), a reset that is not
atomic on both sides, Byzantine peers lying about their heads (it only hurts their own indicator),
a peer that loses its history (heads only grow), presence and other ephemeral messages.

| Property | Rule | Meaning |
|---|---|---|
| `PeerHonest` | W33 per peer | « Synchronisé » for `p` only if `p` is connected, really holds every change of mine, has been heard since the (re)connection, and every change it reported is in my document |
| `GlobalHonest` | W33 global | « Synchronisé » in the header only with at least one connected member, and then the above for every connected member |
| `TheirSound` | lemma | what a device heard another has, that one has |
| `EventuallySynced` (weak fairness on sending, receiving, rule L resync, reconnection, Links samples) | W33 liveness, spec §5.5 rule L | with finitely many edits, losses and outages, the header ends at « Synchronisé » and stays there |

| Configuration | Result |
|---|---|
| `SyncIndicator.cfg`: one peer, MaxEdits 2, MaxLoss 1, MaxDown 1, QMax 2 | **exhaustive, no error**: 469 897 states, 118 301 distinct, depth 23, liveness included, 11 s |
| `SyncIndicatorMesh.cfg`: two peers, MaxEdits 1, MaxLoss 0, MaxDown 1, QMax 1 | **exhaustive, no error**: 2 671 195 states, 530 696 distinct, depth 28, liveness included, 60 s (losses are covered by the one-peer configuration; with MaxLoss 1 it exceeds 1.1 million distinct states) |

Mutants (`mutants/SyncIndicatorMutants.tla`, each config overrides one definition of the sound
model, anchor = the override line in the config; the sound module is never edited):

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `SyncIndicatorPeerCount` | `PeerState <- PeerCountState` | today's `statusOf`: « Synchronisé » as soon as a person is connected | `PeerHonest` |
| `SyncIndicatorIgnoreRemote` | `PeerState <- IgnoreRemoteState` | the peer's reported heads are ignored (a known remote change is missing after a lost message) | `PeerHonest` |
| `SyncIndicatorSentIsAcked` | `PeerState <- SentIsAckedState` | what I sent counts as acknowledged (`lastSentHeads` instead of the peer's heads) | `PeerHonest` |
| `SyncIndicatorQueueOnly` | `PeerState <- QueueOnlyState` | my side read from the Links `pendingMsgs = 0` | `PeerHonest` |
| `SyncIndicatorEveryVacuous` | `Global <- EveryGlobal` | `peers.every(synced)`, true when nobody is connected | `GlobalHonest` |
| `SyncIndicatorAnyPeer` | `Global <- AnyGlobal` | `peers.some(synced)`: one synced member is enough (mesh config) | `GlobalHonest` |
| `SyncIndicatorNoRuleL` | `Resync <- NoResync` | a transport loss raises no PeerDown/PeerUp (sync/1 before RL1) | `EventuallySynced` |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config SyncIndicator.cfg SyncIndicator
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/SyncIndicatorPeerCount.cfg mutants/SyncIndicatorMutants
```

## WriteBudget (block WB, sync/1 write budget)

**Superseded by SendPath** (design W38, RC6): this model has no transport window and no reverse path, so the
sender sees progress at the reader's rate on every tick, which is not true on a relayed circuit. It stays as
the record of block WB; its rule WB-2 is checked again, on the windowed path, by the SP1 configurations of
`SendPath`.

Rule: `docs/plans/2026-11-08-netsim-impaired-plan.md`, section "Block WB: executable specification". It has two
parts, WB-1 and WB-2, and refines spec §5.5 rule L's « flux … sans progrès » for sync/1.

`WriteBudget.tla` models one large message sent to one peer: `mux.sendFragmented` over `frame.WriteProgress`.

- **Time** is a variable advanced by `Tick`, with upper-bound timers (Lamport, "Real Time is Really Simple"):
  `Tick` is disabled once a deadline has expired, so the cut happens before time can pass it.
- **Rule L:** a failed write loses the message, which is resent from its first byte.
- **The rule:**
  - WB-1: fail after `Idle` ticks without progress;
  - WB-2: fail once elapsed ≥ `Idle + S / Floor`, `Floor` being `FloorRate`.
- **Scale-free:** one tick = `WriteIdle / Idle`, and one unit = `Floor` × one tick.
- **An honest link** is a token bucket at `Floor` (slack `Burst ≤ Floor · Idle`) that never stalls for `Idle`
  ticks.
- **Any other reader** takes 0..`MaxRate` units per tick, and may stop for good:
  - `Stop` has no fairness, as ConnCredit's `Stall`;
  - weak fairness on `Tick` is fairness on time, never a promise that such a reader reads again.

Not covered:

- the interactive frames between fragments: the mux guarantees fragments one frame in four, and `FloorRate`
  accounts for it;
- the rule I cadence: it delays a resend, never prevents one;
- several peers: there is one sender per peer, each with its own budget;
- the v2 LARGE lane.

| Property | Rule | Meaning |
|---|---|---|
| `Delivered` (weak fairness on `Tick`, `Fail`, `Deliver`) | WB-2, §5.5 rule L | no resend loop: on a link with sustained progress ≥ `Floor`, every message is eventually delivered |
| `StopCut` | WB-1 | a reader that stops reading is cut within `Idle` ticks |
| `HoldBound` | WB-2 | whatever the reader does, one attempt holds the sender at most `HoldMax = Idle + S / Floor` |
| `CutEventually` | WB-1, WB-2 | whatever the reader does, every started attempt ends (delivered or cut) |

| Configuration | Result |
|---|---|
| `WriteBudget.cfg`: honest; S 8, Idle 2, Floor 1, MaxRate 4, Burst 1, MaxMsg 1 | **exhaustive, no error**: 295 states, 68 distinct, depth 10, liveness included, < 1 s |
| `WriteBudgetAny.cfg`: any reader, constants of `WriteBudget.cfg` | **exhaustive, no error**: 700 states, 290 distinct, depth 12, liveness (`CutEventually`) included, < 1 s |

Reachability witnesses were checked once, outside the task: an idle cut and a hold of exactly `HoldMax` are
reachable, so the properties are not vacuous.

Mutants (`mutants/WriteBudgetMutants.tla`, each config overrides one definition of the sound model, anchor = the
override line in the config; the sound module is never edited):

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `WriteBudgetFixedFloor` | `Limit <- FixedLimit` | today's `WriteIdle + S / MinRate`, with the link at half of `MinRate` (NI-3: 8 vs 16 KiB/s) | `Delivered` (fail, resend, fail: a lasso) |
| `WriteBudgetIdleOnly` | `Expired <- IdleOnlyExpired` | no total bound (nginx `send_timeout` style): a drip-feed holds the write | `HoldBound` |
| `WriteBudgetTotalOnly` | `Expired <- TotalOnlyExpired` | no idle bound: a reader that stops is cut only at the total deadline | `StopCut` |
| `WriteBudgetNoCut` | `Expired <- NeverExpired` | neither bound (a plain blocking write) | `CutEventually` |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config WriteBudget.cfg WriteBudget
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/WriteBudgetFixedFloor.cfg mutants/WriteBudgetMutants
```

## SendPath (W38, the send path end to end; rules SP-1..SP-9)

Rule: `docs/plans/2026-11-09-send-path-design.md`, §3.3 (SP-1..SP-8), §4, and the section "SP1: executable
specification" (rule SP-9, block SP1). One module checks two rule sets on the **same path**: the v2 target
(LARGE lane with receiver credit) and the v1 interim of block SP1 (sync/1). The rule set is chosen by constants.

**The path** (one unit = C = 8 KiB; one tick = C / the link rate, so the floor F = 2 KiB/s gives 4 s ticks and
NI-3's 8 KiB/s gives 1 s ticks):

- the object's **stream window** `Wt`, reopened by the reader's window updates in quanta `Uq` (yamux: `Wt/2`);
- the **session queue** (yamux `sendCh`): a `Write` returns once its frames are queued;
- the **hop** Alice → relay: the relay's receive window `H` on her circuit stream, reopened in quanta `Uh`;
- the relay's **forward buffer** (RC3's bufferbloat), then the **bottleneck queue** of `Qn` data units (netem),
  **shared by both directions**: Bob's window updates, credits and pongs wait in it behind Alice's data. Its
  delay when full, `Qn` ticks, is the contract's `D`;
- small frames (typed units, RST, announce, updates, credits, pings) cost no service time but keep their place;
- the **muxer's liveness timers**, each its own cut of the whole session (every stream, the typing lane
  included): the write valve (`ConnectionWriteTimeout`: the send loop blocked on the hop for `Tw`) and the
  keepalive (every `Ka`, a ping put at the head of the session queue, flushed and then answered, each within
  `Tk`; go-yamux uses `ConnectionWriteTimeout` for both). The keepalive starts in any phase;
- rule L (§5.5): a cut or a lost session changes the epoch and tells the webview; the next epoch announces the
  object again and the reader answers with its offset (SP-5; v1: 0);
- time: Lamport upper-bound timers, as in `WriteBudget`: `Tick` is disabled while an urgent step is enabled;
- the reader: honest (reads at once, tells the truth) or any (drips, stops for good with no fairness, forges one
  credit, states any offset up to what it holds);
- v1 only: the reader's partial-message timer (`partialTTL`, `Tpart`), and one silence of the bottleneck
  (`Blk`: NI-2's blackout plus QUIC's probe backoff).

**Not covered:** the receiver-side SP-4 timer (a writer that drips); several objects or lanes on one session
(bulk, REPO bursts: SP-3's hop margin must count every lane's credit); rate changes and cross traffic (the honest
link is the contract's worst case, rate exactly F, queue full to D; a faster link only brings credit sooner,
argued, not checked); yamux window auto-tuning (`Wt` is the window in effect; the grown case is a configuration);
QUIC's own loss recovery; Byzantine window updates; Bob's own keepalive (symmetric: his pong to Alice queues
behind her data); the rule I cadence; CONTROL (`ConnCredit`'s `ControlLive`, reused with its two mutants).

| Property | Rule (design §3.3, spec) | Meaning |
|---|---|---|
| `NoFalseCut` | SP-4 derivation, SP-6 | an honest link inside the contract is never cut |
| `NoSessionLoss` | SP-3 (hop), SP-9 | an honest link never loses its session to the muxer's write valve or keepalive |
| `WindowNeverBinds` | SP-3 | the object's `Write` is never blocked by the transport window |
| `InteractiveBound` | SP-7 | a typed unit reaches the reader within `Lmax` ticks |
| `StopCut` | SP-4 | a reader that stops is cut within `T_p` + `D` of its last report |
| `HoldBound` | SP-4 (v1: WB-2) | any reader holds the lane at most `(S/P + 1)·T_p` (v1: the WB-2 total) |
| `BoundedWaste` | SP-5, SP-6 | units sent ≤ `S` + (lost attempts) × `Win` |
| `CreditSane` | SP-8 | the sender never believes more was consumed than it sent |
| `NoSilentLoss` | §5.5 rule L | every lost attempt changed the epoch and told the webview |
| `NoSilentDrop` | §5.5 rule L (v1) | the reader never drops a partial message that its sender goes on writing |
| `DeliveredHonest` (weak fairness on time and every urgent step) | SP-6 | honest link ⇒ delivered, in one attempt per lost session |
| `CutEventually` (same fairness) | SP-4 | whatever the reader does, every open attempt ends |

**Rule SP-3, measured on the model.** In units, the stream never binds while `Win ≤ Wt − Uq + 1`, and the hop never
fills while `Win ≤ H − Uh + Qn` (the reader's updates lag its reads by less than one quantum; the bottleneck holds
`Qn` units downstream of the relay). The design's `(1 − θ)·Wt` is the byte form, with one byte of slack instead of
one unit; `SendPathCapAboveWindow` uses the first binding value in units, 5.

| Configuration (sound) | Constants | Result (TLC 1.8.0, 2 workers) |
|---|---|---|
| `SendPathHonest.cfg` | rules SP-1..SP-9; S 8, Wt 6 / Uq 3, hop 8 / 2, Qn 2, Win 2, P 4, T_p 10, Tk 10, Ka 7, one loss, one typed unit | **exhaustive, no error**: 233 153 states, 89 858 distinct, depth 78, liveness included, 04s |
| `SendPathHop.cfg` | as above, hop 2 / 1 (smaller than the window), Win = Wmax = 3 | **exhaustive, no error**: 709 393 states, 259 015 distinct, depth 81, liveness included, 12s |
| `SendPathAny.cfg` | any reader; S 4, Wt 4 / Uq 2, Qn 1, Win 2, P 2, T_p 4, one forged credit, keepalive off | **exhaustive, no error**: 227 022 states, 97 031 distinct, depth 76, liveness included, 04s |

Smallest safe values, found by bisection on the sound configurations (the value below each one fails): `T_p`
6 ticks (24 s at the floor; the design's 40 s keeps 16 s of margin); ping timeout 3 ticks with Win 2 and 5 ticks
with Win 3 (12 s and 20 s: today's 10 s is too short even on v2, hence SP-9).

Mutants (`mutants/SendPathMutants.tla` holds the overrides; the other mutants change rule-set constants; anchor =
the changed config line):

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `SendPathWriteReturn` | `Measure = "write"` (with `Credit = FALSE`, `Tp = 4`) | progress = « the `Write` returned », idle below `U/R` + delay: scenario 7 | `NoFalseCut` |
| `SendPathFixedFloor` | `Total = 6` | a size-dependent total timer above the link's rate | `DeliveredHonest` |
| `SendPathNoCap` | `Win = 6` | no cap on the credit window | `InteractiveBound` |
| `SendPathCapAboveWindow` | `Win = 5` | `Wmax` above `(1 − θ)·Wt` | `WindowNeverBinds` |
| `SendPathSharedWriter` | `Shared = TRUE` (with v1 progress, no credit) | v1's single stream: the typed unit waits for the bulk `Write` | `InteractiveBound` |
| `SendPathFromZero` | `Resume = FALSE` | the reader's offset ignored (no SP-5) | `BoundedWaste` |
| `SendPathStackedWindows` | `Win = 4` (hop configuration) | the credit sized against the stream window only: the hop below it binds by accident | `NoSessionLoss` (write valve) |
| `SendPathMuxWriteTimeout` | `Tw = 2` (no credit, window 8 over a hop of 4) | today's 10 s write valve on a stream paced by its transport window (E1) | `NoSessionLoss` (write valve) |
| `SendPathKeepaliveBehindData` | `Tk = 2` (hop configuration) | today's 10 s ping timeouts: the ping queues behind data (E1b) | `NoSessionLoss` (keepalive) |
| `SendPathSilentAbandon` | `RuleL <- NoRuleL` | a cut without rule L | `NoSilentLoss` |
| `SendPathNoCut` | `Expired <- NeverExpired` | no progress timer | `StopCut` |
| `SendPathNoCutLive` | `Expired <- NeverExpired` | the same, liveness form | `CutEventually` |
| `SendPathZeroOnly` | `Quantum <- UnitQuantum` | the timer re-armed by every unit | `HoldBound` |
| `SendPathTrustCredit` | `CreditOK <- TrustAll` | a credit beyond what was sent is believed | `CreditSane` |
| (`ControlLive`) | the `ConnCredit` mutants | CONTROL waits behind data | `ControlLive` |

## SP1 (block SP1: the v1 sync/1 interim, on the SendPath model)

Rule: the design doc's sections "SP1: executable specification" and "SP1 decisions". sync/1 as it is (no credit,
progress = a 32 KiB fragment `Write` returned, one stream, resend from byte 0), with the decided constants:
`WriteIdle` 90 s, the WB-2 total `WriteIdle + S / 2 KiB/s`, `partialTTL` 120 s, the yamux stream window pinned at
256 KiB (`Wt` = 32, update 128 KiB), and the muxer's `ConnectionWriteTimeout` stated at 120 s, keepalive every 30 s.
The 879 KiB paste is `S` = 110 units. The relay hop is a **hypothesis fitted to E1** (128 KiB, reopened in 64 KiB
quanta: 10.7 s at 6 KiB/s against E1's 10.0 s block before the session closed). Rounding is conservative: timers
down, delays up.

How the muxer's one constant becomes model timers (go-yamux v5.1.0 `session.go`):

- the **write valve** `Tw` is the earliest firing, `ConnectionWriteTimeout` / 2 = 60 s of blocking on one frame
  (`sendLoop` extends the conn deadline to now + 120 s only once less than 60 s remains, so a blocked frame write
  fails after 60 to 120 s): 60 ticks at 8 KiB/s, 45 at 6 KiB/s, 15 at the floor;
- the **ping timeouts** `Tk` get the full 120 s each (`Ping`: one timer for the flush, reset for the answer): 120,
  90 and 30 ticks; the keepalive interval `Ka` 30 s is rounded down (28 s at the floor).

| Configuration (sound) | Link | Result (TLC 1.8.0, 2 workers) |
|---|---|---|
| `SP1Relay.cfg` | NI-3: 8 KiB/s, queue 6 s (E1's first-stall excess 5.8 s), one typed unit, `Lmax` 36 s | **exhaustive, no error**: 1 100 747 states, 410 949 distinct, depth 590, liveness included, 22s |
| `SP1Floor.cfg` | F = 2 KiB/s, D = 5 s (queue 8 s), `Lmax` 36 ticks (144 s) | **exhaustive, no error**: 286 975 states, 140 354 distinct, depth 590, liveness included, 06s |
| `SP1RelayKeepalive.cfg` | SP1Relay with the keepalive (30 s, any phase) | **exhaustive, no error**: 1 716 522 states, 620 782 distinct, depth 600, liveness included, 35s |
| `SP1FloorKeepalive.cfg` | SP1Floor with the keepalive (28 s) | **exhaustive, no error**: 221 814 states, 95 344 distinct, depth 614, liveness included, 04s |
| `SP1Hop.cfg` | 6 KiB/s, window 256 KiB, hop 128 KiB, keepalive on | **exhaustive, no error**: 607 469 states, 251 064 distinct, depth 604, liveness included, 12s |
| `SP1Grown90.cfg` | 6 KiB/s, stream window grown to 512 KiB, hop 512 KiB, keepalive off (a margin beyond the pin) | **exhaustive, no error**: 56 099 states, 20 495 distinct, depth 582, liveness included, 01s |
| `SP1GrownKeepalive.cfg` | SP1Grown90 with the keepalive (a residual at 90 s, closed by 120 s) | **exhaustive, no error**: 4 150 710 states, 1 460 463 distinct, depth 596, liveness included, 01min 34s |
| `SP1Blackout.cfg` | SP1Relay plus one 50 s silence during the paste (keepalive off: NI-2 is direct QUIC, no yamux) | **exhaustive, no error**: 134 560 states, 45 191 distinct, depth 636, liveness included, 03s |
| `SP1Silence.cfg` | one silence as long as `WriteIdle` (90 s), no write valve (direct QUIC), `NoSilentDrop` and `CutEventually` only (such a silence may cut the sender) | **exhaustive, no error**: 129 574 states, 42 745 distinct, depth 676, liveness included, 03s |
| `SP1Any.cfg` | any reader, small sizes (S 6, Wt 2, idle 3, total 9) | **exhaustive, no error**: 17 854 states, 9 486 distinct, depth 88, liveness included, 00s |

**What SP1 guarantees (proved on these configurations):**

- an honest link at 8 KiB/s or at the 2 KiB/s floor is never cut by `WriteIdle` and delivers the paste in one
  attempt: no resend loop (NI-3 C1, and C3's « resyncs 0 ») (`NoFalseCut`, `DeliveredHonest`; SP1-1, SP1-2);
- the session survives the muxer's timers at 120 s: at 8 KiB/s, at 6 KiB/s through the E1 hop, at the floor through
  a 128 KiB hop (`NoSessionLoss`; SP1-4, SP1-5);
- a 50 s silence (NI-2) during the paste neither cuts it nor makes the reader drop it, and no silence as long as
  `WriteIdle` makes the reader drop a message its sender still writes (`NoSilentDrop`; SP1-3);
- a stopped reader is cut within `WriteIdle` + D, a drip within the WB-2 total (`StopCut`, `HoldBound`,
  `CutEventually`; SP1-1, SP1-2).

Smallest safe values found by bisection (invariants only; the value below each one fails): `WriteIdle` 22 s at
8 KiB/s (E1's first stall is 21.8 s) and 76 s at the floor, so 90 s keeps 14 s of margin there; the write valve 9 s
of blocking at 8 KiB/s and 36 s at the floor (so `ConnectionWriteTimeout` ≥ 72 s; 120 s gives a valve of 60 s at
its earliest); the ping timeouts 28 s at 8 KiB/s and 80 s at the floor (120 s); `partialTTL` 91 s against a 90 s
silence (`SP1Silence`). The model has no reverse propagation delay, so the last bound has no margin: the design keeps
`WriteIdle` + 2D + 10 s = 120 s.

**What SP1 does not guarantee (residual witnesses, which must violate):**

- typing latency on the relay: the path alone holds a typed frame up to **36 s at 8 KiB/s** (smallest bound by
  bisection; 144 s at the floor): it waits for the blocked fragment `Write`, then behind a stream window in the
  relay. NI-3 C2 (10 s) stays a declared known failure until ADR 0034 step 6 (`SP1TypingResidual`);
- the keepalive when the relay buffers a whole 256 KiB window at the floor: the ping then needs 140 s (35 ticks;
  34 fail), above the decided 120 s (`SP1FloorKeepaliveBigHop`). E1b did not measure a relay buffering that much,
  and did not run at 2 KiB/s; the gate's NI-3 runs at 8 KiB/s;
- resent bytes: a lost session still resends the whole message (v1 has no SP-5);
- a silence of 60 s or more while the session's send loop is blocked closes the session (the valve at its
  earliest): that is outside the contract (D = 5 s) and goes through rule L, never silently.

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `SP1Idle20` | `Tp = 20` | `WriteIdle` back to 20 s: scenario 7's false cut (first stall 16 + 5 s) | `NoFalseCut` |
| `SP1FixedFloor` | `Total = 36` | the WB-2 total at the old 16 KiB/s | `DeliveredHonest` |
| `SP1FloorGrown` | `Wt = 64` | window auto-tuning (512 KiB): the quantum doubles and 90 s is short at the floor | `NoFalseCut` |
| `SP1HopValve10` | `Tw = 7` | yamux 10 s: today's write valve, hop 128 KiB at 6 KiB/s (E1's session close) | `NoSessionLoss` |
| `SP1KeepaliveBehindData` | `Tk = 10` | yamux 10 s: today's ping timeouts at 8 KiB/s (E1b's keepalive timeout) | `NoSessionLoss` |
| `SP1PartialTTL30` | `Tpart = 30` | today's `partialTTL` under `WriteIdle` 90 s: a silent loss after a 50 s silence | `NoSilentDrop` |
| `SP1PartialTTLAtIdle` | `Tpart = 90` | `partialTTL` = `WriteIdle`: the reader's timer and the sender's re-arming meet at the end of a 90 s silence | `NoSilentDrop` |
| `SP1NoTotal` | `TotalBound <- NoTotal` | no WB-2 total | `HoldBound` |
| `SP1NoCut` | `Expired <- NeverExpired` | neither bound | `StopCut` |
| `SP1TypingResidual` | `Lmax = 10` | residual witness: NI-3 C2's 10 s | `InteractiveBound` |
| `SP1FloorKeepaliveBigHop` | `H = 64` | residual witness: a relay that buffers a whole window | `NoSessionLoss` |

The whole `mise run formal` (TLA+: every model of this directory, 62 runs; then Alloy) took 320 s on 2 workers
(2026-11-09, after the SP1 decisions); every sound configuration above held and every mutant and residual witness
was caught.

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config SendPathHonest.cfg SendPath
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/SP1Idle20.cfg mutants/SendPathMutants
```

## LinkProbe (spec §8.5, ADR 0036: `/inkwell/linkprobe/1.0.0`)

`LinkProbe.tla` models one direction of one QUIC connection over several probe streams: the prober P sends data
packets and PROBEs on its stream; the responder R counts **every** packet of P it receives (the PROBE's own
included), answers PROBEs from a credit kept **per connection**, bounds the PROBE rate by the stream's age, holds
at most one inbound stream and refuses a new one for `Gap` (B/2) after any end and `B` after a violation, and may
accept a stream up to `AccDelay` ticks after P opened it (PROBEs above the rate bound by at most `DropMax` are
then dropped, not a fault: review 3, F6); P keeps
outstanding PROBEs and accepted samples and computes the figure from two samples at least `W/2` apart. Each rule
of §8.5.5 and §8.5.6 is a named operator, so that a mutant overrides exactly one (16 properties, 19 mutants,
4 configurations; revised for reviews 2 and 3 of the probe). An honest configuration needs `MaxOut` above the round
trip in intervals (`O = 8` against `PTO / I` in the spec): with `MaxOut = 1`, TLC finds echoes that always
arrive after the next PROBE and a figure that never becomes known.

- **Time** is ages in ticks (one tick = E = 500 ms, the cost of one echo and the fastest PROBE rate). Each age is
  capped where nothing distinguishes larger values (an in-flight item at `D`, an outstanding PROBE at `PTO + 1`,
  a sample at `2W + 1`, a stream's age at `MaxProbes + 2`, the elapsed time at `MaxStreams × MaxProbes`), so the
  state space is finite while time is unbounded. `Tick` is disabled while the prober's timer is due (a PROBE per
  interval, a stream as soon as it may open, a reset to see) and, on an honest network out of a stall, while an
  item has been in flight `D` ticks (Lamport's upper-bound timer).
- **Honest = TRUE:** R answers truthfully from its credit (refilled one echo per tick, at most `CreditMax`),
  drops a PROBE without credit and without fault, resets the stream on a seq not increasing or above the rate
  bound; data packets may be lost or overtake PROBEs (a ghost records each crossing); the network may **stall**
  (holds everything, then delivers in a burst); the transport may end a stream; the connection may restart
  (bounded by `MaxRestarts`).
- **Greedy = TRUE:** the prober ignores the spacing and the backoff, reopens at once and opens a second stream
  while one is open (a flooding peer, honest otherwise): the credit, the rate bound, the single inbound stream
  and the refusal must still hold (`CreditMax = 1`, so that the credit, not the rate bound, bounds the echoes;
  `DropMax = 0`, so that three PROBEs at once exceed the bound).
- **Honest = FALSE:** R is Byzantine: for each PROBE it answers the right seq, the one before or the one after,
  with a count of 0, the truth or more than was ever sent, or does not answer; the network has no deadline.
- **Streams:** a stream ends by a violation either side detects (P's samples cleared on its own), by a reset R
  raised, or by the transport (samples kept); P reopens after `B` ticks; whatever was in flight on an ended stream
  has no effect when it arrives (§8.5.3).

Not covered (corpus `linkprobe.json` instead): the negotiation and `na`, the transport and role table, the
one-message writer, ignorable messages and their budget, bytes read in chunks, the 10 s negotiation and closing
timeouts, the 1024 bytes read after the FIN, the 1/64 clock margin of the rate bound (it only widens the bound), the level of §8.5.5 step 6 (pure arithmetic: vectors and
`TestStepBoundaries`). The size of the figure's error under crossings is statistical (`scripts/netsim/c5model.py
errors`, ADR 0036 « Levels »); the model proves its zero point (`Exact`).

| Property | Rule | Meaning |
|---|---|---|
| `Bounded` | §8.5.7 | at most `MaxOut` outstanding PROBEs and `MaxSamples` samples |
| `SampleBounded` | §8.5.5 step 3 | no accepted count exceeds what P sent, whatever R says |
| `NoLateSample` | §8.5.5 step 3 | no ECHO more than one PTO after its PROBE becomes a sample |
| `Monotone` | §8.5.5 step 3 | samples never go backwards in sent or received counts |
| `CurrentConnection` | §8.5.5, restart | every sample belongs to the current connection |
| `FigureInRange` | §8.5.5 step 5 | a known figure is in 0..1000 ‰, against a hostile responder with data packets (Any) and when a data packet overtakes a PROBE (Honest) |
| `SampleMatches` | §8.5.3, post-end rule | (honest) a sample pairs an ECHO with the very PROBE it answers, never one of an ended stream |
| `HonestNoViolation` | §8.5.6 steps 1, 3, 4, 5, §8.5.8 | an honest responder and an honest prober never raise a violation and the prober is never refused, a stall and its burst included, and an acceptance delay (`LinkProbeAccept`) |
| `EchoBound` | §8.5.6 step 3, §8.5.10 | (honest responder, flooding prober included) at most `CreditMax + t` echoes in `t` ticks on a connection, across its streams |
| `Exact` | §8.5.5 « Exactitude », §8.5.6 | (honest) with no loss and no packet crossing a PROBE (delivered or still in flight), a known figure is exactly 0 |
| `OneInbound` | §8.5.6 step 1 | (any responder, flooding prober) R never holds two inbound streams |
| `BanHolds` | §8.5.6 steps 1, 5 | R never accepts a stream while it is refusing |
| `RateBound` | §8.5.6 step 3 | (honest) R never reads a seq more than `DropMax` above `age / E + 2` on its stream (a flood is cut) |
| `Live` (weak fairness on opening, sending, delivery, `Tick`, seeing a reset, the end of a stall) | §8.5.5 | (honest) once no stall, stream end or restart remains and a fresh stream is open, the figure becomes known |
| `Reopens` | §8.5.5 step 7 | (honest) a stream that ended reopens |
| `NeverBlocked` | §8.5.5 step 2 | (any responder) an open stream sends all its PROBEs or ends: P never waits on R |

| Configuration | Result |
|---|---|
| `LinkProbeHonest.cfg`: honest; 3 PROBEs per stream, 2 streams, 1 data packet, MaxOut 2, MaxSamples 2, PTO 2, W 3, NMin 1, D 1, B 1, Gap 1, AccDelay 0, DropMax 1, CreditMax 2, 1 stall, 1 transport end, no restart (checked in the Byzantine configuration) | **exhaustive, no error**: 3 605 582 states, 1 677 688 distinct, depth 40, liveness (`Live`, `Reopens`) included, 95 s (2 workers) |
| `LinkProbeGreedy.cfg`: honest responder, **flooding prober**; 3 PROBEs per stream, 2 streams, no data, CreditMax 1, DropMax 0, 1 transport end | **exhaustive, no error**: 8 127 states, 4 588 distinct, depth 22, `NeverBlocked` included, 1 s |
| `LinkProbeAny.cfg`: Byzantine responder, untimed network; 2 PROBEs per stream, 2 streams, 1 data packet, MaxOut 1, NMin 2, 1 restart, no stall and no transport end (checked in the honest configuration) | **exhaustive, no error**: 1 997 816 states, 546 786 distinct, depth 27, liveness (`NeverBlocked`) included, 26 s |
| `LinkProbeAccept.cfg`: honest; the responder accepts the stream up to 2 ticks after the prober opened it (AccDelay 2, DropMax 1); 4 PROBEs, 1 stream, no data, no stall (`Live` is checked by the honest configuration: here all 4 PROBEs may go to the delay) | **exhaustive, no error**: 1 530 states, 1 024 distinct, depth 24, `NeverBlocked` included, 1 s |

Mutants (`mutants/LinkProbeMutants.tla`, each config overrides one rule of the sound model with the constants of
the configuration named, anchor = the override line in the config):

| Mutant | Anchor (config line) | Broken rule | Configuration | Caught by |
|---|---|---|---|---|
| `LinkProbeAcceptLate` | `LateOK <- AcceptLate` | no late rule: the count of a held PROBE is taken | Any | `NoLateSample` |
| `LinkProbeZeroPTO` | `LateOK <- NeverOnTime` | a late bound too small for the path: every echo is late | Honest | `Live` |
| `LinkProbeNoSentBound` | `SentOK <- NoSentBound` | a forged count above what was sent is taken | Any | `SampleBounded` |
| `LinkProbeStrictSentBound` | `SentOK <- StrictSentBound` | `r < sent` instead of `r ≤ sent`: a peer that received everything is a violation | Honest | `HonestNoViolation` |
| `LinkProbeNoMonotone` | `MonoOK <- NoMonotone` | a forged decreasing count is taken | Any | `Monotone` |
| `LinkProbeNoClamp` | `Clamp <- NoClamp` | the loss without `min(ΔR, ΔS)`: negative when a data packet overtakes a PROBE | Honest | `FigureInRange` |
| `LinkProbeNoPendingBound` | `KeepPending <- NoPendingBound` | outstanding PROBEs never forgotten | Any | `Bounded` |
| `LinkProbeKeepAcrossRestart` | `RestartSamples <- KeepAcrossRestart` | samples survive a new connection (state keyed by peer, not by connection) | Any | `CurrentConnection` |
| `LinkProbeNoReopen` | `MayOpen <- OpenOnce` | a stream that ended never reopens | Honest | `Reopens` |
| `LinkProbeStopAndWait` | `MayProbe <- StopAndWait` | a PROBE waits for the ECHO of the previous one: a silent responder stops the prober | Any | `NeverBlocked` |
| `LinkProbeNoStreamCheck` | `SameStream <- AnyStream` | an ECHO of an ended stream is taken against the new stream's PROBE | Honest | `SampleMatches` |
| `LinkProbeFloodLimit` | `DropFault <- AnyDropFault` | a dropped PROBE is a fault (review 1, finding 4): a stall's burst resets the stream | Honest | `HonestNoViolation` |
| `LinkProbeNoCredit` | `CreditOK <- UnlimitedCredit` | no credit: every PROBE is echoed (replaces review 1's per-stream bucket, equivalent once the refusal holds) | Greedy | `EchoBound` |
| `LinkProbeNoRefusal` | `Accepts <- NoRefusal` | a new stream is accepted during the refusal (review 2) | Greedy | `BanHolds` |
| `LinkProbeAcceptSecond` | `Accepts <- AnyCount` | a second stream is accepted while one is open (review 2) | Greedy | `OneInbound` |
| `LinkProbeLongRefusal` | `EndRefusal <- LongRefusal` | the refusal after an ordinary end outlasts the prober's backoff: an honest prober is refused (review 2) | Honest | `HonestNoViolation` |
| `LinkProbeNoSeqBound` | `SeqOK <- NoSeqBound` | no bound on the PROBE rate: a flood is processed (review 2, finding 1) | Greedy | `RateBound` |
| `LinkProbeDataOnly` | `Counted <- DataOnly` | the responder counts data packets only, not the PROBE's own packet (review 2) | Honest | `Exact` |
| `LinkProbeNoDropBand` | `SeqOK <- NoDropBand` | every PROBE above the rate bound is a fault: an honest prober behind a slow accept is reset (review 3, F6) | Accept | `HonestNoViolation` |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config LinkProbeHonest.cfg LinkProbe
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/LinkProbeFloodLimit.cfg mutants/LinkProbeMutants
```

## OISingleInstance (W41, block OI, ADR 0037: one Inkwell per profile and the hand-off)

`OISingleInstance.tla` models several launches of Inkwell on one profile folder (not a peer protocol: a local
interface, kept here because `scripts/tla-check.sh`, `mise run tla`, `formal` and `done` check this directory).
Each launch is a process: it tries the profile lock (`flock` or `LockFileEx`: released by the kernel when its holder
dies); the holder (primary) removes any leftover socket file, binds, then opens the identity, serves, and on closing
answers « closing » until it closes the identity and exits; a launch that does not get the lock dials the socket,
waits for the acknowledgement, and exits 0, or backs off and tries the lock again. Faults: `Crash` (any process, any
time, bounded by `MaxCrashes`), `Hang` (a primary stops answering and keeps the lock). Time is abstract: a per-try
timeout is always allowed while no reply came; giving up (the 5 s deadline) is allowed only while the lock holder is
hung. The initial socket file is absent or stale (a crash of a previous run).

Fairness: three terms per launch, none on `CloseStart` (the person decides), `Hang` or `Crash` (faults). Each is an
assumption on the real system, and each strong term is shown necessary (weakened to `WF`, the sound model violates
`Settles`, measured 2026-11-12):

| Term | Assumes | Why this strength |
|---|---|---|
| A1 `WF_vars(Step(p))`, `Step` = the launch's own steps (`TryLock` … `Backoff`) | the OS keeps running every live process that is not hung | the steps are enabled in disjoint control states and stay enabled until the launch moves, so weak fairness on the disjunction is weak fairness on each step |
| A2 `SF_vars(Answered(p))`, `Answered(q)` = `\E p : Accept(p, q) \/ ReplyClosing(p, q)` | a primary that keeps answering answers a client that keeps reconnecting | the client's per-try `Timeout` disables the answer between tries; with `WF`, a launch times out forever against a serving primary |
| A3 `SF_vars(GiveUp(p))` | a client whose retries keep meeting a hung lock holder reaches its 5 s deadline | each retry (`Backoff`) disables it; with `WF`, a launch retries forever against a hung holder (`OISingleInstanceHang.cfg`) |

A2 is implied by strong fairness on every `Accept(p, q)` and `ReplyClosing(p, q)` (SF(A) /\ SF(B) implies SF(A \/ B)),
so the grouped form admits more behaviours and proves at least as much. Why grouped (measured): TLC puts
`Fairness /\ ~Property` in disjunctive normal form, and each SF term (`<>[]~ENABLED A \/ []<>A`) doubles it. The first
draft had one SF per pair and per `GiveUp`, 21 SF terms for 3 launches, i.e. 2^21 disjuncts per branch: TLC ran out
of heap (2–6 GB) in `LNConj.toDNF` (`Liveness.processLiveness`) before exploring a state; with 2 launches (10 SF
terms) it finished in 2 s. Six SF terms now (2^6). No `SYMMETRY` (unsound with liveness in TLC).

Not covered (named tests instead, plan section 9): the wire format and its decoder (`FuzzDecodeRequest`, vectors),
the socket path rule and its fallback (`TestSocketPathFallback`), peer credentials (`TestHandoffPeerCheck`), the
bounds (queue, connections, sizes), the 15 s closing budget as a duration, what the window does with a request
(`internal/launch`).

| Property | Rule | Meaning |
|---|---|---|
| `MutualExclusion` (S1) | D-LOCK, D-ORDER | at most one process has the identity open (the defect this block removes) |
| `IdentityNeedsLock` (S2) | D-ORDER | the identity is open only by the lock holder |
| `OnlyHolderListens` (S3) | D-UNLINK | a live socket belongs to the lock holder |
| `ExitOkImpliesQueued` (S4) | D-ACK | a launch that exits 0 was queued by a primary: no silent loss |
| `GaveUpJustified` (S5) | D-CLOSING | a launch gives up only on a hung holder, never on « closing » |
| `Settles` (L1) | D-KERNEL, D-RETRY | every launch eventually serves, hands off, gives up on a hung holder, or crashed |
| `AckedApplied` (L2) | D-ACK, D-DRAIN | a queued request is applied unless its primary stopped serving |

| Configuration | Result |
|---|---|
| `OISingleInstance.cfg`: 3 launches, 1 crash, no hang | no error, liveness included: 28,022 states generated, 11,832 distinct, depth 32, 12 temporal branches over 141,984 behaviour-graph nodes; 14 s (2 workers, 2 GB heap, 2026-11-12) |
| `OISingleInstanceHang.cfg`: as above, a primary may hang | no error, liveness included: 35,702 states generated, 15,234 distinct, depth 32, 12 temporal branches over 182,808 behaviour-graph nodes; 18 s |

Every mutant below is caught by its listed property and every witness is violated (the 16 OI runs of
`scripts/tla-check.sh` take 56 s); the `AckedApplied` counterexample of NoDrain ends with a primary serving forever
with an acknowledged request queued and never applied; the three `Settles` counterexamples end in a fair cycle: launches retrying against a
lock held by a crashed process (CrashKeepsLock), a launch redialling a stale socket with the lock free (NoRetryLock),
a launch looping on an absent socket while the primary it unlinked serves (SecondUnlinks).

Mutants (`mutants/OISingleInstanceMutants.tla`, each config overrides one rule of the sound model with the constants of
`OISingleInstance.cfg`, anchor = the override line in the config) and witnesses (anti-vacuity: the sound model must
violate them):

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `OISingleInstanceNoLock` | `MayLead <- NoLockLead` | primary = « no socket answers » (the socket-only design) | `MutualExclusion` |
| `OISingleInstanceEarlyIdentity` | `IdentityAtStart <- EarlyIdentity` | identity opened before the lock (today's order) | `MutualExclusion` |
| `OISingleInstanceAckNoQueue` | `Enqueue <- AckNoQueue` | acknowledged before queueing | `ExitOkImpliesQueued` |
| `OISingleInstanceFireAndForget` | `WaitsForReply <- FireAndForget` | exit 0 after sending, no acknowledgement (Wails macOS) | `ExitOkImpliesQueued` |
| `OISingleInstanceLockReleasedEarly` | `ReleaseOnClose <- ReleaseEarly` | lock released when closing starts | `MutualExclusion` |
| `OISingleInstanceCrashKeepsLock` | `ReleaseOnCrash <- KeepLockOnCrash` | pid file or `O_EXCL` file left by a crash | `Settles` |
| `OISingleInstanceGiveUpOnClosing` | `GiveUpOnClosing <- GiveUpClosing` | « closing » read as « already running » | `GaveUpJustified` |
| `OISingleInstanceSecondUnlinks` | `UnlinksStale <- SecondUnlinksStale` | a second launch removes a « stale » socket (it may be the new primary's) | `Settles` |
| `OISingleInstanceNoRetryLock` | `AfterBackoff <- DialAgain` | retries reconnect without retrying the lock | `Settles` |
| `OISingleInstanceNoDrain` | `ToApply <- NoDrain` | requests acknowledged and queued, never read by the window (D-DRAIN) | `AckedApplied` |
| `OISingleInstanceWitnessHandoff` | invariant `NoHandoff` | witness: some launch exits 0 | (must violate) |
| `OISingleInstanceWitnessGiveUp` | invariant `NoGiveUp`, `Hangs = TRUE` | witness: some launch gives up on a hung holder | (must violate) |
| `OISingleInstanceWitnessTakeover` | invariant `NoTakeover` | witness: a launch serves after a primary that served crashed | (must violate) |
| `OISingleInstanceWitnessHandover` | invariant `NoHandover` | witness: a launch serves after a primary closed normally | (must violate) |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config OISingleInstance.cfg OISingleInstance
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/OISingleInstanceNoLock.cfg mutants/OISingleInstanceMutants
```
