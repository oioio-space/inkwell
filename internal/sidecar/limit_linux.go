package sidecar

// withLimits installs byte-exact limits in our child before the target exec.
func withLimits(spec Spec) (string, []string, error) {
	path, args, err := sandboxPrefix(sandboxConfig{ReadPaths: spec.ReadPaths, WorkingDir: spec.Dir, Memory: spec.MemoryLimit, File: spec.FileSizeLimit, Overlay: probeOutputMount()})
	return path, append(append(args, spec.Path), spec.Args...), err
}
