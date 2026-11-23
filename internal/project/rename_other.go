//go:build !linux && !darwin && !windows

package project

import "os"

// renameNative is not available here; the move falls back to a link and an unlink.
func (p *Project) renameNative(src, dst string) error { return errNoNative }

// renameChild is [RenameNoReplace] on checked names: no rename that never replaces exists here,
// so nothing is renamed.
func renameChild(*os.Root, string, string) error { return errNoNative }
