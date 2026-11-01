# Revue Claude Fable n°2 (tour 2) de la spécification, révision 2 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01, consigné presque mot pour mot.
> Correspondance : [CHANGES.md](../CHANGES.md), section « Tour 2 » (F4-1 à F4-7).

1. **Critique — §5.1/§10.2/§10.7.** QUIC has a CONNECTION window
   ([RFC 9000 §4.1](https://www.rfc-editor.org/rfc/rfc9000#section-4.1)); go-libp2p v0.51.0
   sets 10 MiB per stream and 15 MiB per connection (`quicreuse/config.go`; Codex:
   quic-go `stream_flow_controller.go:125` takes min(stream, connection)). One unread REPO plus
   one unread LARGE (legal) hold 20 MiB > 15 MiB → CONTROL/PRESENCE stall. "Control never
   behind data" only holds on TCP+yamux. Fix by design (e.g. the receiver always drains
   CONTROL/PRESENCE and caps unread bytes per class below the connection window; set libp2p
   QUIC windows explicitly; or put LARGE on a separate connection-independent budget) and
   model it.
2. **Majeur — §10.7/§7.3 vs tla/.** The claims exceed the model (Deliver unconditional, no
   bridge credit, no LARGE, no connection window, two peers, no inbound-RESET action; only
   RepoGenSmall is exhaustive). Rephrase claims to what is checked AND extend the model to the
   connection-credit and inbound-reset cases that the spec relies on.
3. **Majeur — §6.2 + §9.4.** Go keeps only the highest seq per (doc, signer)
   (`internal/trust/attest.go:35`) and coverage walks deps from an attested head the receiver
   holds (`frontend/src/net/coverage.ts:43-67`); with reader-only relays or a
   vanished/Byzantine writer attesting a phantom head, expiry → rule L every 30 s between
   honest peers (forbidden loop). Treat expiry as refusal plus a later history-request (§9.6);
   keep attestation bytes for every head still in the coverage set.
4. **Majeur — §16.2 + rule I.** The local outbound queue (16 MiB) → rule L, plus resetting an
   in-flight LARGE → zero-progress loop with slow peers at startup (Automerge emits one initial
   sync per document). Bound by count (≤ 1 outstanding message per document); never reset on a
   LOCAL outbound queue.
5. **Majeur — §5.1/§11.4/A.1.** Relay→direct overlap: the generation restarts at 0 per
   session and PeerUp carries (peer, generation) only → late messages of the old session pass
   rule T. Add a session nonce or make the generation monotone per peer for the process
   lifetime. (Codex adds: the Resource Manager control limit of one stream per direction per
   peer across connections blocks the handover — reserve overlap capacity.)
6. **Majeur (N12) — §4.9/§4.10/§9.7.** The fold has one `trust_op+apply` vector; add vectors
   for seniority, concurrent cuts, exits, suspension, Q/L1–L5, G9 (schema only in ADR 0030 §3 →
   bring it into the spec), SET/ATTEST_WANT/GOAWAY frames, `author_batch`/evidence/listing/
   final_ack.
7. **Mineurs.** §1.4.4 "signed bytes start with the version byte" is false for trust-op,
   policy-op, final-ack, listing-op and changesig — fix the text; §1.4.2 ignorable frames capped
   at 16/min/peer will trip OVERLOADED for future periodic extensions — raise or document in
   §20.1; §7.3 "suivre" opens a stream that is not counted against the receiver cap "2 en 2 s";
   §6.2 an attestation whose signer is not yet in the list must be held like SET orphans, not
   dropped; §8.1 want length "1 ≤" vs multiples of 16,384; §4.3 doc ≤ 256 vs CDDL document-id
   ≤ 64; an old node silently stops relaying unknown-version ops unless admins set `format` —
   address it.

revise
