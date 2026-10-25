// Package identity holds the per-machine identity: an Ed25519 device key
// persisted under an [os.Root], domain-separated signatures, and a short
// human-comparable fingerprint made of four BIP39 words.
//
// The private key never leaves this package except through
// [Device.Libp2pKey], whose only legitimate consumer is the transport.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"
	"unicode"
)

const (
	keyFile = "device.key"
	pemType = "PRIVATE KEY"

	maxMachineNameRunes = 40
	fallbackMachineName = "machine"
	maxKeyFileBytes     = 4 << 10
)

// Device is the identity of this machine. The zero value is unusable; obtain
// one with [LoadOrCreate].
type Device struct {
	priv ed25519.PrivateKey
}

// LoadOrCreate loads the device key from "device.key" in dir, creating it
// (PEM PKCS#8, mode 0o600) when absent. It refuses a key file readable by
// group or others, and any path that escapes dir through a symbolic link.
func LoadOrCreate(dir *os.Root) (*Device, error) {
	d, err := load(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return create(dir)
	}
	return d, err
}

func load(dir *os.Root) (*Device, error) {
	f, err := dir.Open(keyFile)
	if err != nil {
		return nil, fmt.Errorf("identity: open %s: %w", keyFile, err)
	}
	defer func() { _ = f.Close() }() // read-only handle

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("identity: stat %s: %w", keyFile, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("identity: %s is not a regular file", keyFile)
	}
	// Windows has no Unix permission bits: Go reports a synthetic 0o666 (or
	// 0o444) for every file, so the check would always reject a valid key.
	// There the key is protected by the profile directory's ACL instead.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		return nil, fmt.Errorf("identity: %s has mode %04o, want 0600: readable by group or others", keyFile, perm)
	}
	if info.Size() > maxKeyFileBytes {
		return nil, fmt.Errorf("identity: %s is too large", keyFile)
	}

	buf := make([]byte, info.Size())
	if _, err := f.ReadAt(buf, 0); err != nil && info.Size() > 0 {
		return nil, fmt.Errorf("identity: read %s: %w", keyFile, err)
	}
	block, _ := pem.Decode(buf)
	if block == nil || block.Type != pemType {
		return nil, fmt.Errorf("identity: %s is not a PEM %q block", keyFile, pemType)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("identity: parse %s: %w", keyFile, err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("identity: %s holds a %T, want an Ed25519 key", keyFile, key)
	}
	return &Device{priv: priv}, nil
}

func create(dir *os.Root) (*Device, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("identity: generate key: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("identity: marshal key: %w", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der})

	err = install(dir, data)
	if errors.Is(err, fs.ErrExist) {
		// Another process created the key first: its key wins, ours is dropped.
		return load(dir)
	}
	if err != nil {
		return nil, err
	}
	return &Device{priv: priv}, nil
}

// install publishes data as device.key if and only if no such file exists,
// returning an error wrapping [fs.ErrExist] otherwise. The content is written
// and synced to a uniquely named temporary file first, then hard-linked to
// the final name: link(2) fails atomically when the target exists (unlike
// rename, which silently replaces it) and the key becomes visible only when
// complete, so no reader or concurrent creator ever sees a partial or
// overwritten key. The temporary name is removed in all cases.
func install(dir *os.Root, data []byte) (err error) {
	tmp := keyFile + "." + rand.Text() + ".tmp"
	f, err := dir.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("identity: create %s: %w", tmp, err)
	}
	defer func() {
		if rmErr := dir.Remove(tmp); rmErr != nil && err == nil {
			err = fmt.Errorf("identity: remove %s: %w", tmp, rmErr)
		}
	}()
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("identity: write %s: %w", tmp, err)
	}
	if err = dir.Link(tmp, keyFile); err != nil {
		return fmt.Errorf("identity: install %s: %w", keyFile, err)
	}
	return syncDir(dir)
}

// syncDir flushes the directory entry of the new key. Windows cannot fsync a
// directory handle, and NTFS journals metadata, so it is skipped there.
func syncDir(dir *os.Root) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := dir.Open(".")
	if err != nil {
		return fmt.Errorf("identity: open directory: %w", err)
	}
	err = d.Sync()
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("identity: sync directory: %w", err)
	}
	return nil
}

// Public returns the device public key.
func (d *Device) Public() ed25519.PublicKey {
	return d.priv.Public().(ed25519.PublicKey)
}

// Sign signs msg under the domain-separated prefix "inkwell/<domain>/v1\x00",
// so a signature made for one purpose never verifies for another. Verifiers
// must apply the same prefix. domain must not contain '/' or NUL.
func (d *Device) Sign(domain string, msg []byte) []byte {
	prefix := "inkwell/" + domain + "/v1\x00"
	buf := make([]byte, 0, len(prefix)+len(msg))
	buf = append(buf, prefix...)
	buf = append(buf, msg...)
	return ed25519.Sign(d.priv, buf)
}

// Libp2pKey returns a copy of the private key. Its only legitimate use is to
// build the transport node; never log, store or transmit it.
func (d *Device) Libp2pKey() ed25519.PrivateKey {
	return append(ed25519.PrivateKey(nil), d.priv...)
}

// MachineName returns the host name stripped of control and unusual
// characters and truncated to 40 runes. It never returns an empty string.
func MachineName() string {
	host, _ := os.Hostname()
	return sanitizeMachineName(host)
}

func sanitizeMachineName(s string) string {
	if s = CleanName(s); s == "" {
		return fallbackMachineName
	}
	return s
}

// CleanName makes s safe to display as a name: it keeps only letters, digits
// and " -_.[]", which drops control characters, bidirectional overrides and
// isolates, zero-width and other invisible format characters; trims spaces;
// and truncates to 40 runes. The result may be empty.
func CleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(" -_.[]", r) {
			return r
		}
		return -1
	}, s)
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxMachineNameRunes {
		s = strings.TrimSpace(string(r[:maxMachineNameRunes]))
	}
	return s
}
