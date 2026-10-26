package main

import "slices"

// versionRequested reports whether args, the command line without the program
// name, ask only for the version. It is checked before anything else starts,
// so that "inkwell --version" never opens a window, a project or the network.
func versionRequested(args []string) bool {
	return len(args) == 1 && slices.Contains([]string{"--version", "-version", "version"}, args[0])
}
