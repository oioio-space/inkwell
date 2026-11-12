package app

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"syscall"

	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/transport/p2p"
)

// addrBookName is the device's address book (see [p2p.Options.Book]), next to its identity: the
// addresses the other members last announced, so that a restart finds them again. It is
// location data of people (docs/plans/2026-11-12-rc1-reconnect.md §5): private to the user, never
// logged (only counts and failures are).
const addrBookName = "addresses.json"

// addrBookFile keeps the address book in the device's home directory, mode 0600, replaced
// atomically; a book the node refuses is set aside as addresses.json.bad (one slot). It
// implements [p2p.BookStore].
type addrBookFile struct {
	root *os.Root
	log  *slog.Logger
}

var _ p2p.BookStore = (*addrBookFile)(nil)

// openAddrBook opens the address book of the device whose home is home; it holds its own handle
// on the directory, so home may be closed first. The caller closes it once the node is closed.
func openAddrBook(home *os.Root, log *slog.Logger) (*addrBookFile, error) {
	root, err := home.OpenRoot(".")
	if err != nil {
		return nil, fmt.Errorf("opening the address book directory: %w", err)
	}
	return &addrBookFile{root: root, log: log}, nil
}

// errNotRegular reports an address book that is not a regular file (a FIFO, a device): it is
// not read, so that it cannot block the start.
var errNotRegular = errors.New("the address book is not a regular file")

// Load implements [p2p.BookStore]. The file is opened without blocking and read only if it is a
// regular file. A book readable by others is made private again; at most one byte more than
// [p2p.MaxBookSize] is read, so that an oversized book is refused, not loaded.
func (f *addrBookFile) Load() ([]byte, error) {
	file, err := f.root.OpenFile(addrBookName, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening the address book: %w", err)
	}
	defer func() { _ = file.Close() }() // read only: nothing to flush
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("reading the address book: %w", err)
	}
	if !info.Mode().IsRegular() {
		f.log.Warn("address book ignored: not a regular file")
		return nil, errNotRegular
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := file.Chmod(0o600); err != nil {
			f.log.Warn("making the address book private failed", "err", err)
		}
	}
	data, err := io.ReadAll(io.LimitReader(file, p2p.MaxBookSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading the address book: %w", err)
	}
	return data, nil
}

// Save implements [p2p.BookStore].
func (f *addrBookFile) Save(data []byte) error {
	if err := store.WriteAtomic(f.root, addrBookName, data, 0o600); err != nil {
		f.log.Warn("saving the address book failed", "err", err)
		return err
	}
	return nil
}

// Quarantine implements [p2p.BookStore]. reason is a category, never text of the book.
func (f *addrBookFile) Quarantine(reason string) error {
	f.log.Warn("address book refused, set aside", "reason", reason, "file", addrBookName+".bad")
	if err := f.root.Rename(addrBookName, addrBookName+".bad"); err != nil {
		return fmt.Errorf("setting the address book aside: %w", err)
	}
	return store.SyncDir(f.root, ".")
}

// Close releases the directory handle.
func (f *addrBookFile) Close() error { return f.root.Close() }
