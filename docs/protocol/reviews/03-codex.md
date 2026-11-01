# Revue Codex n°2 (tour 2) de la spécification, révision 2 (2026-11-01) — verdict : revise

- C1 CLOSED (Critique) — §4.5–4.8, §5.4, §9.4–9.7, §12: mandatory `core/2`, author proofs and G1–G12 now define the receive chain.
- C2 CLOSED (Critique) — §7.3–7.4, A.1: stream generations, asynchronous checks, cancellation and u32 exhaustion address the original reset race.
- C3 PARTIAL (Critique) — §7.2, §10.6, §16.1: with 64 peers, 63 slow senders can retain 4 MiB each; nobody exceeds their share, yet an honest 8 MiB LARGE cannot reserve space. Define admission for indivisible objects larger than a share.
- C4 PARTIAL (Mineur; hypothesis) — §21; ADR 0034 Migration steps 6–7: overflow Reset disappears before rule L is installed everywhere. Explicitly gate these changes together or behind `sync2`; intermediate merge safety is not established.
- C5 PARTIAL (Critique) — §5.1, §10.2, §10.7: separate streams do not reserve connection credit. Local quic-go `stream_flow_controller.go:125` takes `min(stream, connection)`; libp2p permits 10 MiB per stream against 15 MiB per connection. Unread REPO/LARGE can block CONTROL; reserve connection capacity and model it.
- C6 CLOSED (Mineur) — §10.3, A.1: separate inline/LARGE credit lanes and creditless presence remove the original bridge blockage.
- C7 PARTIAL (Mineur) — §8.1 contradicts itself: final SLICE may shorten `length`, but matching requires the requested `length` exactly; `bulk.json#slice-final-group` returns 4464 for a 16384-byte request. State the matching exception.
- C8 CLOSED (Mineur) — §3.2–3.3, §14: canonical point decoding, prime-subgroup membership and reload/import validation are explicit.
- C9 PARTIAL (Mineur) — §9.2.1, §21: the profile is defined, but `generate-repo.cjs.txt` detects duplicate keys by encoded bytes. Reproduced: append `7804747970656772657175657374` to the `sync` vector with map count incremented; duplicate `type` passes and becomes `request`. Compare decoded keys.
- C10 CLOSED (Mineur) — §9.2.3, `cddl/repo.cddl`: identities, bounds, nested maps and coordinate coupling are specified; §1.4 defines unknowns, must-understand, GREASE and authenticated negotiation.
- C11 CLOSED (Mineur) — §21, vectors/, tla/: public schema vectors and generation traces exist; all 39 REPO vectors regenerated identically, and sampled Go layouts and SHA-256 expectations matched.
- C12 CLOSED (Mineur) — §6.2: repeated ATTEST_WANT and bounded pending recovery replace reconnect-only repair.
- C13 PARTIAL (Critique) — §6.1: periodic digests replace cursors for flat sets without a received-object frontier; a response taking over 15 seconds can repeatedly restart its prefix and starve its tail. Preserve an active cursor or define resumable reconciliation.
- New Critique — §5.1, §16.1: relay→direct keeps the old session until the new one is Active, but peer/protocol CONTROL limits allow one stream per direction across connections. The replacement cannot activate. Reserve overlap capacity; P-TLA’s small exhaustive result does not cover this handover or shared-resource failures.
revise

