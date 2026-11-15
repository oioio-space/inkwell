package sidecar

// networkIsolation reports the tier the registered sidecars run in: true for
// N (a private network namespace among the others), false for F (no network
// through seccomp and Landlock, no namespace), an error wrapping
// [ErrSandboxRefused] or [ErrProbeInconclusive] when they cannot run. The
// probe's target is verified in store.
func networkIsolation(store *Store) (bool, error) {
	for _, p := range programs {
		if err := unproven(p); err != nil {
			return false, err
		}
	}
	facts, err := sandboxFacts(store)
	if err != nil {
		return false, err
	}
	decided := tierNone
	for _, p := range programs {
		t, err := decideTier(p, facts)
		if err != nil {
			return false, err
		}
		decided = max(decided, t)
	}
	return decided == tierN, nil
}
