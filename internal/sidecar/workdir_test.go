package sidecar

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMandatoryProjectDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, dir := range []string{"", home, filepath.Dir(home), string(filepath.Separator)} {
		s := spec("echo")
		s.Dir = dir
		if _, err := Run(t.Context(), s); err == nil {
			t.Fatalf("accepted directory %q", dir)
		}
		sup := NewSupervisor(SupervisorConfig{Command: func(context.Context) (*exec.Cmd, error) {
			cmd := helperCmd("echo")
			cmd.Dir = dir
			return cmd, nil
		}})
		if _, err := sup.runOnce(t.Context()); err == nil {
			t.Fatalf("supervisor accepted directory %q", dir)
		}
	}
	project := t.TempDir()
	if _, err := workingDirectory(project); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(home, alias); err == nil {
		if _, err := workingDirectory(alias); err == nil {
			t.Fatal("accepted symlink to home")
		}
	}
}

func TestConfigurationDirectoryAncestorsRefused(t *testing.T) {
	base := t.TempDir()
	config := filepath.Join(base, "not-created", "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	for _, dir := range []string{base} {
		if _, err := workingDirectory(dir); err == nil {
			t.Fatal("accepted configuration ancestor")
		}
	}
}

func TestXDGBaseDirectoriesRefused(t *testing.T) {
	for _, key := range []string{"XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "XDG_CONFIG_HOME"} {
		t.Run(key, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "base")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(key, dir)
			for _, path := range []string{base, dir} {
				if _, err := workingDirectory(path); err == nil {
					t.Fatalf("accepted %s", path)
				}
			}
		})
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workingDirectory(filepath.Join(home, ".cache")); err == nil {
		t.Fatal("accepted default cache")
	}
	for _, dir := range []string{"/etc", "/usr", "/var", "/proc", "/sys", "/dev", "/boot", "/run"} {
		if _, err := workingDirectory(dir); err == nil {
			t.Fatalf("accepted system directory %s", dir)
		}
	}
}

func TestProtectedDescendantsRefused(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, name := range []string{".ssh", ".gnupg", ".config", filepath.Join(".local", "share"), filepath.Join(".local", "state")} {
		dir := filepath.Join(home, name, "nested")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if _, err := workingDirectory(dir); err == nil {
			t.Fatalf("accepted protected descendant %s", dir)
		}
		alias := filepath.Join(t.TempDir(), "alias")
		if err := os.Symlink(dir, alias); err == nil {
			if _, err := workingDirectory(alias); err == nil {
				t.Fatal("accepted protected symlink")
			}
		}
	}
	project := filepath.Join(home, "projects", "safe")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := workingDirectory(project); err != nil {
		t.Fatal(err)
	}
}

// Only secret directories refuse what is inside them: removable drives under
// /run/media, Silverblue homes under /var/home and caches hold user folders.
func TestBroadDirectoriesAllowDescendants(t *testing.T) {
	home := filepath.Join(t.TempDir(), "u")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR", "INKWELL_HOME", "XDG_DATA_DIRS", "XDG_CONFIG_DIRS"} {
		t.Setenv(key, "")
	}
	secret, broad, err := protectedDirs()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path   string
		accept bool
	}{
		{"/run/media/u/x", true},
		{"/var/home/u/proj", true},
		{filepath.Join(home, ".cache", "inkwell", "proj"), true},
		{filepath.Join(home, "Documents", "proj"), true},
		{filepath.Join(home, ".ssh", "x"), false},
		{filepath.Join(home, ".gnupg", "x"), false},
		{filepath.Join(home, ".config", "x"), false},
		{"/proc/self/cwd", false},
		{"/run", false},
		{"/var", false},
		{home, false},
	} {
		if err := checkWorkdir(tc.path, secret, broad); (err == nil) != tc.accept {
			t.Errorf("checkWorkdir(%q) = %v, want accepted %v", tc.path, err, tc.accept)
		}
	}
}
