//go:build darwin

package memory

import "testing"

func TestFreeMemoryUnavailable(t *testing.T) {
	if got := FreeMemory(); got != 0 {
		t.Fatalf("FreeMemory=%d, want unavailable (0)", got)
	}
}
