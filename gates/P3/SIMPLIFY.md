# /simplify — protocole étapes 2 et 3, outillage formel (2026-11-08)

Périmètre : tout ce qui a été produit depuis la fusion de L3 (`82a4735..HEAD` et l'arbre de travail) :
`protocol/sig`, `protocol/conformance`, `protocol/repo`, `frontend/packages/protocol`, `frontend/src/net`,
scripts (`check-env-reference.py`, `alloy_check.py`, `block_done.py`, `opencode-free.sh`, `claude-or.sh`,
`tla-check.sh`), modèles Alloy. Quatre relectures en parallèle (réutilisation, simplification,
efficacité, altitude), 33 constats, triés contre les invariants de `CLAUDE.md` et les portes figées.

## Appliqué

**Lot 1 — Go (`protocol/*`)**
- Tests qui ne prouvaient rien, supprimés : `TestDecoderDepthBounds`, `TestDecoderItemsBounds`,
  `TestMessageHandshakeSizeBounds` (corps vides), `TestProfileRefusalSizeBound`,
  `TestProfileRefusalItemsBounds` (`t.Skip` permanents), `TestRunInvalidStepEnv` (testait `strconv`),
  `TestFixtureErrors` (erreur jetée), `TestRunWithEnvStep` (sous-ensemble d'un autre).
  `TestMessageEphemeralDataBounds` (vide, prétendait être couvert par un vecteur qui ne sonde que 1025)
  remplacé par un vrai test de borne.
- Aides réécrites remplacées par la bibliothèque standard (`strings.Contains`, `bytes.Equal`) ; un seul
  `testutil.Unhex` pour tout le module (`protocol/internal/testutil`).
- Échelles copiées-collées converties en tables ; tests de prédicats regroupés.
- `conformance` : une seule table de décodeurs (`decoderSpec`) au lieu de trois structures ;
  `strict.go` réduit à une enveloppe ; `Context` passé tel quel à `repo.Decode`.
- `CurrentStep = 3` et `StepFromEnv()` : `go test` (la CI) impose tous les vecteurs jusqu'à l'étape
  courante, plus seulement les portes ; tests `TestStepFromEnv*` (défaut, valeur explicite, valeur
  malformée qui n'abaisse jamais l'exigence).
- `repo.Decode` : classification des types de messages dans une seule table.
- `cbor.go` (encodage des tables) : clés et valeurs encodées dans le tampon partagé, tri par
  `slices.SortFunc`, au lieu d'une allocation par paire et de `sort.Slice`.
- `sig.Verify` : la clé n'est plus décompressée deux fois ; somme SHA-512 dans un tampon de 64 octets.

**Tests manquants révélés par la mutation (P-BORNES)** — chacun nomme la règle qu'il prouve :
largeurs d'en-tête CBOR (23/24, 255/256, 65535/65536, 2^32−1/2^32, 2^53−1 accepté, 2^53 refusé) ;
entiers signés acceptés à partir de 0, refusés à −1 ; borne de 4096 éléments (4096 accepté,
4097 refusé) ; 1024 octets des données éphémères et de la poignée de main (1024 passe, 1025 refusé) ;
conteneurs et arguments sur 2, 4 et 8 octets tronqués : erreur propre, jamais de lecture au-delà de
la fin.

## Mutation

mutation protocol/repo : efficacité gremlins **74,5 % → 99,0 %** (103 tués, 1 survivant).
- Survivant équivalent `cbor.go:174` (`d.item(depth + 1)` pour une clé de table) : une clé est
  toujours une chaîne de texte (vérifié juste avant), qui ne descend jamais ; la profondeur passée ne
  change aucun comportement observable.
- « Non couverts » `cbor.go:308–314` (`case n < …` d'`appendHead`) : artefact de gremlins (les
  conditions de `case` ne sont pas des instructions dans le profil de couverture). Mutations manuelles
  `<` → `<=` aux lignes 312 et 314 : **tuées** par `TestEncodeHeadUsesTheShortestWidth`.
  `cbor.go:20` (`maxSafe = 1<<53 - 1`) : déclaration de constante, non exécutable.

## Écarté (avec raison)

- Renommer `signed_bytes_hex` / `message_hex` dans les vecteurs : les vecteurs font partie de la spec
  normative ; c'est une décision de spec, pas un nettoyage.
- Valider chaque clé une seule fois avec un cache : changement de conception de la sécurité, à faire
  à l'étape où les clés sont admises ; seule la double décompression est supprimée ici.
- `reference.ts` : validation des clés réécrite avec `@noble/curves` plutôt qu'à la main — exécuteur
  de référence ; à reprendre au lot 2 si noble exprime le refus d'ordre mixte.

## Lot 2 — TypeScript (appliqué, vérifié)

- **Défaut réel trouvé par la relecture « efficacité »** : l'encodeur `cbor.ts` étalait chaque octet
  dans un tableau JS (`out.push(...bytes)`). Sur un message de 2 Mio il plantait
  (`RangeError: Maximum call stack size exceeded`). Introduit par l'étape 3 (l'adaptateur réseau
  encode tout par lui) : la synchronisation d'un gros document aurait échoué. Réécrit avec un
  `Writer` à tampon extensible (une copie par charge utile, en-têtes écrits en place, sans BigInt).
  Test de régression `cbor.test.ts` « 2 MiB » : **échoue sur l'ancien code, passe sur le nouveau**
  (vérifié par Claude en remettant l'ancien fichier).
- Décodeur : doublons détectés par `Object.hasOwn` (plus de `Set` par table).
- `repo.ts` `present()` : décompte de clés plus `Object.hasOwn`, fermetures remontées au module.
- Documents admis : `Set` construit une fois (TS) au lieu de `includes` à chaque message.
- `encodeCBOR` typé sans `as CBORValue` aux appels ; utilitaire de test commun pour les messages
  conformes (`senderId`, `targetId`, `peerMetadata`).
- Résultats : 305 tests (`packages/protocol`, `src/net`, `src/workspace`) ; vecteurs étape 3 en TS.

## Lot 3 — scripts

Un seul utilitaire `scripts/fetch_pinned.py` (récupération SHA-256 épinglée) partagé par TLA+ et
Alloy, avec ses tests ; voir le résultat ci-dessous.

Appliqué : `tla-check.sh` délègue cache, vérification et renommage atomique à `fetch_pinned.py`
(suppression du contrôle SHA-256 et du `mv` en double) ; `alloy_check.py` l'importe.
Tests : réponse réseau simulée par `io.BytesIO`. Le premier jet utilisait un `MagicMock` dont
`read()` ne rendait jamais `b""` : `shutil.copyfileobj` bouclait sans fin et a saturé la mémoire
(21 Go, OOM global le 2026-11-08) ; `test:py` passe désormais par `heavy.sh`.
Vérifié : `test:py` (11 tests), `mise run formal` depuis un cache vide (téléchargement épinglé,
modèles et mutants TLA+ et Alloy).

## Lot 4 — garde-fous ajoutés après l'OOM (2026-11-08)

Fichiers : `scripts/check_claude_md.py` (+ tests), `.claude/hooks/heavy-guard.sh`, `scripts/block_done.py`
(étape 3 lance `lint:claude-md`), `mise.toml` (`lint:claude-md` dans `ci`, `test:py` et `fe:check` sous
`heavy.sh`), `scripts/tla-check.sh`.

Appliqué :
- `check_claude_md.py` : un hook à `command` vide faisait planter le contrôle (`split()[0]`) ; gardé.
- `tla-check.sh` : cache et vérification délégués à `fetch_pinned.py` (voir lot 3), plus de `rm`/`mv` en double.
- Un seul contrôle de cohérence de CLAUDE.md, appelé par `ci` et par `done` (pas de copie de la liste).

Écarté :
- `heavy-guard.sh` accepte toute commande qui *contient* `heavy.sh`, `memcap.sh` ou `systemd-run`, même
  hors du test (ex. `cat scripts/heavy.sh; go test`). Analyser la ligne de commande en shell serait
  fragile ; le hook vise l'oubli, pas le contournement délibéré.
- `ROOTS` du contrôle est une liste fixe : un chemin cité hors de ces racines (`.typstdeps/`, régénéré)
  n'est pas vérifié. Volontaire : ce sont des répertoires produits, absents d'un arbre neuf.

Vérifié : `test:py` (17 tests), `lint:claude-md` vert, hook éprouvé sur 12 commandes (7 refusées,
5 acceptées).

Corrections de lint du lot 1 (golangci-lint, rattrapées par `done`) : point final d'un commentaire (`bounds_test.go`), commentaire de paquet (`protocol/internal/testutil`), déclaration courte (`conformance_extra_test.go`).

## mutation protocol/conformance

97,9 % (95 tués, 2 survivants, `--timeout-coefficient 3` comme `done`). Avant : 77,5 %.
Correction de fond trouvée par la mutation : `verify()` ne contrôlait l'aller-retour binaire que si
une ré-encodage existait (`c.encoded != nil`) ; sauter le ré-encodage désactivait la règle sans qu'aucun
test ne le voie (4 mutants « équivalents » selon l'agent, en fait un trou). Désormais chaque décodeur
qui a un encodeur déclare `roundTrip`, et un ré-encodage absent est un échec. Aussi : `blake3_hex`
mal formé était lu après le contrôle `fieldErr` et donc ignoré ; lu en premier. Les deux tests
échouent sur l'ancien code (vérifié en le rétablissant) et passent sur le nouveau.
Survivants équivalents :
- `reference.go:148:32` : `spec.hasInput` est faux pour `public_key`, le `&&` court-circuite.
- `reference.go:479:11` : les deux passes (ordre direct, ordre inverse) s'exécutent une fois
  chacune ; échanger laquelle inverse donne les deux mêmes plis.

## Fuzz de `repo.Decode` (audit sécurité 2026-11-08)

`FuzzRepoMessage` appelait deux fois `DecodeValue` et jamais `Decode`, le point d'entrée des messages
de pairs. Il fuzze maintenant `Decode` sous le contexte des vecteurs : pas de panique, message accepté
lié à l'expéditeur authentifié et au destinataire, document admis, profil respecté, aller-retour
des valeurs. 60 s, 15,6 M exécutions, aucun échec.

## Test CBOR 2 Mio (frontend)

Le test dépassait 15 s sous la suite complète. Mesure : encodage 2,0 ms, décodage 0,2 ms, mais `toEqual` de Vitest 3,4 s par comparaison de deux tableaux de 2 Mio (deux appels). Comparaison d'octets exacte et linéaire à la place, délai de 15 s retiré (le défaut de 5 s s'applique, plus strict) : 6,7 s → 25 ms, contrôles inchangés.

## mutation protocol/cmd/vectors — test doré du générateur

Le générateur n'avait aucun test (0 %). Ajout d'un test doré : `cases.json` de
novifinancial/ed25519-speccheck épinglé (commit 5e4bfc45, SHA-256 08e47a36…, Apache-2.0) dans
`protocol/testdata/`, régénération dans un dossier temporaire, égalité octet pour octet avec
`protocol/vectors/`. Il a trouvé une dérive : `crypto.json` avait été retouché à la main après sa
génération (étape 1, dc100ab) sur 11 `reason` (champ non normatif, ignoré par l'exécuteur) ; aucun
verdict ne change. Corpus régénéré : le générateur redevient la seule source de vérité, et le test
doré interdit désormais toute retouche à la main. Conformité Go et TypeScript vertes après régénération.
Résultat : efficacité 100 % (77 tués, 0 survivant ; 8 non couverts exclus par gremlins).

`heavy-guard.sh` : faux positif sur un mot dans le texte d'un printf ; un lanceur de tests ne compte plus qu'en position de commande (24 cas vérifiés).

## mutation protocol/internal/testutil

`Unhex` (lecture de `input_hex` dans tous les tests du protocole) n'avait pas de test propre : test direct (hex valide décodé exactement, hex invalide fait échouer le test) ; efficacité 100 % (1 tué).

## Mesure de mutation (outil `done`)

Trois mesures de ce bloc étaient nulles sans que `done` le dise : tous les mutants « timed out » (278 sur 278 pour `trust`), lus comme 0 % « justifié ». Cause : délai = 3 × la couverture, trop court pour un paquet dont les tests prennent 14 s seuls et plus avec 4 workers. Corrigé : coefficient 10, `GOFLAGS=-count=1`, et `done` refuse une mesure sans mutant évalué ou avec plus de 10 % d'expirés (tests ajoutés). Les mesures valides de ce bloc (repo 99 %, sig 100 %, conformance 97,9 %, vectors 100 %, testutil 100 %) avaient des mutants réellement évalués.

## Surveillance des tâches longues (W30)

`block_done.py` diffuse la sortie de gremlins ligne à ligne et rapporte son code de sortie (137 : plafond mémoire) au lieu de l'avaler ; `scripts/watch-log.sh` (progression, échecs, silence, fin) sert de commande de Monitor ; hook `long-task-monitor.sh`. Testé : progression, survivant, score, « stalled », erreur, sortie.
- Correctif : une variable locale `code` masquait la fonction `code()` dans l'étape 6 (attrapé par le Monitor, test ajouté).

## mutation protocol/trust — hors périmètre de P3 (décision de l'utilisatrice, 2026-11-08)

P3 ne touche `protocol/trust` que par la modernisation mécanique `go fix` (`range` sur entier,
`slices.Backward` dans `fold.go`) ; son comportement ne change pas et ses tests passent. Mesure valide
(binaire installé, `-count=1`, plafond 1,8 Go par binaire de test) : 92,4 % (255 tués, 21 survivants,
6 non couverts). Les survivants datent d'avant P3 ; certains sont des trous réels (tri de `capPrev`,
règle 3b, quand il y a plus de MaxPrev têtes), d'autres équivalents (`attest.go:109`, doublé par
`attest.go:49`). Ils deviennent le bloc W31 (objectif ≥ 95 %), avec cette liste :
attest.go:109:11 fold.go:88:58 fold.go:371:52 fold.go:387:11 fold.go:485:28 fold.go:502:13 fold.go:502:20 list.go:175:5 list.go:391:58 list.go:399:109 list.go:458:16 list.go:463:47 list.go:465:12 list.go:465:12 list.go:469:13 list.go:469:13 list.go:500:39 list.go:516:12 list.go:518:16 op.go:320:11 op.go:364:11 
- `lint:env` : `INKWELL_AS_LIMIT` et `XDG_CACHE_HOME`, lues par `block_done.py`, déclarées dans `.env.reference`.
- `watch-log.sh` : signatures d'échec reconnues quelle que soit la casse (`gate.sh` écrit « fail ») et ligne de fin rapportée ; `heavy-guard.sh` ignore le texte entre guillemets (motif de `grep`).
- `base-guard` : 4 dérogations justifiées (`.golangci.yml`, `mise.toml`, `alloy_check.py`, `tla-check.sh`), décision dans `docs/objectif-en-cours.md`.
- Précision : `watch-log.sh` ne retient finalement que les marqueurs en majuscules et le mot « fail(ed) » isolé, pas le mot générique « Error » que les tests qui passent impriment exprès.
- `scripts/gate.py` effaçait tout `gates/<lot>/` à chaque porte, rapports compris : ce rapport (reconstitué depuis les transcriptions des deux sessions, opérations bloquées exclues) et les reconstitutions L3/P2 ont été perdus ainsi. Désormais `reset_evidence` garde les `*.md` (tests ajoutés).
- `CLAUDE.md` : piège « une porte ne vide que les preuves » ajouté à la ligne existante des rapports.
