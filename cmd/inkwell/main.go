// Command inkwell is the CLI entry point for github.com/oioio-space/inkwell.
package main

import (
	"fmt"
	"os"

	inkwell "github.com/oioio-space/inkwell"
)

func main() {
	name := ""
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	fmt.Println(inkwell.Greet(name))
}
