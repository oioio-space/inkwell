# L-maj — dependency upgrades (2026-11-03)

Each dependency is pinned to its newest **stable** release that is **at least 7 days
old** on 2026-11-03 (user decision, ADR 0008 cooldown and ADR 0010 `minimumReleaseAge`).
Newer stable releases exist for several of them; they are listed under
« À reprendre le 2026-11-08 ». Pre-releases (nightly, alpha, beta, rc) were never taken.
Publication times come from the Go module proxy (`.info` Time), the npm registry
(`time`) and GitHub releases (`publishedAt`).

## Major or breaking upgrades

| Dependency | From → to | Source | What affected us |
|---|---|---|---|
| `github.com/wailsapp/wails/v3` (Go, `wails3` CLI, `@wailsio/runtime`) | v3.0.0-beta.34 → v3.0.0 (2026-10-26) | https://github.com/wailsapp/wails/releases/tag/v3.0.0 ("Changes since v3.0.0-beta.34") | Only Windows crash fixes, updater hardening and CLI fixes since beta.34; no API we use changed. Linux stays cgo GTK4/WebKitGTK 6.0. Bindings regenerated. |
| `@biomejs/biome` | 2.5.15 → 2.6.3 | https://github.com/biomejs/biome/blob/main/packages/%40biomejs/biome/CHANGELOG.md (2.6.x) | YAML formatting is new: the generated `pnpm-lock.yaml` is excluded. `linter.rules.recommended` is deprecated: `biome migrate` → `"preset": "recommended"`. Newly recommended rules: four targeted `biome-ignore` for `useDomNodeTextContent` (two e2e reads of the rendered `innerText`, two Playwright `Locator.innerText()` calls, which are not DOM nodes); one for `noConfusingVoidType` on the Playwright auto-fixture typed `void` (Playwright's own idiom); one genuinely unused import removed. |
| `axe-core` (via `@axe-core/playwright` 4.12.1 → 4.14.0) | 4.12.1 → 4.14.0 | https://github.com/dequelabs/axe-core/blob/develop/CHANGELOG.md (4.14.0: `label-content-name-mismatch` enabled by default) | UI-06 found a real WCAG 2.5.3 (label in name) failure: the preview's fit-to-width button shows "90 %" but was named "Ajuster à la largeur". Its accessible name now starts with the visible text (`preview.zoomFitLabel`); the tooltip is unchanged. Covered by PRV-84 (Vitest) and UI-06 (e2e). |

## Minor and patch upgrades

- Go, both modules (`go get -u ./...`, then every too-young version lowered to the
  newest one at least 7 days old): pion stack (webrtc v4.2.25), prometheus/common
  v0.73.0 (with go-abnf v0.5.1, the version it requires), klauspost/compress 1.20.3,
  x/net v0.61.0, x/tools v0.52.0 (deadcode pin in `scripts/deadcode.sh`), x/mod,
  x/time, protobuf, ulikunitz/xz 0.5.17, go.yaml.in/yaml/v3 v3.0.6 (`go get -t`);
  `protocol/`: x/sys 0.49.0, cpuid 2.4.2. All 40 changed module versions were
  re-checked against the proxy after `go mod tidy`. Go 1.27.3 is the latest
  stable toolchain (unchanged).
- Frontend: @codemirror/merge 6.12.3, @codemirror/view 6.43.15, cbor-x 1.6.8
  (2026-10-20; decoder-only DoS fix, encoder unchanged:
  https://github.com/kriszyp/cbor-x/compare/v1.6.7...v1.6.8), svelte 5.57.3,
  vite 8.3.5; transitive refresh within ranges (`pnpm update -r --depth Infinity`,
  which pnpm's release-age policy also gates).
- mise tools: jq 1.8.2, gh 2.103.3, benchstat 2026-10-09, syft 1.55.0,
  grype 0.121.0, goreleaser 2.19.0, pnpm 12.16.1; `latest`-tracked tools relocked
  (gofumpt 0.12.0, govulncheck 1.9.0, watchexec 2.8.0). All ≥ 7 days old.
- Unchanged: Node 24.22.1 (newest LTS at least 7 days old; 26.12.1 is older than
  7 days but a non-LTS "Current", and ADR 0010 pins an LTS), typst 0.15.1 and
  tinymist 0.15.10 (same SHA-256 as `main`, so `.typstdeps` compatibility is unchanged).

## À reprendre le 2026-11-08

| Dependency | Pinned now | Target | Published |
|---|---|---|---|
| Wails (Go, `wails3`, `@wailsio/runtime`) | v3.0.0 | v3.1.0 (or the newest weekly ≥ 7 days old) | 2026-11-01 |
| Node.js | 24.22.1 | 26.13.0 LTS "Lithium" (24 → 26 is major: CHANGELOG_V26 removals checked, none used) | 2026-10-28 |
| tinymist | 0.15.10 | 0.15.12 (built on typst 0.15.1) | 2026-11-03 |
| github.com/pion/webrtc/v4 | v4.2.25 | v4.2.26 | 2026-10-28 |
| github.com/prometheus/common | v0.73.0 | v0.73.1 | 2026-10-30 |
| github.com/pandatix/go-abnf | v0.5.1 | v0.5.5 | 2026-10-31 |
| golang.org/x/net | v0.61.0 | v0.62.0 — fixes GO-2026-6866 and GO-2026-6860 (`x/net/html`, not reachable from our code per govulncheck; `main` has v0.60.0, also affected); 7 days old from 2026-11-03 19:00 UTC, so it can be taken first | 2026-10-27 19:00 UTC |
| vitest | 5.0.5 | 5.1.0 | 2026-11-02 |
| svelte-check | 4.7.7 | 4.7.8 | 2026-11-01 |
| @biomejs/biome | 2.6.3 | 2.6.4 | 2026-10-28 |

Each target needs the same checks as this lot; Wails and tinymist also need the
ADR 0008 replay/PDF comparison (see below) and, for tinymist, new seccomp traces.

## Kept back (not an age question)

- `github.com/quic-go/quic-go` stays v0.64.1 (v0.65.0 exists): the latest
  `github.com/quic-go/webtransport-go` v0.13.0, pulled in by go-libp2p v0.51.0
  (latest), does not compile against it —
  `webtransport-go@v0.13.0/server.go:429:9: assignment mismatch: 1 variable but
  w.(http3.HTTPStreamer).HTTPStream returns 2 values`. go-libp2p v0.51.0 itself
  requires v0.64.1.
- `typescript` stays 6.0.3 (7.0.2 is stable): `svelte-check` 4.7.x still declares
  `typescript: ^5 || ^6` (ADR 0010 already defers TypeScript 7 for this reason).
- `@automerge/automerge-repo` stays 2.5.6: npm `latest` is 2.6.0-alpha.3, a
  pre-release; 2.5.6 is the newest stable.

## Security re-verification

- Seccomp: `scripts/sidecar-trace-ci.sh` (strace 7.3 from the Fedora 44 RPM,
  signature checked with `rpm -K`) traced the pinned typst 0.15.1 / tinymist 0.15.10
  on PM-1; `sidecar-allowlist.py --check` passes and regenerating the tables leaves
  `syscalls-linux.txt` and `syscalls_linux_{amd64,arm64}.go` unchanged. Hashes of
  the binaries, of every trace and the observed syscall list:
  [`2026-11-03-L-maj-evidence/seccomp-traces.txt`](2026-11-03-L-maj-evidence/seccomp-traces.txt)
  (syscall set sha256 `07e888b6…`, identical to L1).
- WebKitGTK helpers under Wails v3.0.0: compared with beta.34, no WebKit
  sandbox/process API call is added or removed, and the only non-test
  `exec.Command` in `pkg/application` is still `autostart_darwin.go`;
  WebKitWebProcess and friends remain launched by WebKitGTK itself (L1 note).
- Protocol vectors: `protocol/vectors` and `docs/protocol/vectors` are unchanged;
  the Go and TypeScript conformance suites pass on them, and `pnpm run vectors:repo`
  (Automerge 3.5.0 + cbor-x 1.6.8) regenerates `repo.json` byte-identical
  (sha256 `3ff065cd…`).
- Presence and admission: the Vitest suites `src/net/admission*.test.ts`,
  `enforcement.test.ts` and the automerge-repo contract
  (`automerge-contract.test.ts`), the Go `internal/invite`, `internal/app` and
  `protocol/trust` tests, and the e2e presence cases (remote carets, SHEET-MU-01)
  pass unchanged on the upgraded stack.

## UX walkthrough of this lot

- UX-LUP-2 fixed: coming back to a file restores its caret (`Editor.svelte`),
  e2e `frontend/e2e/ux/lup.spec.ts`.
- UX-LUP-3 fixed: « En ligne · main.typ » had no space before « · »
  (`PeopleDialog.svelte`), Vitest UX-LUP-3.
- UX-LUP-1 (the History drawer pushes the layout below 1300 px) is the known
  narrow-window problem S13, also on `main`: its test is `test.fixme` with a
  reviewed guard override, and S13 is a lot of its own in
  `docs/objectif-en-cours.md` (« Lots suivants »).

## ADR 0008 procedure and AD-19

- tinymist and typst end the lot on the same pins as `main` (same SHA-256), so their
  part of ADR 0008:22 is not triggered by this lot.
- Wails beta.34 → v3.0.0 triggers it. `mise run pkg:build` then `mise run pkg:test`:
  every check that ran (PKG-01, 04, 06, 07, 08, 10) passes on debian:13, ubuntu:24.04 and fedora:44 (amd64), including the
  offline smoke run and the upgrade from 0.0.0-a.1; PKG-11 (signatures) is skipped
  because local packages are unsigned (the release workflow signs them).
- The history replay and the reference-PDF comparison cannot be run here: the
  repository has no corpus of real `.automerge` histories, no reference PDFs, and
  no task or test that replays them (`git ls-files` lists no `.automerge` file and
  no PDF besides `docs/conception.pdf`). Building that tooling is its own work. The
  nearest existing checks pass: e2e PRV-102 (PDF export through the app),
  persistence and reload specs, and the byte-identical protocol vectors. Note that
  Wails is outside both paths: the CRDT runs in the webview (ADR 0011) and the PDF
  comes from the unchanged typst binary. **AD-19 stays open** until the replay and
  the PDF comparison exist and pass.

## Verification (2026-11-03, final pins)

`mise run lint`, `fe:check` (Biome clean, svelte-check, 1065 Vitest), `test` (both
modules), `e2e` (96 passed; 7 skipped: the 5 `fixme` already on `main` plus the two
UX-LUP-1 cases, S13), `scan:code` (gosec clean; govulncheck: nothing reachable — three
module-level advisories: GO-2026-6866 and GO-2026-6860 in x/net, fixed by the
deferred v0.62.0, and GO-2026-5932 in x/crypto/openpgp, fixed only in an untagged
pseudo-version), `pkg:build` + `pkg:test`, and `gate -- L2` (L0, L1, L2 pass; base
guard pass with the reviewed UX-LUP-1 override). Perf A/B against `main` (benchstat,
n=20): Relay/Message −2.5 %, no other significant change, no allocation change.
