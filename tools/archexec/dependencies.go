package archexec

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

type dependency struct {
	ImportPath, Dir            string
	Imports, GoFiles, CgoFiles []string
	Standard, DepOnly          bool
}

// ScanDependencies consumes go list -deps -json for exactly the build being checked.
// Unlike export data, Imports includes implementation-only dependencies.
func ScanDependencies(r io.Reader) error {
	packages := map[string]dependency{}
	dec := json.NewDecoder(r)
	for {
		var p dependency
		if err := dec.Decode(&p); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		packages[p.ImportPath] = p
	}
	dangerous := map[string]bool{"os/exec": true}
	if err := markExecEntries(packages, dangerous); err != nil {
		return err
	}
	var reaches func(string, map[string]bool) bool
	reaches = func(path string, seen map[string]bool) bool {
		if allowedDependency(path) || path == "syscall" || path == "golang.org/x/sys/unix" || path == "golang.org/x/sys/windows" || seen[path] {
			return false
		}
		if dangerous[path] {
			return true
		}
		seen[path] = true
		for _, child := range packages[path].Imports {
			if reaches(child, seen) {
				return true
			}
		}
		return false
	}
	var violations []string
	for path, p := range packages {
		if p.Standard || p.DepOnly || allowedDependency(path) {
			continue
		}
		if reaches(path, map[string]bool{}) {
			violations = append(violations, "forbidden process-launch dependency: "+path)
		}
	}
	if len(violations) != 0 {
		return fmt.Errorf("%s", strings.Join(violations, "\n"))
	}
	return nil
}

func allowedDependency(path string) bool {
	// Only the developer analyzer executable may invoke the Go toolchain.
	// Do not exempt its general-purpose dependencies for application callers.
	return path == "github.com/oioio-space/inkwell/tools/cmd/arch-exec" || path == "github.com/oioio-space/inkwell/internal/sidecar" || strings.HasPrefix(path, "github.com/wailsapp/wails/v3/")
}

func execEntry(name string) bool {
	return name == "SYS_FORK" || name == "SYS_VFORK" || name == "SYS_EXECVE" || name == "SYS_EXECVEAT" || name == "SYS_CLONE" || name == "SYS_CLONE3" || name == "Exec" || name == "ForkExec" || name == "StartProcess" || name == "CreateProcess" || name == "CreateProcessAsUser"
}

func markExecEntries(packages map[string]dependency, dangerous map[string]bool) error {
	for path, p := range packages {
		if p.Standard || allowedDependency(path) {
			continue
		}
		if len(p.CgoFiles) != 0 {
			dangerous[path] = true
		}
		for _, name := range append(p.GoFiles, p.CgoFiles...) {
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(p.Dir, name), nil, 0)
			if err != nil {
				return err
			}
			markFileExecEntries(f, path, dangerous)
		}
	}
	return nil
}

func markFileExecEntries(f *ast.File, path string, dangerous map[string]bool) {
	aliases := map[string]bool{}
	for _, imp := range f.Imports {
		imported, _ := strconv.Unquote(imp.Path.Value)
		if imported == "C" {
			dangerous[path] = true
		}
		if imported != "os" && imported != "syscall" && imported != "golang.org/x/sys/unix" && imported != "golang.org/x/sys/windows" {
			continue
		}
		alias := filepath.Base(imported)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		aliases[alias] = true
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && aliases[id.Name] && execEntry(v.Sel.Name) {
				dangerous[path] = true
			}
		case *ast.Ident:
			if aliases["."] && execEntry(v.Name) {
				dangerous[path] = true
			}
		}
		return true
	})
}
