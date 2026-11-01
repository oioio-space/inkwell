# Revue Claude Fable n°4 (tour 4) de la spécification, révision 4 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01. Position finale sur D1 :
> [09-debate-D1.md](09-debate-D1.md). Correspondance : [CHANGES.md](../CHANGES.md), « Tour 4 ».

- **D1** : synthèse sur C … (voir 09-debate-D1.md pour la position finale).
- **Majeur — §7.2.** No lane-eligibility signal in LARGE.
- **Majeur — §7.2.2/§7.2.6.** The 10-minute forget vs the lane FIFO wait → spurious rule L
  between honest peers; and "il revient en fin de file" is undefined after rule L.
- **Mineur — §5.1(c)/§7.3 rule I/§16.1.** Rule I cancels REPO and LARGE of the old generation
  but "1 cancelled stream pending per session" — a cancelled LARGE read continuously holds
  only its unread remainder, not counted.
- **Mineur — §5.1(d).** The ≥ 4.5 MiB control margin holds for one pending switch only — if
  the peer's gen+1 stream arrives before the previous RESET, two unread REPO streams can hold
  15 MiB; the real guarantee is ≤ 1 RTT honest / 10 s teardown.
- **Mineur — §5.1.** Tie-break on equal keys (Byzantine nonce reuse): keep the oldest Active.
- **Mineur — §11.2.** The emitter row still says "reprise de part (§10.6)" and omits
  "Demandé" of §7.4.
- **Mineur — tla/ConnCredit.** `HonestNoTeardown` is defined but not in the .cfg and is vacuous
  once torn — add a cfg variant or drop it.
- **Fermés et vérifiés** : F6-1, F6-2, F6-4 à F6-9 ; le seuil de débit a disparu hors de la voie.

revise
