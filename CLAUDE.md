# inkwell — project guidance

State & roadmap: see `PROGRESS.md`. Tooling is managed by **mise** — run everything with
`mise run …` (lint, test, ci, fmt, scan:*, sbom, …). Commits go through git hooks
(janitor → secrets → vulns → style → **/simplify review attestation**) and Claude
PreToolUse hooks (style/simplify/secret/vuln review, modern-Go injection on `.go` edits).
The `/simplify` review is a **mandatory gate on every commit path**: `.githooks/pre-commit`
requires the approval marker (`$GIT_DIR/claude-simplify-ok`, keyed to the exact staged
diff) — arm it only after genuinely completing the review; `--no-verify` bypasses
(emergency only).

## Commands (all via mise)

```bash
mise run setup           # bootstrap toolchain + wire git hooks
mise run test            # unit tests        | mise run test:watch  (TDD)
mise run sidecar:fetch   # typst + tinymist (SHA-256 épinglés, build-time) ; -- -target all
mise run lint            # golangci-lint v2  | mise run fmt
mise run cover:check     # coverage gate (COVER_MIN=85)
mise run ci              # full gate = what CI runs (lint+test+scans)
mise run bench:baseline  # then change code, then: mise run bench:compare (benchstat)
mise run scan:code       # gosec + govulncheck | scan:secrets | scan:sbom (grype)
mise run clean           # remove regenerable artifacts
mise run mutate:go -- ./internal/trust   # mutation testing (gremlins), memory-capped
INKWELL_GOFLAGS=-exec=$PWD/scripts/memcap.sh mise run mutate:go -- ./internal/trust  # + per-test address-space cap (INKWELL_AS_LIMIT)
```

## Architecture

Module `github.com/oioio-space/inkwell` — éditeur de bureau collaboratif Typst/Markdown,
**sans serveur** (Wails v3 + CodeMirror 6 + Automerge + go-libp2p). Finalité :
`docs/conception.pdf` ; feuille de route : `PROGRESS.md` (étapes 0→6, critère de sortie
par étape). Détail : `docs/architecture.md` (paquets, règles de dépendance),
`docs/adr/` (décisions — lire avant de remettre une frontière en cause),
`docs/glossary.md`, `docs/ux-principles.md`, `docs/open-questions.md`.

**Règle : conception moderne, modulaire, performante et sécurisée.** Chaque principe a sa
preuve (gate, test, revue) dans `docs/engineering-principles.md` — le lire avant de
concevoir ou de revoir quoi que ce soit ; une exception exige une ADR.

**Invariants — ne jamais les violer sans nouvelle ADR :**

- Le CRDT vit dans la webview ; Go ne transporte que des **octets opaques**. Aucun code Go
  n'importe Automerge (ADR 0011) ; la convergence se teste en TypeScript.
- Tout le réseau passe par l'interface `internal/transport.Transport` ; seul
  `internal/transport/p2p` importe libp2p.
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

**Registre des exigences :** chaque demande de l'utilisatrice est dans `docs/exigences.md` (date, où, état) — l'y ajouter dès qu'elle arrive, le mettre à jour à chaque fusion.

**Tester comme un humain :** toute modification d'interface passe par le skill `ux-walkthrough` (agent `ux-tester`) avant fusion et avant release.

**Processus par étape :** `superpowers:brainstorming` → plan dans `docs/plans/` →
TDD (`superpowers:test-driven-development`) → revue → cocher `PROGRESS.md` → démontrer
le critère de sortie. Toute brique se recherche d'abord en ligne (règle research-first).

**Docs des briques** (skill `inkwell-stack`) : Wails v3 `https://v3.wails.io/llms.txt`,
Automerge `https://automerge.org/llms.txt`, Go via **gopls MCP** (`.mcp.json` — suivre
ses instructions : `go_workspace`, `go_package_api`, `go_diagnostics` après chaque édition).


## ⚡ Benchmark the hot path & prove perf changes

Any perf-affecting change to a hot-path package is proven with benchstat — never by feel
(go-benchmark skill): `mise run bench:baseline` → change → `mise run bench:compare`
(`-count` ≥ 10, `-benchmem`); keep the change only on a statistically significant gain
with no alloc/throughput regression. Hot-path packages carry `Benchmark…` tests.
List them on the line below (space-separated package paths; read by
`.claude/hooks/benchmark-context.sh` — keep the exact `HOT_PATHS:` prefix):

HOT_PATHS: internal/blob internal/transport/frame internal/transport/mux

## Research-first

Ground non-trivial / technical answers in CURRENT external sources before answering —
WebSearch/WebFetch for the state of the art, GitHub (`scripts/ghx.sh`) for prior art &
libraries, official docs. **Then go beyond the existing**: critique it, look for
improvements, and consider out-of-the-box / novel approaches. Prefer sources over memory,
compare alternatives, recommend the best (even if uncommon), cite. A `UserPromptSubmit`
hook (`.claude/hooks/research-grounding.sh`) reinforces this every prompt; skip for
trivial asks.

## Sub-agent routing (token-economical, no quality loss)

Each agent in `.claude/agents/` pins its own tier via frontmatter.

| Task | Agent | Model / effort |
|------|-------|----------------|
| Commit-message draft, long-log triage, i18n translation draft | `scripts/llm.sh` (OpenRouter: DeepSeek V4.1 Flash / Mistral Large 4, ZDR — ADR 0013) | hosted open-weight |
| Run checks (lint/test/ci/scan), trivial format fixes | `quality-runner` | Haiku / low |
| Docs, PROGRESS.md, commit messages | `scribe` | Haiku / low |
| Code/file search, fan-out, "where is X" | `explorer` (or built-in `Explore`) | Haiku / low |
| Implement Go code & tests, refactors | `go-dev` | Sonnet / medium |
| Review a Go diff | `go-reviewer` | Sonnet / medium |
| Novel algorithm / architecture design | `architect` | Opus / high |
| Deep security / vuln audit | `security-auditor` | Opus / high |

**Règle — router modèle ET effort pour chaque tâche** (agent, sous-tâche, skill, revue, audit) : le modèle le moins cher et l'effort le plus bas qui gardent la qualité, choisis explicitement à chaque lancement (`model:` + `effort:`). Défauts : mécanique → Haiku/`llm.sh`, effort low ; code, revue, simplify, UX → Sonnet, medium ; sécurité, architecture → Opus, high (xhigh seulement pour un audit final de release).

**Économie de tokens, pratique :** pousser avec `scripts/qpush.sh` (verdict seul) ; préciser `model:` à chaque agent lancé (Opus seulement pour sécurité/architecture) ; rapports courts ; textes mécaniques via `scripts/llm.sh`.

**Rule — token economy without quality loss:** cheapest tier that keeps quality; never
re-read a file already in context; read logs through `rtk` / `llm.sh triage`, code through
gopls MCP (`go_package_api`, `go_search`) rather than whole files. Hosted open-weight models
(`scripts/llm.sh`, exit 3 → fall back to Haiku) take mechanical text only — never security,
design or code review. No local models.

Rule of thumb: mechanical → hosted open-weight or Haiku; writing/reviewing Go → Sonnet; novel algorithm design
or security judgement → Opus. Never use Opus for what a cheaper tier handles at equal
quality. Run independent sub-tasks in parallel.

### Revue avant commit

Un seul hook (`pre-commit-simplify.sh`) bloque un commit tant que la revue n'est pas attestée. Il affiche une seule ligne : les points à vérifier, avec les points Go seulement si du Go est indexé. Les listes complètes sont dans les skills `go-style-guide`, `vuln-guard`, `secret-guard`, `simplify` et `helper-ergonomics`. Un gros diff Go va à `go-reviewer` (Sonnet) ; un petit diff se revoit dans la boucle principale.

**Économie de tokens dans la boucle principale (Opus) :**
- La boucle principale orchestre ; elle ne débogue pas, ne lit pas de journaux et ne résout pas de gros conflits. Tout cela va à un agent Sonnet ou Haiku.
- Lire peu : `grep`, `head` ou `tail` ciblés, jamais un fichier entier quand quelques lignes suffisent.
- Briefs d'agent courts ; rapports de 10 lignes au plus.
- Réponses à l'utilisatrice brèves.
- Ne jamais écrire le mot-clé de commit de git dans une commande qui n'en est pas une : les hooks se déclencheraient.

**Cohérence (règle) :** une fonctionnalité nouvelle s'intègre à l'existant, jamais en « verrue ». Elle doit :
- réutiliser les composants, jetons et motifs Adwaita déjà présents (barre d'en-tête, panneaux à onglets, bannières, toasts, popovers) ;
- reprendre le même vocabulaire et les mêmes raccourcis ;
- trouver sa place dans un panneau ou un menu existant avant d'en créer un ;
- passer une revue de cohérence visuelle (captures clair et sombre, comparées aux écrans voisins) avant la fusion.

**Mémoire (règle) :** tout travail lourd passe par `scripts/heavy.sh` :
- Concerne les tests Go, la couverture, les tests de bout en bout, netsim et le mutation testing (gremlins, Stryker).
- Les tâches mise `test`, `test:ci`, `cover`, `e2e*` et `mutate:go` l'appellent déjà.
- Au plus 2 tâches lourdes tournent en même temps sur la machine ; chacune est plafonnée à 8 Go, sans swap, dans son propre cgroup. Un dépassement tue la tâche seule, pas la session.
- Pas plus de 4 agents en parallèle.
- Le mutation testing se limite à 4 workers et à un paquet à la fois.
- Deux plantages globaux faute de mémoire le 2026-10-26 sont à l'origine de cette règle.
