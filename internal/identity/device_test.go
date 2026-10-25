package identity_test

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oioio-space/inkwell/internal/identity"
)

func newRoot(t *testing.T) (*os.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatalf("os.OpenRoot(%q) = %v", dir, err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, dir
}

func TestLoadOrCreateStable(t *testing.T) {
	root, dir := newRoot(t)

	first, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatalf("LoadOrCreate() first call = %v", err)
	}
	second, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatalf("LoadOrCreate() second call = %v", err)
	}
	if got, want := second.Public(), first.Public(); !bytes.Equal(got, want) {
		t.Errorf("Public() after reload = %x, want %x", got, want)
	}

	info, err := os.Stat(filepath.Join(dir, "device.key"))
	if err != nil {
		t.Fatalf("stat device.key: %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("device.key mode = %v, want %v", got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory has %d entries, want only device.key", len(entries))
	}
}

func TestRejectsLoosePermissions(t *testing.T) {
	for _, mode := range []os.FileMode{0o644, 0o640, 0o604, 0o660} {
		t.Run(mode.String(), func(t *testing.T) {
			root, dir := newRoot(t)
			if _, err := identity.LoadOrCreate(root); err != nil {
				t.Fatalf("LoadOrCreate() = %v", err)
			}
			if err := os.Chmod(filepath.Join(dir, "device.key"), mode); err != nil {
				t.Fatalf("chmod: %v", err)
			}
			if _, err := identity.LoadOrCreate(root); err == nil {
				t.Errorf("LoadOrCreate() with mode %v = nil error, want error", mode)
			}
		})
	}
}

func TestRejectsCorruptKey(t *testing.T) {
	root, dir := newRoot(t)
	if err := os.WriteFile(filepath.Join(dir, "device.key"), []byte("not a pem"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.LoadOrCreate(root); err == nil {
		t.Error("LoadOrCreate() on garbage = nil error, want error")
	}
}

func TestSymlinkEscapeRefused(t *testing.T) {
	root, dir := newRoot(t)
	outside := filepath.Join(t.TempDir(), "outside.key")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "device.key")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := identity.LoadOrCreate(root); err == nil {
		t.Error("LoadOrCreate() through escaping symlink = nil error, want error")
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "secret" {
		t.Errorf("outside file = %q, %v; want untouched", got, err)
	}
}

func TestSignDomainSeparated(t *testing.T) {
	root, _ := newRoot(t)
	dev, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("hello")
	sigA := dev.Sign("trust", msg)

	prefixed := append([]byte("inkwell/trust/v1\x00"), msg...)
	if !ed25519.Verify(dev.Public(), prefixed, sigA) {
		t.Error("Sign(trust) does not verify over the documented prefix")
	}
	if ed25519.Verify(dev.Public(), msg, sigA) {
		t.Error("Sign(trust) verifies over the bare message")
	}
	sigB := dev.Sign("attest", msg)
	if bytes.Equal(sigA, sigB) {
		t.Error("signatures for different domains are equal")
	}
	if ed25519.Verify(dev.Public(), prefixed, sigB) {
		t.Error("attest signature verifies in the trust domain")
	}
}

func TestLibp2pKeyMatchesPublic(t *testing.T) {
	root, _ := newRoot(t)
	dev, err := identity.LoadOrCreate(root)
	if err != nil {
		t.Fatal(err)
	}
	priv := dev.Libp2pKey()
	if got, want := priv.Public().(ed25519.PublicKey), dev.Public(); !bytes.Equal(got, want) {
		t.Errorf("Libp2pKey().Public() = %x, want %x", got, want)
	}
}

func TestMachineName(t *testing.T) {
	got := identity.MachineName()
	if got == "" {
		t.Error("MachineName() is empty")
	}
	if n := len([]rune(got)); n > 40 {
		t.Errorf("MachineName() has %d runes, want <= 40", n)
	}
}

func TestSanitizeMachineName(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "my-laptop", "my-laptop"},
		{"control chars dropped", "a\x00b\nc\x1b[31m", "abc[31m"},
		{"trimmed", "  pc  ", "pc"},
		{"empty falls back", "", "machine"},
		{"only junk falls back", "\x00\x01", "machine"},
		{"truncated to 40 runes", strings.Repeat("é", 60), strings.Repeat("é", 40)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := identity.SanitizeMachineName(tc.in); got != tc.want {
				t.Errorf("SanitizeMachineName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
