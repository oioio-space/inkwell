// SPDX-License-Identifier: MPL-2.0

package protocol

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The reviewed fixture binds every complete normative row to its constants and
// exact values (durations in nanoseconds). Some rows contain several bounds;
// qualitative rows and repeated bounds introduce no new constant. Pinning the
// complete row also catches scope, unit, recovery and nonnumeric policy drift.
func TestLimitsMatchSpecification(t *testing.T) {
	var expected []struct {
		Row       string
		Constants map[string]string
	}
	data, err := os.ReadFile("testdata/limits.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile("../docs/protocol/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	inSection := false
	for _, line := range strings.Split(string(doc), "\n") {
		if line == "## 16. Table des bornes" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(line, "## ") {
			break
		}
		if !inSection || !strings.HasPrefix(line, "|") {
			continue
		}
		if strings.HasPrefix(line, "| Objet |") || strings.Trim(line, "|-: ") == "" {
			continue // table header or separator
		}
		if cells := strings.Split(line, " | "); !strings.HasPrefix(line, "| ") || !strings.HasSuffix(line, " |") || len(cells) < 3 {
			t.Fatalf("unparsable §16 table row: %q", line)
		}
		rows = append(rows, line)
	}
	var wantRows []string
	values := map[string]string{}
	for _, row := range expected {
		wantRows = append(wantRows, row.Row)
		for name, value := range row.Constants {
			if _, exists := values[name]; exists {
				t.Fatalf("constant %s mapped more than once", name)
			}
			values[name] = value
		}
	}
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatal("§16 rows changed: review and update testdata/limits.json and limits.go together")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "limits.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("protocol", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	actual := map[string]string{}
	for _, name := range pkg.Scope().Names() {
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok || !c.Exported() || name == "Version" {
			continue
		}
		actual[name] = constant.ToInt(c.Val()).ExactString()
	}
	if !reflect.DeepEqual(actual, values) {
		t.Errorf("limits differ from reviewed §16 row values:\nactual: %v\nexpected: %v", actual, values)
	}
}
