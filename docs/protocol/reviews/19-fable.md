# Revue Claude Fable n°9 (tour 9) de la spécification, révision 9 (2026-11-01) — verdict : accept

> Relevé transmis par la coordination le 2026-11-01. La divergence avec Codex (équité des
> emplacements) a été tranchée par le débat [20-debate-D2.md](20-debate-D2.md). Correspondance :
> [CHANGES.md](../CHANGES.md), « Tour 9 ».

1. §8.1/§16.1: the round-8 Majeur is closed (single definition, only actives pulled, atomic
   reservation on new announce/dedup/departure or removal/restart; bound ≤ connected × 16 MiB ≤
   1 GiB; X/Y and 63+1×2 in P-BYZ; minors applied).
2. Liveness checked: no new Majeur for an entry with a connected honest holder; Byzantine-only
   holders: unavoidable.
3. **Mineur — §8.1.** No trigger on slot release; the order between inactive entries and the
   peer's new announce is unspecified (suggested: on release reattribute the most-recently-
   advanced inactive held by this peer).
4. **Remarque.** A holder removed from the member list must be dropped from the entry's holder
   list (a dead holder must not satisfy "if one exists" for doubled ranges).
5. **Remarque.** The §16.1 index row says "≤ 1 per 4 MiB" vs §8.1 "≈ 4 MiB".
6. Overall: the slot model is sound and fully specified.

accept
