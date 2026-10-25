package store

import (
	"fmt"
	"os"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// A re-executed test binary plays the writer that the parent kills.
	if mode := os.Getenv(childEnv); mode != "" {
		if err := runChild(mode, os.Getenv(childDirEnv)); err != nil {
			fmt.Fprintln(os.Stderr, "child:", err)
			os.Exit(2)
		}
		return
	}
	goleak.VerifyTestMain(m)
}
