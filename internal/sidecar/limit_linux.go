package sidecar

// withLimits installs byte-exact limits in our child before the target exec.
func withLimits(spec Spec) (string, []string, error) {
	overlay, err := probeOutputMount()
	if err != nil {
		return "", nil, err
	}
	path, args, err := sandboxPrefix(sandboxConfig{
		RequireDeath: spec.DeathTest != "",
		ReadPaths:    spec.ReadPaths,
		WorkingDir:   spec.Dir,
		Memory:       spec.MemoryLimit,
		File:         spec.FileSizeLimit,
		Overlay:      overlay,
	})
	return path, append(append(args, spec.Path), spec.Args...), err
}
