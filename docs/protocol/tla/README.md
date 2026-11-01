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
toujours** ; un changement de génération. Propriété : `ControlLive` (toute trame de contrôle
voulue finit par être lue). Résultats (TLC exhaustif, 2026-11-01) :

| Variante | Résultat |
|---|---|
| règles de la spec (`PullLarge = TRUE`, `CancelOld = TRUE`) | aucune erreur (13 987 états, 3 516 distincts) |
| LARGE ouvert sans LARGE_PULL (`PullLarge = FALSE`) | `ControlLive` violée : REPO et LARGE non lus prennent tout le crédit |
| ancien flux REPO non annulé (`CancelOld = FALSE`) | `ControlLive` violée |

Non couverts : les valeurs réelles des fenêtres (règle « connexion ≥ flux + 4 Mio », vérifiée
par un test au démarrage contre la version épinglée de go-libp2p), yamux (sans fenêtre de
connexion, le cas facile).

## Lancer

```
java -cp tla2tools.jar tlc2.TLC -workers 4 -config RepoGenSmall.cfg RepoGen   # exhaustif, ~5 s
java -cp tla2tools.jar tlc2.TLC -workers 4 RepoGen                             # RepoGen.cfg, long
java -cp tla2tools.jar tlc2.TLC -workers 2 ConnCredit                          # exhaustif, ~1 s
```

`mise run tla` (ADR 0034, étape 3) récupère `tla2tools.jar` épinglé par SHA-256 et lance les
deux configurations ; c'est une porte avant la bascule.
