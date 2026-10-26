# Changelog

Toutes les évolutions notables d'Inkwell. Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/),
les versions suivent [SemVer](https://semver.org/lang/fr/) et ce fichier est généré
depuis les [Conventional Commits](https://www.conventionalcommits.org/fr/) (ADR 0025).

## [Unreleased]

### Bug fixes

- **frontend:** CodeMirror styling under strict CSP, names in avatars
- **hooks:** Check the repo the command runs in (agent worktrees)
- **identity:** Make key install race-safe, bound fingerprint cache
- **e2e:** Regenerate Wails bindings before building the frontend
- **trust:** Replace the replay fold with a ranked seniority sweep
- **trust:** Close re-audit findings in the seniority fold
- **trust:** Exits stand only if unconcurrent to a cut and spare acknowledged ops (audit 3)
- Relay settings in Start, Invite needs an address, repo restart on arrive
- **mise:** Stop ~/go/bin from shadowing pinned tools
- **scan:** Keep agent worktrees out of gosec
- **scan:** Justify bounded conversions flagged by gosec 2.30
- **project:** Don't report the empty file a truncate-then-write shows
- **project:** Hold an empty file at watch start too; no wall-clock budget in TXT-13
- Two Inkwell on one machine no longer share a TCP port; e2e ports per checkout

### Features

- Bootstrap from go-starter (mise, hooks, agents, skills, CI)
- **transport:** Transport interface and fault-injecting memnet
- Stage 1 core — collaborative editor over the Wails bridge
- **identity:** Per-machine Ed25519 key and 4-word fingerprint
- **trust:** Signed member list as a hash-DAG CRDT, attestations
- **transport/p2p:** Libp2p transport passing the shared contract
- **app:** Switch to libp2p transport; remove stage-1 TCP
- **p2p:** Reconnect loop, relay, hole punching; inkwell-relay
- **app:** Channel mux, member admission, attestation gossip
- **store,project:** Atomic verified storage, projection, store stream
- **net:** Signed-heads write enforcement
- **invite:** Tickets via wormhole code, text, and LAN acceptance
- **editor:** Cursor position, word count and stats (T11 A2)
- **project:** Text encoding, line endings and typed file list
- **editor:** Per-language formatting commands (Typst, Markdown)
- **ui:** Adwaita shell with formatting bar, status bar and preferences
- **search:** Find and replace engine with a Notepad++-style results panel (B6)
- **editor:** Typst and Markdown highlighting, status derived from peers
- **brand:** Inkwell app icon — two strokes merging into one

### Performance

- Reorder struct fields to cut GC scan work (betteralign 0.15.4)

