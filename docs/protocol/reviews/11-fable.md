# Revue Claude Fable n°5 (tour 5) de la spécification, révision 5 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01. Aucune Critique, aucune divergence.
> Correspondance : [CHANGES.md](../CHANGES.md), « Tour 5 » (F11-1 à F11-11).

1. **Majeur — §8.1 attente sur disque / §16.1.** Within the interim cap (one indivisible change
   per author per receiver), "a new one from the same author replaces the old (LRU)" is
   arrival-preemption → an honest author with two indivisible changes in two documents, pulled
   by a receiver slower than ~16 MiB/60 s, thrashes forever (ADR 0028). Fix: never replace a
   pull in progress; defer the second (rule H retries later); evict only finished or
   no-longer-wanted entries (document unadmitted, change obtained elsewhere).
2. **Majeur — §8.1 reprise vs §14 point 6 (Codex aussi).** Staging goes to "le cache privé
   (§14.6)" — there is no §14.6; §14 point 6 caches are cleaned at end, on abandon and after
   24 h → contradicts resume after restart and "never time-based". Put staging under the data
   dir with its own retention rule (or amend §14); verify progress survives > 24 h.
3. **Mineur — §7.2(5) vs §5.1(c)/§7.3.** The peer applying rule I resets its outgoing LARGE,
   so our old-generation inbound LARGE ends in RESET_STREAM, counted as a loss → rule L → an
   extra generation bump; state that a reset/failure of a LARGE of a past epoch is not a loss
   (rule T applies to events).
4. **Mineur — §8.1 Service.** "Keeps the change indexed by root while a request is in progress"
   is undefined between verdict = 0 and the first WANT, and after a receiver restart; define a
   window (≥ 10 min after the last WANT/LARGE_REPLY) or persist the root index for every change
   > r (rare, 32 B each).
5. **Mineur — §8.1 liaison / §4.7.** Author binding and the per-author cap rely on the author
   tuple (proposed, step 8) → define the interim principal (the announcing peer).
6. **Mineur — §7.2/§8.1/§9.7.** An indivisible announce with size > 16 MiB has no outcome; rule
   H retries every 60 s forever → definitive refusal, bound the indivisible size by the guard.
7. **Mineur — §5.1 préambule, repo.cddl:42, §16.1.** size ≤ 67,108,797 and max
   65536..67108797 while r = 4 MiB and every LARGE > r is refused → tighten both to 4,194,304.
8. **Mineur — §5.5/§8.1.** While a pull for document D runs, rule H must suspend
   history-request for D.
9. **Mineur — N12.** Vectors for the indivisible binding (change-hash mismatch, size ≠
   announce, WANT by root for an unadmitted document → MISSING).
10. **Mineur — ADR 0034 l.163–165 (Codex aussi).** Leftover "a LARGE object above its share can
    be reclaimed (rule L)… n ≤ 4, share ≥ 64 MiB" contradicts fixed shares; the §11.2 receiver
    row lacks Refusé.
11. **Remarque.** Fixed shares are sound; note in §9.6 the ⌈H/4 MiB⌉ sequential pages cost on
    initial sync and leave room for one-ahead pipelining.

revise
