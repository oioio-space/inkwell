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
