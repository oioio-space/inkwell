# Revue Claude Fable n°6 (tour 6) de la spécification, révision 6 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01. Aucune Critique, aucune divergence ; le
> Majeur F13-2 a été trouvé indépendamment par Codex (12-codex). Correspondance :
> [CHANGES.md](../CHANGES.md), « Tour 6 ».

1. Round-5 points 1–7, 9–11 closed; point 8 closed as asked but it was wrong (see 2).
2. **Majeur — §8.1 « règle H suspendue » + « un tirage par principal » (vivacité N7).** A
   solicited member B can announce an indivisible LARGE for document D with a garbage root and
   drip bytes (bulk fails only on zero progress; a pull is never replaced, never time-based) →
   rule H for D is suspended at that receiver while B drips, so a receiver depending on pages
   never obtains D from honest holders; after step 8, B can reuse an honest author's genuine
   change hash and occupy that author's only pull slot. Fix (simpler, removes the step-8
   dependency): charge the slot and the staging to the ANNOUNCING PEER always, credit the
   author's N10 quota only after `change` verifies; drop the suspension; dedup by root
   (re-announce of a root in flight adds a holder, never a second transfer); keep
   history-request to other holders allowed during a pull.
3. **Mineur — §9.6 deux pages en vol.** have = heads ∪ received-but-unapplied hashes is safe
   only if a page is a causally closed prefix (every dependency in have's past or earlier in
   the page) — state the prefix rule.
4. **Mineur — §7.2(5)/§5.1(c)/§16.1.** A past-epoch inbound LARGE read to the end with failures
   ignored can drip and hold the peer's fixed reservation for the session — the 60 s progress
   timer still resets it and frees the reservation; only rule L is skipped.
5. **Mineur — §7.4/§16.3.** An ordinary sync message > r is an "advice" the sender does not
   keep, yet it must cross the bridge (64 MiB cap) to compute root; a 200 MiB initial-sync
   message has no defined outcome — let an advice carry root = 0 and be announced from size
   alone, without shipping bytes.
6. Codex-side page-vs-bulk change: sound.
7. Overall: clear and coherent; four mechanisms; implementable by a third party once step-8
   codecs exist; evolvable per N14.

revise
