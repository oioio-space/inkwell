package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// RT1 rule B-3 on the join (docs/plans/2026-11-25-rt1-root-kept.md): a storage that cannot be read holds nothing
// proven, so the join is not completed on it.
func TestRT1B3JoinRefusedWhileRootUnknown(t *testing.T) {
	c := newCompleteCase(t, "")
	c.store(t, joinedRoot, "snapshot")
	name := filepath.Join(c.folder, ".inkwell", "repo", strings.TrimPrefix(joinedRoot, "automerge:"), "snapshot", "abc123~")
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff // the one chunk is damaged, with no backup
	if err := os.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// The first call reads the chunk and quarantines it; the second finds none left: the quarantine still says the
	// document is damaged, not absent (review M1).
	for i := range 2 {
		err = c.rt.Session.CompleteJoin(joinedRoot, string(c.joined.Host))
		if m := c.manifest(t); err == nil || !strings.Contains(m, "pendingJoin") || c.proj.Meta().RootURL != "" {
			t.Errorf("B-3 a damaged root is not stored (call %d): CompleteJoin = %v; project.json is %s; want refused, the record kept", i+1, err, m)
		}
	}
}
