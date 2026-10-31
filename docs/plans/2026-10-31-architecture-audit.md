# Audit d'architecture — référentiel, seuils, barème (fixés avant mesure)

**Règle :** rien au hasard ni à l'intuition. Un constat n'est retenu que s'il a les trois éléments
suivants : (1) une **preuve** (`fichier:ligne`, sortie d'outil, mesure reproductible avec sa commande),
(2) la **règle** violée, tirée du référentiel ci-dessous, (3) une **gravité** selon le barème. Sans
preuve ou sans règle, le constat est écarté. Un choix de conception passe par une ADR qui compare au
moins deux options.

## Référentiel

| Domaine | Référence |
|---|---|
| Modèle de qualité | ISO/IEC 25010:2023 : adéquation fonctionnelle, efficacité, compatibilité, capacité d'interaction, fiabilité, sécurité, maintenabilité, flexibilité, sûreté |
| Règles du projet | `docs/engineering-principles.md`, invariants de `CLAUDE.md`, `docs/adr/`, `docs/architecture.md` |
| Style et idiomes Go | Google Go Style Guide (guide, decisions, best-practices), Go Code Review Comments, Effective Go |
| Conception | règle de dépendance : les paquets du cœur n'importent rien d'applicatif ; interfaces petites, définies chez le consommateur ; patterns GoF seulement quand l'idiome Go ne suffit pas |
| Sécurité | OWASP ASVS 5.0 (niveau 2), CWE Top 25, NIST SSDF SP 800-218, règles §4 du projet |
| Concurrence | chaque goroutine a un propriétaire et une condition de sortie ; `go test -race` ; goleak |
| Performance | budgets du §3 ; preuve par `benchstat` (`-count ≥ 10`, p < 0,05, aucune régression d'allocations) |

## Mesures et seuils (outils et versions consignés dans le rapport)

| Mesure | Outil | Seuil |
|---|---|---|
| Complexité cognitive par fonction | gocognit | > 15 à signaler, > 30 = majeur |
| Complexité cyclomatique par fonction | gocyclo | > 15 à signaler |
| Couverture par paquet | `mise run cover` | < 85 % (`COVER_MIN`) = majeur |
| Règle de dépendance | `go list -deps` sur chaque paquet du cœur | toute arête interdite = critique |
| Code dupliqué | dupl | ≥ 100 jetons à signaler |
| Lint, vet, race, vulnérabilités | golangci-lint, `go vet`, `-race`, govulncheck, gosec | tout constat = à trier |

## Barème de gravité

- **Critique** : viole un invariant ou une ADR ; faille ASVS niveau 2 ; perte de données ou plantage démontrés.
- **Majeur** : viole une règle de `engineering-principles.md` ou un point « decisions » du guide Google, avec un impact mesurable (seuil dépassé, goroutine sans propriétaire, allocation prouvée dans un chemin critique).
- **Mineur** : écart aux « best-practices » sans impact mesuré.
- **Écarté** : ni preuve, ni règle.

## Format d'un constat

`ID · gravité · règle (référence précise) · preuve (fichier:ligne + commande) · correctif proposé · test qui le prouvera`

## Constats retenus

Sources : mesures (`quality-runner`, Haiku), audit `architect` (Opus), audit Codex. Un constat
signalé par une seule source est revérifié par un autre modèle avant d'être retenu.

| ID | Gravité | Règle | Preuve | Vérifié par |
|---|---|---|---|---|
| C1 | Critique | `architecture.md:74` : `trust` n'importe pas `transport` | `internal/trust/sync.go:10` ; `arch-check.sh` ne teste pas cette règle | architect + Codex (A01) + relecture |
| C2 | Critique, remède documentaire | ADR 0004, `CLAUDE.md:45` (« tout le réseau passe par Transport ») contredit `architecture.md:76` | `internal/invite/wormhole.go:41-49` | architect + relecture |
| S1 (A04) | Critique | ASVS 5.0 V2.4.1 (épuisement de ressources) | `frontend/src/net/coverage.ts:193,240,242,247-253` ; `workspace.svelte.ts:166` partage tout document ; un pair en lecture fait retenir environ 279 Mio avec 300 messages | Codex + `security-auditor` (Opus), test jetable |
| S1b | Critique | ASVS 5.0 V2.4.1 | messages éphémères non filtrés (`WailsNetworkAdapter.ts:342,396`) : environ 114 Mio retenus ; identifiants demandés jamais oubliés (`CollectionSynchronizer.js:83-89`) ; un fichier d'état de synchronisation par poignée de main | `security-auditor` (Opus), tests jetables |
| R1 | Majeur (régression du premier correctif S1) | ISO/IEC 25010 adéquation fonctionnelle | partage limité à `repo.handles` : une variante créée après l'arrivée n'est plus répliquée ; e2e ZZ-2 réussit avant, échoue après | `security-auditor`, e2e |
| S2 (A05) | Critique (erreur disque ou plantage) | ASVS 5.0 V2.3.3 (transaction complète ou annulée) | `internal/app/persist.go:133-137` efface l'ancienne liste avant d'écrire la nouvelle ; un échec fait perdre un membre ou un projet | Codex + `security-auditor`, test jetable |
| S3 (A03) | Majeur | ASVS 5.0 V2.4.1 | `internal/app/attest.go:49` sans plafond ; signataire = personne qui écrit ; 10 001 attestations gardées | Codex + `security-auditor` |
| S4 (A02) | Mineur | §4, décodage strict | `WailsNetworkAdapter.ts:302` : un CBOR nul provoque une TypeError et la trame est perdue | Codex + `security-auditor` |
| M1 | Majeur | §3 : pas d'allocation dans les boucles chaudes | blob : environ 3,1× le volume par tranche de 1 Mio, 7,1× par téléchargement (`store.go:609,623`, `service.go:755`) | benchmark de l'architecte |
| M2 | Majeur | §3 : idem | `internal/app/wiring.go:341` : 2 allocations / 96 o par appel, ≥ 3 appels par mise à jour relayée | benchmark de l'architecte |
| M3 | Majeur | gocognit > 30 | 6 fonctions (`project/watch.go:27` 57, `trust/fold.go:416` 52, `:270` 39, `typst/watch.go:32` 35, `typst/packages.go:337` 35, `trust/list.go:452` 34) | mesures + architect + Codex |
| M4 | Majeur | couverture < 85 % par paquet | `sidecar/fetch` 83,8 %, `transporttest` 77,7 % ; `cover.sh` ne contrôle que le total | mesures + architect |
| M5 | Majeur | §2 : un paquet = un rôle | `internal/app` a 4 rôles ; ses tests tirent 610 paquets et un binaire de test de 104 Mo | architect |
| F1 | Majeur (fiabilité, à diagnostiquer) | ISO/IEC 25010 fiabilité ; un test doit être déterministe | `internal/app` `TestPreviewCompilesTheProject/PRV-10` : 1 échec dans `mise run test`, 3 succès isolés (2026-10-31) | à diagnostiquer (`superpowers:systematic-debugging`), sans le déclarer instable au jugé |
| F3 | Critique (plantage) | ISO/IEC 25010 fiabilité | `internal/transport/p2p` `TestWantAgainClearsResetHold` : déréférencement de pointeur nul dans libp2p `swarm_dial.go:476`, 1 fois sur 6 ; indépendant du lot 2 (`p2p` n'importe pas `trust`) | `quality-runner`, à diagnostiquer |
| F1 (suite) | Majeur | idem F1 | même famille : `PRV-32` (`preview_test.go:516`), 1 échec puis 5 succès sur 5 | `quality-runner` |
| F5 | Majeur (sécurité, à corriger) | §4 sidecars bornés ; ASVS 5.0 V2.4.1 | `internal/sidecar` : la borne de taille du dossier surveillé est contrôlée par sondage ; sous charge, `TestRunDirectoryBound` (`limits_test.go:69`) voit 152 Mio écrits pour une marge de 128 Mio (pré-push du 2026-10-31, 1 échec sur 1 780) ; un Typst hostile peut donc dépasser la borne en proportion de la charge | CI ; à corriger par une borne non fondée sur le temps (RLIMIT_FSIZE, quota, ou écriture via un système de fichiers borné) |
| S1-R6 | Majeur (résiduel de l'étape 1) | N7 : un pair malveillant ne gêne jamais les autres | 1 024 identifiants de pairs qui épuisent leur budget de présence occupent toutes les entrées de l'admission : un nouveau pair est refusé jusqu'à environ 33 min (`frontend/src/net/admission.ts`, plafond 1 024) | revue Sonnet ; à fermer à l'étape 3 (identités bornées par la liste signée) |
| F2 | Majeur (à prouver) | §3 concurrence : chaque goroutine a un propriétaire | `adopt` remplace le magasin de confiance pendant que la goroutine de `Wiring` peut sauvegarder ; risque d'écrire des opérations de l'ancien projet dans le nouveau. Après un échec de réouverture, le magasin reste fermé jusqu'au redémarrage | `architect`, à prouver par un test sous `-race` |
| m1–m11 | Mineur | voir le rapport de l'architecte | variables globales servant de crochets de test, API sans appelant, tinymist embarqué (32,8 Mo) mais jamais lancé, `bridge.Storage` à 17 méthodes, doublons p2p, arrêt sans attente des goroutines, E/S hors `os.Root` dans typst | architect |

## Plan (ordre imposé par les dépendances)

Chaque lot : branche, test d'abord (TDD), correctif, `go-reviewer`, `/simplify`, fusion. Routage
indiqué par lot (règle : modèle et effort explicites, Claude **et** Codex).

1. **Gates** : `arch-check.sh` vérifie les interdits du cœur par `go list -deps` ; `gocognit` (> 30 en échec), `gocyclo` et `dupl` dans `.golangci.yml` ; `cover.sh` contrôle chaque paquet ; `deadcode` et `goleak` (blob). *Codex, `--effort medium` ; revue `go-reviewer` (Sonnet).*
2. **C1** : `trust.Sync` générique sur le type de pair. *Codex, medium.*
3. **S2 + S3** (Go : remplacement atomique `trust.next/` + reprise à l'ouverture ; plafond par signataire) *`go-dev` (Sonnet, medium)* ; **S1 + S4** (frontend : aucun état par document avant attestation vérifiée, empreinte au lieu du message, plafond, partage restreint ; validation de forme après décodage) *agent Sonnet, medium*. Revue des deux : `security-auditor` (Opus, high).
4. **C2** : ADR qui remplace ADR 0004 pour le canal d'invitation, mise en cohérence de `CLAUDE.md` et d'`architecture.md`. *`scribe` (Haiku) sur un texte que je rédige.*
5. **M5** : extraire `internal/session` (préalable à `internal/policy`, ADR 0030). *Conception `architect` (Opus) ; exécution `go-dev` (Sonnet).*
6. **M1 et M2** avec benchstat ; ajouter `internal/session` à `HOT_PATHS:`. *Codex, `--effort high`.*
7. **Code mort, puis complexité (M3)** ; mutation testing sur les prédicats de `trust`. *Codex, medium.*
8. **Mineurs m1–m11**, lot par lot. *Codex, `--model spark` ou medium selon le lot.*

## Pièces du dossier

Tout ce qui fonde les constats et les décisions est versionné dans
[`audit-2026-10-31/`](audit-2026-10-31/) :

| Fichier | Contenu |
|---|---|
| `01-architect-report.md` | audit d'architecture (Opus) : constats C1–C2, M1–M5, m1–m11, restructuration proposée |
| `02-metrics.md` | mesures brutes : lint, gosec, couverture, gocognit, gocyclo, dupl, règle de dépendance (commandes et versions) |
| `03-codex-audit.md` | audit indépendant de Codex (A01–A11) |
| `04-security-verify-A02-A05.md` | contre-vérification (Opus) des constats de sécurité de Codex : verdicts et gravités corrigées |
| `05-review-stage1-candidate.md` | revue du premier correctif S1 : fuites restantes et régression R1 prouvée par l'e2e ZZ-2 |
| `06-admission-design.md` | conception de la frontière d'admission (étape 1 figée, puis révision byzantine) ; source de l'ADR 0031 |
| `07-codex-design-review-1.md`, `08-codex-design-review-2.md` | revues critiques de cette conception par Codex |

## Décisions d'architecture (journal)

Chaque décision cite sa raison ; une décision qui touche une frontière passe par une ADR.

| # | Décision | Raison | Où |
|---|---|---|---|
| D1 | Référentiel, seuils et barème fixés **avant** toute mesure ; un constat sans preuve ou sans règle est écarté | demande « rien au hasard ni à l'intuition » (W20) | ce document |
| D2 | Travail réparti entre Claude (Haiku, Sonnet, Opus) et Codex, modèle et effort explicites à chaque appel ; tout constat d'une seule source est revérifié par un autre modèle | W21 ; une revue croisée a trouvé 3 critiques que les tests ne couvraient pas | `CLAUDE.md`, routage |
| D3 | Codex passe par le plugin officiel `openai/codex-plugin-cc` (`codex mcp-server` est retiré depuis la v0.154) ; OpenRouter en profil `--profile openrouter` tant que le crédit le permet, connexion ChatGPT sinon | état de l'outil au 2026-10-31 | `~/.codex/config.toml` (hors dépôt) |
| D4 | Gates d'abord (lot 1) : `arch-check.sh` vérifie le cœur par `go list -deps` ; gocognit et gocyclo échouent au-delà de 30 (cible 15) ; dupl à 100 jetons ; couverture ≥ 85 % **par paquet** (exemptés : `cmd/inkwell`, `transporttest`) ; `deadcode` avec base figée ; pas de Python dans les scripts, `jq` épinglé | empêcher les récidives et prouver les lots suivants ; chaque gate a échoué sur une violation volontaire | `scripts/`, `.golangci.yml`, `mise.toml` |
| D5 | `trust.Sync[P cmp.Ordered]` : générique sur le type de pair, instancié par `internal/app` avec `transport.PeerID` | C1 ; l'ordre est requis pour une livraison déterministe | `internal/trust/sync.go` |
| D6 | Remplacement de la liste signée : préparation dans `trust.next/`, installation par renommages ; **le point de validation est le renommage d'installation** ; tout échec ultérieur est `errCommitted` et l'appelant suit la nouvelle liste ; à l'ouverture et avant chaque remplacement, une bascule interrompue est reprise, `trust.prev` l'emporte si `trust` manque, une préparation orpheline échoue fermée ; `store.SyncDir` après chaque renommage ou suppression | S2 (ASVS 5.0 V2.3.3) ; injection d'échec et de plantage à chaque étape | `internal/app/persist.go` |
| D7 | Concurrence du magasin de confiance : `Wiring.AdoptProject` persiste puis change de projet dans une seule commande sur la goroutine de la wiring ; `opStore` protégé par un `RWMutex`, réouverture paresseuse | F2 : course prouvée sous `-race`, puis absente | `internal/app/wiring.go`, `persist.go` |
| D8 | **Modèle de menace byzantin complet** : même une personne qui écrit, malveillante, ne doit jamais gêner les autres ; lecteurs et pairs non attestés strictement bornés | décision de l'utilisatrice (N7) | `docs/exigences.md` |
| D9 | L'étape 1 de l'admission est fusionnée sans attendre la suite ; S1 reste **Critique** tant que la garde structurelle de l'ADR 0030 n'est pas en place | décision de l'utilisatrice (N8) | `06-admission-design.md` §« Stage 1 » |
| D10 | Frontière d'admission unique dans l'adaptateur réseau de la webview, **avant** automerge-repo : décodeur strict des types connus, ensemble de documents admis, bornes sur la présence et les poignées de main, files bornées par pair ; tout pair traité `isEphemeral`, son `storageId` ignoré, les anciens états de synchronisation supprimés par Go avant l'attache de la webview | les cartes internes d'automerge-repo grandissent avant toute règle de partage (preuves 04, 05) | ADR 0031 (proposée) |
| D11 | Options écartées pour D10 : règle de partage seule (la bibliothèque enregistre avant de la consulter) ; filtrage côté Go du format d'automerge-repo (contraire aux ADR 0002 et 0011) ; patch de la bibliothèque (ADR 0008), gardé seulement en repli si un test de contrat montre un trou | comparaison dans 06 et 07 | ADR 0031 |
| D12 | Plafond d'attestations par signataire **abandonné** (une personne honnête l'épuise par créations et suppressions) ; ensemble vivant tiré de l'arborescence de fichiers **abandonné** (les variantes y manquent) ; S3 attend l'ensemble admis unique de l'ADR 0031 | revues Codex | ce document |
| D13 | F3 : refuser toute numérotation dès le début de `Node.Close` n'est qu'une défense en profondeur ; la correction exige l'ordre de fermeture qui arrête les services de numérotation de libp2p avant le swarm, prouvé sur 500 exécutions | course de go-libp2p v0.51.0 (`swarm_dial.go:476`) | `internal/transport/p2p` |
| D14 | Aucun test n'est déclaré « instable » sans mesure de taux et cause prouvée | D1 | F1, F3 |

## État au 2026-10-31

| Lot | État | Commits ou branche |
|---|---|---|
| Documentation (référentiel, plan, routage) | ✅ fusionné | `f000a96` |
| Lot 1, gates | ✅ fusionné | `1e0abbe`, fusion `c47937e` |
| Lot 2, C1 | ✅ fusionné | `4943dee`, fusion `4daba07` |
| S2 + F2 | ✅ fusionné | `957cc3d`, fusion `c5e9a4d` |
| Étape 1 de l'admission (S1 partiel, S4, R1) et suites F1–F4 | ✅ fusionné ; S1 reste Critique (garde ADR 0030 à venir) ; PRS-37 a échoué 1 fois dans une suite e2e complète, taux à mesurer | fusions `ad2cf86`, `f2f1460` |
| Révision byzantine de l'admission, ADR 0031 | 🔨 conception (architect) | `audit-2026-10-31/06-admission-design.md`, `docs/adr/0031-*.md` |
| F3, plantage libp2p à la fermeture | 🔨 diagnostic ; correctif partiel non fusionné | branche `fix/p2p-dial-panic` |
| F1, tests d'aperçu | 🔨 diagnostic | branche `fix/preview-determinism` |
| S3, attestations bornées | ⏸ attend l'ADR 0031 | — |
| C2 : ADR qui remplace l'ADR 0004 pour le canal d'invitation | ⏳ | — |
| M5 : extraire `internal/session` | ⏳ (après l'ADR 0031, préalable à `internal/policy` de l'ADR 0030) | — |
| M1, M2 : allocations des chemins critiques, avec benchstat | ⏳ | — |
| Code mort, puis M3 complexité | ⏳ | — |
| Mineurs m1–m11 ; délai sur fsync (suite de S2) | ⏳ | — |
| Garde structurelle de l'ADR 0030 | ⏳ prérequis de l'étape 2 de l'admission | — |

Branches **abandonnées**, à ne pas fusionner : `fix/peer-state-bounds` (premier correctif S1, remplacé
par l'étape 1) ; `fix/trust-atomic-save` (S2 refait et fusionné ; sa partie S3 est caduque, D12).

## Reprendre le travail (pour une autre session ou une autre IA)

1. Lire `CLAUDE.md` (invariants, routage, gates), puis ce document en entier, puis
   `docs/exigences.md` (W19–W21, N7, N8) et `audit-2026-10-31/06-admission-design.md`.
2. Règles de travail en vigueur : D1, D2, D14 ; TDD ; un test qui échoue d'abord, puis le correctif ;
   revue par un **autre** modèle que l'auteur ; revue `/simplify` réelle avant chaque commit (le
   hook `.githooks/pre-commit` exige le marqueur, armé dans un appel séparé) ; au plus 4 agents et
   2 tâches lourdes (`scripts/heavy.sh`) en même temps.
3. Codex depuis Claude Code : agent `codex:codex-rescue` ou `/codex:rescue`, avec `--effort` et
   `--write` si besoin. Son bac à sable ne peut pas écrire dans `.git` ni ouvrir de sockets : créer la
   branche et le worktree soi-même, et faire tourner les tests réseau et e2e par un agent Claude.
4. Ordre des prochaines actions :
   1. relire et fusionner l'étape 1 (revue de sécurité Opus, e2e ZZ-2 et suites existantes) ;
   2. soumettre l'ADR 0031 à l'utilisatrice, puis l'implémenter par étapes ;
   3. finir F3 et F1 (cause prouvée, 0 échec sur 500 exécutions) ;
   4. garde structurelle de l'ADR 0030 ;
   5. S3, puis C2, M5, M1/M2, code mort, M3, mineurs ;
   6. mettre à jour ce tableau et `docs/exigences.md` à chaque fusion.
