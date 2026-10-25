package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUserDirs(t *testing.T) {
	const home = "/home/ada"
	tests := []struct {
		name, content, want string
	}{
		{"home relative", "XDG_DESKTOP_DIR=\"$HOME/Bureau\"\nXDG_DOCUMENTS_DIR=\"$HOME/Documents\"\n", "/home/ada/Documents"},
		{"absolute", "XDG_DOCUMENTS_DIR=\"/data/docs\"\n", "/data/docs"},
		{"comment and spaces", "# c\n  XDG_DOCUMENTS_DIR=\"$HOME/Docs/\"  \n", "/home/ada/Docs"},
		{"relative path refused", "XDG_DOCUMENTS_DIR=\"docs\"\n", ""},
		{"missing", "XDG_MUSIC_DIR=\"$HOME/Music\"\n", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseUserDirs(tt.content, home); got != tt.want {
				t.Errorf("parseUserDirs() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocumentsDirPrecedence(t *testing.T) {
	home, cfg := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_DOCUMENTS_DIR", "")

	if got, err := documentsDir(); err != nil || got != filepath.Join(home, "Documents") {
		t.Errorf("default = %q, %v; want ~/Documents", got, err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "user-dirs.dirs"), []byte("XDG_DOCUMENTS_DIR=\"$HOME/Mes documents\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := documentsDir(); err != nil || got != filepath.Join(home, "Mes documents") {
		t.Errorf("user-dirs.dirs = %q, %v", got, err)
	}
	t.Setenv("XDG_DOCUMENTS_DIR", "/srv/docs")
	if got, err := documentsDir(); err != nil || got != "/srv/docs" {
		t.Errorf("environment = %q, %v", got, err)
	}
}
