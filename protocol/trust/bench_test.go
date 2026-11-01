// SPDX-License-Identifier: MPL-2.0

package trust

import "testing"

// BenchmarkFoldAtCap folds lists of about maxOps ops of three shapes.
func BenchmarkFoldAtCap(b *testing.B) {
	shapes := map[string]func(testing.TB) *List{
		// One admin's self-ops in a line: the longest ancestor chains.
		"chain": func(t testing.TB) *List {
			a := newSigner(1)
			l := project(t, a)
			if err := chain(t, l, a, maxOps-1); err != nil {
				t.Fatal(err)
			}
			return l
		},
		// The cascade of the audit: every verdict depends on the previous one.
		"cascade": func(t testing.TB) *List {
			l, _, _ := buildCascade(t, 250)
			return l
		},
		// 100 admins added in a line, each then adding nine keys concurrently.
		"wideFan": func(t testing.TB) *List {
			a := newSigner(1)
			l := project(t, a)
			prev := l.Frontier()
			var admins []*testSigner
			for i := range 100 {
				admins = append(admins, key(20, i))
				o := craft(t, l, a, OpAdd, admins[i], RoleAdmin, "", prev)
				mustApply(t, l, o)
				prev = h(o)
			}
			for i, s := range admins {
				for j := range 9 {
					mustApply(t, l, craft(t, l, s, OpAdd, key(21, i*9+j), RoleWriter, "", prev))
				}
			}
			return l
		},
	}
	for name, build := range shapes {
		b.Run(name, func(b *testing.B) {
			l := build(b)
			b.ReportMetric(float64(l.Len()), "ops")
			for b.Loop() {
				fold(l.ops)
			}
		})
	}
}
