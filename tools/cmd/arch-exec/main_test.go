package main

import (
	"os"
	"path/filepath"
	"strings"
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
			t.Fatalf("got %d want %d", got, tc.want)
		}
	}
	old := os.Stdin
	defer func() { os.Stdin = old }()
	for _, data := range []string{"", "bad json"} {
		f, err := os.CreateTemp(t.TempDir(), "deps")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = f.Close() }()
		if _, err := f.WriteString(data); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		os.Stdin = f
		want := 0
		if strings.Contains(data, "bad") {
			want = 1
		}
		if got := run([]string{"-deps"}); got != want {
			t.Fatalf("deps: %d", got)
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
