//go:build !linux && !darwin && !windows

package project

// renameNative is not available here; the move falls back to a link and an unlink.
func (p *Project) renameNative(src, dst string) error { return errNoNative }
