package transport_test

import (
	"go/ast"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// site is a place in the code: the last element of the package path, the file, and the top-level
// function around it (Type.Method for a method; a closure belongs to its function).
type site struct{ pkg, file, fn string }

// The link probe's figure and level are display-only (spec §8.5.5: a prober MUST NOT use them for a
// protocol decision; Annex A.1). Each has one producer and, at most, one reader, named here exactly;
// anything else in the module that touches them fails TestLinkProbeOnlyBridgeReads. Keys are
// Type.member.
var (
	producers = map[string][]site{
		"Link.ProbeLevel":       {{"p2p", "links.go", "Node.Links"}},
		"Link.ProbeLoss":        {{"p2p", "links.go", "Node.Links"}},
		"LinkStep.Level":        {{"p2p", "links.go", "peerLink.step"}},
		"LinkStep.LossPermille": {{"p2p", "links.go", "peerLink.step"}},
	}
	readers = map[string][]site{
		"Link.ProbeLevel":       {{"bridge", "links.go", "PeerLink.Quality"}},
		"Link.ProbeLoss":        {{"bridge", "linksnotice.go", "linkEntry"}},
		"LinkStep.Level":        {{"app", "linklog_server.go", "linkStepSink"}},
		"LinkStep.LossPermille": {{"app", "linklog_server.go", "linkStepSink"}},
		"Host.Level":            {{"p2p", "probe.go", "probeConn.view"}},
		"Host.Loss":             {{"p2p", "probe.go", "probeConn.view"}},
		"PeerLink.Quality":      {{"bridge", "linksnotice.go", "linkEntry"}},
		"Prober.Level":          nil,
		"Prober.Loss":           nil,
	}
)

// owner returns the name of the named type behind t (a pointer is looked through).
func owner(t types.Type) string {
	if t == nil {
		return ""
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

// memberKey names what an identifier refers to, as Type.member, when it is one of the figure's or
// level's members: a field or a method of the types above, found by type, not by spelling.
func memberKey(info *types.Info, id *ast.Ident, recv types.Type) string {
	obj := info.Uses[id]
	if obj == nil {
		return ""
	}
	switch o := obj.(type) {
	case *types.Var:
		if !o.IsField() || o.Pkg() == nil {
			return ""
		}
		switch {
		case strings.HasSuffix(o.Pkg().Path(), "/internal/transport") && (o.Name() == "ProbeLevel" || o.Name() == "ProbeLoss"):
			return "Link." + o.Name()
		case strings.HasSuffix(o.Pkg().Path(), "/p2p") && (o.Name() == "Level" || o.Name() == "LossPermille") && owner(recv) == "LinkStep":
			return "LinkStep." + o.Name()
		}
	case *types.Func:
		sig, ok := o.Type().(*types.Signature)
		if !ok || sig.Recv() == nil || o.Pkg() == nil {
			return ""
		}
		switch r := owner(sig.Recv().Type()); {
		case strings.HasSuffix(o.Pkg().Path(), "/protocol/linkprobe") && (r == "Host" || r == "Prober") && (o.Name() == "Level" || o.Name() == "Loss"):
			return r + "." + o.Name()
		case strings.HasSuffix(o.Pkg().Path(), "/internal/bridge") && r == "PeerLink" && o.Name() == "Quality":
			return "PeerLink.Quality"
		}
	}
	return ""
}

// isOnLinkStep reports whether id is the OnLinkStep field of the node's options.
func isOnLinkStep(info *types.Info, id *ast.Ident) bool {
	v, ok := info.Uses[id].(*types.Var)
	return ok && v.IsField() && v.Name() == "OnLinkStep" && v.Pkg() != nil && strings.HasSuffix(v.Pkg().Path(), "/p2p")
}

// serverOnly reports whether the file is constrained to the server build tag.
func serverOnly(t *testing.T, name string) bool {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(string(b), "\n")
	return strings.HasPrefix(first, "//go:build") && strings.Contains(first, "server") && !strings.Contains(first, "!server")
}

// funcName is the name of the declaration around a node: Type.Method or the function's.
func funcName(d *ast.FuncDecl) string {
	if d.Recv != nil && len(d.Recv.List) == 1 {
		t := d.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name + "." + d.Name.Name
		}
	}
	return d.Name.Name
}

// TestLinkProbeOnlyBridgeReads proves, with type information and in the default and the server
// builds, that the probe's figure and level go only where they are allowed: written by the node's
// link sampler, read by the bridge (and by the server build's diagnostics log), never by anything
// that could decide with them. It flags a use of transport.Link.ProbeLevel or .ProbeLoss,
// linkprobe.Host.Level or .Loss, bridge.PeerLink.Quality or p2p.LinkStep.Level or .LossPermille
// outside the exact function that may write or read it, and any assignment of
// p2p.Options.OnLinkStep outside a file constrained to the server build.
func TestLinkProbeOnlyBridgeReads(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tags := range []string{"none", "server"} {
		cfg := &packages.Config{
			Dir:        root,
			Mode:       packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
			BuildFlags: []string{"-tags=" + tags},
		}
		pkgs, err := packages.Load(cfg, "./internal/...", "./cmd/...")
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range pkgs {
			if pkg.TypesInfo == nil {
				t.Errorf("tags %s: %s was not type-checked: %v", tags, pkg.PkgPath, pkg.Errors)
				continue
			}
			for _, f := range pkg.Syntax {
				file := pkg.Fset.Position(f.Pos()).Filename
				here := func(fn string) site { return site{path.Base(pkg.PkgPath), filepath.Base(file), fn} }
				for _, decl := range f.Decls {
					fn := ""
					if d, ok := decl.(*ast.FuncDecl); ok {
						fn = funcName(d)
					}
					check := func(key string, write bool, pos ast.Node) {
						allowed := readers[key]
						if write {
							allowed = producers[key]
						}
						kind := map[bool]string{true: "writes", false: "reads"}[write]
						ok := false
						for _, s := range allowed {
							ok = ok || s == here(fn)
						}
						if ok {
							seen[key+" "+kind] = true
							return
						}
						t.Errorf("%s: %s %s %s in %s: only %v may", pkg.Fset.Position(pos.Pos()), "code", kind, key, fn, allowed)
					}
					var stack []ast.Node
					ast.Inspect(decl, func(n ast.Node) bool {
						if n == nil {
							stack = stack[:len(stack)-1]
							return true
						}
						stack = append(stack, n)
						switch x := n.(type) {
						case *ast.SelectorExpr:
							recv := pkg.TypesInfo.TypeOf(x.X)
							if key := memberKey(pkg.TypesInfo, x.Sel, recv); key != "" {
								write := false
								if len(stack) > 1 {
									if as, ok := stack[len(stack)-2].(*ast.AssignStmt); ok {
										for _, l := range as.Lhs {
											write = write || l == ast.Expr(x)
										}
									}
								}
								check(key, write, x)
							}
							if isOnLinkStep(pkg.TypesInfo, x.Sel) && len(stack) > 1 {
								if as, ok := stack[len(stack)-2].(*ast.AssignStmt); ok {
									for _, l := range as.Lhs {
										if l == ast.Expr(x) && !serverOnly(t, file) {
											t.Errorf("%s: OnLinkStep assigned outside a server-tagged file", pkg.Fset.Position(x.Pos()))
										}
									}
								}
							}
						case *ast.KeyValueExpr:
							id, ok := x.Key.(*ast.Ident)
							if !ok || len(stack) < 2 {
								break
							}
							lit, ok := stack[len(stack)-2].(*ast.CompositeLit)
							if !ok {
								break
							}
							if key := memberKey(pkg.TypesInfo, id, pkg.TypesInfo.TypeOf(lit)); key != "" {
								check(key, true, x)
							}
							if isOnLinkStep(pkg.TypesInfo, id) {
								seen["OnLinkStep set"] = true
								if !serverOnly(t, file) {
									t.Errorf("%s: OnLinkStep set outside a server-tagged file", pkg.Fset.Position(x.Pos()))
								}
							}
						}
						return true
					})
				}
			}
		}
	}
	for key, sites := range producers {
		if len(sites) > 0 && !seen[key+" writes"] {
			t.Errorf("%s is never written where it is allowed: the check looks at nothing", key)
		}
	}
	for key, sites := range readers {
		if len(sites) > 0 && !seen[key+" reads"] {
			t.Errorf("%s is never read where it is allowed: the check looks at nothing", key)
		}
	}
}
