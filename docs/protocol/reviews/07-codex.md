# Revue Codex n°4 (tour 4) de la spécification, révision 4 (2026-11-01) — verdict : revise

D1: synthesis because A’s reservation and B’s protected object permit indefinite Byzantine trickling; C bounds occupancy but repeatedly discards an honest transfer slower than its quantum permits, contradicting [§10.2](../spec.md).
Proposal: retain C’s pages ≤ share; for indivisible changes, use bounded per-principal partial storage and resumable, hash-verified slices, with fair scheduling and no slowness reset. Reusing §8.1 bulk/Bao adds resume bookkeeping but avoids a separate timed lane; specify it as an explicit extension. This synthesis remains to be validated.
C3 PARTIAL — Majeur, §7.2/§10.6: shares shrink when peers join, yet existing reservations are “never” reclaimed; admission of every new under-share object is therefore unproved.
C5 PARTIAL — Majeur, §5.1/§7.3/§10.7: delayed credit reclamation is correctly described in [quic-go v0.64.1](module cache: github.com/quic-go/quic-go@v0.64.1/receive_stream.go:137); however, I cancels REPO **and** LARGE while permitting only one pending cancellation; [ConnCredit.Switch](../tla/ConnCredit.tla) models only REPO.
C5 window minor CLOSED — §5.1 and [ADR P-QUIC](../../../docs/adr/0034-protocole-unifie.md): initial versus maximum windows distinguished.
H concurrent branches CLOSED — §5.5/§9.6 and [CDDL](../cddl/repo.cddl): causal complement and empty terminal response specified.
Handover CLOSED — §5.1/§7.3/A.1: deterministic session selection and rejection of stale PeerDown/Credit epochs specified.
Emitter minor CLOSED — §7.4: message regeneration and separate LARGE waiting storage specified.
Attestation minor CLOSED — §4.3/§6.2/§17: retained attestations, selection and eviction aligned.
Parser minor CLOSED — §9.2/§21 and [generator](../vectors/generate-repo.cjs.txt:169): closed envelope and inner critical-key rejection; both negative vectors present.
C4/C7/C9/C13 remain CLOSED — ADR Migration/§8.1/§9.2.1/§6.1; gates retained. [TLA results](../tla/README.md) document bounded exhaustive runs; large RepoGen and session/bridge coverage remain limited.
New Majeur — §5.2/§7.2/§9.6: LARGE announces only `{id,size}`; refusing it reveals neither document nor whether it contains one indivisible change. Define metadata enabling H and exceptional-lane admission.
New Majeur — §5.2/§7.2/§7.3: LARGE_REPLY lacks generation although IDs restart each generation; a delayed old grant can match a new announcement. Bind replies to generation.
New Majeur — §7.2/§9.6/G1: a permitted 16 MiB change plus its CBOR history-page envelope exceeds the 16 MiB lane; budget encoded overhead explicitly.
revise

