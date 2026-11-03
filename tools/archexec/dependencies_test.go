package archexec

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImplementationOnlyOpenerDependency(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./...")
	cmd.Dir = "testdata/opener"
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	err = ScanDependencies(bytes.NewReader(data))
	if err == nil || !strings.Contains(err.Error(), "fixture") {
		t.Fatalf("opener.Open(string) escaped: %v", err)
	}
}

func TestHiddenProcessDependencies(t *testing.T) {
	for _, fixture := range []string{"rawsyscall", "cgo"} {
		t.Run(fixture, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", fixture))
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(dependency{ImportPath: "fixture", Dir: dir, GoFiles: []string{"launch.go"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := ScanDependencies(bytes.NewReader(data)); err == nil {
				t.Fatal("dependency escaped")
			}
		})
	}
}

func TestProcessSyscallVariants(t *testing.T) {
	for _, entry := range []string{"SYS_FORK", "SYS_VFORK", "SYS_EXECVE", "SYS_EXECVEAT", "SYS_CLONE", "SYS_CLONE3"} {
		for _, call := range []string{"RawSyscall", "Syscall", "RawSyscall6", "Syscall6"} {
			source := "package fixture; import s \"syscall\"; func launch() { s." + call + "(s." + entry + ", 0, 0, 0) }"
			f, err := parser.ParseFile(token.NewFileSet(), "", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			dangerous := map[string]bool{}
			markFileExecEntries(f, "fixture", dangerous)
			if !dangerous["fixture"] {
				t.Fatalf("missed %s/%s", call, entry)
			}
		}
	}
}
