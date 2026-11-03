package archexec

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

// ScanSources checks imports even in packages entirely excluded by build tags.
// Tests share the analyzer's exemption for process-launch probes.
func ScanSources(root string) error {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "bindings", "vendor", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == "internal/sidecar" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		for _, imp := range file.Imports {
			name, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if name == "C" || name == "os/exec" {
				violations = append(violations, fmt.Sprintf("%s: forbidden import %s outside launcher", fset.Position(imp.Pos()), name))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("%s", strings.Join(violations, "\n"))
	}
	return nil
}
