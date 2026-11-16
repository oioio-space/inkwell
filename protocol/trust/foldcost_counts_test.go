// SPDX-License-Identifier: MPL-2.0

package trust

// failer is what the count tests pass to foldCost: a *testing.T or a *rapid.T.
type failer interface {
	Helper()
	Fatalf(format string, args ...any)
}

// foldCost reads the cost of the fold that made v: its sweeps and the ops
// they swept (rules K-1..K-3 of docs/plans/2026-11-16-tc1-fold-cost-by-counts.md).
// It is read right after fold: later reads of a stored view add their own sweeps.
func foldCost(_ failer, v *view) (sweeps, swept int) {
	return v.sweeps, v.swept
}
