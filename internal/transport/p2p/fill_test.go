package p2p_test

import (
	"go/constant"
	"go/types"
	"os"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const fillID = "/inkwell/netsim-fill/1.0.0"

// fillObjects returns the package-level objects of the node package, built with the given tags,
// that are the netsim filler: whatever is named after it or holds its protocol identifier.
func fillObjects(t *testing.T, tags string) map[string]string {
	t.Helper()
	cfg := &packages.Config{
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedImports,
		BuildFlags: []string{"-tags=" + tags},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 || pkgs[0].Types == nil {
		t.Fatalf("loading the package with tags %q: %v %v", tags, err, pkgs)
	}
	pkg := pkgs[0]
	found := map[string]string{}
	for _, name := range pkg.Types.Scope().Names() {
		obj := pkg.Types.Scope().Lookup(name)
		if c, ok := obj.(*types.Const); ok && c.Val().Kind() == constant.String && strings.Contains(constant.StringVal(c.Val()), "netsim-fill") {
			found[name] = pkg.Fset.Position(obj.Pos()).Filename
		}
		if strings.HasPrefix(strings.ToLower(name), "fill") || strings.Contains(name, "NetsimFill") {
			found[name] = pkg.Fset.Position(obj.Pos()).Filename
		}
		if named, ok := obj.Type().(*types.Named); ok { // methods of a type: n.fill*, n.ProtocolFill
			for m := range named.Methods() {
				if strings.HasPrefix(m.Name(), "fill") {
					found[named.Obj().Name()+"."+m.Name()] = pkg.Fset.Position(m.Pos()).Filename
				}
			}
		}
	}
	return found
}

// TestNetsimFillOnlyInServerBuild: the netsim filler exists in the `-tags server` build only. Loaded
// with type information, the default build has no object that is the filler's (a constant holding
// its protocol identifier, a function or a method named after it), and the server build has them,
// every one in a file constrained to the server tag.
func TestNetsimFillOnlyInServerBuild(t *testing.T) {
	if got := fillObjects(t, "none"); len(got) != 0 {
		// The no-op hooks of fill_default.go are named fill* too: they are the only allowed.
		for name, file := range got {
			if !strings.HasSuffix(file, "fill_default.go") {
				t.Errorf("the default build has %s (%s), want only the no-op hooks of fill_default.go", name, file)
			}
		}
	}
	server := fillObjects(t, "server")
	if _, ok := server["ProtocolFill"]; !ok {
		t.Fatalf("the server build has no ProtocolFill: the check looks at nothing (%v)", server)
	}
	for name, file := range server {
		if strings.HasSuffix(file, "fill_default.go") {
			t.Errorf("the server build still has %s from fill_default.go", name)
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if first, _, _ := strings.Cut(string(b), "\n"); first != "//go:build server" {
			t.Errorf("%s (%s) is the filler's but its file starts with %q, want //go:build server", name, file, first)
		}
	}
}
