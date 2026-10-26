// Package version carries the version of the running program: the SemVer
// number, the commit and the build date that the release build injects with
// -ldflags -X (see .goreleaser.yaml), shown by "inkwell --version" and by the
// About dialog.
package version

import (
	"runtime/debug"
	"strings"
)

// DevVersion is what a build made without the release linker flags reports.
const DevVersion = "0.0.0-dev"

// Set by the release build, for example
// -X github.com/oioio-space/inkwell/internal/version.Version=0.1.0-rc.1.
// They are variables, not constants, because the linker can only set those.
var (
	// Version is the SemVer number, without the "v" of the tag.
	Version = DevVersion
	// Commit is the full hash of the commit that was built.
	Commit = ""
	// Date is the commit time, RFC 3339 in UTC.
	Date = ""
)

// shortCommit is how many hexadecimal digits of the commit are displayed.
const shortCommit = 7

// Info describes the running program.
type Info struct {
	// Version is the SemVer number, for example "0.1.0-rc.1".
	Version string
	// Commit is the hash of the commit that was built; empty if unknown.
	Commit string
	// Date is the commit time in RFC 3339; empty if unknown.
	Date string
	// Go is the toolchain that compiled the program; empty if unknown.
	Go string
}

// Current returns the version of the running program.
func Current() Info {
	bi, _ := debug.ReadBuildInfo()
	return resolve(Version, Commit, Date, bi)
}

// resolve completes the injected values with the VCS stamp that "go build"
// leaves in bi, so that a plain build still tells which commit it came from.
func resolve(version, commit, date string, bi *debug.BuildInfo) Info {
	info := Info{Version: version, Commit: commit, Date: date}
	if bi == nil {
		return info
	}
	info.Go = bi.GoVersion
	for _, s := range bi.Settings {
		switch {
		case s.Key == "vcs.revision" && info.Commit == "":
			info.Commit = s.Value
		case s.Key == "vcs.time" && info.Date == "":
			info.Date = s.Value
		}
	}
	return info
}

// String returns the line printed by "inkwell --version", for example
// "inkwell 0.1.0-rc.1 (0123456, 2026-10-26)".
func (i Info) String() string {
	var details []string
	if i.Commit != "" {
		details = append(details, i.Commit[:min(len(i.Commit), shortCommit)])
	}
	if day, _, _ := strings.Cut(i.Date, "T"); day != "" {
		details = append(details, day)
	}
	s := "inkwell " + i.Version
	if len(details) > 0 {
		s += " (" + strings.Join(details, ", ") + ")"
	}
	return s
}
