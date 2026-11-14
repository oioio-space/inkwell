# Documentation — la carte

Une information a **une seule place**. Pour savoir où en est le projet et ce qui reste : **[../PROGRESS.md](../PROGRESS.md)**.
Règles de travail : [../CLAUDE.md](../CLAUDE.md). Tout fichier ou dossier ajouté à `docs/` doit figurer ici
(`mise run lint:claude-md` le vérifie).

## Je cherche…

| …ceci | Seule place | Remarque |
|---|---|---|
| Où on en est, le bloc en cours, ce qui reste pour R1, la suite | [../PROGRESS.md](../PROGRESS.md) | résumé ordonné avec liens |
| La finalité du produit | [conception.pdf](conception.pdf) | document d'origine |
| Les demandes de l'utilisatrice et leur état | [exigences.md](exigences.md) | une ligne par exigence |
| Ce qui reste à trancher | [open-questions.md](open-questions.md) | questions closes gardées avec leur lien |
| Le registre des blocs lu par les outils, les décisions de pilotage en vigueur | [objectif-en-cours.md](objectif-en-cours.md) | bloc `gate-lots` lu par `scripts/gate.py` |
| Comment on travaille (méthode, preuves) | [engineering-principles.md](engineering-principles.md), [plans/2026-11-12-method-v2.md](plans/2026-11-12-method-v2.md) | |
| Lancer chaque exécutant IA (Codex, OpenRouter, opencode, Gemini), prérequis hors dépôt, pièges | [executors.md](executors.md) | point d'entrée des IA hors Claude : [../AGENTS.md](../AGENTS.md) |
| Les choix tranchés en autonomie | [autonomous-decisions.md](autonomous-decisions.md) | décisions durables seulement |
| Les choix produit de l'utilisatrice | [design/decisions-utilisatrice.md](design/decisions-utilisatrice.md) | |
| Les décisions d'architecture | [adr/](adr/) | MADR, numérotées |
| Doutes et amendements aux spécifications acceptées | [architecture/decisions.md](architecture/decisions.md) | |
| L'architecture (paquets, dépendances, frontières) | [architecture.md](architecture.md) → [architecture/](architecture/README.md) | spec, cohérence §5 (périmètre MVP/v1), traçabilité |
| Le protocole public | [protocol/](protocol/README.md) | spec normative, CDDL, TLA+, Alloy, vecteurs |
| L'interface : règles, système visuel, vocabulaire | [ux-principles.md](ux-principles.md), [glossary.md](glossary.md), [design/](design/README.md) | |
| Les parcours d'interface et leurs captures | [ux/](ux/) | rapports datés |
| Le plan détaillé d'un bloc et sa liste de restes | [plans/](plans/) | un fichier daté par bloc |
| Les mesures de performance | [perf/](perf/) | rapports datés ; preuves des portes dans `gates/<lot>/` |
| Les revues (Codex, tests) et leur couverture | [reviews/](reviews/codex-coverage.md) | |
| Publier une version | [releasing.md](releasing.md) | |
| Le relais auto-hébergé | [relay.md](relay.md) | |
| Les limites des sidecars par plateforme | [sidecar-platform-limits.md](sidecar-platform-limits.md) | |
| L'historique remplacé (journaux) | [archive/](archive/) | lecture seule |
