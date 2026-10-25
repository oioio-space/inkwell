package typst

import (
	"errors"
	"fmt"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

var (
	// ErrCompile is wrapped by every [*CompileError]: the document has errors.
	ErrCompile = errors.New("typst: the document has errors")
	// ErrMissingPackage means the document imports a package that is neither
	// bundled with Inkwell nor in the project's .typstdeps folder. Inkwell never
	// downloads packages.
	ErrMissingPackage = errors.New("typst: package not available offline")
	// ErrTimeout means the compilation was stopped because it ran too long.
	ErrTimeout = errors.New("typst: compilation took too long")
	// ErrMemoryLimit means the compilation asked for more memory than allowed.
	ErrMemoryLimit = errors.New("typst: compilation used too much memory")
	// ErrOutputTooLarge means the result has more pages or bytes than allowed.
	ErrOutputTooLarge = errors.New("typst: the result is too large")
	// ErrInvalidMain means the main file is not a file inside the project.
	ErrInvalidMain = errors.New("typst: the main file must be a file inside the project")
	// ErrUnsafeProject means the project holds a symbolic link that leads out of
	// the project folder, or has too many files to be checked.
	ErrUnsafeProject = errors.New("typst: the project links outside its folder")
	// ErrUnsafeArchive means a package archive could escape its folder or is
	// larger than allowed.
	ErrUnsafeArchive = errors.New("typst: unsafe package archive")
	// ErrIntegrity is the integrity failure of an embedded file (a sidecar
	// binary or a bundled package that does not match its pinned SHA-256).
	ErrIntegrity = sidecar.ErrIntegrity
)

// CompileError is returned when Typst reports errors in the document. The
// diagnostics are in [Result.Diagnostics] too.
type CompileError struct {
	// Diagnostics holds the errors and the warnings, in Typst's order.
	Diagnostics []Diagnostic
	// MissingPackages lists the packages that could not be loaded.
	MissingPackages []string
}

// Error summarizes the first error.
func (e *CompileError) Error() string {
	for _, d := range e.Diagnostics {
		if d.Severity != Error {
			continue
		}
		if d.File != "" {
			return fmt.Sprintf("typst: %s:%d:%d: %s", d.File, d.Line, d.Column, d.Message)
		}
		return "typst: " + d.Message
	}
	return ErrCompile.Error()
}

// Is makes errors.Is match [ErrCompile], and [ErrMissingPackage] when a
// package could not be loaded.
func (e *CompileError) Is(target error) bool {
	return target == ErrCompile || (target == ErrMissingPackage && len(e.MissingPackages) > 0)
}
