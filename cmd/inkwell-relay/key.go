package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// loadOrCreateKey returns the relay's identity from the key file at path.
// A missing file is created, readable by its owner only, with a new random
// key: the first run of a relay makes its identity, and every later run keeps
// it, since peers know the relay by its key.
//
// The file holds the 32-byte seed of an Ed25519 key in hex, on one line. It
// is a secret: it is never printed or logged.
func loadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("key directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	for {
		data, err := root.ReadFile(name)
		if err == nil {
			return parseKey(data)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read key: %w", err)
		}
		key, err := createKey(root, name)
		if errors.Is(err, fs.ErrExist) {
			continue // created meanwhile, by another run: read that one
		}
		return key, err
	}
}

// createKey makes a new key file called name in root. It fails with
// [fs.ErrExist] if there is one.
func createKey(root *os.Root, name string) (ed25519.PrivateKey, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("new key: %w", err)
	}
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create key: %w", err)
	}
	_, err = f.WriteString(hex.EncodeToString(seed) + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = root.Remove(name) // do not leave a truncated key behind
		return nil, fmt.Errorf("write key: %w", err)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// parseKey decodes the contents of a key file.
func parseKey(data []byte) (ed25519.PrivateKey, error) {
	seed, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(seed) != ed25519.SeedSize {
		// Say nothing of the contents: they may be a real key, damaged.
		return nil, errors.New("key file is not a 32-byte hex seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}
