package typst

import (
	"fmt"
	"strings"
	"testing"
)

func TestClipBytesAndTruncate(t *testing.T) {
	for _, tc := range []struct {
		in       string
		n        int
		clip     string
		truncate string
	}{
		{"abc", 3, "abc", "abc"},
		{"abcd", 3, "abc", "abc…"},
		{"abc", 0, "", "…"},
		{"", 0, "", ""},
		{"éé", 4, "éé", "éé"},
		{"éé", 3, "é", "é…"}, // cuts inside the second é: back to its start
		{"éé", 2, "é", "é…"}, // exactly the first é
		{"éé", 1, "", "…"},   // inside the first é
		{"a😀", 4, "a", "a…"}, // inside the emoji
		{"a😀", 5, "a😀", "a😀"},
	} {
		if got := clipBytes(tc.in, tc.n); got != tc.clip {
			t.Errorf("clipBytes(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.clip)
		}
		if got := truncate(tc.in, tc.n); got != tc.truncate {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.truncate)
		}
	}
}

// TestParseDiagnosticsLimitsAreExact: every bound of the parser holds at its
// edge: the input size, the number of diagnostics, the length of a message.
func TestParseDiagnosticsLimitsAreExact(t *testing.T) {
	lim := limits{maxInput: 100, maxDiagnostics: 3, maxMessage: 20}
	line := func(i int) string { return fmt.Sprintf("main.typ:%d:1: error: e%d\n", i, i) }

	three := line(1) + line(2) + line(3)
	if d, trunc := parseDiagnostics([]byte(three), "/p", "/c", limits{maxInput: len(three), maxDiagnostics: 3, maxMessage: 20}); len(d) != 3 || trunc {
		t.Errorf("input exactly at maxInput: %d diagnostics, truncated=%t; want 3, false", len(d), trunc)
	}
	if d, trunc := parseDiagnostics([]byte(three), "/p", "/c", limits{maxInput: len(three) - 1, maxDiagnostics: 3, maxMessage: 20}); !trunc || len(d) != 3 {
		t.Errorf("input one byte over maxInput: %d diagnostics, truncated=%t; want the cut reported", len(d), trunc)
	}
	if d, trunc := parseDiagnostics([]byte(three), "/p", "/c", lim); len(d) != 3 || trunc {
		t.Errorf("three diagnostics with room for three: %d, truncated=%t; want 3, false", len(d), trunc)
	}
	if d, trunc := parseDiagnostics([]byte(three+line(4)), "/p", "/c", limits{maxInput: 1000, maxDiagnostics: 3, maxMessage: 20}); len(d) != 3 || !trunc {
		t.Errorf("four diagnostics with room for three: %d, truncated=%t; want 3, true", len(d), trunc)
	}
	// A line after the dropped diagnostics is not glued to the last kept one.
	d, _ := parseDiagnostics([]byte(three+line(4)+"stray\n"), "/p", "/c", limits{maxInput: 1000, maxDiagnostics: 3, maxMessage: 100})
	if len(d) != 3 || strings.Contains(d[2].Message, "stray") {
		t.Errorf("a line after the dropped diagnostics was attached to %q", d[2].Message)
	}

	// Continuation lines: kept while the message is within maxMessage, then no more.
	cont := "main.typ:1:1: error: x\nmore\nmore\nmore\nmore\nmore\n"
	short, _ := parseDiagnostics([]byte(cont), "/p", "/c", limits{maxInput: 1000, maxDiagnostics: 3, maxMessage: 1 << 20})
	if len(short) != 1 || strings.Count(short[0].Message, "more") != 5 {
		t.Errorf("continuation lines = %q, want all five", short[0].Message)
	}
	capped, _ := parseDiagnostics([]byte(cont), "/p", "/c", limits{maxInput: 1000, maxDiagnostics: 3, maxMessage: 6})
	if len(capped) != 1 || strings.Count(capped[0].Message, "more") > 2 {
		t.Errorf("continuation lines past maxMessage = %q, want them stopped", capped[0].Message)
	}
	// Positions: the largest column accepted, one past refused as a bare message.
	ok, _ := parseDiagnostics([]byte(fmt.Sprintf("main.typ:2:%d: error: big\n", 1<<30-1)), "/p", "/c", lim)
	if len(ok) != 1 || ok[0].Line != 2 || ok[0].File == "" {
		t.Errorf("column 2^30-1 = %+v, want a positioned diagnostic", ok)
	}
	over, _ := parseDiagnostics([]byte(fmt.Sprintf("main.typ:2:%d: error: big\n", 1<<30)), "/p", "/c", lim)
	if len(over) == 1 && over[0].File != "" {
		t.Errorf("column 2^30 = %+v, want it refused as positioned", over)
	}
	zero, _ := parseDiagnostics([]byte("main.typ:0:0: error: top\n"), "/p", "/c", lim)
	if len(zero) != 1 || zero[0].File != "main.typ" || zero[0].Column != 1 {
		t.Errorf("line 0 column 0 = %+v, want accepted, column 1", zero)
	}
}
