# Principes d'ingénierie — moderne, modulaire, performant, sécurisé

**Règle du projet, non négociable.** Chaque principe ci-dessous a une **preuve** : un gate,
un test ou une revue. Un principe sans preuve n'est qu'un vœu. Une exception exige une ADR.

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
| Webview : CSP stricte (`default-src 'self'`), aucun contenu distant, aucun `eval` ; chaque message du pont validé et typé côté Go | test du pont ; inspiration [checklist sécurité Electron](https://www.electronjs.org/docs/latest/tutorial/security) |
| Sidecars : hachage SHA-256 vérifié avant chaque lancement, timeouts, mort avec le parent | test d'altération du binaire → refus |
| Moindre privilège : fichiers `0o600`/`0o700`, aucune élévation, aucune donnée de document dans les logs ou rapports de bug | revue ; `gosec` |
| Chaîne d'approvisionnement : versions exactes, `govulncheck`, SBOM + `grype`, `gitleaks`, Dependabot sans auto-merge | gates `mise run ci` (ADR 0008) |

## Sources

[Go security best practices](https://go.dev/security/best-practices) ·
[os.Root (Go blog)](https://go.dev/blog/osroot) ·
[Electron security checklist](https://www.electronjs.org/docs/latest/tutorial/security) ·
[Go Code Review Comments — interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)
