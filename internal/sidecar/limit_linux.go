package sidecar

// withLimits installs byte-exact limits in our child before the target exec.
func withLimits(spec Spec, plan launchPlan) (string, []string, error) {
	path, args, err := sandboxPrefix(baseConfig(spec, plan))
	return path, append(append(args, spec.Path), spec.Args...), err
}
