# Revue Codex n°5 (tour 5) de la spécification, révision 5 (2026-11-01) — verdict : revise

- C3 **CLOSED** — [spec §10.6, §16.1](../spec.md:1263): one reservation ≤4 MiB per peer, ≤64 peers; joins cannot exhaust another peer’s share.
- C5 **CLOSED** — [§5.1(c), §7.3](../spec.md:564): only inbound REPO is cancelled; old-generation LARGE is drained and discarded, matching ConnCredit.
- LARGE metadata **CLOSED** — [§5.2, §7.2](../spec.md:806): document, indivisibility, root, change hash and size supplied.
- LARGE_REPLY binding **CLOSED** — [§7.2](../spec.md:813): `{gen,id,verdict}`; stale-generation replies ignored.
- Envelope overflow **PARTIAL / Majeur** — [§7.2, §9.6](../spec.md:1159): 16 MiB lane overflow removed, but raw change ≤`max` whose encoded page exceeds `max` has no permitted route. Define indivisibility using encoded singleton-page size; add boundary vectors.
- D1 **PARTIAL / new Majeur** — [§8.1](../spec.md:989) explicitly stages under §14.6, whose [24-hour cleanup](../spec.md:1406) contradicts LRU-only retention and persistent resume. Exempt staged changes; verify progress survives >24 hours.
- D1 otherwise follows the agreement: [§8.1](../spec.md:979) specifies root-based pull, authorization, doc/author/hash/length/root binding, per-author N10/interim cap, DRR, resume, failover → H.
- **Mineur — stale policy remains:** [ADR 0034, Conséquences](../../../docs/adr/0034-protocole-unifie.md:163) still permits LARGE reclamation with dynamic shares, contradicting fixed shares and no reclamation.
- Layout spot-check passes: [session vectors](../vectors/session.json), [Go generator](../vectors/generate.go.txt:463): LARGE body 102 bytes for doc length 28; reply 9 bytes. [CDDL](../cddl/repo.cddl) covers CBOR payloads; production sync2 codecs remain pending (§21).
- No new Critique established. [TLA results](../tla/README.md) retain bounded/exhaustiveness limits and ConnCreditHonest; [migration gates](../../../docs/adr/0034-protocole-unifie.md:169), including P-COMPAT and atomic L/H rollout, remain intact.
revise

