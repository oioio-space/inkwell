// SPDX-License-Identifier: MPL-2.0

package directup

import "time"

// Bucket is a token bucket on a clock passed in: it holds up to burst tokens and gains one per refill
// period, counted from the moment it stopped being full. In any window of length w it gives at most
// burst + ⌊w / refill⌋ tokens. The zero Bucket gives none.
type Bucket struct {
	burst  int
	refill time.Duration
	tokens int
	at     time.Time // the refill clock: when the next token is counted from
}

// NewBucket returns a full bucket.
func NewBucket(burst int, refill time.Duration, now time.Time) Bucket {
	return Bucket{burst: burst, refill: refill, tokens: burst, at: now}
}

// fill adds the tokens due by now.
func (b *Bucket) fill(now time.Time) {
	if b.refill <= 0 || b.tokens >= b.burst {
		return
	}
	if n := int(now.Sub(b.at) / b.refill); n > 0 {
		b.tokens = min(b.tokens+n, b.burst)
		b.at = b.at.Add(time.Duration(n) * b.refill)
	}
}

// Take takes a token if there is one at now.
func (b *Bucket) Take(now time.Time) bool {
	b.fill(now)
	if b.tokens == 0 {
		return false
	}
	if b.tokens == b.burst {
		b.at = now // the refill clock starts with the first token taken
	}
	b.tokens--
	return true
}

// Give returns the token just taken, as if it had not been: for a call that turned out not to count.
func (b *Bucket) Give() {
	b.tokens = min(b.tokens+1, b.burst)
}

// Tokens reports how many tokens there are at now.
func (b *Bucket) Tokens(now time.Time) int {
	b.fill(now)
	return b.tokens
}

// Next returns when a token is available: now if there is one.
func (b *Bucket) Next(now time.Time) time.Time {
	if b.Tokens(now) > 0 || b.refill <= 0 {
		return now
	}
	return b.at.Add(b.refill)
}

// Full reports whether the bucket is full at now: it then carries no information and may be dropped.
func (b *Bucket) Full(now time.Time) bool {
	return b.Tokens(now) == b.burst
}

// Budgets are one peer's budgets at the two choke points (H-12), whoever initiates: the hole-punch rounds
// the address filter admits and the non-relay destinations the connection gater lets through.
type Budgets struct {
	Rounds Bucket
	Dests  Bucket
}

// NewBudgets returns a peer's full budgets.
func NewBudgets(now time.Time) Budgets {
	return Budgets{Rounds: NewBucket(RoundBurst, RoundRefill, now), Dests: NewBucket(DestBurst, DestRefill, now)}
}

// Full reports whether both budgets are full: they can then be dropped and made again.
func (b *Budgets) Full(now time.Time) bool { return b.Rounds.Full(now) && b.Dests.Full(now) }
