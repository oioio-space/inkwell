# Revue Claude Fable n°3 (tour 3) de la spécification, révision 3 (2026-11-01) — verdict : revise

> Relevé transmis par la coordination le 2026-11-01, consigné presque mot pour mot.
> Correspondance : [CHANGES.md](../CHANGES.md), section « Tour 3 » (F6-1 à F6-9).

1. **(Avec Codex) — §5.1(b), §10.7, CHANGES F4-1.** `CancelRead` does NOT return connection
   credit immediately: quic-go v0.64.1 `receive_stream.go:410` calls `Abandon()` only if
   `isNewlyCompleted()`, which needs `finalOffset` (`:141-143`); credit returns when the peer's
   RESET_STREAM arrives (≥ 1 RTT, [RFC 9000 §4.5](https://www.rfc-editor.org/rfc/rfc9000#section-4.5))
   and never if the peer ignores STOP_SENDING. Fix the text, model cancellation as a deferred
   step in ConnCredit (Switch currently frees `old` immediately), and bound the exposure (honest
   ≤ 1 RTT within the 4 MiB margin; Byzantine only stalls its own session — say how the session
   is torn down if RESET never comes).
2. **Majeur — §5.1.** "At most two sessions" has no tie-break: two simultaneous direct
   connections (DCUtR does this) → each side keeps "its" new one, both close → reconnect loop
   between honest peers. Add a deterministic rule (e.g. keep the connection opened by the
   smaller PeerID, or the oldest Active).
3. **Majeur — §7.2.** Preemption under contention is a slowness-based reset (< 16 KiB/s): with
   n > 16 peers a share < 16 MiB is smaller than a history page (§9.6) → a slow honest peer
   (ADR 0028) is starved repeatedly. Simple fix: history page ≤ share, or "an object may be
   preempted at most once (then protected)".
4. **Mineur.** Rule I "stop reading" vs §5.1 "cancelled" — align (and the old-generation
   inbound LARGE).
5. **Mineur.** §7.4/§11.2 "60 s without progress" lacks "after LARGE_PULL" of §7.2.6 (an
   emitter waiting in FIFO legitimately would trigger rule L).
6. **Mineur.** §9.6 history-request recipient unnamed (deliverer? any holder?).
7. **Mineur.** The 64 "first message without reply" slots (§7.4) have no timeout — a message
   dropped `#not-admitted` pins a slot.
8. **Mineur.** 64 + 64 connections (§16.1) vs 2 per peer during handover for 64 members is at
   the limit.
9. **Mineur.** §16.1 CONTROL "always read" yet a 512 KiB inbound queue can suspend reading —
   say it (CPU/disk only).

revise
