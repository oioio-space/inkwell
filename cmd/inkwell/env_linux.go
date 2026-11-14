package main

import (
	"bytes"
	"os"
	"sync"
)

// startEnv is the environment this process was started with. GTK 4 takes XDG_ACTIVATION_TOKEN and
// DESKTOP_STARTUP_ID out of the environment in a constructor that runs when the library is
// loaded, before Go copies the environment (gdk.c, stash_and_unset_environment): os.Getenv never
// sees them, though a second launch must forward them. The kernel keeps the strings the process
// was started with in /proc/self/environ, which unsetenv does not touch; when it cannot be read,
// os.Getenv is what there is.
var startEnv = sync.OnceValue(func() func(string) string {
	b, err := os.ReadFile("/proc/self/environ")
	if err != nil {
		return os.Getenv
	}
	env := parseEnviron(b)
	return func(key string) string { return env[key] }
})

// parseEnviron reads the NUL-separated KEY=value entries of /proc/<pid>/environ; the first entry of
// a key wins, as for getenv.
func parseEnviron(b []byte) map[string]string {
	env := map[string]string{}
	for entry := range bytes.SplitSeq(b, []byte{0}) {
		key, value, ok := bytes.Cut(entry, []byte{'='})
		if !ok || len(key) == 0 {
			continue
		}
		if _, seen := env[string(key)]; !seen {
			env[string(key)] = string(value)
		}
	}
	return env
}
