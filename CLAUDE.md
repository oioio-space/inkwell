# inkwell — consignes du projet

État et feuille de route : `PROGRESS.md`. L'outillage est géré par **mise** : tout se lance par
`mise run …`. Garde-fous automatiques :
- hooks git (`.githooks/pre-commit`) : nettoyage → secrets (gitleaks) → vulnérabilités (gosec,
  govulncheck) → Biome des fichiers frontend indexés (`fe:check:staged`) → style Go → **attestation `/simplify`** (marqueur `$GIT_DIR/claude-simplify-ok`, lié au
  diff indexé exact ; voir « Définition de fini », étape 3) ; `--no-verify` en urgence seulement ;
- hooks Claude (`.claude/settings.json`) : `pre-commit-simplify.sh` (bloque le commit sans revue
  attestée), `heavy-guard.sh` (refuse un test non plafonné, voir « Mémoire »), `long-task-monitor.sh` (rappelle
  la surveillance de toute commande lancée en arrière-plan, voir « Tâches longues »), `gate-freeze.sh`
  (refuse de modifier une porte `gates/<bloc>.toml` déjà commitée), `post-edit-check.sh` (après chaque
  édition : gofmt et go vet pour un `.go`, Biome pour le frontend, `lint:claude-md` pour les fichiers
  IA ; un problème revient aussitôt à Claude), `research-grounding.sh` (rappel research-first à
  l'écriture d'un plan ou d'une ADR, là où une décision s'écrit).

## Commandes (toutes via mise)

```bash
mise run setup           # installe l'outillage et branche les hooks git
mise run test            # tests unitaires Go  | mise run test:watch (TDD)
mise run sidecar:fetch   # typst + tinymist (SHA-256 épinglés, à la construction) ; -- -target all
mise run lint            # golangci-lint v2    | mise run fmt
mise run cover:check     # porte de couverture (COVER_MIN=85)
mise run ci              # porte complète = ce que lance la CI (lint, tests, scans, formal, lint:env, test:py)
mise run bench:baseline  # puis modifier le code, puis : mise run bench:compare (benchstat)
mise run scan:code       # gosec + govulncheck | scan:secrets | scan:sbom (grype)
mise run clean           # supprime les artefacts régénérables
mise run mutate:go -- ./internal/trust   # mutation testing (gremlins), mémoire plafonnée
INKWELL_GOFLAGS=-exec=$PWD/scripts/memcap.sh mise run mutate:go -- ./internal/trust  # + plafond par test (INKWELL_AS_LIMIT)
mise run done -- <bloc>  # définition de « fini », dans l'ordre ; --skip-gate = boucle rapide
mise run gate -- <lot>   # porte figée gates/<lot>.toml ; mesure native : INKWELL_NATIVE_PERF_RUN=1, machine au repos
mise run gate -- --resume <lot>  # reprise sur le même candidat : critères finis réutilisés, échec gardé, mesure native relancée
mise run formal          # TLA+ (tla) + Alloy (alloy), mutants compris ; Java ≥ 17 (outil de dev)
mise run lint:env        # toute variable lue est déclarée dans .env.reference
mise run lint:claude-md  # tout ce que cite ce fichier existe (tâches, chemins, agents, skills, hooks)
mise run test:py         # tests Python de l'outillage | mise run fe:check (Biome, svelte-check, Vitest)
cd protocol && ../scripts/heavy.sh go test ./...   # module imbriqué : `go test ./protocol/...` depuis la racine échoue
INKWELL_PROTOCOL_STEP=N  # étape de conformité imposée (défaut : conformance.CurrentStep, à relever à chaque étape)
```

## Architecture

Module `github.com/oioio-space/inkwell` — éditeur de bureau collaboratif Typst/Markdown,
**sans serveur** (Wails v3 + CodeMirror 6 + Automerge + go-libp2p). Finalité :
`docs/conception.pdf` ; feuille de route : `PROGRESS.md` (étapes 0→6, critère de sortie par étape).
Détail : `docs/architecture.md` (paquets, règles de dépendance), `docs/adr/` (décisions — les lire
avant de remettre une frontière en cause), `docs/glossary.md`, `docs/ux-principles.md`,
`docs/open-questions.md`. Docs des briques : skill `inkwell-stack` (Wails v3
`https://v3.wails.io/llms.txt`, Automerge `https://automerge.org/llms.txt`, Go via le MCP gopls de
`.mcp.json` : `go_workspace`, `go_package_api`, `go_diagnostics` après chaque édition).

**Règle : conception moderne, modulaire, performante et sécurisée, « en béton armé » partout.** Chaque
principe a sa preuve (porte, test, revue) dans `docs/engineering-principles.md` (§ 0 pour toute fonction) — le lire avant de concevoir ou de revoir
quoi que ce soit ; une exception exige une ADR.

**Invariants — ne jamais les violer sans nouvelle ADR :**

- Le CRDT vit dans la webview ; Go ne transporte que des **octets opaques**. Aucun code Go
  n'importe Automerge (ADR 0011) ; la convergence se teste en TypeScript.
- La synchronisation entre pairs passe par l'interface `Transport` de `internal/transport` ; seuls
  `internal/transport/p2p` et le module `protocol/libp2p` (étape 4 de l'ADR 0034) importent libp2p.
  Le rendez-vous d'invitation Wormhole est un canal séparé, limité à `internal/invite` (ADR 0032).
- Le protocole est spécifié publiquement dans `docs/protocol/` (ADR 0034), chaque partie selon les dix
  points de `docs/protocol/README.md`. Le module `protocol` (cœur pur, sans E/S) n'importe rien
  d'inkwell ; inkwell importe `protocol` et n'a aucune copie privée de ses codecs.
- Une seule source de vérité par chose : texte → Automerge, binaires → magasin BLAKE3,
  membres → liste signée, versions → go-git (projection).
- Tout octet venu d'un pair est vérifié (signature, rôle) et son décodeur est fuzzé.
- Pont webview ↔ Go = Streams Wails (aucun port local, ADR 0009). Autres serveurs locaux :
  `127.0.0.1`, port éphémère, jeton comparé en temps constant, jamais logué.
- Typst : jamais de téléchargement à l'exécution. `typst` et `tinymist` sont épinglés
  (`internal/sidecar/manifest.json`), récupérés à la construction, embarqués et revérifiés (SHA-256)
  avant chaque lancement ; paquets `@preview` fournis ou dans `.typstdeps/` (ADR 0024).
- cgo est requis par Wails (Linux GTK4/WebKitGTK 6.0, macOS) ; builds natifs par OS en CI (ADR 0003).
- « pair » dans le code, « personne » dans l'UI ; zéro jargon dans l'interface (glossaire).
- Un paquet = un rôle, créé à l'étape qui en a besoin — pas de paquet vide « pour plus tard ».

## Processus

**Par étape :** recherche en ligne (« Research-first ») → `superpowers:brainstorming` → plan dans
`docs/plans/` → chaque bloc suit la définition de « fini » ci-dessous → démontrer le critère de sortie.

**Définition de « fini » d'un bloc (règle unique, 2026-11-08 ; vaut quel que soit le modèle) :**
la méthode v2 (`docs/plans/2026-11-12-method-v2.md`, issue d'un débat entre deux familles de modèles) en fixe
l'exécution : travail en cours limité, revue avant gel puis après le smoke, rigueur selon le risque (liste fermée
pour « complet »), niveaux T0–T3 aux obligations inchangées, `next` → `main` sur le candidat exact, outillage
sur gain démontré.
un bloc = un lot, une étape du protocole ou un lot d'outillage. Il suit cet ordre, et
`mise run done -- <bloc>` (`scripts/block_done.py`) vérifie les étapes 1 à 8 dans le même ordre :
1. **Spécifier d'abord, en exécutable** : vecteurs (`protocol/vectors`), modèles TLA+ (`docs/protocol/tla`)
   et Alloy (`docs/protocol/alloy`) avec mutants, tests à modèle ; puis la porte figée `gates/<bloc>.toml`,
   commitée **avant** le code puis gelée, une fois chacune de ses commandes lancée, rouge pour la bonne raison et
   capable de passer avec les vrais outils (où ils écrivent, ce qu'ils produisent)
   (méthode : `docs/plans/2026-11-08-spec-executable.md` ; gabarit : skill `new-gate` ; modèles : agent
   `formal-modeler`). Le niveau de rigueur du bloc (léger, standard, complet) est choisi d'abord
   (`docs/engineering-principles.md` § 0).
2. **Implémenter** jusqu'à ce que la porte passe, en TDD (`superpowers:test-driven-development`) ;
   tout modèle ; un agent délégué ne commite ni ne stashe jamais ; aucune de ses affirmations
   (« déjà cassé », « instable ») n'est crue sans preuve rejouée, par exemple sur un arbre d'avant
   le changement. Un bloc est commité avant que le suivant commence, et chaque bloc s'implémente dans son
   propre worktree (deux blocs mêlés dans un arbre ne se vérifient plus séparément, 2026-11-12). À chaque
   itération, la boucle rapide (tests des paquets touchés et de leurs consommateurs, tests nommés du défaut,
   `mutants.py --check` et le mutant Go concerné), puis un smoke de 20 à 30 min avant toute porte complète
   (`docs/plans/2026-11-08-spec-executable.md`, « Iteration loop »). La revue d'une autre famille (Codex)
   porte sur la spécification et la porte avant le code, pas seulement après.
3. **`/simplify`** sur tout le diff du bloc (réutilisation, simplification, efficacité, altitude),
   dans le respect des invariants, des portes et des exigences ; constats, appliqué et écarté dans
   `gates/<bloc>/SIMPLIFY.md`. Il passe **avant** les vérifications, qui portent ainsi sur le code
   final. Listes de contrôle : skills `go-style-guide`, `vuln-guard`, `secret-guard`, `simplify`,
   `helper-ergonomics`. C'est cette revue qu'atteste le marqueur du hook pre-commit — ne jamais
   l'armer sans l'avoir faite.
4. Vérifications rapides : lint, `fe:check`, `lint:env`, `lint:claude-md` ; puis `mise run formal`
   (TLA+, Alloy, mutants) ; puis les tests (Go, frontend, `test:py`) ; toute modification d'interface
   passe en plus par le skill `ux-walkthrough` (agent `ux-tester`), avant fusion et avant release, et toute
   fonction visible par une courte séance avec l'utilisatrice devant l'application (SEQ consigné, R14).
5. **Mutation** (gremlins) sur les paquets `protocol/*` touchés : efficacité ≥ 95 %, ou survivants
   justifiés comme équivalents dans `SIMPLIFY.md` (« mutation <paquet> »). Un test nomme la règle de
   la spec qu'il prouve ; l'utilité se mesure à la mutation, pas à la couverture.
6. **Porte complète** (`mise run gate -- <bloc>`, régressions et mesure native comprises ; aucune porte n'en
   lance une autre : un `gate -- X` imbriqué est consigné « différé » et X tourne seul au besoin), en dernier
   car la plus longue, seule sur la machine, lancée dès qu'elle est libre ; après un incident,
   `mise run gate -- --resume <bloc>` ne refait que les critères inachevés.
7. Documents : `PROGRESS.md` (la liste courante unique) et `docs/exigences.md` à jour.
8. **Seule la session Claude commite**, après relecture du diff ; le push relance `mise run ci`. On pousse
   après chaque bloc commité (hors mesure en cours) : la CI part d'une copie propre et attrape ce qu'un
   arbre de travail chargé de blocs non commités masque (un test dépendant de fichiers non commités est
   resté rouge sur `main` le 2026-11-12).

**Registre des exigences :** chaque demande de l'utilisatrice est dans `docs/exigences.md` (date, où,
état) — l'y ajouter dès qu'elle arrive, le mettre à jour à chaque fusion. Elle reçoit aussitôt **sa place
dans le plan global** : un lot de `docs/architecture/coherence.md` §5 (et l'ordre de `docs/objectif-en-cours.md`),
rattachée à un écran ou un mécanisme existant — jamais une tâche isolée ni une « verrue » (voir « Cohérence de
l'interface »).

**Cohérence de ce fichier (règle) :** toute modification de `CLAUDE.md` le garde cohérent :
- une règle vit à un seul endroit — on complète la règle existante, on n'en écrit pas une seconde ;
- tout ce qui est cité existe (tâches, chemins, agents, skills, hooks) ; un chemin à venir porte son
  échéance « (étape N) » ; `mise run lint:claude-md` le vérifie et fait partie de `ci` ;
- ce qui est nouvellement écrit dans le projet (code, nouveaux docs, commits, rapports) est en anglais ;
  les docs existantes restent en français (pas de traduction) ; les messages à l'utilisatrice sont en français ;
- après la modification, relire le fichier entier pour les contradictions ; audit périodique avec
  `/claude-md-management:claude-md-improver`.

## Règles de travail

**Autonomie (règle, 2026-11-12) :** avancer sans s'arrêter, en préférant toujours le choix d'ingénierie pérenne à la vitesse (ce qui évite de perdre du temps ou de créer des difficultés ensuite), en faisant soi-même les choix les plus pertinents dans les orientations déjà prises par le projet (règle d'or, définition de « fini », invariants) ; trancher soi-même tous les choix, produit compris, et consigner chacun dans `docs/autonomous-decisions.md` (date, choix, raison, alternatives) pour le lui présenter quand elle demande où on en est ; elle réoriente à ce moment-là ; enchaîner aussitôt sur le bloc suivant.

**Research-first :** fonder toute réponse technique non triviale sur des sources externes actuelles
(WebSearch/WebFetch pour l'état de l'art, GitHub via `scripts/ghx.sh` pour l'existant, docs
officielles), **puis aller au-delà** : critiquer l'existant, chercher mieux, envisager des approches
neuves ; comparer, recommander la meilleure (même peu courante), citer. Toute décision technique nouvelle
s'écrit avec ses sources dans `docs/plans/` ou `docs/adr/` (le hook `research-grounding.sh` le rappelle à
l'écriture ; la revue après le smoke vérifie qu'aucune décision du diff n'y manque) ; inutile pour une demande
triviale.

**Go moderne (règle) :** toujours la dernière version stable de Go (`go.mod` des deux modules et
`mise.toml` alignés sur https://go.dev/dl, montée à chaque version) et ses capacités nouvelles quand
elles sont cohérentes avec le code. Le skill `use-modern-go` porte une section par version, mise à
jour depuis les notes officielles (`https://go.dev/doc/go1.N`) à chaque montée ; `go fix ./...` applique les modernisations et le linter `modernize`
(`.golangci.yml`) refuse les idiomes dépassés.

**Discipline de modification (d'après andrej-karpathy-skills, MIT) :**
- énoncer ses hypothèses avant d'agir ; en cas de doute, vérifier dans le code ou demander ;
- modifications chirurgicales : chaque ligne changée se rattache à une exigence ou à une porte, pas de
  nettoyage opportuniste hors du périmètre (le `/simplify` porte sur le diff du lot) ;
- critère de réussite vérifiable par une machine avant d'écrire du code (porte du lot, test) ;
- ne jamais affaiblir un test, un seuil ou un délai pour faire passer une porte ;
- **règle d'or** : on traite chaque problème à la racine, clairement et simplement, une fois et bien ; quand
  un problème devient trop difficile ou que les correctifs s'enchaînent, on s'arrête, on redéfinit le problème,
  on remet à plat, on reformalise (spec, modèle, mutants, nouvelle porte), puis on fait bien ; jamais de
  rustine ni de bidouille au coup par coup (2026-11-09, précisée le 2026-11-10).

**Cohérence de l'interface :** une fonctionnalité nouvelle s'intègre à l'existant, jamais en « verrue ».
Elle doit :
- réutiliser les composants, jetons et motifs Adwaita déjà présents (barre d'en-tête, panneaux à
  onglets, bannières, toasts, popovers) ;
- reprendre le même vocabulaire et les mêmes raccourcis ;
- trouver sa place dans un panneau ou un menu existant avant d'en créer un ;
- passer une revue de cohérence visuelle (captures clair et sombre, comparées aux écrans voisins)
  avant la fusion ;
- respecter les règles de `docs/ux-principles.md` et leurs preuves exécutables (jetons Adwaita,
  accessibilité, cliquabilité, effort par tâche, parcours en machines à états) : `docs/engineering-principles.md` §5.

**Performance mesurée :** tout changement qui touche la performance d'un paquet du chemin critique se
prouve avec benchstat, jamais au jugé (skill `go-benchmark`) : `mise run bench:baseline` → changement →
`mise run bench:compare` (`-count` ≥ 10, `-benchmem`) ; on ne garde le changement que sur un gain
statistiquement significatif sans régression d'allocations ni de débit. Ces paquets portent des tests
`Benchmark…` ; la ligne ci-dessous les liste (lue par `scripts/perf/regression.py` — garder
le préfixe exact `HOT_PATHS:`) :

HOT_PATHS: internal/blob internal/transport/frame internal/transport/mux internal/app

**Tâches longues (règle) :** toute tâche longue (plus de 2 minutes ou lancée en arrière-plan :
`done`, `gate`, `ci`, tests complets, mutation, fuzz, mesure native, agent) est **toujours surveillée**,
jamais attendue à l'aveugle ni sondée à coups de `sleep` :
- sa sortie va dans un journal du scratchpad (`cmd > LOG 2>&1; echo "exit $?" >> LOG`) ;
- un Monitor est armé aussitôt avec `scripts/watch-log.sh LOG [secondes]` : il remonte la progression
  (étapes, scores par paquet, compte de mutants), **toute** signature d'échec (ERROR, FAIL, panic,
  OOM, 137, expirations, mesure nulle), le silence prolongé (« stalled ») et la fin ; réarmé à chaque
  expiration jusqu'à la fin ;
- les scripts longs écrivent leur progression ligne à ligne, sans la retenir (`block_done.py` diffuse la
  sortie de gremlins) ; un code de sortie non nul est toujours rapporté, jamais avalé ;
- l'utilisatrice est tenue au courant **au fur et à mesure** : chaque étape franchie, chaque résultat, et,
  quand une étape longue reste silencieuse, un point de progression toutes les 5 minutes (ce qui tourne,
  depuis combien de temps, dernier résultat) ; un échec est traité dès qu'il remonte ;
- une tâche qui peut durer plus que la session (porte complète, `done`) est lancée détachée
  (`setsid nohup bash -c "cmd > LOG 2>&1; echo \"exit \$?\" >> LOG" & disown`) ;
- sa vie se vérifie avec `rtk proxy pgrep -af …` (le filtre rtk peut masquer des processus) ; un journal
  silencieux n'est pas une preuve de mort, et une seconde exécution n'est jamais lancée dans le même
  arbre avant que la première soit prouvée morte (deux portes C5S concurrentes, 2026-11-12).
Le hook `long-task-monitor.sh` le rappelle à chaque lancement en arrière-plan.

**Mémoire (règle) :** tout travail lourd (tests Go, Python et frontend, couverture, bout en bout,
netsim, mutation testing gremlins et Stryker) passe par `scripts/heavy.sh` :
- au plus 2 tâches lourdes en même temps sur la machine, chacune dans son cgroup, plafonnée à 8 Go
  sans swap : un dépassement tue la tâche seule, pas la session ;
- les tâches mise `test`, `test:ci`, `test:py`, `fe:check`, `cover`, `e2e*` et `mutate:go` l'appellent
  déjà ; toute autre commande de test, à la main ou par un agent, passe par `heavy.sh` (ou
  `systemd-run --user --scope -p MemoryMax=…`) avec un `timeout` ;
- le hook `heavy-guard.sh` refuse toute commande de test non plafonnée (Claude et ses sous-agents ;
  Codex et opencode n'y sont pas soumis, d'où les tâches mise plafonnées) ;
- pas plus de 4 agents en parallèle ; mutation testing limité à 4 workers et à un paquet à la fois ;
- une mesure (perf native, netsim, porte complète) a la machine à elle : elle tourne sur les cœurs P
  (`taskset -c 0-11` sur ce portable), les agents et le reste sur les cœurs E (`taskset -c 12-19`), et aucun
  agent ne lance de travail lourd pendant une porte complète (un modèle chronométré a expiré sous la charge,
  2026-11-12) ;
- origine : deux plantages globaux le 2026-10-26 (mutant à 8,7 Go, Chrome à 3,4 Go), un troisième le
  2026-11-08 (test Python non plafonné à 21 Go, voir « Pièges connus »).

## Routage des modèles et économie de tokens

**Règle :** pour chaque tâche (agent, sous-tâche, skill, revue, audit), le modèle le moins cher et
l'effort le plus bas qui gardent la qualité, choisis explicitement à chaque lancement (`model:` +
`effort:`). Chaque agent de `.claude/agents/` fixe son niveau dans son en-tête.

| Tâche | Exécutant | Modèle / effort |
|-------|-----------|-----------------|
| Brouillon de message de commit, tri de longs journaux, brouillon de traduction i18n | `scripts/llm.sh` (ordre : Groq → Cloudflare Workers AI → OpenRouter gratuit → OpenRouter payant ZDR ; traduction et rédaction : OpenRouter payant seul — ADR 0013 ; quotas : `mise run quotas`) ; texte mécanique seulement, jamais secret, sécurité, conception ni revue de code ; code de sortie 3 → Haiku | open-weight hébergé |
| Implémentation bien bornée, enquête sur un bogue, second avis, revue adverse (sans tokens Claude) | Codex : agent `codex:codex-rescue`, `/codex:rescue`, `/codex:review`, `/codex:adversarial-review` | connexion ChatGPT (quota limité) ; Claude relit avant fusion |
| Implémentation guidée par une porte, sans crédit | `mise x -- bash scripts/opencode-free.sh -m <modèle> < consigne.md` (modèles gratuits OpenRouter ou Zen ; gortex branché, rtk injecté) | gratuit ; jamais de sécurité ; ne commite jamais |
| Relecture critique d'une spécification de sécurité ou d'une ADR (crédit compté) | `scripts/claude-or.sh -t opus` ou `scripts/claude-api.sh`, et Codex (`/codex:adversarial-review`) dès qu'il est disponible : deux familles de modèles | un seul appel ciblé chacun |
| Lancer les vérifications (lint, tests, ci, scans), corrections de format triviales | `quality-runner` | Haiku / low |
| Docs, `PROGRESS.md`, messages de commit | `scribe` | Haiku / low |
| Recherche de code ou de fichiers, « où est X » | `explorer` (ou `Explore` intégré) | Haiku / low |
| Écrire du Go et ses tests, refactorisations | `go-dev` | Sonnet / medium |
| Relire un diff Go | `go-reviewer` | Sonnet / medium |
| Parcours d'interface comme une personne | `ux-tester` | Sonnet / medium (comme simplify) |
| Algorithme nouveau, architecture | `architect` | Opus / high |
| Modèles TLA+ et Alloy, mutants (étape 1 d'un bloc) | `formal-modeler` | Opus / high |
| Audit de sécurité approfondi | `security-auditor` | Opus / high (xhigh seulement pour l'audit final de release) |

**Codex, même règle :** chaque appel `/codex:*` ou `codex:codex-rescue` précise `--model` et
`--effort` (mécanique → `--model spark`, `--effort low` ; `--profile openrouter-cheap` seulement en CLI
`codex` ; code et revue → modèle par défaut, `--effort medium` ; diagnostic difficile → `--effort high`).

**Boucle principale (Opus) :**
- elle orchestre ; elle ne débogue pas, ne lit pas de journaux et ne résout pas de gros conflits : tout
  cela va à un agent Sonnet ou Haiku ;
- lire peu : jamais un fichier déjà en contexte ; journaux via `rtk` ; code via gortex ou gopls ;
  `grep`, `head` ou `tail` ciblés plutôt qu'un fichier entier ;
- briefs d'agent courts, rapports de 10 lignes au plus, tâches indépendantes en parallèle ;
- réponses à l'utilisatrice brèves, en français (le reste en anglais), et chacune compréhensible seule (elle ne suit pas en
  continu ni n'a les documents sous les yeux) : quelle tâche et pourquoi, où on en est, ce qui reste ;
  pousser avec `scripts/qpush.sh` (verdict seul) ;
- ne jamais écrire le mot-clé de commit de git dans une commande qui n'en est pas une : les hooks se
  déclencheraient.

## Pièges connus

- La mesure native échoue si la machine est chargée (CI en parallèle : « cadence slower than
  nominal ») : la relancer au repos, ne pas conclure.
- `gates/guard-overrides.json` est indexé par empreinte de contenu : modifier un fichier couvert exige
  une nouvelle entrée justifiée.
- Les tests qui fabriquent des messages entre pairs utilisent `encodeCBOR` de `@inkwell/protocol`,
  jamais `cbor-x` (étiquettes refusées par le profil) ; `targetId` partout.
- gremlins marque « non couvertes » les conditions de `case` : vérifier par une mutation manuelle
  avant d'écrire un test. Son délai est un multiple de la mesure de couverture : sans
  `GOFLAGS=-count=1` (posé par `mutate:go` et `done`), le cache de tests la rend instantanée et tous
  les mutants finissent « timed out » ; coefficient de délai 10 ; chaque binaire de test plafonné à
  1,8 Go par `memcap.sh` (4 workers sous les 8 Go de `heavy.sh` : un mutant qui alloue sans fin échoue
  seul au lieu de faire tuer la mesure entière) ; `done` refuse une mesure où plus de
  10 % des mutants expirent (ils sortent du score et peuvent cacher des survivants). Le module
  `protocol` se mute depuis `protocol/` (`mutate:go` depuis la racine ne le voit pas ; `done` le fait).
- Avant de supprimer un worktree ou un dossier, lister aussi les fichiers ignorés
  (`git status --porcelain --ignored`) : « ignoré » ne veut pas dire « régénérable ». Les rapports de
  porte de L3 et P2, alors ignorés, ont été perdus ainsi le 2026-11-08 ; les rapports `gates/*/*.md`
  sont désormais versionnés, seuls les journaux bruts restent ignorés. Une porte (`gate.py`) ne vide que
  les preuves de `gates/<lot>/` et garde ses `*.md` : son ancien `rmtree` avait effacé un rapport.
- Chaque worktree coûte au démon gortex un graphe complet (une vue par checkout) : 16 worktrees, dont 10 oubliés de séances anciennes, l'ont porté à 14 Go et ont fait tuer des processus par le noyau (OOM global, 2026-11-12). Le worktree d'un bloc est retiré dès le bloc fusionné (règle ci-dessus d'abord, modifications non commitées archivées hors du dépôt) ; au plus un worktree par bloc actif.
- Simuler un flux d'E/S avec `io.BytesIO`, jamais un `MagicMock` dont `read()` rend toujours la même
  valeur : `shutil.copyfileobj` boucle sans fin (21 Go le 2026-11-08 ; `/tmp` est un tmpfs, donc en RAM).
