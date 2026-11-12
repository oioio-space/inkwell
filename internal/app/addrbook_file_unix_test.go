//go:build unix

package app

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestAddrBookFileNotRegular: security review 11, an address book that is a FIFO is not read (a
// blocking read would hang the start): Load returns at once with an error.
func TestAddrBookFileNotRegular(t *testing.T) {
	f, home := openTestBook(t)
	if err := syscall.Mkfifo(filepath.Join(home.Name(), addrBookName), 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := f.Load()
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, errNotRegular) {
			t.Errorf("Load(FIFO) = %v, want %v", err, errNotRegular)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Load(FIFO) blocked")
	}
}
