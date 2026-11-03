// Package archexec enforces the production process-launch boundary.
package archexec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Analyzer rejects alternate launch paths, raw syscalls and native code.
var Analyzer = &analysis.Analyzer{Name: "archexec", Doc: "enforce the sidecar launcher boundary", Run: run}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg.Path() == "github.com/oioio-space/inkwell/internal/sidecar" {
		return nil, nil
	}
	for _, f := range pass.Files {
		// Test helpers may launch probes; production files never may.
		if strings.HasSuffix(pass.Fset.Position(f.Pos()).Filename, "_test.go") {
			continue
		}
		checkSource(pass, f, 0)
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if ok {
				checkReference(pass, id)
			}
			return true
		})
	}
	for _, path := range append(append([]string{}, pass.OtherFiles...), pass.IgnoredFiles...) {
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			data, err := pass.ReadFile(path)
			if err != nil {
				return nil, err
			}
			f, err := parser.ParseFile(pass.Fset, path, data, parser.ParseComments)
			if err != nil {
				return nil, err
			}
			checkSource(pass, f, pass.Files[0].Pos())
		}
		switch filepath.Ext(path) {
		case ".c", ".s", ".S":
			pass.Reportf(pass.Files[0].Pos(), "forbidden native source %s outside launcher", path)
		}
	}
	return nil, nil
}

func checkSource(pass *analysis.Pass, f *ast.File, anchor token.Pos) {
	report := func(pos token.Pos, format string, args ...any) {
		if anchor != 0 {
			pos = anchor
		}
		pass.Reportf(pos, format, args...)
	}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path == "C" || reachesExec(importedPackage(pass.Pkg, path), make(map[string]bool)) {
			report(imp.Pos(), "forbidden import %s outside launcher", path)
		}
	}
	for _, group := range f.Comments {
		for _, comment := range group.List {
			if strings.HasPrefix(comment.Text, "//go:linkname") {
				report(comment.Pos(), "forbidden go:linkname outside launcher")
			}
		}
	}
}

func checkReference(pass *analysis.Pass, id *ast.Ident) {
	obj, ok := pass.TypesInfo.Uses[id].(*types.Func)
	if !ok || obj.Pkg() == nil {
		return
	}
	p, name := obj.Pkg().Path(), obj.Name()
	forbidden := p == "os" && name == "StartProcess"
	if p == "syscall" || p == "golang.org/x/sys/unix" {
		forbidden = name == "Exec" || name == "ForkExec" || name == "StartProcess" ||
			strings.HasPrefix(name, "Syscall") || strings.HasPrefix(name, "RawSyscall")
	}
	if forbidden {
		pass.Reportf(id.Pos(), "forbidden launch or syscall %s.%s", p, name)
	}
}

// Stop at the audited launcher boundary and the desktop framework, whose
// native platform helpers are part of Wails rather than application launch code.
func reachesExec(pkg *types.Package, seen map[string]bool) bool {
	if pkg == nil || seen[pkg.Path()] {
		return false
	}
	path := pkg.Path()
	if path == "github.com/oioio-space/inkwell/internal/sidecar" || strings.HasPrefix(path, "github.com/wailsapp/wails/v3/") {
		return false
	}
	if path == "os/exec" {
		return true
	}
	seen[path] = true
	for _, child := range pkg.Imports() {
		if reachesExec(child, seen) {
			return true
		}
	}
	return false
}

func importedPackage(pkg *types.Package, path string) *types.Package {
	for _, child := range pkg.Imports() {
		if child.Path() == path {
			return child
		}
	}
	return nil
}
