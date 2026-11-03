# L2 implementation evidence

G-0a opens the sidebar path without reopening the project. Only `main.typ` edits
its existing Automerge handle. Other text is read through a bounded Go projection
endpoint; it does not acquire ownership or write authority. CSV reuses Sheet and
Grid over a read-only plain port. Images are bounded regular project files,
converted to inert data URLs; other files show a card. The header and ephemeral
presence follow the selected file. An increasing ticket discards stale reads.
The legacy editable spreadsheet suite uses an explicitly instrumented e2e-only
fixture over the main handle instead of a production URL override.

The new endpoints validate portable project paths, reject reserved roots,
symlinks and hardlinks, and use os.Root. Existing owned-projection reads and
writes retain their ownership checks. Read-only content is selectable and
syntax-highlighted; its banner quotes design §4.16 exactly.

Validation logs and manifests are under gates/L2/. The pinned sidecars were
fetched before testing. Checks skip install/binding task dependencies because
this worktree's node_modules and bindings are read-only symlinks; Vite uses its
runner config loader so no temporary bundle is written into node_modules.
Go test scratch follows scripts/heavy.sh (GOTMPDIR, which t.TempDir() reads
directly). The sidecar refuses a project inside ~/.ssh, ~/.gnupg, the XDG
config/data/state directories, Inkwell's identity and the kernel
pseudo-filesystems; home, caches, /var and /run may hold projects but are never
one (removable drives under /run/media, Silverblue homes under /var/home).

PERF-TYPE is deferred to L3: the actual desktop builds and was launched, including
a private writable runtime/X11/software-rendering retry. No usable native editor
was established; the screenshot and logs are retained. No native p95 or numeric
threshold is claimed. PERF-TYPE.json records cold/warm empty samples and the
required threshold formula, native p95 × 1.10. The L0 reference is unchanged.

The user clarified N42 on 2026-11-03 ("2026-11-03 — décision de l'utilisatrice
(N42 précisée)" in docs/objectif-en-cours.md): no separately installed program
may be required; platform libraries, the default browser, the file manager,
D-Bus and the keychain are fine. The per-dependency audit (audited.json, the
dependency-graph walk and its heuristics), the Wails runtime guard, the analyzer
rules on OpenURL/OpenFile/OpenFileManager/pkg/updater, the framework-launcher
disarming and the pbnjay/memory fork were therefore removed. One rule remains:
our own code imports no os/exec and starts no process outside internal/sidecar
(tools/archexec, type-checked per build configuration plus a tag-independent
syntax pass). Security is a separate rule and is kept: sandbox, workdir
refusals, Supervisor environment allowlist, image-handler token, listing checks
and presence validation. A link opens only through frontend/src/ui/external.ts,
on a trusted click still being dispatched, for http, https or mailto; a
source-scan test keeps every other module away from Browser.OpenURL, the file
manager and markup injection.

Review before merge remains external: code/security review and the project's
in-person ux-walkthrough. No commit is made by the inner loop.
