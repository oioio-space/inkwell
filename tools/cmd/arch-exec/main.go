// Command arch-exec is the architecture vet tool.
package main

import (
	"fmt"
	"os"

	"github.com/oioio-space/inkwell/tools/archexec"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-deps" {
		if err := archexec.ScanDependencies(os.Stdin); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "-syntax" {
		if err := archexec.ScanSources(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	singlechecker.Main(archexec.Analyzer)
}
