# Revue Claude Fable n°8 (tour 8) de la spécification, révision 8 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01. Un seul Majeur, le même que Codex
> (16-codex) ; aucune Critique. Correspondance : [CHANGES.md](../CHANGES.md), « Tour 8 ».

1. Round-7 points closed (max pinned, index bound, server index wording, doubled ranges after
   30 s first-complete-wins, sync-state reset after Advice, staging `.inkwell/staging/<root>`
   with holders, kind 19 `doc str16`); Codex T7-7 replay vector sound.
2. **Majeur — §8.1 attente sur disque / §16.1 (T7-2).** Two definitions of "active" conflict:
   (a) "one of its holders is connected and serves it" vs (b) the bound "active ≤ 1 per
   connected peer", which only holds if active ⇔ occupies the slot of a connected attributed
   peer. Under (a): X stages R1…Rn one at a time then disconnects; connected Y announces all Ri
   (dedup adds holder) and drips → every Ri active via Y, none in a slot, never removable → cap
   exceeded or the 65th honest peer denied. Fix (one sentence): active ⇔ attributed to a
   connected peer's slot; an entry whose connected holders all have their slot taken is
   inactive and not pulled; re-attribution on dedup/removal only into a free slot.
3. **Mineur — §8.1.** "si ce pair est retiré de la liste" is ambiguous (member list vs holders
   vs disconnect) — say which.
4. **Remarques.** The index bound "1 per 4 MiB" is strictly "> r − envelope" → say "≈ 4 MiB";
   §17 could mention the staging cap; ADR 0034 l.89 still cites spec "brouillon.2".
5. Overall: everything raised in tour 7 closed; the global cap is the right design; one
   definitional gap, trivially repaired.

revise
