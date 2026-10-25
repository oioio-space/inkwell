// Package inkwell is the root of the inkwell desktop application: a
// serverless, collaborative Typst editor (see docs/architecture.md).
//
// The package itself only carries the compiled frontend, embedded so the
// binary ships as a single file; the program lives in cmd/inkwell and the
// logic in internal/.
package inkwell

import "embed"

// Assets holds the built frontend (frontend/dist). Before the first frontend
// build it contains only a placeholder, which keeps the module compilable;
// `mise run app:build` produces the real assets.
//
//go:embed all:frontend/dist
var Assets embed.FS
