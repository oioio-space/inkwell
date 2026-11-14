# Protocol conformance audit — brief for the multi-agent run (2026-11-14)

Purpose: one systematic, adversarial audit of the synchronisation protocol, so that the specifications of blocks TR1,
IV1 and AD1 (and ADR 0034 steps 4 to 8) start from every defect at once instead of finding them one by one. The run
is **read-only**: no file of the repository is edited, no test or gate is run, nothing heavy is started (machine
rules in `CLAUDE.md`, « Mémoire »). Its output is one report, `docs/reviews/2026-11-14-protocol-audit.md`, written
by the integrator from the verified findings.

## Scope: eight parts, one auditor each

| # | Part | Specification (`docs/protocol/spec.md`) | Code (Go / TypeScript) | Executable evidence |
|---|---|---|---|---|
| P1 | Identities, keys, signatures, fingerprints | §3 | `protocol/sig`, `internal/identity` | `protocol/vectors`, `protocol/conformance` |
| P2 | Signed objects and the member-list fold | §4.1–§4.4, §4.9 | `protocol/trust`, `internal/trust`, `internal/app/wiring.go`, `attest.go`, `suspend.go` | vectors; TLA+ `RepoGen`; Alloy `Decisions`, `Registry`, `Tree`; Codex review of 2026-11-12 (TR1) |
| P3 | Admitted set of documents and the repository layer | §4.10, §7, §9, §12 | `frontend/src/net/admission*.ts`, `scope.ts`, `coverage.ts`, `WailsNetworkAdapter.ts`, `frontend/packages/protocol/src`, `protocol/repo` | vectors; cloud review of 2026-11-14 (AD1) |
| P4 | Session, classes, flow control and fairness | §5, §6, §10, §11 | `internal/transport/p2p`, `internal/transport/mux`, `internal/transport/frame` | TLA+ `ConnCredit`, `SendPath`, `WriteBudget`, `SyncIndicator`; netsim |
| P5 | Bulk transfer (blobs) | §8 (bulk) | `protocol/blob`, `internal/blob` | vectors; fuzzers |
| P6 | Lobby, ticket, invitation, rendezvous | §8 (lobby), ADR 0032 | `protocol/invite`, `internal/invite`, `internal/transport/p2p/lobby.go` | vectors; cloud review of 2026-11-14 (IV1) |
| P7 | Link probe | §8 (probe), ADR 0036 | `protocol/linkprobe`, `internal/transport/p2p/probe.go`, `links.go` | TLA+ `LinkProbe` |
| P8 | Cross-cutting: verification order, errors, persistence, bounds, versioning, conformance | §1.3–§1.4, §12–§14, §16, §19–§21 | the codecs of P1–P7 in Go **and** TypeScript; `frontend/src/net/conformance.ts`; `protocol/conformance` | `docs/protocol/inventaire.md`, `docs/protocol/CHANGES.md` |

Out of scope: the webview ↔ Go bridge (Annex A, already reviewed: blocks BR1), the sidecar sandbox (SB2), the user
interface.

## What each auditor checks (the same five questions for every part)

1. **Spec → code**: every normative sentence (DOIT / NE DOIT PAS / bound of §16) of the part has an implementation;
   name the line, or report « missing ».
2. **Code → spec**: everything the code accepts, sends or stores on the wire for that part is in the specification;
   anything more is « undocumented behaviour ».
3. **Go = TypeScript**: where both implement the part, they accept and refuse exactly the same bytes (same limits,
   same order of checks, same errors); a private copy of a codec in the application (invariant of `CLAUDE.md`) is a
   finding.
4. **Proof**: each rule has a vector, a named test and a mutant that would fail it; a rule held by nothing is a
   finding; a model (TLA+/Alloy) that does not match the code it claims to model is a finding.
5. **Hostile peer**: under the threat model (full Byzantine writer; readers and unattested peers strictly bounded;
   local files are the person's, W64), what a peer can make this part do: unbounded memory or time, a state change
   before the last check that can refuse, an equivocation not detected, a silent loss.

Known findings are **not** re-reported, only referenced: TR1 (`docs/reviews/codex-coverage.md`, « Trust and
membership »), the two cloud reviews of 2026-11-14 (`docs/reviews/2026-11-14-ultrareview-*.md`), W58a/PC1
(`gates/W58/REVIEW.md`).

## Verification

Every finding goes to an independent verifier that tries to refute it from the code (file:line) before it is kept;
a finding that cannot be shown at a line is dropped. Kept findings carry: part, severity (critical / major / minor),
the rule or invariant broken, evidence, a concrete fix, and the block it belongs to (TR1, IV1, AD1, a step of
ADR 0034, or « new »).

## Size

Eight auditors, one verifier per finding batch, within the session's guideline (about ten agents at a time); no more
than four agents run at once (`CLAUDE.md`, « Mémoire »).
