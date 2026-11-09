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
