# Second-family review coverage (Codex)

Which critical parts have had an adversarial review by a second model family (Codex), and which have not.
Kept up to date after each review; the next reviews are taken from the top of « Queue » whenever Codex quota is
available (rule: docs/autonomous-decisions.md, 2026-11-12 « models »). A review's findings are fixed at the root
in the owning block, each with a red-first test, and the row records the outcome.

## Reviewed

| Area | When | Scope | Outcome |
|---|---|---|---|
| Architecture and design (rev. 1–8, confirmations) | 2026-11-01/02 | `docs/architecture/reviews/02,04,06,08,10,12,14,16,18,19-codex-*.md`, `coherence-codex.md` | folded into architecture rev. 8.x |
| Admission design (stage 1) | 2026-10-31/11-01 | `docs/plans/audit-2026-10-31/03,07,08,10-codex-*.md` | folded into ADR 0030/0031 |
| Acceleration method (GC, RG, perf lane) | 2026-11-12 | debate; GC and RG plans | GC re-scoped then deferred (measured ≈ 0 gain), block GP first; RG amended (7 findings) |
| RC1 reconnect after restart | 2026-11-12 | `internal/transport/p2p/addrbook.go`, `dial.go`, `mdns.go`, `internal/app/addrbook_file.go` | 4 findings fixed (starvation under forged sightings, roaming with a full entry, save retry, revocation during a clock hold) |
| C5S link quality | 2026-11-12 | `internal/transport/p2p/probe.go`, `links.go`, `stream.go` | 3 connection-identity races fixed |
| SB1 sandbox memory and crash reports (ADR 0042) | 2026-11-12 | ADR 0042, plan, gate SB1 | "amend first": ADR amendment and gate SB1b replace SB1 |
| Test verdicts on a loaded, single development machine | 2026-11-12 | heavy.sh, PB, RG, netsim, real-time Go tests | block MV (placement on P-cores, validity collector, on-machine calibration; calibration follow-up pending) |
| Trust and membership | 2026-11-12 | `protocol/trust`, `internal/trust`, `internal/app/wiring.go`, `attest.go` | 5 findings (circular witness after handover, divergent saturation, revocation not durable before ack, non-atomic suspension, attestation equivocation) → block TR1 after R1; 13 of the 21 surviving mutants flagged |
| Global plan and R1 scope; methods without Codex | 2026-11-12 | PROGRESS, objectif-en-cours, coherence §5, exigences, decisions | R1 re-ordered and acceptance extended; trusted-collaborators alpha; falsification registry (FR) adopted |
| Sidecar sandbox and bridge (cloud review, not Codex) | 2026-11-14 | `internal/sidecar`, `internal/bridge`, `protocol/bridge` (58 files) | 10 findings, none replayed yet: [report](2026-11-14-ultrareview-sandbox-bridge.md) → blocks SB2 (sandbox, complete) and BR1 (bridge, standard) |
| CRDT admission frontier and invitation (cloud review) | 2026-11-14 | `frontend/src/net`, `internal/invite`, `protocol/invite`, `frontend/packages/protocol/src` (21 files) | 10 findings, 9 re-read, none replayed: [report](2026-11-14-ultrareview-admission-invite.md) → blocks IV1, AD1 (complete), NA1 (standard) |

## Queue (most critical first)

| # | Area | Why critical | Files |
|---|---|---|---|
| 7 | Blob store and Bao transfer | peer-supplied data on disk, quotas | `internal/blob`, ADR 0023 |
| 8 | Persistence (store, backups, integrity) and project projection (fsnotify ownership) | data loss | `internal/store`, `internal/project` |
| 9 | Relay (self-hosted) | public network surface | `cmd/inkwell-relay`, `internal/transport/p2p/relay.go` |
| 10 | Release and supply chain | signed artifacts, pinned sidecars | `.github/workflows/release.yml`, `internal/sidecar/manifest.json`, `scripts/pkg` |
| 11 | OI single instance (after implementation) | local socket and file hand-off | `internal/instance`, `internal/launch` |
