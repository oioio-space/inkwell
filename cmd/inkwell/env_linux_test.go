package main

import (
	"os"
	"testing"
)

// TestStartEnvKeepsActivationToken: the activation token GTK 4 takes out of the environment when
// it loads is read back from the environment the process started with.
func TestStartEnvKeepsActivationToken(t *testing.T) {
	env := parseEnviron([]byte("A=1\x00XDG_ACTIVATION_TOKEN=tok=en\x00\x00=bad\x00noequals\x00A=2\x00EMPTY=\x00"))
	want := map[string]string{"A": "1", "XDG_ACTIVATION_TOKEN": "tok=en", "EMPTY": ""}
	if len(env) != len(want) {
		t.Errorf("parseEnviron = %q, want %q", env, want)
	}
	for k, v := range want {
		if got, ok := env[k]; !ok || got != v {
			t.Errorf("parseEnviron[%q] = %q, %v; want %q", k, got, ok, v)
		}
	}
	// This test binary links GTK 4 (through Wails), whose constructor would have taken the token
	// out: what the process was started with is still read.
	if home := startEnv()("HOME"); home != os.Getenv("HOME") {
		t.Errorf("startEnv HOME = %q, want %q", home, os.Getenv("HOME"))
	}
}
