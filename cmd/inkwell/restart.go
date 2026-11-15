package main

import (
	"os"
	"slices"
	"strings"
)

// restartArgv0 is the program name of a restarted image: fixed, never the one this image was
// given (rule N-12).
const restartArgv0 = "inkwell"

// restartCommand returns the argument list and the environment of the image that restarts on the
// project folder dir: the arguments are rebuilt from nothing, and the environment is environ
// without any afterTakeOverEnv, plus afterTakeOverEnv=1 (rule N-12).
func restartCommand(dir string, environ []string) (argv, env []string) {
	env = slices.DeleteFunc(slices.Clone(environ), func(kv string) bool {
		return strings.HasPrefix(kv, afterTakeOverEnv+"=")
	})
	return []string{restartArgv0, dir}, append(env, afterTakeOverEnv+"=1")
}

// restartMark, in the profile folder, holds the project folder of a change whose restart failed:
// no image got the restart marker, so the next launch that opens that folder says how the change
// went (rule N-12). It lives in the profile, never in the project folder, which other devices see.
const restartMark = "restart-failed"

// leaveRestartMark records, in the profile folder, that the restart on folder failed.
func leaveRestartMark(profile, folder string) error {
	root, err := os.OpenRoot(profile)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.WriteFile(restartMark, []byte(folder), 0o600)
}

// takeRestartMark reports whether the restart of a change on folder failed, and forgets it: true
// once. A mark for another folder stays for the launch that opens that one.
func takeRestartMark(profile, folder string) bool {
	root, err := os.OpenRoot(profile)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	b, err := root.ReadFile(restartMark)
	if err != nil || string(b) != folder {
		return false
	}
	return root.Remove(restartMark) == nil
}
