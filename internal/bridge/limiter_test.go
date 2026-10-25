package bridge

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestLimiterAllowsOncePerSecondPerReason(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var l limiter
		if n, ok := l.allow("a"); !ok || n != 0 {
			t.Fatalf("first allow(a) = %d, %v; want 0, true", n, ok)
		}
		for range 3 {
			if _, ok := l.allow("a"); ok {
				t.Error("allow(a) within a second = true, want false")
			}
		}
		if n, ok := l.allow("b"); !ok || n != 0 {
			t.Errorf("allow(b) = %d, %v; reasons must be independent", n, ok)
		}
		time.Sleep(time.Second)
		if n, ok := l.allow("a"); !ok || n != 3 {
			t.Errorf("allow(a) after 1s = %d, %v; want 3, true", n, ok)
		}
		if n, ok := l.allow("a"); ok || n != 0 {
			t.Errorf("allow(a) right after = %d, %v; want 0, false", n, ok)
		}
	})
}
