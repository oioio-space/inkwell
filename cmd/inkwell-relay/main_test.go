package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

func TestKeyCreatedPrivateAndKept(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.key")
	first, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("loadOrCreateKey(new) = %v", err)
	}
	if len(first) != ed25519.PrivateKeySize {
		t.Fatalf("key is %d bytes, want %d", len(first), ed25519.PrivateKeySize)
	}
	if runtime.GOOS != "windows" { // Windows has no permission bits to check
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("key file mode = %o, want 600", perm)
		}
	}
	second, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("loadOrCreateKey(existing) = %v", err)
	}
	if !first.Equal(second) {
		t.Error("the key changed between runs")
	}
}

func TestKeyRejectsDamagedFile(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"empty":    "",
		"not hex":  "zz\n",
		"too long": strings.Repeat("ab", 40) + "\n",
		"short":    "abcd\n",
	} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		key, err := loadOrCreateKey(path)
		if err == nil {
			t.Errorf("loadOrCreateKey(%s file) = %x, want an error", name, key)
		} else if strings.Contains(err.Error(), "ab") && name == "too long" {
			t.Errorf("error %q repeats the file's content", err)
		}
	}
}

func TestKeyInMissingDirectory(t *testing.T) {
	if _, err := loadOrCreateKey(filepath.Join(t.TempDir(), "no", "such", "relay.key")); err == nil {
		t.Error("loadOrCreateKey in a missing directory succeeded, want an error")
	}
}

// lines sends each line written to it to a channel.
type lines chan string

func (l lines) Write(p []byte) (int, error) {
	sc := bufio.NewScanner(bytes.NewReader(p))
	for sc.Scan() {
		l <- sc.Text()
	}
	return len(p), nil
}

func TestRunPrintsAddresses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "relay.key")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	out := make(lines, 16)
	done := make(chan error, 1)
	args := []string{"--key", path, "--print-addr", "--listen", "/ip4/127.0.0.1/udp/0/quic-v1"}
	go func() { done <- run(ctx, args, out, io.Discard) }()

	var addr string
	select {
	case addr = <-out:
	case err := <-done:
		t.Fatalf("run returned %v before the relay listened", err)
	case <-time.After(10 * time.Second):
		t.Fatal("run printed no address")
	}
	key, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := key.Public().(ed25519.PublicKey)
	if want := "/p2p/" + string(p2p.IDFromKey(pub)); !strings.HasPrefix(addr, "/ip4/127.0.0.1/udp/") || !strings.HasSuffix(addr, want) {
		t.Errorf("printed %q, want a loopback QUIC address ending with %s", addr, want)
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("run after cancel = %v, want nil", err)
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		args []string
	}{
		{"unknown flag", []string{"--bogus"}},
		{"stray argument", []string{"--key", filepath.Join(dir, "k"), "extra"}},
		{"invalid allowed peer", []string{"--key", filepath.Join(dir, "k"), "--allow", "nobody"}},
		{"invalid listen address", []string{"--key", filepath.Join(dir, "k"), "--listen", "nope"}},
		{"missing key directory", []string{"--key", filepath.Join(dir, "no", "k")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := run(t.Context(), tc.args, io.Discard, io.Discard); err == nil {
				t.Errorf("run(%v) succeeded, want an error", tc.args)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	var usage bytes.Buffer
	if err := run(t.Context(), []string{"-h"}, io.Discard, &usage); err != nil {
		t.Errorf("run(-h) = %v, want nil", err)
	}
	for _, flag := range []string{"-key", "-listen", "-print-addr", "-allow"} {
		if !strings.Contains(usage.String(), flag) {
			t.Errorf("usage lacks %s:\n%s", flag, &usage)
		}
	}
}

func TestListFlag(t *testing.T) {
	var l list
	for _, v := range []string{"a,b", " c ", ",,", "d"} {
		if err := l.Set(v); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := l.String(), "a,b,c,d"; got != want {
		t.Errorf("list = %q, want %q", got, want)
	}
}
