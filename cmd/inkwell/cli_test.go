package main

import "testing"

func TestVersionRequested(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"--version"}, true},
		{[]string{"-version"}, true},
		{[]string{"version"}, true},
		{[]string{"--version", "extra"}, false},
		{[]string{"--versions"}, false},
		{[]string{"project.typ"}, false},
	}
	for _, tt := range tests {
		if got := versionRequested(tt.args); got != tt.want {
			t.Errorf("versionRequested(%q) = %v, want %v", tt.args, got, tt.want)
		}
	}
}
