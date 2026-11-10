# Principes d'ingénierie — moderne, modulaire, performant, sécurisé

**Règle du projet, non négociable.** Chaque principe ci-dessous a une **preuve** : un gate,
un test ou une revue. Un principe sans preuve n'est qu'un vœu. Une exception exige une ADR.

## 0. Béton armé, pour toute l'application (2026-11-09)

**Le but** (demande de l'utilisatrice) : que tout soit reproductible par d'autres, très clair et très performant, et
qu'on découvre les problèmes *avant* de les coder, jamais par des correctifs hasardeux. Cette section dit **ce
qu'une fonction, un mécanisme ou un écran doit avoir** pour être accepté. Elle ne redit pas les règles qui vivent
ailleurs ; elle y renvoie :

| Ce qu'il faut | Où c'est appliqué (source unique) |
|---|---|
| Un comportement écrit et sans ambiguïté avant le code (table de décision, machine à états ou règle normative, cas limites compris), puis une porte figée | `CLAUDE.md`, définition de « fini », étape 1 |
| Un modèle formel avec mutants dès qu'il y a un état, de la concurrence ou un parcours à plus de trois états | même étape ; pour l'interface : § 5 (UF-3) ; pour le protocole : `docs/protocol/README.md`, point 9 |
| Des entrées bornées, chaque valeur hors plage traitée, chaque erreur dite avec une action | § 4 (octets venus d'un pair) ; `docs/ux-principles.md`, règle 5 |
| Des tests qui prouvent la règle, pas la couverture | `CLAUDE.md`, étapes 2 et 5 (TDD, mutation) |
| Des budgets de performance chiffrés avant le code, puis mesurés | § 3 ; `CLAUDE.md`, « Performance mesurée » |
| Sécurité et confidentialité pensées pour chaque fonction | § 4 ; agent `security-auditor` |
| Ce qui arrive est observable : événements, santé visible | W44, `docs/plans/2026-11-09-diagnostics.md` |
| Le texte suffit pour que quelqu'un d'autre le refasse (liste vide de « ce qu'il faudrait deviner ») | pour le protocole : `docs/protocol/README.md`, point 10 ; pour le reste : la relecture du plan à l'étape 1 |
| Quand ça devient difficile : on s'arrête, on reformalise, jamais de rustine | `CLAUDE.md`, « Discipline de modification » |

Ce qui existe déjà est audité au regard de ce tableau : le protocole (bloc PA), puis le reste de l'application,
écran par écran et paquet par paquet (bloc AA).

**Proportion (décision du 2026-11-09).** Le tableau s'applique selon le risque, choisi au début du bloc et écrit
dans sa porte :

- **léger** (documentation, petit outillage sans état) : relecture, vérifications rapides ;
- **standard** (fonction sans état complexe) : table de décision, tests nommés, walkthrough si l'interface change ;
- **complet** (état, concurrence, protocole, sécurité, données venues d'un pair) : tout le tableau, modèle formel avec
  mutants et relecture indépendante (une autre famille de modèles quand elle est disponible).

Les relectures indépendantes sont faites par des agents neufs, jamais par l'utilisatrice ; elle décide (ADR, choix
de portée) et teste l'usage lors d'une courte séance devant l'application.

En cas de doute, on prend le niveau au-dessus.

**Borne de temps (décision du 2026-11-10).** Au plus deux relectures indépendantes par bloc ; après la seconde,
seuls les constats graves bloquent, les autres deviennent des suivis écrits dans le plan du bloc. Une tâche simple
reste simple : la rigueur se règle sur le risque, pas sur le temps disponible.

## 1. Moderne

| Règle | Preuve |
|---|---|
| Dernière version mineure de Go avec correctifs de sécurité ; idiomes récents (`min`/`max`, `range` sur entiers, itérateurs, `errors.Join`, `slog`, `testing/synctest`) | `mise.toml` épinglé ; skill `use-modern-go` ; `go fix` (modernizers) sans diff |
| API standard avant dépendance : `os.Root`, `crypto/*`, `log/slog`, `net/http`, `embed` | revue ; toute nouvelle dépendance justifiée dans la PR |
| Frontend TypeScript `strict`, ES modules, Vite ; aucune dépendance non maintenue | `tsc --noEmit`, lint frontend en CI (étape 1) |
| Docs des briques lues à jour, jamais de mémoire | skill `inkwell-stack`, règle research-first |

## 2. Modulaire

| Règle | Preuve |
|---|---|
| Un paquet = un rôle, sous `internal/` ; API exportée minimale et documentée | revue GoDoc (pre-commit) |
| Interfaces **petites, définies chez le consommateur** ; on accepte des interfaces, on renvoie des structs | revue ; assertion de conformité `var _ I = (*T)(nil)` |
| Dépendances orientées vers le cœur : `transport`, `trust`, `identity` n'importent rien d'applicatif ; seul `internal/app` câble tout | test d'architecture (`go list -deps`) en CI dès l'étape 1 |
| Aucune variable globale mutable, aucun `init()` à effet de bord ; injection explicite par constructeur | lint + revue |
| Chaque couche remplaçable derrière une interface (Transport, stockage, sidecar, pont) | une implémentation de test par interface (`memnet`, magasin en mémoire…) |

## 3. Performant

| Règle | Preuve |
|---|---|
| Mesurer avant d'optimiser ; toute optimisation est prouvée par `benchstat` (`-count ≥ 10`) | `mise run bench:compare` ; paquets listés dans `HOT_PATHS:` (CLAUDE.md) |
| Budgets explicites : frappe → affichage distant < 100 ms en LAN ; aperçu Typst < 300 ms après la dernière frappe ; démarrage < 1 s ; document de 50 000 opérations ouvert < 2 s | tests de charge et e2e (étapes 1, 3, 5) |
| Pas d'allocation dans les boucles chaudes (relais d'updates, hachage de blobs) ; flux plutôt que `ReadAll` | `-benchmem` ; revue |
| Concurrence bornée : chaque goroutine a une condition de sortie et un propriétaire ; `context` partout | skill `goroutine-leak-check` ; `go test -race` |
| PGO (`default.pgo`) sur le binaire une fois un profil réel disponible | étape 6 |

## 4. Sécurisé (sécurité par défaut)

| Règle | Preuve |
|---|---|
| **Tout octet venu d'un pair est hostile** : taille bornée, décodage strict, signature et rôle vérifiés **avant** usage | fuzz natif de chaque décodeur (`go test -fuzz`) ; tests de rejet |
| E/S fichier du projet uniquement via **`os.Root`** (anti-traversée `..` / liens) ; `--root` pour typst et tinymist | revue ; test d'évasion par lien symbolique |
| Crypto standard uniquement (`crypto/ed25519`, `crypto/rand`, Noise via libp2p) ; comparaisons de secrets en temps constant | `gosec` (gate pre-commit) |
| Serveurs locaux : `127.0.0.1`, port éphémère, jeton aléatoire, jamais logué | revue ; test d'accès sans jeton → refus |
| Webview : CSP stricte (`default-src 'self'`, styles par **nonce par lancement** — `internal/app/csp.go` + `EditorView.cspNonce`, jamais `'unsafe-inline'` en production ; seul `'wasm-unsafe-eval'` pour Automerge), aucun contenu distant, aucun `eval` ; chaque message du pont validé et typé côté Go | test du pont ; inspiration [checklist sécurité Electron](https://www.electronjs.org/docs/latest/tutorial/security) |
| Sidecars : hachage SHA-256 vérifié avant chaque lancement, timeouts, mort avec le parent | test d'altération du binaire → refus |
| Moindre privilège : fichiers `0o600`/`0o700`, aucune élévation, aucune donnée de document dans les logs ou rapports de bug | revue ; `gosec` |
| Chaque partie du protocole est publique, complète et réutilisable par d'autres (les dix points de `docs/protocol/README.md`) | vecteurs en fichiers de données, décodeur fuzzé, modèle formel avec mutants, liste vide de « ce qu'il faudrait deviner » ; audit des parties existantes (bloc PA) |
| Chaîne d'approvisionnement : versions exactes, `govulncheck`, SBOM + `grype`, `gitleaks`, Dependabot sans auto-merge | gates `mise run ci` (ADR 0008) |

## 5. Interface (W39, W40, 2026-11-09)

| Règle | Preuve |
|---|---|
| Ce que l'interface affiche comme vrai (« Synchronisé », rôles, états) suit un modèle d'état formel | modèle TLA+ et test fast-check à modèle, mutants (`docs/plans/2026-11-09-ui-formal.md`, bloc SI1) |
| Accessibilité WCAG 2.2 AA dans **chaque** spec Playwright, dialogues et popovers compris (UF-1) | fixture axe-core commune ; violation semée ⇒ rouge (bloc UF1) |
| Ce qui doit être cliquable l'est : chaque élément interactif visible est la cible de son centre (UF-2) | invariant `elementFromPoint` dans la fixture ; mutant de recouvrement attrapé (bloc UF1) |
| Un parcours de plus de trois états est une machine XState v5, ses tests Playwright en sont générés (UF-3) | `xstate/graph` chemins les plus courts, assertions par état, mutant par machine (blocs UF2…) |
| Composants Adwaita, jetons sémantiques seuls, aucune valeur brute (UF-4) | lint dans `fe:check` ; couleur brute semée ⇒ rouge (bloc UF1) |
| Effort borné pour les cinq tâches (créer, partager, relire, présenter, exporter) (UF-5) | comptes d'opérateurs et temps KLM par tâche, budgets figés ; étape ajoutée ⇒ rouge (bloc UF2) |
| Simple à voir, rien d'inaccessible : budget de commandes visibles par vue ; chaque commande dans la palette et à deux pas au plus d'un menu (UF-6) | énumération du registre de commandes |
| Annuler plutôt que confirmer (UF-7) | toute transition destructive d'une machine a son « Annuler » |
| L'apparence ne dérive pas (UF-8) | captures de référence par vue, clair et sombre (bloc UF1) |
| Tout geste qui lance une action a un retour visible en moins de 100 ms (UF-9) | invariant de la fixture ; mutant de toast retardé attrapé |
| Chaque mode d'un parcours a son propre arbre d'accessibilité ; chaque vue a son instantané ARIA (UF-10, UF-11) | `toMatchAriaSnapshot` par état ; bouton renommé ⇒ rouge |
| Chaque widget suit le contrat clavier de l'APG WAI-ARIA (UF-12) | tests générés par type de widget ; Échap cassé ⇒ rouge |
| L'utilisabilité se mesure, ne se prouve pas | skill `ux-walkthrough` (agent `ux-tester`), captures clair et sombre ; en personne à chaque version : SEQ par tâche, SUS et UEQ-S (jamais bloquant en CI) |

## Sources

[Go security best practices](https://go.dev/security/best-practices) ·
[os.Root (Go blog)](https://go.dev/blog/osroot) ·
[Electron security checklist](https://www.electronjs.org/docs/latest/tutorial/security) ·
[Go Code Review Comments — interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)
