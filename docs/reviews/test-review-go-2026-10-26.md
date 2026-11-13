# Revue des tests unitaires Go — 2026-10-26

Objectif : des tests qui attrapent de vrais défauts, pas du remplissage de couverture. Méthode
objective : tests de mutation (gremlins v0.6.0), puis confrontation aux matrices de cas du plan
T11 et aux ADR, puis ajout de tests, chacun vérifié contre une implémentation volontairement
cassée.

## Méthode et limites

- Mutation : `scripts/heavy.sh go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0 unleash
  --workers 4` (la commande de `mise run mutate:go`), un paquet à la fois, dans le cgroup de 8 Go.
  `mise` écrase `GOFLAGS`, donc le plafond d'espace d'adressage par binaire de test (`ulimit -v`,
  via `-exec`) n'est posé que par l'appel direct : un mutant de `internal/trust` qui allouait 8,7 Go
  tuait sinon le job entier.
- « Efficacité » = tués / (tués + survivants). Les `TIMED OUT` (3 à 9 par paquet) sont comptés à
  part : un mutant qui boucle est détecté.
- Artefact gremlins, important : une expression de `case` (`case err != nil:`) tombe dans un bloc de
  couverture que le profil Go ne compte pas, d'où des `NOT COVERED` sur du code bien testé
  (`internal/project/safe.go`, `tree.go`, `internal/transport/mux/fragment.go`). Ces lignes ont été
  mutées à la main (`mutcheck`, un remplacement à la fois, puis les tests du paquet) : voir par paquet.
  À l'inverse, quelques `LIVED` sont des faux positifs sous charge (mutant tué à la main).
- `internal/typst` : les tests de compilation lancent le vrai `typst` et dépassent 8 Go avec des
  mutants ; la mutation a été restreinte aux tests purs (`-run` : diagnostics, PDF, archives, deck,
  limites). `internal/app` : mesuré en entier.
- Mutants équivalents écartés (liste honnête, non exhaustive) : gardes de taille placées avant une
  analyse qui échouerait de toute façon avec la même erreur (`op.go:340`, `attest.go:90`,
  `sync.go:180`) — le test ajouté vérifie néanmoins le message, donc l'ordre garde/analyse ;
  conseils de capacité (`make(..., cap)`) ; `i--` / `i++` après `slices.Delete` dans `list.go`
  (la boucle externe repasse) ; bornes `<` / `<=` sur des valeurs jamais égales ;
  `off < 0` sur `uint32` en 64 bits.

## Scores (tués / survivants, efficacité)

| Paquet | Avant | Après | Remarque |
|---|---|---|---|
| internal/trust | 235 / 44, 84,2 % | 258 / 21, 92,5 % | 6 non couverts (constantes) |
| internal/project | 263 / 53, 83,2 % | 285 / 34, 89,3 % | 105 « non couverts » = artefact ; sécurité vérifiée à la main |
| internal/blob | 280 / 60, 82,4 % | 302 / 40, 88,3 % | |
| internal/bridge | 207 / 78, 72,6 % | 254 / 31, 89,1 % | |
| internal/transport/frame | 26 / 5, 83,9 % | 36 / 0, 100 % | |
| internal/transport/mux | 49 / 31, 61,3 % | 75 / 8, 90,4 % | + 8 mutations manuelles tuées (voir plus bas) |
| internal/sidecar | 157 / 26, 85,8 % | 159 / 23, 87,4 % | 3 survivants vérifiés tués à la main |
| internal/app | 331 / 1, 99,7 % | — | déjà solide ; un test instable corrigé |
| internal/typst (tests purs) | 253 / 97, 72,3 % | 258 / 93, 73,5 % | 175 non couverts : compilation exclue ; reste surtout `attachments.go` (analyseur PDF) |

## Par paquet : trous trouvés, tests ajoutés

### transport/frame (`frame_extra_test.go`, `budget_extra_test.go`, `progress_extra_test.go`)
Trous : format sur le fil jamais épinglé (octets du préfixe), frontières de `limit`
(`size == limit` accepté, `limit+1` refusé après lecture de l'en-tête seulement), limite négative,
longueur `0xFFFFFFFF`, trame vide non nulle, lecture octet par octet à travers les blocs de 64 Kio,
troncature après le premier bloc, causes d'erreur conservées (jamais transformées en EOF), pas de
préallocation pour un en-tête menteur, `ReadReserve` (appelé avec la taille, refus avant lecture,
jamais appelé si trop gros / EOF), `Budget` sans `time.Sleep` (synctest) : comptage exact,
libération partielle, `Acquire(0)`, `Close` réveille tous, `Broadcast`, `TryAcquire`,
`WriteProgress` : octets identiques à `Write`, découpe en blocs de 32 Kio, échéances `now+idle`
plafonnées par la limite globale, taux nul, erreur d'armement, échec à chaque étape, `MaxFrame`.

### transport/mux (`mux_extra_test.go`, `fragment_extra_test.go`, `fragment_hostile_test.go`, `gate_test.go`, `export_test.go`)
Trous : les fragments hostiles étaient testés par cas, mais la **cohérence id / canal / total
d'un message en cours** ne l'était pas (mutation manuelle : retirer `cur.ch != ch` survivait ; un
message commencé sur un canal pouvait se terminer sur un autre), ni les doublons d'un fragment du
milieu, ni le recouvrement et le trou, ni l'indépendance des pairs, ni la comptabilité mémoire
(capacité 32 Kio puis doublement plafonné au total, tout rendu à la fin, `PeerDown` libère tout de
suite, TTL de 30 s exact, le lecteur survit à l'expiration), ni les bornes de file (256
évènements, 32 Mio, un message plus gros accepté dans une file vide), ni l'ordre du portier
(trois tours interactifs pour un tour d'arrière-plan, pas de crédit accumulé sans arrière-plan,
annulation), ni la comptabilité des portiers (`export_test.go`), ni le format sur le fil des
fragments et la frontière 64 Kio. Mutations manuelles tuées : id/canal/total, `<=` du doublon,
quota du portier, `len(g.lo) >= 0`, bornes de `total`, bornes des files.

### trust (`limits_extra_test.go`, `sync_extra_test.go`, `guards_extra_test.go`, `acl_extra_test.go`)
Trous : `bitset` au-delà de 64 positions ; limites de nom, d'URL de racine, de parents (64 / 65,
doublons, désordre) et de longueurs hostiles patchées sur le fil ; limites des attestations
(document, têtes, version, troncature) ; `Verify` par rôle (admin, écrivain, lecteur, inconnu,
autre projet, signataire falsifié, champs altérés) ; `ApplyAll` rapporte les ops libérées et les
refus ; une op en attente qui échoue à l'intégration n'est **pas** rapportée donc pas relayée ;
péremption des ops en attente après exactement deux ticks, dans une liste âgée ; **machine de
synchronisation jamais testée sans `Tick`** (le test réseau convergeait grâce au `Tick`, ce qui
masquait un `relay` mort) : relais à tous sauf l'expéditeur, une seule fois, budget de deux `Have`
par pair et par tick, liste scellée sans annonce, erreurs : seules les malformations remontent ;
découpe en messages au octet près ; gardes de taille ; changement de rôle d'un membre suspendu ;
ACL-08 (rétrogradation révoque les attestations) et ACL-09 (liste figée : l'écriture continue).

### project (`boundaries_extra_test.go`, `text_extra_test.go`, `claim_extra_test.go`)
Trous : ordre naturel (zéros, casse, `` ` `` avant `A`, `Z`, chiffres en fin de nom), dossiers
d'abord, profondeur maximale de listage, lecture tronquée au milieu d'un caractère, point de
synchronisation de 8 Kio exact (et conservé après redémarrage, JSON invalide, vide), `readLimited`,
collisions de dossiers jusqu'à 1000, `folderName`, limites de l'arbre (profondeur 32 / 33, blob de
taille 0 / -1, texte sans document), collisions (casse, normalisation, fichier à la place d'un
dossier, dans les deux ordres), dominante des fins de ligne mixtes (égalités), numéro de ligne de
l'octet invalide, bornes de Windows-1252, aller-retour de tous les octets définis, repli en UTF-8,
UTF-16 (ordre, substituts, NUL), `Claim` d'un fichier modifié ou absent (mutation `&&` survivait :
un fichier modifié depuis sa lecture pouvait être revendiqué).

### blob (`wire_boundaries_test.go`, `download_extra_test.go`, `service_limits_test.go`, `service_global_test.go`)
Trous : bornes du fil (Bao exact, `maxBao`, `Busy` à 1 min, millisecondes, décalage max) ; carte de
reprise (`parseMap` : bits d'un groupe inexistant, taille, CRC, magie, longueur) ; `next` / `covers`
(trous, plafond 1 Mio, dernier groupe court) ; vidage exact tous les 8 Mio ; reprise ou
redémarrage ; `hold` (taille max, quota exact, confirmation, espace disque exact, annulation d'un
`hold` refusé) ; options par défaut ; requêtes bornées (5 `Busy` attendus puis 8 tentatives) ;
attente de `Busy` plafonnée à 5 s ; créneaux de service libérés ; **limite globale de service
jamais testée avec plusieurs pairs** (la mutation `v.serving--` survivait : un seul client était
arrêté par la limite par pair).

### bridge (`codec_limits_test.go`, `range_limit_test.go`, `fuzz_small_test.go`)
Trous : aucune propriété « tout préfixe strict et toute extension d'un encodage valide sont
refusés » ; listes vides ; bornes exactes de chaque champ (types, drapeaux, tailles int64, chemins,
URL, références par type, nombre d'entrées, `MaxValue` au octet près, `MaxDoc`, `MaxHeads`,
`MaxMembers`, pair de 64 octets, `Suspended` 0/1, trame max) ; limite d'une réponse de plage au
octet près ; nouveau fuzz `FuzzSmallCodecs` (réponses, écriture avec point, infos blob/arbre/fichier :
ce que le décodeur accepte, l'encodeur le réécrit à l'identique ; 20 s sans trouvaille).

### sidecar (`limits_extra_test.go`, `artifact_extra_test.go`)
Trous du superviseur (redémarrage / reprise) : valeurs par défaut exactes (30 s, 3 s, 10 échecs),
facteur 1 / 3 / 0, plafond sous le délai initial, `MaxFailures` 0 → 10, 1 → abandon immédiat,
négatif → jamais ; un lancement stable remet les échecs à zéro ; `capped` au octet près ;
`dirBytes` ; arrondi des limites `ulimit` ; chaque champ d'un artefact épinglé.

### app
Déjà à 99,7 %. **Test instable corrigé** : `PRV-32` échouait 1 fois sur 30 (le résultat de la
compilation lancée à l'attachement et celui de la fenêtre demandée pouvaient arriver chacun après
le relevé du compteur) ; ajout de `settle`, 60 passes sans échec.

### typst
Ajouts : `clipBytes` / `truncate` aux frontières UTF-8, limites exactes de l'analyseur de
diagnostics (taille, nombre, continuation, colonne `2^30`).

## Vrais défauts

Aucun défaut de code de production confirmé : toutes les mutations « sécurité » vérifiées à la main
(`safe.go`, `tree.go`, `Claim`, cohérence des fragments, quotas) sont maintenant tuées, et les
survivants initiaux étaient des **trous de tests**. Ce qui aurait pu passer sans être vu avant cette
revue :

- un fragment pouvait terminer un message sur un autre canal que celui qui l'avait commencé (test
  absent — le code est correct) ;
- `Claim` revendiquait un fichier modifié depuis sa lecture si la condition était affaiblie (test absent) ;
- `Split()` sans canal lance une goroutine de lecture impossible à arrêter (observé par `goleak`
  pendant la rédaction) : sans conséquence dans l'usage actuel (toujours au moins un canal),
  non corrigé ;
- `WriteProgress` sur un contexte déjà annulé écrit jusqu'à ce que l'échéance ramenée à « maintenant »
  par le rappel d'annulation agisse : correct pour un vrai flux, observable seulement avec un
  écrivain qui ignore les échéances ; non corrigé.

## Trous restants

- `internal/trust/list.go` `capPrev` (frontière de plus de 64 têtes) : l'ordre de préférence des
  têtes « qui tiennent » n'est pas épinglé (survivants `list.go:454-514`).
- `mux` : plafond de réassemblage de 128 Mio exact (demande 128 Mio de données), compteur de réponses
  `Busy` en vol de `blob`.
- `typst` : la mutation des tests de compilation (vrai `typst`) n'est pas possible sous le plafond
  de 8 Go ; `engine.go` et `watch.go` ne sont donc pas mesurés.
- Matrice T11 : sur 462 identifiants, `ACL-08/09` ont maintenant un test ; HIS, LCK, LNK, MDL, OUT,
  REL, RSZ, TODO, ZOOM, DIF, FM (fonctions pas encore livrées ou purement interface), PKG (CI de
  paquets), PERF-12 (zstd absent du code) n'ont pas de test Go à écrire aujourd'hui.
- `internal/transport/p2p` et `internal/store`, `internal/identity`, `internal/invite` n'ont pas été
  mesurés (hors liste demandée).
