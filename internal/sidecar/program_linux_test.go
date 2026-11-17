package sidecar

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// S-10: typst and tinymist are two entries of one descriptor table, with the
// lists measured on the runners (gates/SB2/MEASUREMENTS.md, runs 3 and 4), and
// no other code path of the package names them.
func TestProgramDescriptors(t *testing.T) {
	var names []string
	for _, p := range sidecarPrograms() { // the test binary's own descriptor aside
		names = append(names, p.name)
	}
	if !slices.Equal(names, []string{Typst, Tinymist}) {
		t.Fatalf("S-10 descriptor: programs %v, want [typst tinymist]", names)
	}
	for _, c := range []struct {
		name      string
		prctl     []uint32
		ioctl     []uint32
		perimeter []string
	}{
		{Typst, nil, []uint32{unix.TIOCGWINSZ}, []string{"compile-pdf", "compile-svg", "query", "eval"}},
		{Tinymist, []uint32{unix.PR_SET_NAME}, []uint32{unix.TIOCINQ, unix.TCGETS}, []string{"lsp"}},
	} {
		p, ok := lookupProgram(c.name)
		if !ok {
			t.Fatalf("S-10 descriptor: %s not registered", c.name)
		}
		if !slices.Equal(p.prctl, c.prctl) || !slices.Equal(p.ioctl, c.ioctl) || !slices.Equal(p.perimeter, c.perimeter) {
			t.Errorf("S-10 descriptor: %s: prctl %v ioctl %#x perimeter %v; want %v %#x %v", c.name, p.prctl, p.ioctl, p.perimeter, c.prctl, c.ioctl, c.perimeter)
		}
		if !slices.Equal(p.proven, []string{"amd64"}) {
			t.Errorf("S-10 descriptor: %s proven on %v, want amd64 only until SB2a2", c.name, p.proven)
		}
		for _, a := range sysArches {
			if len(p.syscalls[a.name]) == 0 {
				t.Errorf("S-10 descriptor: %s has no allow-list for %s", c.name, a.name)
			}
		}
	}
	if _, ok := lookupProgram("sh"); ok {
		t.Error("S-10 descriptor: an unregistered program was found")
	}

	// No other non-test file names the two programs: descriptors are the only
	// place their lists live (manifest.go defines the names themselves).
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") || file == "manifest.go" || file == "program_linux.go" {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if n.Name == "Typst" || n.Name == "Tinymist" {
					t.Errorf("S-10 descriptor: %s names %s", fset.Position(n.Pos()), n.Name)
				}
			case *ast.BasicLit:
				if s, err := strconv.Unquote(n.Value); err == nil && (strings.EqualFold(s, Typst) || strings.EqualFold(s, Tinymist)) {
					t.Errorf("S-10 descriptor: %s names %q", fset.Position(n.Pos()), s)
				}
			}
			return true
		})
	}
}

// withProgram registers p for the test, as a plugin host would be (AD-57).
func withProgram(t *testing.T, p program) {
	t.Helper()
	saved := programs
	programs = append(slices.Clip(programs), p)
	t.Cleanup(func() { programs = saved })
}

// S-10: a third descriptor gets its own lists, not typst's or tinymist's, and
// decideTier follows its own proven architectures.
func TestProgramFilterFollowsDescriptor(t *testing.T) {
	const fionbio = 0x5421
	third := program{
		name:      "third",
		syscalls:  map[string][]uint32{"amd64": {0, 1, 16, 157}, "arm64": {63, 64, 29, 167}}, // read, write, ioctl, prctl
		proven:    []string{"amd64", "arm64"},
		ioctl:     []uint32{fionbio},
		prctl:     []uint32{unix.PR_GET_NAME},
		perimeter: []string{"echo"},
	}
	withProgram(t, third)
	p, ok := lookupProgram("third")
	if !ok {
		t.Fatal("S-10 descriptor: the third program is not registered")
	}
	for _, a := range sysArches {
		for _, tr := range []tier{tierN, tierF} {
			f, err := seccompFilter(p, a, 1234, tr)
			if err != nil {
				t.Fatalf("S-10 descriptor: %s %v: %v", a.name, tr, err)
			}
			at := func(nr uint32, w uint32, args ...uint64) {
				t.Helper()
				if got := verdict(t, f, a, nr, args...); got != w {
					t.Errorf("S-10 descriptor: %s %v nr %d args %#x: verdict %#x, want %#x", a.name, tr, nr, args, got, w)
				}
			}
			at(third.syscalls[a.name][0], retAllow)
			at(third.syscalls[a.name][1], retAllow)
			at(a.ioctl, retAllow, 0, fionbio)
			at(a.prctl, retAllow, unix.PR_GET_NAME)
			// typst's and tinymist's lists are not applied to it.
			at(a.ioctl, retEPERM, 0, unix.TIOCGWINSZ)
			at(a.ioctl, retEPERM, 0, unix.TCGETS)
			at(a.prctl, retEPERM, unix.PR_SET_NAME)
			at(a.tgkill, retEPERM, 1234) // in the shared rules, but not in its allow-list
			for _, nr := range allowedSyscallsFor(a) {
				if !slices.Contains(third.syscalls[a.name], nr) && nr != a.clone3 {
					at(nr, retEPERM)
				}
			}
		}
		if got, err := decideTier(p, tierFacts{Arch: a.name, Landlock: 8, Seccomp: true, NS: launchOK}); err != nil || got != tierN {
			t.Errorf("S-10 descriptor: decideTier(third, %s) = %v, %v; want N", a.name, got, err)
		}
	}
	// An architecture the descriptor has no list for builds no filter.
	delete(third.syscalls, "arm64")
	if _, err := seccompFilter(third, archARM64, 1, tierN); err == nil {
		t.Error("S-10 descriptor: a filter was built without an allow-list")
	}
}

// allowedSyscallsFor is the generated table typst and tinymist share today.
func allowedSyscallsFor(a sysArch) []uint32 {
	if a.name == "arm64" {
		return allowedSyscallsARM64
	}
	return allowedSyscallsAMD64
}
