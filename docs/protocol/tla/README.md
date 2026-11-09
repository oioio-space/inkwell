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
