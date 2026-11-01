# Revue Claude Fable n°7 (tour 7) de la spécification, révision 7 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01. Aucune Critique, aucune divergence ; le
> Majeur F15-2 a été trouvé aussi par Codex (14-codex). Correspondance :
> [CHANGES.md](../CHANGES.md), « Tour 7 ».

1. Round-6 points 1–7 closed; Codex T6-2/T6-3 sound.
2. **Majeur — §8.1 « qui sert » / §9.6 / CDDL `history-request.max` 65536..4194304.**
   Indivisibility is relative to the requester's max, but the persistent index and its bound
   assume max = r; a conforming receiver with max 64 KiB makes every 64 KiB–4 MiB change
   indivisible, which the announcer's index (only > 4 MiB) does not hold → MISSING, liveness
   gap; the "annoncée ou détenue" wording reintroduces the undefined transient record. Fix
   (trivial): pin max = r (CDDL 4194304) — or define the index on encoded singleton page >
   65,536 and restate the bound.
3. **Mineur — §8.1, goutte-à-goutte sur une plage.** A rushing relay announces an honest root
   first then drips 1 byte/29 s on one range, never fails; the honest holder added by dedup is
   never asked for that range → a range in flight may be requested concurrently from another
   holder (1-per-(pair, hash) is per pair), completion by any holder cancels the rest; add to
   P-BYZ.
4. **Mineur — §7.2(1) Advice.** Say the Automerge sync state of that (pair, document) is reset
   after an advice (as for a replaced message §7.4).
5. **Remarques.** Staging keyed to the first announcer is dropped on its removal even when
   another holder exists (wasted work only) — rekey to root with holders; A.1 kind 19 `doc
   str8` vs kind 4 `doc str16` — harmonise.
6. Overall: clear, coherent, simpler than rev 6; implementable; evolvable; one consistency gap
   remains.

revise
