package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.go")
	for _, tc := range []struct {
		source string
		want   int
	}{{"package fixture\n", 0}, {"package fixture\nimport \"os/exec\"\n", 1}} {
		if err := os.WriteFile(path, []byte(tc.source), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := run([]string{"-syntax", dir}); got != tc.want {
			t.Fatalf("run(-syntax) on %q = %d, want %d", tc.source, got, tc.want)
		}
	}
}

func TestMainDispatch(t *testing.T) {
	oldArgs, oldChecker := os.Args, analyzerMain
	defer func() { os.Args = oldArgs; analyzerMain = oldChecker }()
	os.Args = []string{"arch-exec", "-syntax", t.TempDir()}
	main()
	called := false
	analyzerMain = func(a *analysis.Analyzer) {
		called = true
		if a == nil {
			t.Fatal("missing analyzer")
		}
	}
	if code := run(nil); code != 0 || !called {
		t.Fatal("analyzer not dispatched")
	}
}
