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
