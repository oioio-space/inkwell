# PROGRESS journal (archive)

The dated change table that `PROGRESS.md` carried until 2026-11-12, kept verbatim. Current status: `PROGRESS.md`; full history: `git log`.

| Date | Changement |
|------|------------|
| 2026-11-12 | PB, Go 1.27.4, C5S, RC1, LF0a3, GR, méthode v2 : mesure appariée (ADR 0038) ; Go 1.27.4 ; qualité du lien en trois niveaux acceptée ; reconnexion sans relais ; coloration incrémentale (résidu LF0b) ; `--resume` ; plus de portes imbriquées (`gates/deferred.jsonl`) ; méthode v2 (`docs/plans/2026-11-12-method-v2.md`). |
| 2026-11-08 | ADR 0034 étape 2 (P2) : `protocol/sig`, Ed25519 strict, porte `gates/P2.json` verte. |
| 2026-11-08 | ADR 0034 étape 3 (P3) : TLC épinglé, profil CBOR, schémas §9 ; porte `gates/P3.json` verte. |
| 2026-11-08 | Bloc RL1 : règle L de la spec sur sync/1 — une écriture échouée relance la synchronisation (plus de perte silencieuse après une coupure, NI-2 vert sur 3 graines) ; mesures de lien v1 et notification Links jusqu'à la webview ; outillage réseau perturbé (NI0, en cours). Porte `gates/RL1.json` verte, régressions et mesure native comprises. |
| 2026-11-09 | Bloc SP1b (chemin d'envoi provisoire, remplace WB et SP1) : plancher 2 Kio/s, garde sans progrès 90 s, yamux configuré (120 s, fenêtre fixe 256 Kio), partiels 120 s ; un gros collage sur relais lent arrive du premier coup, sans coupure ni resynchronisation (NI-3 C1 strict). Frappe sur relais lent encore lente (NI-3 C2, panne connue jusqu'aux étapes 4 à 6). Correctif go-libp2p (fork épinglé, libp2p/go-libp2p#3575, #3576). Porte `gates/SP1b.json` verte, régressions et mesure native comprises. |
| 2026-11-09 | Bloc CI1b (outillage, W35) : au pré-push, le netsim dégradé ne tourne que si le diff touche ses entrées, calculées depuis `go list -deps` des binaires lancés (`scripts/netsim/affected.py`, tout échec ⇒ il tourne) ; sur GitHub il ne tourne plus qu'une fois (job `netsim`), le job `ci` lance `ci:core` ; `done` inchangé. |
| 2026-11-08 | Triage du registre (exigences 🔨) et mise à jour de PROGRESS. |
| 2026-11-03 | Lots L1, L2 (N42 précisée : rien à installer à part), L-maj (dépendances). |
| 2026-11-02 | Lot L0 : portes HDAE et mesure de référence ; spécifications closes, méthode HDAE. |
| 2026-10-25 | Bootstrap go-starter ; fondations : architecture, 8 ADR, glossaire, UX, roadmap, gopls MCP, Dependabot. |
| 2026-10-25 | Étape 3 (C1, compilation de C2) : sidecars `typst`/`tinymist` épinglés, embarqués et supervisés ; `internal/typst` compile hors ligne (PDF, SVG, diagnostics, `query`, `Watch`). Reste : aperçu incrémental et LSP par `tinymist` (stdio), présentations. |
| 2026-10-26 | Étape 3 (C2) : aperçu, problèmes, saut source ↔ aperçu, « Exporter en PDF » et mode réseau (À propos) ; `internal/app.Preview`, `typst.PageCount`/`Anchors`. Reste : LSP `tinymist` (diagnostics à la frappe, aperçu incrémental), ouverture des autres fichiers depuis un problème (B4 côté webview). |
| 2026-11-08 | Lot L3 : tinymist branché sur l'éditeur (N28, N29). Porte L3 verte, mesure native de la frappe comprise. Le manifeste `gates/L3.json` reste compact : les mesures brutes sont dans `gates/L3-setup/`, citées par chemin et SHA-256. |
| 2026-10-25 | Étape 1 : transport (memnet, tcp), pont Streams, éditeur collaboratif Svelte/CodeMirror/Automerge, présence, undo local, e2e deux instances ; ADR 0009–0013. |
