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
timeouts, the 1024 bytes read after the FIN, the 1/64 clock margin of the rate bound (it only widens the bound), the responder's stack discarding a late packet (it is `LoseData`: never counted, spec §8.5.2), the level of §8.5.5 step 6 (pure arithmetic: vectors and
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
waits for the acknowledgement, and exits 0, or backs off and tries the lock again. A request is acknowledged once a
primary ready to show it (identity, project and network up) has received and queued it; from then on the primary owns
the outcome (contract simplified 2026-11-12 after three review rounds; a crash of the primary in between loses the
request: accepted residual). The open stream between Go and the window has one owner at a time, by generation
(rule D-OWNER, 2026-11-13): the window connects (`Connect`), may connect again (`Reconnect`, not fair: a new
generation supersedes the old one, whose undelivered work goes back to Go's queue, and the window starts afresh), and
the superseded stream's handler returns later (`EndStream`: it gives back only what it held). Go sends a queued
request to the current window (`Send`) and keeps it until the window reports it handled (`Apply`: shown, or its
toast up, reported with its generation; `Deliver`: Go accepts a report only from the generation holding the request).
Closing stops the acknowledgements, the window keeps handling during « closing », and the process closes once Go's
queue is empty, or once the bounded wait (2 s) gave up on a window that cannot answer (`Stuck`, a fault: the
residual). `MaxGen` bounds the connections of one primary: 1 in the configurations with a crash, 2 (one
reconnection) in `OISingleInstanceReconnect.cfg` and the stream mutants. A failed start is a `Crash` here: nothing was
queued. Faults: `Crash` (any process, any
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
| `ExitOkImpliesQueued` (S4) | D-ACK | a launch that exits 0 was received and queued by a primary |
| `GaveUpJustified` (S5) | D-CLOSING | a launch gives up only on a hung holder, never on « closing » |
| `Settles` (L1) | D-KERNEL, D-RETRY | every launch eventually serves, hands off, gives up on a hung holder, or crashed |
| `AckedApplied` (L2) | D-ACK, D-DRAIN | a request a primary received is eventually handled by its window (shown, or said why not) unless that primary crashes, hangs, or its window stops answering (`Stuck`: the bounded wait at close gives up); no escape for a primary that closes (it drains first) |
| `SlotsAgree` (S6) | D-OWNER | whatever the current window holds, Go holds as delivered to that window's generation: Go's in-flight count is the window's, nothing is sent twice to one window |
| `RemovedWasShown` (S7) | D-OWNER | Go removes a request only once a window showed it (or said why not) |
| `NoStrandedSlot` (L3) | D-OWNER, D-DRAIN | every request a primary received leaves Go's queue, whatever the reconnections, unless that primary crashes, hangs, or its window stops answering |

| Configuration | Result |
|---|---|
| `OISingleInstance.cfg`: 3 launches, 1 crash, no hang, `MaxGen = 1` | no error, liveness included: 66,002 states generated, 23,418 distinct; 1 min 31 s (2 workers, 2 GB heap, 2026-11-13: one owner by generation, reports delivered as their own step) |
| `OISingleInstanceHang.cfg`: as above, a primary may hang | no error, liveness included: 87,977 states generated, 31,374 distinct; 2 min 9 s |
| `OISingleInstanceReconnect.cfg`: 3 launches, no crash, no hang, `MaxGen = 2` | no error, invariants (S1-S7) only: 214,727 states generated, 70,171 distinct; 2 s (with liveness: no error, 7 min 31 s, measured 2026-11-13, too long for every run) |
| `OISingleInstanceReconnectLive.cfg`: 2 launches, no crash, no hang, `MaxGen = 2` | no error, liveness included: 3,402 states generated, 1,484 distinct; 3 s |

Every mutant below is caught by its listed property and every witness is violated (the 23 OI runs of
`scripts/tla-check.sh` take about 5 min 20 s, 2026-11-13). A launch reads « closing » in the step that sends it
(`ReplyClosing`), and a reply is forgotten once read; the `AckedApplied` counterexample of NoDrain ends with a primary serving forever
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
| `OISingleInstanceAckNoQueue` | `Enqueue <- AckNoQueue` | acknowledged without being queued | `ExitOkImpliesQueued` |
| `OISingleInstanceFireAndForget` | `WaitsForReply <- FireAndForget` | exit 0 after sending, no acknowledgement (Wails macOS) | `ExitOkImpliesQueued` |
| `OISingleInstanceLockReleasedEarly` | `ReleaseOnClose <- ReleaseEarly` | lock released when closing starts | `MutualExclusion` |
| `OISingleInstanceCrashKeepsLock` | `ReleaseOnCrash <- KeepLockOnCrash` | pid file or `O_EXCL` file left by a crash | `Settles` |
| `OISingleInstanceGiveUpOnClosing` | `GiveUpOnClosing <- GiveUpClosing` | « closing » read as « already running » | `GaveUpJustified` |
| `OISingleInstanceSecondUnlinks` | `UnlinksStale <- SecondUnlinksStale` | a second launch removes a « stale » socket (it may be the new primary's) | `Settles` |
| `OISingleInstanceNoRetryLock` | `AfterBackoff <- DialAgain` | retries reconnect without retrying the lock | `Settles` |
| `OISingleInstanceNoDrain` | `ToApply <- NoDrain` | requests queued, never read by the window (D-DRAIN) | `AckedApplied` |
| `OISingleInstanceAckDropOnClose` | `DrainedBeforeClose <- CloseAnyTime`, `MaxCrashes = 0` | the window closes without applying what it acknowledged: a clean shutdown loses it; no crash, so only the shutdown can | `AckedApplied` |
| `OISingleInstanceEarlyDequeue` | `DrainedBeforeClose <- QueueEmpty`, `MaxCrashes = 0` | the process closes once every request was sent to the window instead of once the window reported them handled | `AckedApplied` |
| `OISingleInstanceStaleReportAccepted` | `AcceptsReport <- StaleReport`, `MaxGen = 2` | a report of a superseded stream frees a request the new stream holds (Go then counts one less in flight than the window holds) | `SlotsAgree` |
| `OISingleInstanceResetAllOnDisconnect` | `EndGiveBack <- ResetAll`, `MaxGen = 2` | the late end of a superseded stream gives back every request, the new stream's too (sent twice) | `SlotsAgree` |
| `OISingleInstanceWitnessHandoff` | invariant `NoHandoff` | witness: some launch exits 0 | (must violate) |
| `OISingleInstanceWitnessGiveUp` | invariant `NoGiveUp`, `Hangs = TRUE` | witness: some launch gives up on a hung holder | (must violate) |
| `OISingleInstanceWitnessTakeover` | invariant `NoTakeover` | witness: a launch serves after a primary that served crashed | (must violate) |
| `OISingleInstanceWitnessHandover` | invariant `NoHandover` | witness: a launch serves after a primary closed normally | (must violate) |
| `OISingleInstanceWitnessStaleReport` | invariant `NoStaleReport`, `MaxGen = 2` | witness: a report of a superseded stream is on its way while the new stream holds the same request | (must violate) |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config OISingleInstance.cfg OISingleInstance
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/OISingleInstanceNoLock.cfg mutants/OISingleInstanceMutants
```

## SandboxScope (W49, block SB1b, ADR 0042 amendment A1-A4, A8 and A9: the sandbox's memory scope)

`SandboxScope.tla` models one app session running the sandboxed launches of `Runs` (compiles, tinymist starts, possibly
concurrent). Like OISingleInstance it is a local mechanism, not a peer protocol, kept here because `scripts/tla-check.sh`,
`mise run tla`, `formal` and `done` check this directory. The rows are those of
`docs/plans/2026-11-11-sandbox-crash-reports.md`, section "Gate SB1b" (S, R, M, F, H). The app sets up a delegated
session scope and its holder lazily, under a mutex (S1-S9, one atomic step whose outcomes are the fallback rows F1-F7,
F10, F11; at most `MaxSessions` setups); per run it checks the holder's pidfd (H1), then in one step makes `run-<n>`,
writes and reads back its limits and starts the outer launcher with `CLONE_INTO_CGROUP` (R1-R5: the race the design
must survive, a holder dying between its check and `clone3`, H2, stays in the model), whose first statement checks its
own cgroup (R6); right after `Wait` it reads the counters (R7), then `cgroup.kill` and `rmdir` (R8). Kernel facts are kept apart from what the app believes:
`sc` (the scope as it really is: slice, inherited bound, memory delegated and enabled), `rcg` (the run cgroup absent,
present without its limits, or with them in force), `loc` (where the workload really is), `truth` (what really ended
it). A placement error or a failed write is the fallback (`mode` = `rlimit`, the reported `Output.Containment`).

The memory cause (A4, A8) is computed at R7 from an abstract timeline: how the first local `oom_kill` rise was seen (in
the same read as an `oom` rise, within W = 1 s of a non-fatal `oom` event, later, or with no `oom` rise), whether a
non-fatal event came first, whether the `/proc/vmstat` delta exceeds the run's own (a kill elsewhere, another run's
included: concurrent runs make the attribution conservative, never wrong), whether an ancestor's local `oom` rose,
and whether the watch overflowed or a read failed. These flags only grow during the watch and are read once, so each
is chosen at the event that sets it (a kill) or at R7 (overflow, failed read, a kill elsewhere): the same outcomes as
separate events, with far fewer interleavings. Likewise a run that ended (`done`, or `lost` when the app died) keeps
only the fields later properties read (`Ended`): every invariant on its cause, counters and order was checked in the
state before, where those values are final. Measured: with separate events and full end records, `SandboxScopeFaults`
passed 2.3 million distinct states at depth 20 with 750,000 left on its queue; with both, it ends at 845,384 (before A9; 457,948 after).

`Faults` = TRUE adds every environment fault: any setup step fails (F1-F7, F11, including a job `done` with the holder
not moved, F4), a limit write refused or silently ignored (F8, F9), the app outside the user manager (F10), a launch
path that places the bootstrap outside its run cgroup (the code defect R6 exists for), the holder killed at any point
(H1-H3; systemd then collects the empty scope and its run cgroups), the app SIGKILLed (H4), global and ancestor OOMs
with or without an earlier non-fatal local `oom` (M3, M6, M8), OOM kills elsewhere (M7), watch overflows and unreadable
counters (M9), fallback runs without a PID namespace (every cgroup run has one, A9). The document's own behaviour (reaching the
resident limit, mapping past `RLIMIT_AS`, forking, exiting, running until the timeout) is never a fault.

Fairness: four weak terms, one per process, none on the target, the kernel's OOM decisions or faults. Every fair
action can occur only finitely often (each run passes once, setups are bounded, each holder reaps once, each scope is
collected once), so weak fairness on a group's disjunction forces each of its actions that stays enabled. Grouped WF
terms keep TLC's liveness problem small (the OISingleInstance section explains the blow-up with one term per action).

| Term | Assumes |
|---|---|
| F1 `WF_vars(Setup \/ \E r : AppStep(r))` | the app keeps running while alive: its steps for every run, the timeout included, and the session setup |
| F2 `WF_vars(\E r : BootStep(r))` | the trusted bootstrap (outer launcher, namespace init, second launcher) of a live run keeps running |
| F3 `WF_vars(\E v : HolderReap(v))` | a holder that read EOF and is not killed finishes its cleanup (bounded at 5 s in the design) |
| F4 `WF_vars(\E v : Collect(v))` | systemd collects a scope with no process left (`CollectMode` inactive-or-failed) |

| Property | Rule | Meaning |
|---|---|---|
| `ConfinedFromBirth` (S1) | A1, A3, R3, R6, H2 | every instruction after the membership check (bootstrap, untrusted target, descendants) runs in its run cgroup with its limits in force, or the run reports `rlimit` |
| `AddressBudget` (S2) | A2 | no process maps past `MemoryLimit`: `RLIMIT_AS` is never raised |
| `SessionBounded` (S3) | A3, A8 (aggregate bound) | a workload in a run cgroup sits in a scope inside the app's own slice with the app's leaf bound in force |
| `CountersBeforeCleanup` (S4) | A4, R7, R8 | the app never kills or removes a run cgroup before reading its counters |
| `UnreadNeverZero` (S5) | A4, M9 | a failed read or a watch overflow is `unknown`, never zero |
| `LimitIsLocal` (S6) | A4, A8, M1, M3, M6-M8 | `local-limit` only when the run's own limit killed it, up to residual RS4 (a global OOM that kills only this run within W of a non-fatal local `oom`, indistinguishable by design) |
| `FailureFallsBack` (S7) | A3, F1-F11, H2 | a setup or placement failure is the fallback, never an error |
| `NoOrphan` (L1) | A8, A9, H4-H7 | after the app dies, no process remains in a run cgroup, whatever happened to the holder (the init's Pdeathsig and the PID namespace of every cgroup run) |
| `RunsEnd` (L2) | A3 | every run ends (in its cgroup, in the fallback, or refused by R6), or the app died |
| `NoStaleScope` (L3) | A8, A9, H5, H8 | after the app dies, every session scope and every run cgroup is eventually gone (the holder's cleanup, then systemd's collection) |

| Configuration | Result |
|---|---|
| `SandboxScope.cfg`: 2 runs, at most 2 sessions, no fault | no error, liveness included: 1,323 states generated, 554 distinct, depth 14, 5 temporal branches over 2,770 behaviour-graph nodes; 1 s (2 workers, 2 GB heap, 2026-11-12, after A9) |
| `SandboxScopeFaults.cfg`: as above, every fault | no error, liveness included: 2,452,912 states generated, 457,948 distinct, depth 21, 5 temporal branches over 2,289,740 behaviour-graph nodes; 4 min 13 s (CPUs 12-19, nice 19, machine loaded) |

The 25 SandboxScope runs of `scripts/tla-check.sh` take about 5 min of TLC time on that machine, 253 s of it
`SandboxScopeFaults` (before A9, with a PID namespace chosen per cgroup run, 845,384 distinct states and 6 min 10 s);
every mutant is caught by its listed property (TLC's own message names it) and every witness is violated. The four
liveness mutants are found by TLC's periodic check in 3 s each: OrphanAfterAppKill ends with the app SIGKILLed and the
holder alive and idle, so its scope is never collected; NoPdeathsig and NoPidNamespace end with a holder killed, then the
app SIGKILLed, and the workload (or a descendant) stuttering in `run-<n>`; WaitForScope ends with both setups failed and
the runs stuttering in `need`. HolderDeathEscapes is the H2 race itself: the holder dies
after its pidfd check, systemd collects the empty scope, and the placement error's retry runs in the app's cgroup.

Found by the model and decided in ADR 0042 A9:
- Closed: a holder killed while one of its session's runs keeps the scope alive (H3), then the app SIGKILLed, left that
  run's descendant (no PID namespace) alive in the stale `run-<n>`, since only the holder reaped. A9 makes the kill the
  kernel's: the namespace init's `PR_SET_PDEATHSIG(SIGKILL)` and a PID namespace on every cgroup launch path (H6, H7);
  `NoOrphan` now holds with no exception, and the mutants `NoPdeathsig` and `NoPidNamespace` bring the double fault back.
  The sweep of stale `run-<n>` at the next app start (H9) is not modelled: the model has one app session.
- Residual RS6, shown reachable by `SandboxScopeResidualCollected` (it must violate, like `SP1TypingResidual`): the same
  holder death lets systemd collect the scope once its last process is gone, run cgroups included (A8's own H2
  assumption), so a run whose target already exited can lose its cgroup before R7; its read fails and its cause is
  `unknown` (never zero). `CountersBeforeCleanup` is about the app's own order, which holds.

Not covered (named tests instead, plan section "Gate SB1b"): the D-Bus calls and the job matching (`TestCgroupSessionScopeDelegated`),
the exact read-back values (`TestCgroupRunLimitsVerifiedBeforeStart`), the backoff durations, the descriptors (rows D),
the accounting of the bootstrap's own memory, the 5 s bounds as durations, the process-group kill of the fallback after
the app dies (the fallback makes no cleanup claim when the app is SIGKILLed), the journal evidence (rows J).

Mutants (`mutants/SandboxScopeMutants.tla`, each config overrides one rule of the sound model, anchor = the override
line in the config) and witnesses (anti-vacuity: the sound model must violate them). Each config uses the constants of
`SandboxScope.cfg` when no fault is needed to show it (NotBornInside, CleanupBeforeCounters, SessionUnbounded,
RaiseAddressLimit, WitnessLocalLimit, WitnessAddressLimit), else those of `SandboxScopeFaults.cfg`; its first comment
line names which.

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `SandboxScopeNotBornInside` | `BornInside <- NotBorn` | PIDs of a running launcher handed to systemd: its bootstrap works in the app's cgroup before the move (ADR point 1; SB1b-10) | `ConfinedFromBirth` |
| `SandboxScopeExecBeforeCheck` | `CheckFirst <- NoFirstCheck` | the bootstrap works before checking its own cgroup: a misplaced launch runs outside (R6; SB1b-11, SB1b-12) | `ConfinedFromBirth` |
| `SandboxScopeCleanupBeforeCounters` | `CleanFirst <- EarlyCleanup` | `cgroup.kill` and `rmdir` before the counters are read (R8 before R7; SB1b-17) | `CountersBeforeCleanup` |
| `SandboxScopeUnreadAsZero` | `CauseOf <- UnreadZero` | an unreadable counter or a watch overflow taken as zero (M9; SB1b-18) | `UnreadNeverZero` |
| `SandboxScopeVictimAsLimit` | `CauseOf <- KillIsLimit` | any local `oom_kill` is the limit, local `oom` not checked (M3; SB1b-19) | `LimitIsLocal` |
| `SandboxScopeMixedHistoryAsLimit` | `CauseOf <- CountersOnly` | the cause from counters alone (`oom >= 1`, `oom_kill >= 1`), the window W ignored (M6, A8; SB1b-25) | `LimitIsLocal` |
| `SandboxScopeContinueAfterPartialSetup` | `RunOnFail <- ContinueOnFail` | a refused or silently ignored limit write (F8, F9) not acted on: the run starts in a cgroup without its limits (SB1b-13 to SB1b-16) | `ConfinedFromBirth` |
| `SandboxScopeSessionUnbounded` | `ScopeSlice <- OtherSlice` | the session scope created outside the app's slice: S5 compares with the requested slice and passes, the slice's limit no longer bounds app plus sandbox (A8; SB1b-28) | `SessionBounded` |
| `SandboxScopeJobDoneIsConfined` | `Verified <- JobDone` | the job result `done` taken as confinement, S5-S9 not read back: a scope without its inherited bound (F7) is used (A3; SB1b-22, SB1b-23) | `SessionBounded` |
| `SandboxScopeRaiseAddressLimit` | `AddressLimit <- Raised` | `RLIMIT_AS` raised above `MemoryLimit` when a cgroup is active (ADR point 2, A2; SB1b-20) | `AddressBudget` |
| `SandboxScopeHolderDeathEscapes` | `PlacementRetry <- KeepContainment` | after the holder dies between its pidfd check and `clone3` and the scope is collected (ENOENT, ENODEV), the retry runs in the app's cgroup but keeps Containment `cgroup` (H2) | `ConfinedFromBirth` |
| `SandboxScopeOrphanAfterAppKill` | `ReapOnEOF <- NoReap` | the holder ignores EOF: after the app is SIGKILLed its scope and run cgroups stay (H5, H8; SB1b-29); the processes themselves die by A9 | `NoStaleScope` |
| `SandboxScopeNoPdeathsig` | `InitPdeathsig <- NoPdeathsig` | the namespace init does not die with the app: with its holder killed, the workload lives on in `run-<n>` (A9, H6) | `NoOrphan` |
| `SandboxScopeNoPidNamespace` | `CgroupPidNs <- NoCgroupPidNs` | a cgroup launch path without PID namespace: with its holder killed, a descendant outlives the app in `run-<n>` (A9, H7) | `NoOrphan` |
| `SandboxScopeErrorOnFallback` | `PlacementRetry <- ErrorRetry` | a placement error returns an error instead of the fallback (F10, H2; SB1b-21, SB1b-30) | `FailureFallsBack` |
| `SandboxScopeWaitForScope` | `OnSetupFailed <- WaitScope` | after a failed setup, runs wait for a session instead of using the fallback (A3) | `RunsEnd` |
| `SandboxScopeWitnessLocalLimit` | invariant `NoLocalLimit` | witness: a run ends attributed `local-limit` (M1), with no fault | (must violate) |
| `SandboxScopeWitnessAddressLimit` | invariant `NoAddressDeath` | witness: a run dies at the address budget (M2), with no fault | (must violate) |
| `SandboxScopeWitnessFallback` | invariant `NoFallback` | witness: a run ends in the fallback, Containment `rlimit` (F rows) | (must violate) |
| `SandboxScopeWitnessVictim` | invariant `NoVictim` | witness: a run ends attributed `victim` (M3, M7, M8) | (must violate) |
| `SandboxScopeWitnessMixedHistory` | invariant `NoMixedHistory` | witness: a non-fatal local `oom`, then a kill by someone else, read as `victim` (M6) | (must violate) |
| `SandboxScopeWitnessHolderDeath` | invariant `NoHolderDeath` | witness: a run ends confined in its cgroup although its holder was killed meanwhile (H3) | (must violate) |
| `SandboxScopeResidualCollected` | invariant `NoCollectedBeforeRead` | residual: a run waits for R7 while its cgroup is gone with its collected scope (its cause is `unknown`) | (must violate) |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config SandboxScopeFaults.cfg SandboxScope
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/SandboxScopeNotBornInside.cfg mutants/SandboxScopeMutants
```

## ProjectChange (block PC1, ADR 0043 part 2: the project changes by a restart, never live)

Rule: `docs/plans/2026-11-14-pc1-project-change-by-restart.md`, rules N-5..N-14 (the take-over and the restart) and
J-1..J-5 (the join). Like OISingleInstance it is a local mechanism, kept here because `scripts/tla-check.sh`,
`mise run tla`, `formal` and `done` check this directory. `ProjectChange.tla` models the process images of one
profile on one folder:

- **The Go runtime of each image**: the wiring with its event buffer, the trust store, the invitation host. A
  request (take-over or join) is held in memory; the shutdown closes the wiring, writes (a join: the pending-join
  record in `project.json`, then `opStore.replace`: stage, set aside, install, clean up), returns from `stopAll`,
  passes the termination barrier (no child alive), then restarts. The next image recovers the list (`recoverSwap`),
  loads it, drops the record of a join that did not install, chooses its notice from what loaded and reads
  `INKWELL_AFTER_TAKEOVER` once; the record of a join that installed is consumed once the joined root document is
  held (one atomic rewrite of `project.json`: `rootUrl` set, record removed), never before, so no start can replace
  that root by a fresh document.
- **The page**: coverage, automerge-repo's pending continuations, the root it opened, the startup id it loaded
  with. It is the server build's page, a browser tab that outlives the process; the desktop webview, which ends
  with the process, is the case where the reload is immediate.
- **The restart**, by `Platform`: `exec` (Linux: the same process becomes the next image; every thread and its memory
  end; the profile lock goes with close-on-exec) or `spawn` (macOS and Windows, N35: a new process that claims the
  profile lock like any launch, ADR 0037, while the old one exits holding it to the end). One module, so a port is
  a new implementation of the same rules, not a new design.
- **Durable versus memory**: `trust`, `prev`, `next` (each rename is followed by a directory sync, so each step is
  one durable step), `pj` (the record), `root` (`rootUrl`) survive a crash and a restart; everything else is lost
  (Raft's `Restart`: "it loses everything but" stable storage). A crash may hit any process at any step; a write step
  or the execution may fail (a failed rollback included). Projects are `old` and `new`; roots `R0` (the folder's
  document) and `R1` (the inviter's).
- **The bridge** (N-16): the page's `store` and `net` streams to Go. One storage write of the page per behaviour;
  on a request (or a quit) Go closes both streams for good, refuses a write not yet taken (the page says so) and
  waits for the handler in flight; the member-list write and `stopAll` start only once drained.
- **Environment**: one peer that attests under whatever project the wiring runs, the person (request, quit,
  relaunch), a member of the new project that gives the root document after a join (and announces its URL when the
  genesis carries none), children, faults.

Not covered (named tests instead, plan rows): the archive copy (N-5, N-7: `TestTakeOverArchivesOldList`, MW3, MW6), the
genesis contents and the member's no-op (N-8), the tree hash of the folder (N-6), argv and environment rebuilt
(`TestRestartArgvAndEnvRebuilt`), the descriptors (N-14, `TestRestartBarrier`), another launch taking the lock between
the release and the re-acquisition (R-7: OISingleInstance proves the hand-off), several requests in one behaviour (one
per behaviour; a second request is a new behaviour from the state the first left), the network handshake of the join
(an invitation that fails before the Welcome changes nothing), a Byzantine member announcing another root.

Fairness: three weak terms, grouped as in SandboxScope. F1 `WF_vars(GoStep)`: every live process keeps running its
own steps (the steps of one image are sequential and finite, so weak fairness on the group forces each). F2
`WF_vars(PageStep)`: a waiting page keeps polling the startup id and reloads. F3 `WF_vars(RootStep)`: a member of the
new project is eventually online (an assumption; with every member gone for good the record waits, J-2). None on the
person, the peer, children or faults.

| Property | Rule | Meaning |
|---|---|---|
| `NoOldStateAfterChange` | N-11, N-13, N-14, J-1 | a page bound to an image holds coverage and continuations of that image's project only and opens that project's root; no change authorised under one project reaches an image running another; no op of the old project lands in the installed new list; no child of an older image runs beside a newer one |
| `NewStateSurvives` | N-7, J-4 | once installed, the new list stays the list (crash, `errCommitted`, recovery); a joined root, once recorded, stays |
| `RestartOnce` | N-12 | at most one restart per request: no restart loop |
| `OpensAfterCrash` | N-7, J-1, J-4 | in every state a complete list is recoverable, and with no record left the recorded root is the loaded project's root (never the old text under the new list) |
| `WriteBeforeRestart` | N-12, N-13 | history: no restart ever cut a write short (the restart follows `stopAll`, which follows the write) |
| `JoinOnce` | J-1, J-2 | the pending-join record is consumed at most once and « joined » shows at most once |
| `OneImage` | N-12, N35, ADR 0037 | one process works on the profile at a time: a restarted image starts its work only when the previous one has ended |
| `NoticeMatchesStanding` | N-12, J-2 | the notice tells what loaded: the three outcomes of the write, `errCommitted` included |
| `NoLostWrite` | N-16 | a storage write of the page is never lost silently (while pending, its image still runs, shuts down or crashed) and never lands once the member-list write began |
| `ChangeCompletes` (F1-F3) | N-9, N-12, J-2 | a request ends in a running image of the new project, the page reloaded on it with the project's root and the notice, unless a crash, a failed step or a quit interrupted it |

| Configuration | Result (TLC 1.8.0, 2 workers, 2026-11-15, with the bridge drain N-16) |
|---|---|
| `ProjectChange.cfg`: exec; take-over or join (root in the genesis or announced); 3 images, 1 crash, 1 failed step | **exhaustive, no error**: 174,908 states generated, 81,790 distinct, depth 36, liveness included, 4 s |
| `ProjectChangeSpawn.cfg`: spawn, as above | **exhaustive, no error**: 198,115 states, 93,657 distinct, depth 36, liveness included, 4 s |
| `ProjectChangeFaults.cfg`: exec; 4 images, 2 crashes, 2 failed steps | **exhaustive, no error**: 2,238,681 states, 998,929 distinct, depth 46, liveness included, 51 s |
| `ProjectChangeSpawnFaults.cfg`: spawn, as above | **exhaustive, no error**: 2,567,171 states, 1,155,209 distinct, depth 45, liveness included, 59 s |

Mutants (`mutants/ProjectChangeMutants.tla`, each config overrides one rule of the sound model with the constants of
`ProjectChange.cfg`, `SpawnNoWait` and `WitnessJoinAnnounced` those of `ProjectChangeSpawn.cfg`; anchor = the override
line in the config). `scripts/tla-check.sh` checks that TLC's message names the property of the row, not only that
something is violated.

| Mutant | Anchor (config line) | Broken rule | Caught by |
|---|---|---|---|
| `ProjectChangeLiveAdopt` | `LiveChange <- Yes` | the rejected design: the project adopted in the running session, the adapter reset at the reply; a buffered `Attested` of the old project reaches the new coverage (4 states) | `NoOldStateAfterChange` |
| `ProjectChangeRestartBeforeDurable` | `StoppedOK <- AfterWiring` | the restart before `stopAll` returned (MW8): the write is cut short | `WriteBeforeRestart` |
| `ProjectChangeRestartTwice` | `RestartWanted <- AfterChangeToo` | the restarted image restarts again when the person quits (`INKWELL_AFTER_TAKEOVER` taken as a request) | `RestartOnce` |
| `ProjectChangeRequestBeforeNavigation` | `DiscardContext <- No` | N-15: the request is sent while the page's JavaScript context lives (no navigation to the restart page first); a continuation authorised under the old project persists through the new image | `NoOldStateAfterChange` |
| `ProjectChangeJoinNoRestart` | `RestartWanted <- TakeOverOnly` | N-12: a join is written but only a take-over restarts (constants of `ProjectChange.cfg`, `Kinds = {"join"}`); the image exits and the change never completes | `ChangeCompletes` |
| `ProjectChangeJoinTwice` | `ClearRecord <- KeepRecord` | consuming the pending-join record leaves it in `project.json`; the next start consumes it again | `JoinOnce` |
| `ProjectChangeWriteBeforeWiringStopped` | `WriteOK <- Anytime` | the write starts while the wiring runs (MW10): an op of the old project lands in the new list | `NoOldStateAfterChange` |
| `ProjectChangeWriteBeforeDrain` | `WriteOK <- WriteBeforeDrain` | N-16: the member-list write starts once the wiring stopped but before Go drained the store and net streams: a storage write of the page still in its handler lands after the write began | `NoLostWrite` |
| `ProjectChangeNoBarrier` | `BarrierOK <- NoBarrier` | no termination barrier (MW11): a child of the old image runs beside the new one | `NoOldStateAfterChange` |
| `ProjectChangeSpawnNoWait` | `MayStart <- NoWait` | N35: the spawned image starts its work without claiming the profile lock, while the old one is still exiting | `OneImage` |
| `ProjectChangeJoinRecordAfterInstall` | `StepsOf <- RecordLast` | the new list installed before the pending-join record: a crash between them leaves the new list with the old root | `OpensAfterCrash` |
| `ProjectChangeRecoverPrefersAside` | `RecoveredTrust <- PreferAside` | recovery at open prefers the set-aside list: after the install rename the old list comes back | `NewStateSurvives` |
| `ProjectChangeCarriedOutcome` | `NoticeOf <- AlwaysDone` | the notice follows the request, not what loaded: a write that failed before the install says « maintenant le vôtre » | `NoticeMatchesStanding` |
| `ProjectChangePageNeverReloads` | `ReloadOn <- SameId` | the page reloads only on the startup id it knows: it waits forever (N-9's in-test mutant) | `ChangeCompletes` |
| `ProjectChangeWitnessTakeOver` | invariant `NoTakeOverDone` | witness: a take-over ends with « Ce document est maintenant le vôtre » | (must violate) |
| `ProjectChangeWitnessJoinAnnounced` | invariant `NoJoinAnnounced` | witness (spawn): a join whose genesis has no root ends with the root announced and held after the restart | (must violate) |
| `ProjectChangeWitnessCrashAfterInstall` | invariant `NoCrashAfterInstall` | witness: a crash after the install, then a manual launch loads the new list | (must violate) |
| `ProjectChangeWitnessFailedWrite` | invariant `NoFailedNotice` | witness: a write that fails before the install restarts on the old list and says so | (must violate) |
| `ProjectChangeWitnessWriteRefused` | invariant `NoRefusedWrite` | witness: a storage write the page sent just before the request is refused by the drain (N-16) | (must violate) |

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config ProjectChange.cfg ProjectChange
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/ProjectChangeLiveAdopt.cfg mutants/ProjectChangeMutants
INKWELL_TLA_ONLY='^ProjectChange' ./scripts/tla-check.sh    # the 23 ProjectChange runs only, about 130 s (measured 132 s)
```

## DirectUpgrade (block HP1: a relayed link to a wanted peer keeps trying to become direct)

Rule: `docs/plans/2026-11-16-hp1-direct-upgrade.md`, rules H-1..H-17 and H-19, section 6, revised after the Codex debate
(D1-D6) and the freeze review (B1-B6). `DirectUpgrade.tla` models two nodes. L is modelled in full: its upgrader (the policy of
`protocol/directup`: Off, NotReady = `idle`, Scheduled, InFlight, Direct; ladders; epochs; the attempt bucket; the
app's activity), its calls (Punch = the fork's cancellable `DirectConnectContext`, with the direct dial libp2p makes
first to every peerstore address once one is public; Dial = a forced direct dial), libp2p's one-shot on a new inbound
circuit (`Auto`, which holds libp2p's per-peer lock: our call is then Busy), and the answerer side of R's exchanges.
Every round, whatever its origin, passes L's two choke points: the address filter (the gate: wanted, round bucket,
FIFO admission to a node-wide slot with a lease, one ticket per peer, none for a peer not wanted; a round that gives
up waiting keeps its ticket's place and a grant to it is held for its peer) and the connection gater (wanted, a bucket
of non-relay destinations, and only under a held grant, within its allowance: for libp2p's first direct dial of its
one-shot, R holding no grant, the gater asks one once through the FIFO, `AutoPre`'s `AskGranted` branch, and
otherwise refuses the dial and keeps the ticket). R is honest (its upgrader under the same role rule, paced by time, and its one-shot; its
exchanges run concurrently with L's) or Byzantine (starts exchanges at will, reconnects, toggles `/libp2p/dcutr`,
refuses or stalls as answerer). Other peers ask for slots and may keep them until their lease ends.

- **Circuits and picks**: circuits are named by their dialler (`LR`: L dialled). A DCUtR stream rides the circuit its
  sender's `NewStream` picks (`chL`, `chR`); the answerer resets a stream that arrives on its inbound circuit. Any pair
  of picks may last for ever (Alloy `BothAccepted`: both ends on their own inbound circuit, two exchanges at once);
  only « both outbound » is left by a fair step (`Settle`), because it needs unequal stream counts (Alloy
  `SomeEligibleAccepted`), which last only while an opening is in flight.
- **Overlap**: `crossed` records that the pair's two punch phases overlapped; with `CrossCancel`, such punches may
  both fail. Whether they do on libp2p is unknown; `TwoCircuits` assumes they may and checks the adopted rule H-15
  (with two circuits nobody initiates, the circuit the higher PeerID dialled is closed: `Resolve`).
- **Epochs**: a new circuit (E7) marks the call in flight `stale` and cancels it (D1) unless it is already punching.
- **Time**: `Tick` is one unit; due times and leases count down, `since` is the time since the last call started
  (capped), buckets refill on one free-running clock. `Tick` waits while R's slot is at its last unit and the pair has
  work at L that libp2p's own deadlines end within the lease (an upper-bound timer); a lease that ends cancels what is
  left. Ladders and caps are scaled (fast 1, slow 1 then 2, `Off` 1, leases 2).
- **Budgets** are checked against **reference buckets** driven only by the clock and what really happens: rounds
  (S5), non-Busy calls (S10), non-relay destinations (S12), node-wide admissions (S15). A run stays within a (σ, ρ)
  bound in every window iff its reference never goes negative; a Busy charged shows as a real bucket below its
  reference (S11).
- **Liveness** assumptions (weak fairness): `Tick`, every step of a running call, one-shot or exchange, L's actor,
  identify (E2, E3), both handlers registering, an honest R starting exchanges, `Settle`, `Resolve`, the app waking
  (`<>[]active`, idle periods bounded), the queue (joining, leaving, the semaphore handing a freed slot to the head),
  and `Open` (`<>[]punchable`, closes bounded). No fairness on dials, restarts, losses, wanted flips, closes, pick
  changes, idling, other peers asking for, using or giving back a slot, a round giving up its wait in the
  filter (it keeps its ticket), or a Byzantine R. An honest R that gave up comes back within the hold of its grant
  (`Tick` waits for it at the lease's last unit: its fast ladder is shorter than the 75 s lease).

Not covered (named tests or vectors instead): Settling and the nudge of `maybeUpgrade` (H-13, Go sim); E8 (H-9, pure);
jitter and the real ladder values (H-8, pure); the three address kinds and the ≤ 3 per round (H-12, vectors: one
destination unit per punch here); classification from libp2p errors (H-7, vectors); the filter's 20 s wait bound
(Go sim); whether overlapping punches cancel (Go measurement); more than one other peer.

| Property | Rule | Meaning |
|---|---|---|
| `OneCall` (S1) | H-4 | at most one of L's calls for R is in flight, a cancelled one included |
| `NoLaunchUnwanted` (S2) | H-1, H-14 | no live call for a peer that is not wanted |
| `NoLaunchDirect` (S3) | H-14 | no live call while the session is direct |
| `RoleOK` (S4) | H-3 | a live Punch holds an inbound circuit; a live Dial holds an outbound circuit only, to a public peer |
| `RoundBudget` (S5) | H-12 | rounds through the gate, every origin, stay within (RCap, RPeriod) |
| `NoRoundUnwanted` (S6) | H-12 | zero rounds with a peer that is not wanted |
| `NoLaunchUnready` (S7) | H-2 | no live call before our handler is registered |
| `StaleIgnored` (S8) | H-6 | a result of an older epoch changes neither the ladders, the due time nor the state |
| `GlobalCap` (S9) | H-5 | at most K slots held on the node, every origin |
| `AttemptBudget` (S10) | H-11 | non-Busy calls stay within (ACap, APeriod), across epochs, reconnections and un-want |
| `BusyFree` (S11) | H-7 | a Busy call is never charged |
| `DestBudget` (S12) | H-12 | non-relay destinations, every origin (DirectConnect's first direct dial included), stay within (DCap, RPeriod) |
| `NoDialUnwanted` (S13) | H-12 | no non-relay dial to a peer that is not wanted: a stranger has no direct path |
| `NoLadderCallIdle` (S14) | H-17 | no ladder-driven call while the app is idle |
| `AdmissionBudget` (S15) | H-16 | node-wide slot grants stay within (NCap, APeriod) |
| `NoUnallowedDial` (S16) | H-16 | every non-relay destination, every origin (other peers' rounds sharing a grant, libp2p's first direct dial), is drawn from a held grant's allowance: with S15, node-wide destinations ≤ allowance × admissions |
| `NoTicketUnwanted` (S17) | H-1, H-5 | a peer that is not wanted never takes a ticket nor a grant |
| `LinkBound` (S18) | H-19 | no direct connection is kept that landed after the last circuit was gone (ghost `lateLand`, set by every landing: L's call, its first direct dial, the one-shot, the answerer); `LandOK` (a circuit held) guards every landing branch, a landing without it is the attempt's failure (the late connection closed). Want and Connect, the rule's other reasons, are outside the model: the conservative case |
| `Upgrades` (L1) | H-2, H-3, H-6, H-8, H-10, H-17 | `wanted ∧ relayed ~> direct ∨ ¬wanted ∨ no circuit` |
| `BusyEnds` (L2) | H-4, H-5, H-7 | waiting for a slot or Busy does not repeat forever, even with another peer asking without end and never giving its slot back |
| `DupResolves` (L3) | H-15 | two circuits do not last: the one the higher PeerID dialled is closed |

| Configuration | Result (TLC 1.8.0, 2 workers, 2026-11-16, `heavy.sh --exclusive`, S1-S17 in every one) |
|---|---|
| `DirectUpgradeHonest.cfg`: one circuit at a time in either direction, R public or not (DirectConnect's first direct dial); R restarts once, 2 new circuits, the direct connection lost once; K = 2 | **exhaustive, no error**: 692,918 states, 170,697 distinct, depth 32, liveness included, 44s |
| `DirectUpgradeSilent.cfg`: one circuit R dialled (L is the only initiator); the direct path closes and opens with no event; the app idles once | **exhaustive, no error**: 38,300 states, 10,324 distinct, depth 25, liveness included, 04s |
| `DirectUpgradeTwoCircuits.cfg`: circuits both ways, any durable picks, both PeerID orders, rounds and destinations buckets of 2, overlaps may cancel, H-15 | **exhaustive, no error**: 629,210 states, 152,846 distinct, depth 40, liveness included, 52s |
| `DirectUpgradeTwoPeers.cfg`: one circuit R dialled, redialled once; one slot (K = 1), granted FIFO, another peer asking for ever, dialling within its grants and never giving them back | **exhaustive, no error**: 328,870 states, 69,688 distinct, depth 31, liveness included, 23s |
| `DirectUpgradeAnswerer.cfg`: one circuit L dialled, R not public: R is the only initiator; its rounds wait in L's filter, give up and come back; one slot shared with a peer asking for ever | **exhaustive, no error**: 23,244 states, 5,400 distinct, depth 23, liveness included, 02s |
| `DirectUpgradeLostPush.cfg`: one circuit R dialled (L is the only initiator); R's push after its handler registers may be lost; the poll calls (`remoteReady` a trigger) | **exhaustive, no error**: 20,788 states, 6,914 distinct, depth 24, liveness included, 02s |
| `DirectUpgradeByzantine.cfg`: Byzantine R, one circuit at a time, 2 new circuits, wanted flips once, R public or not, leases and node bucket of 2 (liveness: the honest configurations) | **exhaustive, no error**: 6,854,792 states, 994,752 distinct, depth 29, invariants, 18s |
| `DirectUpgradeLinkLost.cfg` (run `HP1rLinkLost`, block HP1r): Honest's constants and the relay drops every circuit once (a Drop seen from the dropped side: R still reachable, the path open), so the one-shot, a punch phase or the answerer's dial outlives the circuit it started on | **exhaustive, no error**: 1,200,294 states, 263,386 distinct, depth 34, liveness included, 1 min 17 s |

Scope of H-19 here (block HP1r): S18 `LinkBound` covers a landing without a circuit. That an upgraded session survives
its circuit's closure is checked by no property: no action closes a direct connection on a circuit's loss (`RelayLost`
and `Resolve` leave `direct` unchanged; only `DirectLost` and `RRestart` set it FALSE), so it holds by construction; an
invariant would be vacuous, so the witness `HP1rWitnessSurvives` only shows the state is reached, and the Go arm
`kept_upgrade_outbound_circuit_closed` of `TestUpgradeLateLinkClosed` checks the product.

Block HP1c (rules H-20 and P-15, plan `docs/plans/2026-11-23-hp1-reformalize.md` §2): R may crash (`RCrash`). Its
process dies without closing anything, so every circuit L holds stays open at L and is dead (a **ghost**, `cr.ghost`):
L sees it (`Sees("L")`) and nothing it carries arrives. R comes back on a new circuit in the same direction (R dials
again) or the other (L does), a new epoch (E7) that starts libp2p's one-shot at L as `Dial` does. H-20 is the rule
operator `RoundGhost` (FALSE: L's rounds open on their cause circuit, which is live); P-15 proves every older circuit
when a new one registers (`cr.pl` for a live one, `cr.pg` for a ghost): a live one answers and stays
(`ProveAnswered`, rule operator `KeepAnswered`), a ghost is closed (`ProveClose`; the newer circuit's sync ACK, outside
this model, comes since R is honest). H-15's `Resolve` waits for the proofs, so a ghost is never taken for a duplicate.
Modelling bounds: P-15 is modelled only when `ProveOlder` (required by a crash; with no ghost every proof is answered
and changes nothing but H-15's wait, a stutter, so the 47 HP1 and HP1r runs keep their state spaces); a crash happens only while the pair
has no exchange in progress (a crash during one ends it as `CanFail` does) and with the session relayed (a dead
direct connection is `Supersede`'s case); a crash run has no honest restart. The connection-level rule (T_vie, the
ACK wait, the sole connection) is checked by `Supersede` below.

| Property | Rule | Meaning |
|---|---|---|
| `RoundOnLive` (S19) | H-20 | no round of L (a call's or the one-shot's) starts on a ghost |
| `ProofKeepsLive` (S20) | P-15 | an older live circuit that answers its proof is never closed; crossed live dials still settle by H-15 (`DupResolves`) |
| `ResolveKeepsLive` (S21) | H-15, P-15 | H-15 never closes L's live circuit in favour of a ghost: L settles a duplicate only once the P-15 proofs are done (`ResolveWaits`), so a crossed crash is settled by P-15 closing the ghost |

| Configuration | Result (TLC 1.8.0, 2 workers, 2026-11-23, `heavy.sh`, S1-S21) |
|---|---|
| `DirectUpgradeCrash.cfg` (run `HP1cCrash`): Honest's constants with one dial, no honest restart nor direct loss, one crash with a new circuit in either direction (the same: H-20; the other: H-15 waits for P-15) | **exhaustive, no error**: 317,878 states, 80,974 distinct, depth 30, liveness included, 26 s |
| `DirectUpgradeProofTwoCircuits.cfg` (run `HP1cProofTwoCircuits`): TwoCircuits' constants with aligned picks, an always open path, no overlap cancel and buckets of 1; P-15 modelled, no crash (crossed live dials: the older answers, H-15 settles after the proofs) | **exhaustive, no error**: 155,160 states, 42,140 distinct, depth 26, liveness included, 14 s |

Mutants (`mutants/DirectUpgradeMutants.tla`, each config overrides one rule of the sound model with the constants of
the configuration named; anchor = the override line in the config). `scripts/tla-check.sh` checks that TLC's message
names the property of the row.

| Mutant | Anchor (config line) | Broken rule | Configuration | Caught by |
|---|---|---|---|---|
| `DirectUpgradeNoRetry` (M1) | `Retry <- NoRetry` | no retry, libp2p's one-shot only (H-8) | Silent | `Upgrades` |
| `DirectUpgradeStaleApplied` (M2) | `ApplyStale <- StaleApplied` | a result of an older epoch is applied (H-6) | Byzantine | `StaleIgnored` |
| `DirectUpgradeStaleAppliedLive` (M2) | `ApplyStale <- StaleApplied` | the same: a stale Success taken after a fallback stops the upgrader | Honest | `Upgrades` |
| `DirectUpgradeBusyTakesToken` (M3) | `BusyCharge <- BusyTakesToken` | Busy takes an attempt token (H-7) | Honest | `BusyFree` |
| `DirectUpgradeAnyPeer` (M4) | `WantedOK <- AnyPeer` | no wanted check (H-1) | Byzantine | `NoLaunchUnwanted` |
| `DirectUpgradeOutboundPunch` (M5) | `HasRole <- AnyCircuit` | the side that only dialled punches (H-3) | Honest | `RoleOK` |
| `DirectUpgradeLowerOnly` (M6) | `TieOK <- LowerOnly` | « lower PeerID only » (§3.4): both ends pick the lower side's outbound circuit, the higher side, whose stream would be accepted, never initiates | TwoCircuits | `Upgrades` |
| `DirectUpgradeEpochRefills` (M7) | `EpochRounds <- RefillRounds` (and `EpochAtt <- RefillAttempts`) | a new circuit refills the buckets (H-9) | Byzantine | `RoundBudget` |
| `DirectUpgradeSchedulerOnly` (M8) | `GateBudget <- SchedulerOnly` | the round budget in the scheduler only (H-12) | Byzantine | `RoundBudget` |
| `DirectUpgradeNoLocalReady` (M9) | `LReadyOK <- NoLocalReady` | a call before our handler is registered (H-2) | Honest | `NoLaunchUnready` |
| `DirectUpgradeStopAtCap` (M10) | `StopAfter <- StopAtCap` | a terminal hopeless state (H-10) | Silent | `Upgrades` |
| `DirectUpgradeNoCap` (M11) | `SlotFree <- NoCap` | no node-wide cap (H-5) | TwoPeers | `GlobalCap` |
| `DirectUpgradeWhileDirect` (M12) | `NotDirect <- IgnoreDirect` | a call while the session is direct (H-14) | Honest | `NoLaunchDirect` |
| `DirectUpgradeWhileInFlight` (M13) | `OneCallOK <- AnyTime` | a new call after E7 before the cancelled call returned (H-4) | Honest | `OneCall` |
| `DirectUpgradeGateAnyPeer` (M14) | `GateWanted <- GateAnyPeer` | the gate gives addresses to a peer not wanted (H-12) | Byzantine | `NoRoundUnwanted` |
| `DirectUpgradeNoDeadline` (M15) | `Expire <- Never` | no lease deadline: a holder that never gives its slot back keeps it (H-5, D3) | TwoPeers | `BusyEnds` |
| `DirectUpgradeEpochRefillsAttempts` (M16) | `EpochAtt <- RefillAttempts` | a new circuit refills the attempt bucket (H-11) | Byzantine | `AttemptBudget` |
| `DirectUpgradeRemoteGuard` (M17) | `RemoteGuard <- GuardOn` | the old guard: Punch waits for `remoteReady`; a lost push leaves the only initiator NotReady (scenario 8) | LostPush | `Upgrades` |
| `DirectUpgradeAnyOrder` (M18) | `GrantOrder <- AnyOrder` | slots granted in any order: the other peer retakes every freed slot | TwoPeers | `BusyEnds` |
| `DirectUpgradePreDialFree` (M19) | `GaterBudget <- PreDialFree` | DirectConnect's first direct dial passes the gater without spending (D2) | Byzantine | `DestBudget` |
| `DirectUpgradeGaterAnyPeer` (M20) | `GaterWanted <- GaterAnyPeer` | the gater lets a peer not wanted be dialled: libp2p's one-shot to a stranger (D2) | Byzantine | `NoDialUnwanted` |
| `DirectUpgradeIdleIgnored` (M21) | `ActiveOK <- AlwaysActive` | the ladders call while idle (D4) | Silent | `NoLadderCallIdle` |
| `DirectUpgradeStaySuspended` (M22) | `ActiveOK <- StaySuspended` | no resume once the app is active again (D4) | Silent | `Upgrades` |
| `DirectUpgradeGrantFree` (M23) | `NodeTokOK <- NoNodeBudget` | a slot granted without a node-wide admission token (D4) | TwoPeers | `AdmissionBudget` |
| `DirectUpgradeH15Off` (M24) | `CloseDup <- Disabled` | H-15 off: both sides eligible with two circuits; overlapping punches that cancel repeat for ever (B1) | TwoCircuits | `Upgrades` |
| `DirectUpgradeNoClose` (M25) | `ResolveOK <- Disabled` | the duplicate circuit is never closed (B1) | TwoCircuits | `DupResolves` |
| `DirectUpgradeNoAllowance` (M26) | `OAllowOK <- AnyAllow` | another peer's rounds sharing one grant dial beyond its allowance (B3) | TwoPeers | `NoUnallowedDial` |
| `DirectUpgradeGaterNoLease` (M27) | `GaterLease <- NoLeaseNeeded` | a destination without a grant: libp2p's first direct dial (B3) | Honest | `NoUnallowedDial` |
| `DirectUpgradeTicketAny` (M28) | `TicketOK <- AnyTicket` | a peer that is not wanted takes a ticket (B4) | Byzantine | `NoTicketUnwanted` |
| `DirectUpgradePlaceLost` (M29) | `DropTicket <- AbandonDrops` | a round that gave up waiting loses its ticket's place and re-registers at the tail: R starves behind a peer asking for ever (B6) | Answerer | `Upgrades` |
| `HP1rLateLand` (M30, file `mutants/DirectUpgradeLateLand.cfg`) | `LandOK <- LandAny` | a direct connection that lands after the last circuit is gone is kept (H-19): libp2p's one-shot started on a circuit the relay then dropped | LinkLost | `LinkBound` |
| `HP1rWitnessSurvives` (file `mutants/DirectUpgradeWitnessSurvives.cfg`) | invariant `NoDirectAfterRelayLost` | witness: a session that became direct is still direct after the relay dropped its circuit | LinkLost | (must violate) |
| `HP1cRoundAnyConn` (M31, file `mutants/DirectUpgradeRoundAnyConn.cfg`) | `RoundGhost <- RoundAnyConn` | L's rounds ride the connection libp2p picks, the older circuit, a ghost after R's crash (H-20) | Crash | `RoundOnLive` |
| `HP1cCloseOnTimer` (M32, file `mutants/DirectUpgradeCloseOnTimer.cfg`) | `KeepAnswered <- CloseOnTimer` | the proof's timer closes the older circuit whatever the answer (P-15) | ProofTwoCircuits | `ProofKeepsLive` |
| `HP1cResolveNoWait` (M33, file `mutants/DirectUpgradeResolveNoWait.cfg`) | `ResolveWaits <- ResolveNoWait` | H-15 does not wait for the P-15 proofs: after a crossed crash L takes the ghost for one of two circuits and keeps it, closing the live one (counterexample: Dial(LR), RCrash(RL), Resolve) | Crash | `ResolveKeepsLive` |
| `HP1cWitnessGhost` (file `mutants/DirectUpgradeWitnessGhost.cfg`) | invariant `NoRoundBesideGhost` | witness: a round of L starts while a ghost shares its direction (S19 is not vacuous) | Crash | (must violate) |
| `HP1cWitnessAnswered` (file `mutants/DirectUpgradeWitnessAnswered.cfg`) | invariant `NoProofAnswered` | witness: an older live circuit answers its proof (S20 is not vacuous) | ProofTwoCircuits | (must violate) |
| `HP1cWitnessProofClose` (file `mutants/DirectUpgradeWitnessProofClose.cfg`) | invariant `NoProofClose` | witness: P-15 closes a ghost | Crash | (must violate) |
| `DirectUpgradeWitnessDirect` | invariant `NoDirect` | witness: a link becomes direct | Honest | (must violate) |
| `DirectUpgradeWitnessStale` | invariant `NoStaleResult` | witness: a result of an older epoch arrives | Honest | (must violate) |
| `DirectUpgradeWitnessBusy` | invariant `NoBusy` | witness: a call is Busy | Honest | (must violate) |
| `DirectUpgradeWitnessSlow` | invariant `NoSlowStep` | witness: a Spent call moves the slow ladder | Silent | (must violate) |
| `DirectUpgradeWitnessGateRefusal` | invariant `NoGateRefusal` | witness: a Byzantine flood empties the round bucket | Byzantine | (must violate) |
| `DirectUpgradeWitnessWaitBehind` | invariant `NoWaitBehind` | witness: L waits for a slot another peer holds | TwoPeers | (must violate) |
| `DirectUpgradeWitnessCross` | invariant `NoCross` | witness: the pair's two punch phases overlap | TwoCircuits | (must violate) |

M3 is caught by `BusyFree`, not by `Upgrades`: refills are timers under fairness, so a charged Busy only delays.
M11, M16 and the overlap need buckets or leases of 2: with 1, the node bucket or a frozen clock hides them. The
model's allowance of 2 stands for the plan's 12 (one destination unit per step).

```
java -cp tla2tools.jar tlc2.TLC -workers 2 -config DirectUpgradeHonest.cfg DirectUpgrade
java -cp tla2tools.jar tlc2.TLC -workers 2 -config mutants/DirectUpgradeLowerOnly.cfg mutants/DirectUpgradeMutants
INKWELL_TLA_ONLY='^DirectUpgrade' ./scripts/tla-check.sh    # the 44 DirectUpgrade runs only, 273 s (heavy.sh --exclusive)
INKWELL_TLA_ONLY='^HP1r' ./scripts/tla-check.sh             # block HP1r's two runs (H-19), named outside ^DirectUpgrade so HP1's count stays 44
INKWELL_TLA_ONLY='^HP1c' ./scripts/tla-check.sh             # block HP1c's 17 runs (H-20, P-15: DirectUpgrade and Supersede), 194 s; CI2 puts the seven DirectUpgrade ones (`HP1c[!S]*`) on shard 1/3, the ten `HP1cSupersede*` on 3/3
```

## PeerUpAck (block PU1, spec §8.7: a peer is announced only after its acceptance ACK of this stream)

Model: [PeerUpAck.tla](PeerUpAck.tla); plan
[2026-11-16-pu1-peerup-after-acceptance.md](../../plans/2026-11-16-pu1-peerup-after-acceptance.md). Two symmetric
nodes; connections created on the wire and registered by each side on its own time, a side may close one at its
registration (a late link it does not keep, Admit refusing it) while the other registers it before it learns of the
close (the trace found in block HP1r, reproducible on main); one outbound stream per node carrying the session (`out`)
and, while up, a move to another connection (`nxt`, the relay-to-direct upgrade of `stream.go`); libp2p's lazy path
when the peer's protocol is cached (`cached`: NewStream returns at once, no byte of the peer); a silent responder
(libp2p echoes, the handler never answers); a responder stuck in its handler (`StuckNodes`: it accepts and never
decides, and fairness never forces it); Drop (admission off, every connection closed) and re-admission. Messages are
a set; a stream's echo precedes its ACK or reset (`EchoGone`); bytes of an abandoned stream are stale.

Time (P-5): each opening stream has a clock (`age`, `tot`: ticks since the open, NewStream and the ACK together); the
deadline fires at `age = D` **whatever is in flight**. `HonestTiming = TRUE` is the environment assumption « the
deadline is longer than any honest exchange » (the clock runs only while the stream is silent), used for the honest
liveness; `HonestTiming = FALSE` lets the clock run at any moment (slow negotiation, late ACK, stuck handler), used
for every safety run and for Stuck. `TickFair` makes time pass. Acceptance is remembered per connection **and** stream.

Rule operators (overridden by the mutants): `AckReady(m, c) == admit[m] /\ Usable(m, c)` (P-2, P-3: no wait for the
responder's own PeerUp), `ReturnOpens == FALSE` (P-1), `MoveAtReturn == FALSE` (P-6), `AckMatches(st, m) == st.s = m.s`
(P-4: an ACK counts for its own stream), `TimeoutReady(n, st) == st.age = D` (P-5: unconditional), `ResetOnReturn ==
FALSE` (P-5: one budget).

| Property | Meaning |
|---|---|
| `UpAfterAccept` (S1) | a PeerUp is backed by the peer's sound ACK (admitted, connection held) of the very stream that announced it |
| `SessionOnAccepted` (S2) | the session's stream is only ever a stream the peer accepted, moves included |
| `NoUpOnRefused` (S3) | « no PeerUp on either side »: a connection the peer closed at registration, or has not registered, never announces it |
| `Budget` (S4) | an opening stream, NewStream and ACK together, never outlives the deadline (`tot ≤ D`) |
| `NoUpFromStuck` (S5) | a responder stuck in its handler never gets its opener announced |
| `BothUp` (L1, Honest only) | both sides come up: nobody waits for the other's PeerUp |
| `OpenResolves`, `MoveResolves` (L2) | an opening stream always resolves (ACK, reset, connection lost, deadline) |

| Configuration | Run | What it covers |
|---|---|---|
| `PeerUpAckHonest.cfg` | `PeerUpAckHonest` | one connection, both paths, honest timing; S1-S4, L1, L2 |
| `PeerUpAckLate.cfg` | `PeerUpAckLate` (safety) | two connections, the second refusable at registration, protocol cached, a close at will, a Drop/re-admission, the move, any timing; S1-S4 |
| `PeerUpAckLateLive.cfg` | `PeerUpAckLateLive` | the same with one stream per node and honest timing; S1-S4, L2 |
| `PeerUpAckSilent.cfg` | `PeerUpAckSilent` | a silent responder on one connection, both paths; S1-S4, L2 (the deadline) |
| `PeerUpAckStuck.cfg` | `PeerUpAckStuck` | b's handler never decides, any timing, time passes, D = 2, both paths; S1-S5, L2 |
| `PeerUpAckByzantine.cfg` | `PeerUpAckByzantine` (safety) | silent responder, either connection refusable by either side, admission on or off at start and flipped once, both paths, honest timing (any timing is covered by Late, Stuck and Reuse); S1-S4 |
| `PeerUpAckReuse.cfg` | `PeerUpAckReuse` (safety) | three streams per node on two connections, any timing: a move abandoned at its deadline while its ACK is in flight, then another on the same connection; S1-S4 |

| Mutant (mutants/PeerUpAckMutants.tla) | Override | Caught by |
|---|---|---|
| `PeerUpAckUpOnReturn` (M1, P-1 off: today's code) | `ReturnOpens <- ReturnOpensMut` | `NoUpOnRefused` (Late) |
| `PeerUpAckBeforeAdmit` (M2, P-2 off) | `AckReady <- AckAlways` | `UpAfterAccept` (Byzantine) |
| `PeerUpAckAfterOwnUp` (M3, P-3 off) | `AckReady <- AckAfterUp` | `BothUp` (Honest): the deadlock |
| `PeerUpAckMoveNoAck` (M4, P-6 off) | `MoveAtReturn <- MoveAtReturnMut` | `SessionOnAccepted` (Late) |
| `PeerUpAckNoRetention` (M5, P-2's connection check off) | `AckReady <- AckAdmittedOnly` | `UpAfterAccept` (Late) |
| `PeerUpAckTimeoutGuarded` (M6, P-5: the deadline waits for silence) | `TimeoutReady <- TimeoutWhenSilent` | `OpenResolves` (Stuck) |
| `PeerUpAckAckByConn` (M7, P-4: acceptance by connection only) | `AckMatches <- AckSameConn` | `SessionOnAccepted` (Reuse) |
| `PeerUpAckBudgetPerPhase` (M8, P-5: the clock restarts at NewStream's return) | `ResetOnReturn <- ResetOnReturnMut` | `Budget` (Stuck) |
| `PeerUpAckWitnessLazyLate` (witness) | — | `NotLazyLate`: an opener awaits on a connection its peer refused (S3 is not vacuous) |
| `PeerUpAckWitnessBothUp` (witness) | — | `NotBothUp`: both up in Late |
| `PeerUpAckWitnessMove` (witness) | — | `NoMove`: a move completes (S2 covers it) |
| `PeerUpAckWitnessLateFire` (witness) | — | `NoLateFire`: a deadline fires while the stream's bytes are in flight or its handler is deciding (Stuck) |

Not modelled: frames and their order across a move (NI0's `TestRelayThenDirect`, `move_waits_for_ack`), the bulk
lane (opened after PeerUp), real timings (the Go test `TestPeerUpLatency`, netsim). Results: gates/PU1/WITNESSES.md.

## Supersede (block HP1c, spec §8.7.4: a superseded connection proves that it lives, rule P-15)

Model: [Supersede.tla](Supersede.tla); plan [2026-11-23-hp1-reformalize.md](../../plans/2026-11-23-hp1-reformalize.md)
§2. It EXTENDS `PeerUpAck` and leaves it byte for byte as frozen by `gates/PU1c.toml`: every action of PeerUpAck runs
unchanged (`Keep(A) == A /\ UNCHANGED supv`), the registration is wrapped (`RegisterS`) and P-15's steps are added.
PeerUpAck already has the half-open state: a side closes a connection (`Close`, here the peer's crash) and the other
learns it only later (`LearnClose`, the 30 s idle timeout), while its session stays on the dead connection. A kept
registration ranks the connection newest and proves every older usable one with no proof in progress
(`proof[n][c]`: pending, then answered or silent; an answered one is proved again by a later newer connection). The
peer answers (`ProveAnswer`) while it still holds the connection; `ProveExpire` is T_vie passing with no byte, under
`HonestTiming` only if the peer no longer holds it (T_vie is twice a bad round trip), at any time otherwise (a slow
relay). `ProofAcked` latches that a newer connection carries an acknowledged sync stream, either direction (the
node read its ACK, `out`, or wrote one, `acc`); `ProveClose` closes a silent connection after that, never the newest
nor the only one; its streams end by PeerUpAck's own `StreamLost`, no ClosePeer, no event. `ProofEnd` drops a proof
when no newer connection is left. `LearnFair = FALSE` drops the idle timeout's fairness: P-15 alone must take a session
off a dead connection. Bounds that keep the runs small: `Moves = FALSE` (PeerUpAck's relay-to-direct move is not
explored: after a crash the session changes connection by reopening, `StartOpen`; moves stay PU1's runs), and
`CrashBy` names the nodes whose side may close at will (both for safety; b only for the liveness, a's view of b's
crash, the other side being the same by symmetry).

Rule operators (overridden by the mutants): `StartsProof == TRUE`, `Closable(n, c) == proof[n][c].st = "silent"`,
`AckWaitOK(n, c) == proof[n][c].ack`, `SoleOK(n, c) == NewerUsable(n, c)`.

| Property | Meaning |
|---|---|
| `SupersededBounded` (P15-1) | a usable connection with a newer usable one is under proof (or answered since): its life is bounded by T_vie and the newer's ACK, not by the idle timeout |
| `LiveKept` (P15-2, `HonestTiming`) | a connection the peer still holds is never closed by a proof |
| `FallbackKept` (P15-3) | no proof closes a connection unless, when it closes, a newer one still judged alive (its proof not silent) carries an acknowledged sync stream (the newer opening may still fail: the fallback; review 3b: a dead acknowledged middle connection does not count) |
| `NeverCloseSole` (P15-4) | no proof closes the newest connection nor the only one |
| `SessionLeavesDead` (P15-5, liveness) | a session on a dead connection under proof leaves it, unless no newer connection is left, with no idle timeout assumed |

PeerUpAck's `TypeOK`, `UpAfterAccept`, `SessionOnAccepted`, `NoUpOnRefused` and `Budget` are checked in both safety
runs: the proofs break none of PU1's guarantees.

| Configuration | Run | What it covers | Result (TLC 1.8.0, 2 workers, 2026-11-23, `heavy.sh`) |
|---|---|---|---|
| `SupersedeHonest.cfg` | `HP1cSupersede` (safety) | two connections, either side closes one without the other knowing (the crash), lazy path, no move, honest timing; PU1's S1-S4, P15-1..P15-4 | **exhaustive, no error**: 3,231,721 states, 755,680 distinct, depth 31, invariants, 19 s |
| `SupersedeAnyTiming.cfg` | `HP1cSupersedeAnyTiming` (safety) | one close at will, no move, any timing (a deadline may close the newer connection after its ACK); PU1's S1-S4, P15-1, P15-3, P15-4 | **exhaustive, no error**: 6,611,059 states, 1,273,276 distinct, depth 27, invariants, 29 s |
| `SupersedeThree.cfg` | `HP1cSupersedeThree` (simulation, block HP1c2) | AnyTiming's constants with three connections (review 3b: a dead acknowledged middle connection); P15-1, P15-3, P15-4 | **TLC simulation, no error**: seed 20771, depth 60, 100,000 traces per worker (200,000, 5,979,513 states, 52 s). Not exhaustive: Honest and AnyTiming on three, even with one crashing side or no close, exceed 300 s with their queues still growing (residual → HP1). The same simulation catches `HP1cSupersedeBackwardAck` in 3 s |
| `SupersedeLive.cfg` | `HP1cSupersedeLive` | Honest's constants, only b's side may close, no idle timeout assumed; P15-1, P15-2, liveness P15-5 | **exhaustive, no error**: 1,624,745 states, 380,888 distinct, depth 31, liveness included, 1 min 31 s |

| Mutant (mutants/SupersedeMutants.tla) | Override | Caught by |
|---|---|---|
| `HP1cSupersedeNoProof` (MS1, P-15 off) | `StartsProof <- NoProofMut` | `SupersededBounded` (Honest) |
| `HP1cSupersedeCloseOnTimer` (MS2: the timer closes whatever the answer) | `Closable <- CloseOnTimerMut` | `LiveKept` (Honest) |
| `HP1cSupersedeIgnoreAckWait` (MS3: no wait for the newer's ACK) | `AckWaitOK <- IgnoreAckWaitMut` | `FallbackKept` (Honest) |
| `HP1cSupersedeProveSole` (MS4: the newest or only connection closed; since HP1c2 as MP2, the newest judged like the older ones, its own ACK counting) | `SoleOK <- ProveSoleMut`, `AckWaitOK <- OwnAckMut` | `NeverCloseSole` (AnyTiming: the newer connection closes after its ACK). With `SoleOK` alone it is equivalent since review 3b: the close re-checks a newer usable connection judged alive and acknowledged, so `FallbackKept` implies `NeverCloseSole` |
| `HP1cSupersedeBackwardAck` (MS5, review 3b: any newer acknowledged connection, a silent one included, lifts the exemption) | `AckWaitOK <- BackwardAckMut` | `FallbackKept` (AnyTiming's constants with three connections, its own configuration `mutants/SupersedeBackwardAck.cfg`: the oldest closes while the newest awaits its first ACK; the vector `supersede_dead_middle_acked_keeps_older`), 32 s |
| `HP1cSupersedeWitnessProofClose` (witness) | — | `NoProofClose`: a proof closes a dead connection (Honest) |
| `HP1cSupersedeWitnessAnswer` (witness) | — | `NoAnswer`: a live older connection answers its proof (Honest) |
| `HP1cSupersedeWitnessFallback` (witness) | — | `NoFallbackWait`: a silent proof waits while the newer connection's opening awaits its ACK (Honest; the vector `supersede_newer_unacked_keeps_older`) |

Not modelled: the ping's bytes and T_vie's value (vectors `supersede.json`, `protocol/syncack.Supersede`), an
exhaustive check of the sound rule on three connections or more (only `HP1cSupersedeThree`'s seeded simulation, and the
vectors `supersede_middle_acked`, `supersede_dead_middle_acked_keeps_older`, `supersede_answered_reproved`,
`supersede_loop`; residual → HP1), a Byzantine peer
that answers on a connection it no longer uses (it keeps only its own connection open), real timings (the Go tests of
plan §2).

## CloseScope (block PU1d, spec §8.7.4: a failed opening closes only the connection it names, rules P-16 and P-17)

Model: [CloseScope.tla](CloseScope.tla); plan [2026-11-24-pu1d-close-scope.md](../../plans/2026-11-24-pu1d-close-scope.md).
It EXTENDS `PeerUpAck` and leaves it byte for byte as frozen by `gates/PU1b.toml` and `gates/PU1c.toml`, as Supersede
does. PeerUpAck already closes only the connection a failed session opening ran on (`CloseC` in `RecvReset` and
`Timeout`; their comments still say « ClosePeer », written before PU1d) and already lets any usable connection start an
opening (`StartOpen`), on a connection it picks. This module restates the two session-stream failure branches with
`CloseNamed` (`RecvResetSession`, `TimeoutSession`: the named connection closed by `FailClose`, or, unnamed, all of
them by `CloseAll`, the code before PU1d; the other branches are PeerUpAck's own, selected by
`out' = out`), guards `StartOpen` with `OpenGuard`, and runs every other action unchanged (`Keep`). `evt[n]` abstracts
libp2p's connectedness event: raised on the edge from no usable connection to one, dropped with the last, consumed by an
opening; it is kept until consumed, kinder to MC2 than libp2p (an event that finds an opening in progress is lost in
inkwell's `onConnected`). `Moves = FALSE` (a move's failure closes nothing either way: PU1's runs), `CrashBy` as in
Supersede. Violations (a wrong byte, a byte after the ACK) are not modelled: same rule, carried by the vectors.

Rule operators (overridden by the mutants): `FailClose(n, c) == CloseC(n, c)` (P-16), `Names(st) == st.c` (P-16: the
opening names the connection it runs on from its start, `network.Conn.NewStream`), `OpenGuard(n) == TRUE` (P-17).

| Property | Meaning |
|---|---|
| `CrossedIndependent` (S6, P-16) | a failed session opening never closes another connection that carries the peer's acknowledged session (§8.7.5: two crossed openings are independent) |
| `OpeningNamed` (S7, P-16) | an opening in flight, NewStream not returned included, always names the connection it runs on (review finding 1) |
| `NeverStranded` (L3, P-17) | a node that admits its peer, holds a usable connection and has no session stream and none opening does not stay so |

PeerUpAck's `TypeOK`, `UpAfterAccept`, `SessionOnAccepted`, `NoUpOnRefused` and `Budget`, and `TypeS`, are checked in
both runs.

| Configuration | Run | What it covers | Result (TLC 1.8.0, 2 workers, 2026-11-24, `heavy.sh`) |
|---|---|---|---|
| `CloseScopeAny.cfg` | `PU1dCloseScopeAny` (safety) | two connections both sides keep, lazy path, a Drop/re-admission, no move, any timing (a deadline on an honest exchange: a stalled relay, a dead link); S1-S4, S6, S7 | **exhaustive, no error**: 7,407,601 states, 1,345,391 distinct, depth 34, 33-41 s |
| `CloseScopeNegotiated.cfg` | `PU1dCloseScopeNegotiated` (safety) | two connections, the negotiated path (protocol not cached: NewStream returns after the echo, a deadline may fire before), two openings per node, no Drop, no move, any timing; S1-S4, S6, S7 | **exhaustive, no error**: 17,257,071 states, 2,484,923 distinct, depth 37, 1 min 23 s (with Any's Drop: 116,425,537 states, 14,766,859 distinct, depth 38, 9 min, no error, run once) |
| `CloseScopeLive.cfg` | `PU1dCloseScopeLive` | b's side of one connection may close without a knowing (a crash), honest timing, time passes; S1-S4, S6, S7, L3, L2 `OpenResolves` | **exhaustive, no error**: 63,469 states, 19,957 distinct, depth 28, liveness included, 11 s |

| Mutant (mutants/CloseScopeMutants.tla) | Override | Caught by |
|---|---|---|
| `PU1dCloseScopeClosePeer` (MC1, P-16 off: `ClosePeer` on a refusal or the deadline, spec and code before PU1d) | `FailClose <- FailClosePeer` | `CrossedIndependent` (Any: a's opening on a connection b has not registered yet times out, and ClosePeer closes the one carrying b's acknowledged session; depth 13) |
| `PU1dCloseScopeOnEventOnly` (MC2, P-17 off: an opening starts only on a connectedness event, inkwell before the lobby fix of 2026-11-23) | `OpenGuard <- OpenOnEvent` | `NeverStranded` (Live: the session's connection is lost, the remaining one emits no event, both nodes stay with no stream; depth 16) |
| `PU1dCloseScopeUnnamedOpen` (MC3, review finding 1: the connection named only when NewStream returns, host.NewStream as in today's code) | `Names <- NamesAtReturn` | `OpeningNamed` (Negotiated; depth 4) |
| `PU1dCloseScopeUnnamedCollateral` (MC3's consequence) | `Names <- NamesAtReturn` | `CrossedIndependent` (Negotiated: a deadline before the return names no connection, so the node closes them all, the peer's session one included; depth 14) |
| `PU1dCloseScopeWitnessBeside` (witness) | — | `NoFailBeside`: a session opening fails while the peer's acknowledged session runs on another connection (Any; S6 is not vacuous) |
| `PU1dCloseScopeWitnessQuiet` (witness) | — | `NoQuietOpen`: an opening starts with no connectedness event pending (Live; the state MC2 strands) |
| `PU1dCloseScopeWitnessNeg` (witness) | — | `NoUnreturnedFail`: a session opening fails before NewStream returns (Negotiated; S7 is not vacuous) |

Not modelled: the responder's reset on a connection it is closing (§8.7.3 step 3; PeerUpAck sends nothing then), whose
race with the close made a closing connection's reset read as a refusal: under P-16 the two readings close the same
connection (review finding 2; Go test `TestResetBeforeConnCloseKeepsOther`, step 2); the reopening's real timings (Go
tests of the plan). Ten runs (`INKWELL_TLA_ONLY='^PU1d'`): 139 s with `heavy.sh` (2026-11-24).

## WindowClose (WC1)

[WC1 window-close contract](../../plans/2026-11-21-wc1-window-close.md) refines the close aspect
of OISingleInstance's D-DRAIN. One attempt coordinates a single editor or a parallel drain of
both on quit. Model checking bounds edit prefixes and attempt IDs. `InputFrozen` preserves the
captured cut. `AckIsDurable` compares the cut to the durable prefix at save ACK. Normal close
requires that prefix, the current completion ACK and the presence handoff. Group authorization
requires all targets to be ready before any permit may pass the hook. `OnlyWithConsent`
checks that forced authorization and release have a matching explicit choice from the
abort notice, recorded independently of permit issuance.

`WindowClose` explores failures, retries and permit consumption. The first close event for
an approved window consumes its permit regardless of event origin. Revocation clears any
remaining permits before input resumes. Its fair clock can reach the shared deadline,
after which Expire aborts the attempt. `WindowCloseHealthy`
requests quit with healthy participants and assumes the prerequisites can complete within the
time budget. Weak fairness covers Prepare, Write, SaveAck, launch handling, Goodbye, Ack and
authorization/consumption. `HealthyCloseSucceeds` requires normal closes for all targets.
`WindowCloseCrashed` starts with absent renderers and a fair request and explicit force choice.
`ForceClosesCrashedWindow` requires native teardown in that environment. Go and native tests
cover clock bounds in seconds, notice actions, transport barriers and OS quit routing.
The WC1 gate runs these models through `scripts/wc1-tla.sh` using the shared pinned TLC
distribution.

| TLC run | Counterexample |
|---|---|
| `WindowCloseSkipWait` | `NoAcknowledgedKeystrokeLost`: destroy with buffered edits |
| `WindowCloseTimeoutCloses` | `NoAcknowledgedKeystrokeLost`: the timer destroys before the write |
| `WindowCloseNoGoodbye` | `GoodbyeBeforeClose`: the save ACK authorizes close without presence |
| `WindowCloseStaleAck` | `CloseWaitedForAck`: a previous attempt's ACK authorizes a retry |
| `WindowCloseTypeWhileHeld` | `InputFrozen`: accept a keystroke after capturing the cut |
| `WindowCloseAckBeforeWrite` | `AckIsDurable`: acknowledge the cut before committing it |
| `WindowClosePartialQuit` | `GroupWaitedForAll`: authorize one editor while the other is still draining |
| `WindowCloseNoWrite` | `HealthyCloseSucceeds`: a healthy request stalls on a dirty cut |
| `WindowCloseForceNeedsLive` | `ForceClosesCrashedWindow`: an unavailable renderer prevents explicit force |
| `WindowCloseForceWithoutConsent` | `OnlyWithConsent`: issue a forced permit straight from the abort notice |

## SyncOrder (block PU1b, spec §8.7.3-§8.7.4: the order of a peer's inbound sync streams, two places)

Model: [SyncOrder.tla](SyncOrder.tla); plan
[2026-11-16-pu1b-inbound-order.md](../../plans/2026-11-16-pu1b-inbound-order.md). One responder, one peer's inbound
sync streams: each takes a place in the peer's order on arrival, before its ACK; at most `Cap` places, shared by the
peer's connections and generations; a stream reads only once nobody ahead of it holds a place (`waitFor = 0`); a
stream leaves when it ends — at once, wherever it stands, if it delivered nothing (reset, or closed for writing with
nothing to read), and its successor then waits for
the stream ahead of it (the wake) — or, having delivered frames, once read to its end. Openers: honest (one session
stream, the move relay → direct that swaps after the ACK and closes the old stream for writing, the direct connection
dropping, the reopen on the relay: drain + replacement + reconnect, where a third place is asked for and refused for
capacity, then retried) or Byzantine (opens, writes, ends, resets and holds any stream at will). `Stall` lets a
waiting stream give the order up (`reorderWait`). Time: an opening has a clock (`spent`, ticks since its first open,
retries included); a retry after a capacity refusal needs a tick first (`wait` → `due`: the backoff); at `spent = D`
the budget is out: a session opening closes the peer (`closedBy = "budget"`, every stream of the peer reset), a move
is abandoned. `HonestTiming` (time passes only while neither the responder nor a due retry can act) is the honest
liveness assumption; `SyncOrderAnyTiming` drops it. Its runs are named `SyncOrder*`, outside PU1's `^PeerUpAck` set.

Rule operators (overridden by the mutants): `PlaceFree == Len(list) < Cap` (P-8), `EarlyOK(s) == TRUE` (P-9: a
stream that delivered nothing leaves at once), `Ended(s) == rstd[s] \/ (fin[s] /\ buf[s] = <<>>)` (P-9: a reset or an
end of writing with nothing to read), `Rewire(s) == waitFor[s]` (P-9: the wake), `OnBusyKill == FALSE` (P-8: a
capacity refusal never closes the peer).

| Property | Meaning |
|---|---|
| `Places` | at most two streams of the peer hold a place |
| `Order` (Honest) | the frames are delivered in the order the opener wrote them, across the move and the reconnect |
| `SessionKept` (Honest, AnyTiming) | a capacity refusal never closes the peer (only the budget's end does) |
| `Budget` (Honest, AnyTiming) | an opening, retries included, never outlives its budget |
| `EmptyLeaves` | a stream that ended (reset, or closed for writing with nothing to read) without delivering a frame does not keep its place (fair responder) |
| `AllDelivered` (Honest) | every frame still in a stream is eventually delivered |
| `SessionResumes` (Honest) | after the drop, the session comes back (the capacity refusal is retried) |

| Configuration | Run | What it covers |
|---|---|---|
| `SyncOrderHonest.cfg` | `SyncOrderHonest` | honest opener, three streams, a move, a drop of the direct connection, a reopen, honest timing, D = 3; all properties |
| `SyncOrderAnyTiming.cfg` | `SyncOrderAnyTiming` (safety) | the same under any timing, D = 2: the budget may run out during a capacity refusal; invariants |
| `SyncOrderByzantine.cfg` | `SyncOrderByzantine` | Byzantine opener, three streams, a waiting stream may give the order up; `Places`, `EmptyLeaves` |

| Mutant (mutants/SyncOrderMutants.tla) | Override | Caught by |
|---|---|---|
| `SyncOrderNoWake` (Q1, P-9: no wake) | `Rewire <- RewireStale` | `AllDelivered` (Honest): the successor of a drained stream waits for it for ever |
| `SyncOrderKeepEmpty` (Q2, P-9: the place kept) | `EarlyOK <- EarlyAtTurn` | `EmptyLeaves` (Byzantine): a stream reset behind a head held open keeps its place |
| `SyncOrderClosePeerOnBusy` (Q3, P-8: ClosePeer on capacity) | `OnBusyKill <- OnBusyKillMut` | `SessionKept` (Honest) |
| `SyncOrderBound3` (Q4, P-8: three places) | `PlaceFree <- PlaceFree3` | `Places` (Byzantine) |
| `SyncOrderEofKeeps` (Q5, P-9: an empty end of writing keeps its place) | `Ended <- EndedByReset` | `EmptyLeaves` (Byzantine) |
| `SyncOrderWitnessBusy` (witness) | — | `NoBusy`: an honest opener meets a capacity refusal (drain + replacement + reconnect) |
| `SyncOrderWitnessRemoveWaiting` (witness) | — | `NoRemovalWhileWaiting`: a stream leaves while its successor waits for it |
| `SyncOrderWitnessEmptyEof` (witness) | — | `NoEmptyEof`: a waiting stream closed by its opener without a frame leaves the order (Byzantine) |
| `SyncOrderWitnessBudgetOut` (witness) | — | `NoBudgetOut`: the budget runs out during a capacity refusal and the session opening closes the peer (AnyTiming) |

Not modelled: the exact waits (50 ms doubled to 1 s: the vectors), the bytes of the ACK (PeerUpAck), real timings. Results: gates/PU1b/WITNESSES.md.

## InboundLifecycle (block PU1c, spec §8.1, §8.7.3, §16.1: one lifecycle for the inbound streams of both ordered lanes)

Model: [InboundLifecycle.tla](InboundLifecycle.tla); plan
[2026-11-17-pu1c-inbound-lifecycle.md](../../plans/2026-11-17-pu1c-inbound-lifecycle.md). One peer's inbound streams
on **both** ordered lanes at once (sync and bulk), each lane with its order (`Cap` places), its receive budget and its
consumer, all on **one** QUIC connection whose credit (`ConnWindow` units) they share; a stream holds at most `Win`
units of it, its unread units plus, until its turn, the units its probe took (no window update below a quarter of a
window). Each stream has one reader: `arrived` → `refused` | `accepting` (the ACK, both lanes) → `waiting` → `kept`
| `delivering` ⇄ `blocked` → `gone`. Waiting, the reader probes: a reset is seen (it wins over unread units); else a
unit is taken (at most `ProbeMax`); else an end with nothing unread is seen: an empty prefix leaves at once, a
non-empty one is kept to its turn and given to the decoder, which delivers whole frames and drops a truncated last
one. The turn needs no stream ahead (or the order given up, `Stall`) and the peer up. Delivering charges each whole
frame to the lane's budget; a full budget blocks; a blocked reader is freed by room or by the loss of its connection,
never by its own reset. Openers: honest per lane (session stream, a reopen that swaps after the new stream's ACK,
streams arriving before PeerUp: the late handler, the session's end), Byzantine per lane (opens, whole and partial
frames, FIN, reset, RESET_STREAM_AT, holds), Greedy streams filled to their window by any opener. Its runs are named
`InboundLifecycle*`, outside PU1's `^PeerUpAck` and PU1b's `^SyncOrder` sets; `SyncOrder.tla` and `PeerUpAck.tla` are
untouched.

Rule operators (overridden by the mutants): `PlaceAtArrival(l) == TRUE` (P-6, P-14), `EarlyOK(s) == TRUE` (P-9),
`ProbeOK(s) == TRUE` (P-11), `WaitCharges == FALSE` (P-12), `KeepPrefix(s) == TRUE` (P-11), `CancelOnLoss == TRUE`
(P-12), `ConnWindow == Cap * 2 * Win` (P-13: the streams not read besides a head are at most `Cap * 2 - 1`; the
spec's inequality adds one arriving or refused stream and divides by 1 − θ).

| Property | Meaning |
|---|---|
| `Places` | at most two places per lane |
| `Order` (Honest) | each honest lane's frames reach its consumer in the order written, across late handlers and reopens |
| `WaitNoBudget` | a stream holds budget only from its turn |
| `ArrivingHoldsNothing` (Honest, Credit) | on an honest peer's connection a stream arriving, accepting or refused holds no credit (its opener writes after the ACK): the « + 1 » of P-13's inequality is a margin; a refused stream's units are released by a step of their own (`Release`, the reset processed) |
| `ReservationsReturned` | a stream that left holds no reservation |
| `FinFramesDelivered` | a stream that ended without an abrupt reset and left delivered every whole frame written on it |
| `EmptyLeaves` | a stream that ended before its turn with nothing to read (nothing unread, nothing probed) leaves its place (fair reader) |
| `NoBlindWait` | an accepting or waiting stream whose stream was reset (or whose connection was lost) leaves that state, even after its probe took units |
| `BlockedFreed` (Byzantine) | a reader blocked on the budget is freed by its connection's loss |
| `AllDelivered` (Honest), `HeadProgresses` (Credit) | every frame of an honest lane is delivered; in Credit, with three other streams of both lanes holding their windows and the bulk head blocked |

| Configuration | Run | What it covers |
|---|---|---|
| `InboundLifecycleHonest.cfg` | `InboundLifecycleHonest` | both lanes honest, late handlers, a reopen per lane, the sessions end; consumers fair; all safety, `AllDelivered`, `EmptyLeaves`, `NoBlindWait` |
| `InboundLifecycleByzantine.cfg` | `InboundLifecycleByzantine` | a Byzantine opener on a head and a waiting stream of one lane — an over-approximation of two Byzantine lanes on one credit (plan §6, « Composition of the two lanes »: the other lane acts only by holding credit, which only disables this lane's writes, and those are unfair choices already) —, every end kind in every state, a stalled consumer, `Stall`, a connection loss; safety, `EmptyLeaves`, `NoBlindWait`, `BlockedFreed` |
| `InboundLifecycleByzantineSafety.cfg` | `InboundLifecycleByzantineSafety` (safety) | the same on three streams (a third place refused), a probe keeping two units (a whole and a truncated frame); invariants |
| `InboundLifecycleCredit.cfg` | `InboundLifecycleCredit` | the sync head honest; a second sync stream and both bulk streams filled to their window before the head writes; the bulk consumer stalled; `HeadProgresses` |

| Mutant (mutants/InboundLifecycleMutants.tla) | Override | Caught by |
|---|---|---|
| `InboundLifecyclePlaceSyncLate` (QC1, finding 1: the place after the up-wait) | `PlaceAtArrival <- PlaceSyncLate` | `Order` (Honest) |
| `InboundLifecycleFailedWaits` (QC2, finding 2: a failed stream waits for its predecessor) | `EarlyOK <- EarlyAtTurn` | `EmptyLeaves` (Byzantine) |
| `InboundLifecycleProbeOnce` (QC3, finding 3: the probe blind after a byte) | `ProbeOK <- ProbeOnce` | `NoBlindWait` (Byzantine) |
| `InboundLifecycleWaitCharges` (QC4, finding 5: a waiting stream charges the budget) | `WaitCharges <- WaitChargesMut` | `WaitNoBudget` (Honest) |
| `InboundLifecycleKeepWholeOnly` (QC5, review: a prefix with a truncated tail dropped) | `KeepPrefix <- KeepWholeOnly` | `FinFramesDelivered` (ByzantineSafety) |
| `InboundLifecycleNoCancel` (QC6, review: the budget wait blind to the connection's loss) | `CancelOnLoss <- CancelOnLossMut` | `BlockedFreed` (Byzantine) |
| `InboundLifecycleBulkTail` (QC7, `bulkTail`: bulk place after the up-wait, failed bulk stream waits) | `PlaceAtArrival <- PlaceBulkLate`, `EarlyOK <- EarlyBulkAtTurn` | `Order` (Honest) |
| `InboundLifecycleSmallWindow` (QC8, P-13 off: the connection window too small) | `ConnWindow <- ConnWindowSmall` | `HeadProgresses` (Credit) |
| `InboundLifecycleWitnessKept` (witness) | — | `NoKept`: an end after a probed prefix keeps the place |
| `InboundLifecycleWitnessBlocked` (witness) | — | `NoBlocked`: a reader blocks on a full budget |
| `InboundLifecycleWitnessTruncatedTail` (witness, ByzantineSafety) | — | `NoTruncatedTail`: whole frames delivered, then a truncated last one dropped |
| `InboundLifecycleWitnessExhausted` (witness) | — | `NoCreditExhaustedFirst`: the other streams hold all but the head's share before the head writes |

Not modelled: real times — the temporal properties prove « eventually »; every bound (T = 250 ms, at once, 60 s, 30 s) is proved by Go arms on controlled time (`probe_period`, `wake_within_bound`, `immediate_when_first_and_up`, `interrupt_during_transition`, `matrix/*`) and by the vectors —, byte sizes (units), the auto-tuning of
windows (the stated limit of P-13), the opener's retries (SyncOrder). Results: gates/PU1c/WITNESSES.md.

## RelayFanout (block RF1, spec §16.1: members are never refused because others share their relay or their address; rules RF-1 to RF-6)

Model: [RelayFanout.tla](RelayFanout.tla); plan [2026-11-24-rf1-relay-fanout.md](../../plans/2026-11-24-rf1-relay-fanout.md)
(revision 2, after review round 1: the token bucket modelled, the member share added).

The model has one node and the admission layer around its resource manager. An origin is what a pending inbound
connection is charged to: a relay named by its authenticated PeerID (RF-1), or an IP subnet. The model treats both
alike; the Go tests check how the origin is derived from an endpoint.

- **Honest members.** A member opens inbound connections from its origin (`MemberOpen`). Each stays pending until its
  handshake (`MemberAuth`), which makes it a member connection out of every pending count and removes its token
  debt if still unpaid (RF-2, RF-5). A passage ends (`Settle`), and member connections close finitely often (`Close`).
- **The node.** It dials its members (`XDial`). An outbound connection is a member connection from its start (RF-2).
- **A Byzantine member.** It opens and closes authenticated connections at will, never fair (`ByzOpen`, `ByzClose`).
- **A stranger.** Any number of PeerIDs behind each flooded origin, never fair, open pending connections (`AttOpen`).
  Each one ends within the handshake deadline (`AttExpire`, weakly fair: RF-4).

Quotas and capacity:
- `P` pending connections per origin and `Q` in all (RF-3).
- A token bucket of burst `B` per origin, one period at a time (`Refill`, weakly fair: time passes). The bucket is
  kept as debts: unpaid stranger admissions `sd`, pending members' tokens `held`, and tokens never given back `od`.
  A period pays any one debt, so every payment order is explored (RF-5). Ghost `g`, the strangers' admissions minus
  the periods from its lowest point, gives `RateBound` as « at most `B + k` stranger admissions in any `k` periods ».
- `MemberCap` member connections, of which 2 are each member's share. A member's third or fourth connection
  (`PerCap`) only uses the surplus, never another member's unused share (RF-6).
- The node's capacity is `MemberCap + Q`.

`ev` records the last refusal, with the reasons the truth gives at that moment (`Why`). The truth operators are
never overridden.

Rule operators (overridden by the mutants):
- `OriginLoad(o) == PendOn(o)` and `GlobalLoad == Pending`;
- `Room(m)` (the share, then the surplus), `OutOK(m)` and `AuthOK(m)`;
- `SysCap == MemberCap + Q`;
- `RefillStep(o)` (one debt paid per period) and `RefundStep(m)` (only the member's own unpaid debt);
- the switches `Expires`, `UseOrigin` and `UseToken`, all `TRUE`.

| Property | Meaning |
|---|---|
| `OriginBound` (S1, RF-3) | one origin's strangers and pending members hold at most `P` places (the threat model: a flood through one relay) |
| `GlobalBound` (S2, RF-3) | pending connections hold at most `Q` places |
| `RateBound` (S3, RF-5) | at most `B + k` stranger admissions per origin in any `k` periods |
| `MembersSpendNoRate` (S4, RF-5) | no member token stays missing after its handshake |
| `MemberCapBound` (S5, RF-6) | member connections stay within `MemberCap` |
| `InJustified` (S6, RF-1, RF-2) | a member's inbound opening is refused only for a reason the truth gives (load on its origin or in all, the rate, the member capacity), never because other members already hold connections through the same relay or address |
| `OutNeverRefused` (S7, RF-2, RF-6) | the node's dial to an honest member is never refused: neither a stranger nor a Byzantine member can starve it |
| `AuthNeverRefused` (S8, RF-6) | an honest member's authenticated handshake is never refused |
| `SharesKept` (S9, RF-6) | every roster member's unused share stays free: a Byzantine member's surplus never takes it, and when the roster grows the node sheds surplus until it holds again |
| `MembersEventuallyUp` (L1, RF-2, RF-4) | every honest member ends with a connection to the node and keeps one |

`TypeOK` (the bucket between 0 and `B` included) is checked in every sound run.

| Configuration | Run | What it covers | Result (TLC 1.8.0, 2 workers, 2026-11-24, `heavy.sh`) |
|---|---|---|---|
| `RelayFanoutFlood.cfg` | `RF1Flood` | three honest members behind origin 1, a Byzantine member, a stranger flooding origins 1 and 2 forever, the node dials; `P` 2, `Q` 3, `B` 2, `MemberCap` 9, `PerCap` 4; S1-S9, L1 | **exhaustive, no error**: 1,657,395 states, 163,866 distinct, depth 25, liveness included, 35 s |
| `RelayFanoutInbound.cfg` | `RF1Inbound` | three members behind origin 1, inbound only, a stranger that opens two connections and stops; `MemberCap` 6; S1-S9, L1 | **exhaustive, no error**: 9,331 states, 2,108 distinct, depth 16, under 1 s |
| `RelayFanoutGrowth.cfg` | `RFRGrowth` (review round, outside the gate's 26) | roster growth: member 3 joins the membership after the Byzantine member 4 took the surplus (`MemberCap` 8, three shares), no stranger; the node sheds what members hold above their own (`AdmitLate`, `ShedStep`: any such choice, the code's among them); S1-S9, L1 | **exhaustive, no error**: 25,453 states, 3,010 distinct, depth 16, liveness included, 1 s |
| `RelayFanoutMixed.cfg` | `RF1Mixed` | four members, two behind a relay and two behind a home router, a stranger flooding the relay forever, the node dials; `Q` 4, `MemberCap` 9; S1-S9, L1 | **exhaustive, no error**: 406,549 states, 44,100 distinct, depth 20, 11 s |

| Mutant (mutants/RelayFanoutMutants.tla) | Override | Caught by (trace length) |
|---|---|---|
| `RF1PerIPCount` (MF1, libp2p before RF1: every connection counted under its origin for its whole life, outbound ones and circuits under the relay's address) | `OriginLoad <- OriginLoadLibp2p`, `OutOK <- OutOKLibp2p` | `InJustified` (Flood, 4) |
| `RF1PerIPCountLive` (MF1) | the same | `MembersEventuallyUp` (Inbound, 12: beyond `P` members behind one origin, the next never connects) |
| `RF1OutboundCharged` (MF2, an outbound opening charged to the pending quota until its handshake) | `OutOK <- OutOKCharged` | `OutNeverRefused` (Flood, 5) |
| `RF1OutboundStarved` (MF2) | the same | `MembersEventuallyUp` (Flood, 25: the stranger keeps the quota full) |
| `RF1NoDeadline` (MF3, RF-4 off) | `Expires <- Off` | `MembersEventuallyUp` (Inbound, 13) |
| `RF1NoOriginQuota` (MF4) | `UseOrigin <- Off` | `OriginBound` (Flood, 5) |
| `RF1NoGlobalQuota` (MF5) | `UseGlobal <- Off` | `GlobalBound` (Flood, 5) |
| `RF1NoToken` (MF6, RF-5 off) | `UseToken <- Off` | `RateBound` (Flood, 5) |
| `RF1NoRefund` (MF7, a member's handshake keeps its token) | `RefundStep <- RefundNone` | `MembersSpendNoRate` (Flood, 3) |
| `RF1SharedCap` (MF8, one capacity for members and pending connections, the product's 128 before RF1) | `SysCap <- SharedSysCap` | `OutNeverRefused` (Flood, 12) |
| `RF1AuthShared` (MF9, a handshake checked against every connection, pending ones included) | `AuthOK <- AuthOKShared` | `AuthNeverRefused` (Flood, 13) |
| `RF1NoShare` (MF10, a third or fourth connection checked against the total only) | `Room <- RoomNoShare` | `SharesKept` (Flood, 5: the Byzantine member takes an honest member's share) |
| `RF1NoMemberCap` (MF11) | `Room <- RoomNoCap` | `MemberCapBound` (Flood, 14) |
| `RF1RefundUnheld` (MF12, blind refund `tokens = min(B, tokens + 1)`: a token a period already paid is given back again) | `RefundStep <- RefundUnheld` | `RateBound` (Flood, 8) |
| `RF1WindowRefill` (MF13, round 1's fixed window: a period refills the whole bucket) | `RefillStep <- RefillWindow` | `RateBound` (Flood, 8: up to `2B` across a boundary) |
| `RFRNoShed` (MFR1, review round: no shed at a roster growth) | `ShedStep <- ShedNone` | `SharesKept` (Growth, 5: member 3 joins while the Byzantine member holds the surplus, and 3's share is not there) |
| `RF1WitnessInRefused` (witness) | — | `NoInRefusal`: a member's inbound opening is refused under the flood (S6 is not vacuous) |
| `RF1WitnessRateRefused` (witness) | — | `NoRateRefusal`: the stranger drains the members' bucket and a member is refused for the rate (review item 1's case; L1 holds through the node's dials) |
| `RF1WitnessQuotaFull` (witness) | — | `NoFullQuota`: the stranger fills the pending quota |
| `RF1WitnessPassage` (witness) | — | `NoPassage`: a member holds two connections |
| `RF1WitnessSurplus` (witness) | — | `NoSurplus`: the Byzantine member uses the surplus |
| `RF1WitnessByzRefused` (witness) | — | `NoByzRefusal`: the Byzantine member is refused once only shares are left (S9 bites) |
| `RF1WitnessPaidPending` (witness) | — | `NoPaidPending`: a period pays a pending member's token before its handshake (MF12's state) |
| `RF1WitnessResidual` (witness of the plan's residual → RLY1, its own constants: Inbound's with the stranger never stopping) | — | `MembersEventuallyUp`: inbound only and an endless flood on the members' origin deny them (L1 relies on the node's dials) |

Twenty-six runs (`INKWELL_TLA_ONLY='^RF1'`): 77 s with the Alloy leg of `mise run formal` (2026-11-24, `heavy.sh`).
The code review's round added two (`INKWELL_TLA_ONLY='^RFR'`, kept apart from the 26 that `gates/RF1.toml` froze):
`RFRGrowth` and `RFRNoShed`. The constant `Late` (`{}` in every other configuration) and the variable `admitted` (the
roster: `Owed` and `SharesKept` count its members only) model the membership growing.

Not modelled:
- how an origin is derived from an endpoint (a circuit's `/p2p/<relay>/p2p-circuit`, a /32 or /56): Go test
  `TestCircuitOriginIsRelayPeer`, mutant MG1;
- the remote end's own admission of the node's dial: each node runs the same rules (plan §4);
- invitees admitted by the invitation window: they stay pending until closed, like a stranger;
- a Byzantine member's own pending phase: it authenticates at once and gains nothing from it;
- the relay's bounds (RF-7): Go tests at the bound;
- real times (15 s, 5 s per token, the join and recovery times of plan §4): Go tests on controlled time and the
  simulations `docs/plans/2026-11-24-rf1/rf1_wave.py` and `rf1_recover.py`.
