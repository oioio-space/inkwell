package typst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

const (
	maxQueryBytes    = 16 << 20
	maxSelectorBytes = 4 << 10
)

// Query runs `typst query` on main and returns the JSON array of the elements
// the selector matches: the headings (`heading`), the labels (`<intro>`), the
// metadata (`<data>`), and so on. It is how Inkwell reads the real labels and
// bibliography keys of a project rather than guessing them from the text.
//
// It has the same limits and the same errors as [Engine.Compile].
func (e *Engine) Query(ctx context.Context, p Project, main, selector string, opts ...Option) (json.RawMessage, error) {
	if selector == "" || len(selector) > maxSelectorBytes {
		return nil, errors.New("typst: query: the selector must be 1 to 4096 bytes")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o := applyOptions(opts)
	release, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	j, err := e.prepare(p, main)
	if err != nil {
		return nil, err
	}
	defer j.cleanup()

	runCtx, cancel := e.deadline(ctx)
	defer cancel()
	out, err := e.run(runCtx, j, e.args("query", j, o, "--format", "json", "--", filepath.ToSlash(j.main), selector), maxQueryBytes, "")
	if err != nil {
		if runCtx.Err() != nil {
			return nil, ended(runCtx, e.cfg.Timeout)
		}
		return nil, err
	}
	if out.ExitCode != 0 {
		res := &Result{}
		if err := e.settle(res, j, []sidecar.Output{out}); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("typst: query: the compiler exited with status %d", out.ExitCode)
	}
	if out.Truncated {
		return nil, fmt.Errorf("%w: the query result exceeds %d bytes", ErrOutputTooLarge, maxQueryBytes)
	}
	if !json.Valid(out.Stdout) {
		return nil, errors.New("typst: query: the result is not JSON")
	}
	return json.RawMessage(out.Stdout), nil
}
