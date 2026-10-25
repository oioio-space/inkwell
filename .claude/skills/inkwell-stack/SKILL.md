---
name: inkwell-stack
description: Use before writing or reviewing any code that touches inkwell's stack — Wails v3, CodeMirror 6, Automerge / automerge-repo, go-libp2p, wormhole-william, tinymist / typst, bbolt/sqlite, go-git. Gives the authoritative doc source for each brick, where versions are pinned, and the known traps, so answers come from current docs instead of memory.
---

# inkwell stack — authoritative sources & traps

Rule: **never code a brick from memory.** Fetch its current doc first (sources below),
check the pinned version in the repo, then write. Wails v3 is beta and APIs move.

## Where versions are pinned

| What | File |
|---|---|
| Go toolchain, CLI tools (gopls, wails3, task, node…) | `mise.toml` `[tools]` |
| Go modules (Wails, libp2p, wormhole-william, go-git…) | `go.mod` (exact versions) |
| JS packages (CodeMirror, Automerge…) | `frontend/package.json` + lockfile (exact) |
| Sidecar binaries (tinymist, typst) | hash file next to the embed (étape 3) |

## Doc sources (fetch, don't recall)

| Brick | Source |
|---|---|
| Go code in this repo | **gopls MCP**: `go_workspace`, `go_search`, `go_package_api`, `go_symbol_references`, `go_diagnostics` |
| Wails v3 | `https://v3.wails.io/llms.txt` (index) → linked pages; releases on GitHub `wailsapp/wails` |
| Automerge, automerge-repo | `https://automerge.org/llms.txt`; `automerge-codemirror` README |
| CodeMirror 6 | `https://codemirror.net/docs/ref/` (no llms.txt) — or Context7 |
| go-libp2p | `go_package_api` on the module once in `go.mod`; `docs.libp2p.io`; examples repo `libp2p/go-libp2p/examples` |
| wormhole-william | `go_package_api github.com/psanford/wormhole-william/wormhole` |
| Typst / tinymist | `https://typst.app/docs/` (no llms.txt); tinymist docs `https://myriad-dreamin.github.io/tinymist/` |

## Known traps (from the design doc and research)

- **Wails v3 ≠ v2**: never mix APIs/imports; CLI is `wails3`; `build/config.yml` +
  root `Taskfile.yml` + `build/<os>/Taskfile.yml`. Build tags: `production` (embedded
  assets), `gtk3`, `server` (no window, no cgo). Linux/macOS need cgo.
- **Automerge has no UndoManager**: undo = local inverse ops; never rewrite history
  (`automerge-repo-undo-redo` warns it alters history).
- **Bridge = Wails Streams** (ADR 0009): `app.HandleStream("net", h)` in Go,
  `Stream("net")` from `@wailsio/runtime` in JS (WebSocket-like, `binaryType` defaults to
  `arraybuffer`). Go `Send` takes ownership of the slice — never reuse it after sending.
  No auto-reconnect: recreate in `onclose`. Frame ≤ 64 MB; JS→Go small frames are the slow
  direction → batch if needed. Request/response stays on bindings.
- **Linux = GTK4 / WebKitGTK 6.0** (Wails default; `-tags gtk3` is legacy, removed in v3.1).
  Fedora: `gtk4-devel webkitgtk6.0-devel`; Ubuntu 24.04: `libgtk-4-dev libwebkitgtk-6.0-dev`.
- **TypeScript 6.x**, not 7: `svelte-check` requires `^5 || ^6` (ADR 0010).
- **Presence/cursors**: use automerge-repo's built-in `Presence` (2.5+) with Automerge
  cursors (`getCursor` / `getCursorPosition`), never raw offsets.
- **automerge-codemirror 0.2 does not mark remote changes out of history**: add a
  `transactionExtender` that sets `Transaction.addToHistory.of(false)` on doc changes
  without a `userEvent`, or undo will revert other people's text.
- **No Go code imports Automerge** (ADR 0011): automerge-go is stale (2024, Automerge 2 core).
- **Remote changes must be marked out of CodeMirror's history** so local undo stays local.
- **Typst downloads packages at compile time**: always pass `--package-cache-path
  <project>/.typstdeps` and `--root <project>`; check tinymist's equivalent of `--root`.
- **NAT**: ~30 % of non-LAN pairs need the relay — test the relay path, not only direct.
- **Multicast is often blocked** (corporate, guest Wi-Fi): always offer `ip:port` and the
  Wormhole code; say so after 10 s with no peer.
- Fuzz every decoder of peer bytes (`go test -fuzz`); `pgregory.net/rapid` for properties.

When a trap is discovered, **add it here** in the same commit as the fix.
