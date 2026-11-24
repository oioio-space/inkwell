// SPDX-License-Identifier: MPL-2.0

package admit

import "slices"

// bucket is an origin's admission bucket (RF-5), kept as debts so that a refund is exact: each admission
// owes a token, each period pays the oldest debt still owed, and a member's handshake removes its own debt
// only while no period has paid it. golang.org/x/time/rate cannot do this: its Reservation.CancelAt
// restores nothing once the reservation's time has passed.
//
// The tokens are burst minus the debts. While no debt is owed the bucket is full and its clock stands
// still: the first period of a debt ends refill after the moment the bucket stopped being full.
type bucket struct {
	debts []ID  // unpaid admissions, oldest first
	at    Nanos // the start of the current period, while a debt is owed
}

// pay pays the debts whose periods have ended by now.
func (b *bucket) pay(now, refill Nanos) {
	if len(b.debts) == 0 || refill <= 0 {
		return
	}
	n := int(min(max(now-b.at, 0)/refill, Nanos(len(b.debts)))) // a clock read backwards pays nothing
	b.debts = slices.Delete(b.debts, 0, n)
	b.at += Nanos(n) * refill
}

// take owes a token for c at now; the caller checked that one is left.
func (b *bucket) take(now Nanos, c ID) {
	if len(b.debts) == 0 {
		b.at = now // the clock starts with the first token taken
	}
	b.debts = append(b.debts, c)
}

// refund removes c's debt if it is still owed, and reports whether it was.
func (b *bucket) refund(c ID) bool {
	i := slices.Index(b.debts, c)
	if i < 0 {
		return false
	}
	b.debts = slices.Delete(b.debts, i, i+1)
	return true
}

// owes reports whether c's debt is still owed.
func (b *bucket) owes(c ID) bool { return slices.Contains(b.debts, c) }
