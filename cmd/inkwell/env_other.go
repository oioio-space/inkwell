//go:build !linux

package main

import "os"

// startEnv is the environment this process was started with: os.Getenv, where no library takes
// the activation token out of it before Go starts.
func startEnv() func(string) string { return os.Getenv }
