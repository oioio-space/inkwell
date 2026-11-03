// Command arch-exec is the architecture vet tool.
//
// Run as a go vet -vettool it applies archexec.Analyzer to the type-checked
// build; with -syntax DIR it parses every Go file under DIR, whatever its
// build tags.
package main

import (
	"fmt"
	"os"

	"github.com/oioio-space/inkwell/tools/archexec"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var analyzerMain = singlechecker.Main

func main() {
	if code := run(os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

func run(args []string) int {
	if len(args) == 2 && args[0] == "-syntax" {
		if err := archexec.ScanSources(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	analyzerMain(archexec.Analyzer)
	return 0
}
