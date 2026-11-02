package sidecar

// withLimits installs byte-exact limits in our child before the target exec.
func withLimits(spec Spec) (string, []string, error) {
	if spec.MemoryLimit == 0 && spec.FileSizeLimit == 0 {
		return spec.Path, spec.Args, nil
	}
	path, args, err := sandboxPrefix(sandboxConfig{Memory: spec.MemoryLimit, File: spec.FileSizeLimit})
	return path, append(append(args, spec.Path), spec.Args...), err
}
