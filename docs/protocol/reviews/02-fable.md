# Revue Claude Fable n°2 de la spécification du protocole (2026-11-01) — verdict : sound with changes

> Relevé transmis par la coordination le 2026-11-01, consigné presque mot pour mot. Aucun
> interblocage trouvé. Correspondance avec les changements : [CHANGES.md](../CHANGES.md)
> (identifiants F2-1 à F2-3.x).

1. **SIMPLIFY (top priority).** OFFER/WANT/Bao for in-memory sync messages reinvents the
   transport. Use libp2p streams as receiver-paced ordered channels (QUIC MAX_STREAM_DATA,
   [RFC 9000 §4.1](https://www.rfc-editor.org/rfc/rfc9000#section-4.1); yamux per-stream
   window): one stream per class per peer (control+TRUST+ATTEST / REPO / PRESENCE) and a
   dedicated stream per message > 64 KiB (`LARGE{id}` announced on the REPO stream). Typing
   never waits behind a transfer; the receiver reads only with credit; REPO_RESET = reset the
   REPO stream and open a new one with the generation in its header
   ([RFC 9000 §3.5](https://www.rfc-editor.org/rfc/rfc9000#section-3.5) semantics) — no 120 s
   timers, no OFFER_EXPIRED/PULL_FAILED, smaller TLA+ model. Bao adds nothing inside an
   authenticated connection from the same peer — keep it for blobs only. The
   ATTEST-before-REPO barrier can be kept by carrying the attestation inside the REPO frame
   when heads change. Drop bridge Credit for PRESENCE (lossy, bounded). Control must never
   sit behind data ([RFC 9113 §5.2.1](https://www.rfc-editor.org/rfc/rfc9113#section-5.2.1)
   exempts non-DATA frames).
2. **Per-peer reservations on every shared ceiling** (attestations per signer — today one
   writer fills 65,536 keys → honest changes uncovered → silently dropped; offered/large
   objects per peer with a TOTAL deadline; bulk slots). Every loss in the reliable class,
   including eviction of an uncovered change (`frontend/src/net/coverage.ts:105`; Automerge
   `sentHashes` means no resend), MUST trigger REPO_RESET. Reset only on ZERO PROGRESS, never
   on slowness (current §16.2/§10.2/§7.2 thresholds reset honest peers < 16 KiB/s behind
   relays forever, contrary to ADR 0028); adaptive WANT/LARGE sizing. Define the 10 s resend
   vs 2 s → 60 s rate-limit interaction and u32 wrap.
3. **Self-contained spec for N12**: normative member-list fold (seniority, last admin,
   suspension — today only in ADRs 0005/0017/0021 + code), normative admitted set of
   documentId (ADR 0031 listing chains), REPO/PRESENCE CBOR vectors, state explicitly that
   unsigned messages need no deterministic profile and what cbor-x emits, targetId prose vs
   CDDL, a sender-side state machine, create `docs/protocol/tla/` (the model is cited but
   absent), Bao slice layout in the text, full Chalkias "Taming the many EdDSAs" vector
   suite. Security-bearing capabilities (e.g. `changesig/1`) MUST be receiver-mandatory, not
   negotiable (downgrade). Record the webview CPU/volume residual (BFT CRDTs give
   convergence, not volume resistance) until the N10 quota lands.

sound with changes
