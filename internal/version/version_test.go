package version

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	buildInfo := &debug.BuildInfo{
		GoVersion: "go1.27.3",
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: "2026-10-25T20:00:00Z"},
		},
	}
	tests := []struct {
		name                  string
		version, commit, date string
		bi                    *debug.BuildInfo
		want                  Info
	}{
		{
			name:    "release build keeps what the linker injected",
			version: "0.1.0-rc.1", commit: "abcdef0123456789", date: "2026-10-26T08:00:00Z",
			bi:   buildInfo,
			want: Info{Version: "0.1.0-rc.1", Commit: "abcdef0123456789", Date: "2026-10-26T08:00:00Z", Go: "go1.27.3"},
		},
		{
			name:    "plain go build falls back to the VCS stamp",
			version: DevVersion,
			bi:      buildInfo,
			want:    Info{Version: DevVersion, Commit: "0123456789abcdef0123456789abcdef01234567", Date: "2026-10-25T20:00:00Z", Go: "go1.27.3"},
		},
		{
			name:    "no build info at all",
			version: DevVersion,
			want:    Info{Version: DevVersion},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolve(tt.version, tt.commit, tt.date, tt.bi); got != tt.want {
				t.Errorf("resolve(%q, %q, %q) = %+v, want %+v", tt.version, tt.commit, tt.date, got, tt.want)
			}
		})
	}
}

func TestInfoString(t *testing.T) {
	tests := []struct {
		info Info
		want string
	}{
		{Info{Version: "0.1.0", Commit: "0123456789abcdef", Date: "2026-10-26T08:00:00Z"}, "inkwell 0.1.0 (0123456, 2026-10-26)"},
		{Info{Version: "0.1.0", Commit: "abc"}, "inkwell 0.1.0 (abc)"},
		{Info{Version: "0.1.0", Date: "2026-10-26T08:00:00Z"}, "inkwell 0.1.0 (2026-10-26)"},
		{Info{Version: "0.1.0"}, "inkwell 0.1.0"},
	}
	for _, tt := range tests {
		if got := tt.info.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.info, got, tt.want)
		}
	}
}

func TestCurrentReportsTheLinkedVersion(t *testing.T) {
	if got := Current().Version; got != Version {
		t.Errorf("Current().Version = %q, want %q", got, Version)
	}
}
