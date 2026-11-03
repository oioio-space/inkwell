//go:build linux

package memory

import "testing"

func TestMemory(t *testing.T) {
	total, free := TotalMemory(), FreeMemory()
	if total == 0 || free == 0 || free > total {
		t.Fatalf("free=%d total=%d", free, total)
	}
}
