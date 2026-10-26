package sidecar

import (
	"strings"
	"testing"
)

// TestArtifactValidateEveryField: a pinned artifact is refused as soon as one
// field is off, each field on its own, and accepted at the edges of the
// allowed ones. A manifest is trusted for what it pins, so each check counts.
func TestArtifactValidateEveryField(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	good := Artifact{
		URL:           "https://" + releaseHost + "/o/r/releases/download/v1/x.tar.xz",
		ArchiveSHA256: hex, SHA256: hex, Member: "bin/x", ArchiveSize: 1, Size: 1,
	}
	if err := good.validate(); err != nil {
		t.Fatalf("validate(good) = %v", err)
	}
	for name, mutate := range map[string]func(*Artifact){
		"http":                func(a *Artifact) { a.URL = strings.Replace(a.URL, "https", "http", 1) },
		"other host":          func(a *Artifact) { a.URL = "https://example.com/x" },
		"no URL":              func(a *Artifact) { a.URL = "" },
		"archive hash short":  func(a *Artifact) { a.ArchiveSHA256 = hex[:63] },
		"archive hash upper":  func(a *Artifact) { a.ArchiveSHA256 = strings.ToUpper(hex) },
		"binary hash short":   func(a *Artifact) { a.SHA256 = hex[:63] },
		"binary hash not hex": func(a *Artifact) { a.SHA256 = strings.Repeat("g", 64) },
		"archive size 0":      func(a *Artifact) { a.ArchiveSize = 0 },
		"archive size < 0":    func(a *Artifact) { a.ArchiveSize = -1 },
		"binary size 0":       func(a *Artifact) { a.Size = 0 },
		"binary size < 0":     func(a *Artifact) { a.Size = -1 },
		"no member":           func(a *Artifact) { a.Member = "" },
		"member escapes":      func(a *Artifact) { a.Member = "../x" },
		"member absolute":     func(a *Artifact) { a.Member = "/etc/passwd" },
	} {
		a := good
		mutate(&a)
		if err := a.validate(); err == nil {
			t.Errorf("validate(%s) = nil, want an error", name)
		}
	}
}
