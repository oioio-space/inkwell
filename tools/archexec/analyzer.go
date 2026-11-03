// Package archexec enforces one rule on inkwell's own code: outside the sidecar
// launcher (internal/sidecar), no package imports os/exec or starts a process,
// in any build configuration. The rule covers the module's own packages only;
// dependencies are not scanned (requirement N42 as clarified on 2026-11-03:
// using platform libraries is fine, requiring an installed program is not).
//
// [Analyzer] type-checks each build configuration through go vet; [ScanSources]
// parses every Go file regardless of build tags.
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

// Analyzer reports, in production files outside internal/sidecar, imports of
// os/exec or C, process-start and raw syscall functions, go:linkname
// directives and native sources: the ways own code could start a process.
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
		if path == "C" || path == "os/exec" {
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
	if p == "golang.org/x/sys/windows" {
		forbidden = strings.HasPrefix(name, "CreateProcess") || strings.HasPrefix(name, "ShellExecute")
	}
	if forbidden {
		pass.Reportf(id.Pos(), "forbidden launch or syscall %s.%s", p, name)
	}
}
