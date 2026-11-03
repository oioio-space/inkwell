package sidecar

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectCannotReadHomeSentinel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(home)
	sentinel := filepath.Join(home, "secret")
	if err := os.WriteFile(sentinel, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, mounted := range []bool{false, true} {
		s := spec("sandboxstatus")
		s.Dir = t.TempDir()
		s.Env = append(s.Env, "INKWELL_ATTACK=sentinel", "INKWELL_SENTINEL="+sentinel)
		if mounted {
			s.WatchDir, s.MaxDirBytes, s.WriteFiles = t.TempDir(), 8192, []string{"out"}
		}
		out, err := Run(t.Context(), s)
		if err != nil || out.ExitCode != 0 {
			t.Fatalf("%v: %+v stderr %s", err, out, out.Stderr)
		}
	}
}
