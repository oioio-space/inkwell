package sidecar

import (
	"encoding/json"
	"io/fs"
	"net/url"
	"strings"
	"testing"
)

func TestManifest(t *testing.T) {
	m, err := LoadManifest()
	if err != nil {
		t.Fatalf("LoadManifest() = %v", err)
	}

	t.Run("SIDE-01 every target pins both sidecars", func(t *testing.T) {
		wantVersion := map[string]string{Typst: "0.15.1", Tinymist: "0.15.10"}
		targets := []string{"linux_amd64", "linux_arm64", "darwin_amd64", "darwin_arm64"}
		for name, version := range wantVersion {
			sc, ok := m.Sidecars[name]
			if !ok {
				t.Fatalf("manifest has no %q", name)
			}
			if sc.Version != version {
				t.Errorf("%s version = %q, want %q", name, sc.Version, version)
			}
			if sc.License != "Apache-2.0" {
				t.Errorf("%s license = %q, want Apache-2.0", name, sc.License)
			}
			for _, target := range targets {
				art, ok := sc.Targets[target]
				if !ok {
					t.Errorf("%s has no target %s", name, target)
					continue
				}
				u, err := url.Parse(art.URL)
				if err != nil || u.Scheme != "https" || u.Host != "github.com" {
					t.Errorf("%s/%s url = %q, want https on github.com", name, target, art.URL)
				}
				if !strings.Contains(art.URL, "/v"+version+"/") {
					t.Errorf("%s/%s url %q does not name version %s", name, target, art.URL, version)
				}
			}
		}
	})

	t.Run("Artifact picks one target and names the missing ones", func(t *testing.T) {
		sc, art, err := m.Artifact(Typst, "linux_amd64")
		if err != nil {
			t.Fatalf("Artifact(typst, linux_amd64) = %v", err)
		}
		if sc.Version != "0.15.1" || art.SHA256 == "" {
			t.Errorf("Artifact = %+v %+v, want the pinned typst", sc, art)
		}
		if _, _, err := m.Artifact(Typst, "plan9_mips"); err == nil || !strings.Contains(err.Error(), "plan9_mips") {
			t.Errorf("Artifact(typst, plan9_mips) = %v, want ErrUnsupported naming the target", err)
		}
		if _, _, err := m.Artifact("nope", "linux_amd64"); err == nil {
			t.Error("Artifact(nope, linux_amd64) = nil, want an error")
		}
	})
}

func TestParseManifestRejects(t *testing.T) {
	good, err := LoadManifest()
	if err != nil {
		t.Fatal(err)
	}
	mutate := func(f func(m *Manifest)) []byte {
		raw, err := json.Marshal(good)
		if err != nil {
			t.Fatal(err)
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		f(&m)
		out, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	art := func(m *Manifest) *Artifact {
		a := m.Sidecars[Typst].Targets["linux_amd64"]
		return &a
	}
	set := func(m *Manifest, a *Artifact) { m.Sidecars[Typst].Targets["linux_amd64"] = *a }

	tests := []struct {
		name string
		data []byte
	}{
		{"not json", []byte("{")},
		{"unknown field", []byte(`{"schema":1,"sidecars":{},"extra":true}`)},
		{"wrong schema", mutate(func(m *Manifest) { m.Schema = 2 })},
		{"no sidecars", []byte(`{"schema":1,"sidecars":{}}`)},
		{"short sha256", mutate(func(m *Manifest) { a := art(m); a.SHA256 = "abc"; set(m, a) })},
		{"upper-case sha256", mutate(func(m *Manifest) {
			a := art(m)
			a.SHA256 = strings.ToUpper(a.SHA256)
			set(m, a)
		})},
		{"http url", mutate(func(m *Manifest) {
			a := art(m)
			a.URL = strings.Replace(a.URL, "https", "http", 1)
			set(m, a)
		})},
		{"foreign host", mutate(func(m *Manifest) { a := art(m); a.URL = "https://evil.example/x.tar.xz"; set(m, a) })},
		{"traversing member", mutate(func(m *Manifest) { a := art(m); a.Member = "../typst"; set(m, a) })},
		{"absolute member", mutate(func(m *Manifest) { a := art(m); a.Member = "/typst"; set(m, a) })},
		{"zero size", mutate(func(m *Manifest) { a := art(m); a.Size = 0; set(m, a) })},
		{"bad sidecar name", mutate(func(m *Manifest) {
			m.Sidecars["../x"] = m.Sidecars[Typst]
		})},
		{"trailing data", append(mutate(func(*Manifest) {}), []byte(`{}`)...)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseManifest(tt.data); err == nil {
				t.Errorf("ParseManifest(%s) = nil, want an error", tt.name)
			}
		})
	}
}

func TestLicenses(t *testing.T) {
	for name, want := range map[string]string{
		"typst-LICENSE":    "Apache License",
		"typst-NOTICE":     "third party components",
		"tinymist-LICENSE": "Apache License",
	} {
		data, err := fs.ReadFile(Licenses(), name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !strings.Contains(string(data), want) {
			t.Errorf("%s does not mention %q", name, want)
		}
	}
}
