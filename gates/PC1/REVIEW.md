# PC1 - reviews

## Round 1 — pending (Codex, before the freeze of PC1a and PC1b)

Not yet run: the session that split PC1 (2026-11-15) could not call Codex. Points to give Codex, from the
formal-modeler's notes, in addition to the usual (rules sound, tests and mutants pin them, gates runnable and capped;
scope: the adversary is a peer, local files are the person's):

1. `CompleteJoin` trusts a root URL relayed by the page when the genesis has none (J-1).
2. Whether the `PageGate` rule of N-9 (« no call except the startup-id poll » while the page waits) can be built.
3. `WriteBeforeRestart` checked on the disk rather than on the history of restarts.
4. Gaps the model does not cover: R-7's lock race, one request per behaviour, the archive copy.
5. The join rewriting `project.json` (J-1), next to N-6 and mutant MW5 for the take-over (« folder unchanged »).
6. R-11's sentence (the folder's current text stays but is no longer shown).
7. ADR 0043 point 7, now pointing at PC1b.
Ask for at most 30 lines and one verdict per block (PC1a, PC1b): ACCEPT / ACCEPT WITH CHANGES / REJECT.
