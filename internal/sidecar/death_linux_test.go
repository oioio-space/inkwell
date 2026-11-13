package sidecar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNamespaceDeathIdentityAndManifest(t *testing.T) {
	d := identityFromStatus([]byte("Name:\tactual-name\nPid:\t14000\nNSpid:\t14000\t2\n"))
	if d.PID != 14000 || d.Comm != "actual-name" || len(d.NSPID) != 2 || d.NSPID[1] != 2 {
		t.Fatal(d)
	}
	path := filepath.Join(t.TempDir(), "deaths.jsonl")
	t.Setenv("INKWELL_SANDBOX_DEATHS", path)
	if e := recordRunManifest(t.Name(), Output{Death: &d, Signal: 31, CoreDump: CoreDumpNone}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got["pid"] != float64(14000) || got["coredumped"] != false || got["comm"] != "actual-name" {
		t.Fatal(got)
	}
	if e = recordRunManifest(t.Name(), Output{Signal: 31, CoreDump: CoreDumpNone}); e == nil {
		t.Fatal("missing identity silently accepted")
	}
}
