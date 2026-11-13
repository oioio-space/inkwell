# /simplify — DU

Diff: the DU change (`frontend/pnpm-workspace.yaml` override, `frontend/pnpm-lock.yaml`). Reviewer: Claude (Opus),
inline — two files, no code.

## Applied
- The override range is narrowed from `>=11.1.1` to `^14.0.3`, the version tested (an open range would let any future
  major in on the next resolve, a supply-chain risk; the lockfile pins it anyway, the range now says what was tested).

## Discarded (with the reason)
- Upgrading `@automerge/automerge-repo` instead of overriding: the stable line (2.5.6, ours) depends on uuid 9; only
  the pre-release 2.6.0-alpha.3 moves to `uuid ^14.0.1` (`pnpm view`, 2026-11-13), the same major as our override. The
  override is removed when a stable automerge-repo carries it; no alpha in R1.

## Checked
- automerge-repo calls only `v4()`, `v4(null, buf)`, `validate`, `parse` (dist grep); all work on uuid 14.0.3 (run by
  the implementing agent); uuid 9→14 breaking changes (CommonJS removed, Node 20+ global crypto, browser exports by
  default, TS ≥ 5.4.3) touch none of them; the webview has `crypto.getRandomValues`.
