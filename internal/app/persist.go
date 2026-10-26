package app

import (
	"cmp"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/store"
	"github.com/oioio-space/inkwell/internal/trust"
)

const (
	trustDir = "trust"
	opExt    = ".op"
	rcvExt   = ".rcv" // when this device received the op of the same name.
	tmpExt   = ".tmp"
)

// opStore keeps the ops of one project's member list, one file per op named
// after its hash, in the project's .inkwell/trust directory. Files are
// written with [store.WriteAtomic].
type opStore struct{ root *os.Root }

// openTrust loads the member list of project p, or creates it, titled like p
// and signed by s, on the first run. It returns the list, filled with the ops
// found on disk, and the store that keeps it. The caller closes the store.
func openTrust(p *project.Project, s trust.Signer) (*trust.List, *opStore, error) {
	root, err := p.OpenDir(trustDir)
	if err != nil {
		return nil, nil, err
	}
	st := &opStore{root: root}
	list, err := st.load(s, p.Meta().Title)
	if err != nil {
		return nil, nil, errors.Join(err, root.Close())
	}
	return list, st, nil
}

// load builds the member list from the stored ops. The project is the one
// whose genesis op is stored; if there is none, load makes one, signed by
// signer and titled title, and saves it.
func (s *opStore) load(signer trust.Signer, title string) (*trust.List, error) {
	ops, err := s.readAll()
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(ops, func(o trust.Op) bool { return o.Kind == trust.OpGenesis })
	if i < 0 {
		g, _ := trust.Genesis(signer, title, "")
		if err := s.save(g); err != nil {
			return nil, err
		}
		ops, i = append(ops, g), len(ops)
	}
	list := trust.New(trust.ProjectID(ops[i].Hash()))
	// Parents first: the Lamport number of an op exceeds those of its parents.
	slices.SortStableFunc(ops, func(a, b trust.Op) int { return cmp.Compare(a.Lamport, b.Lamport) })
	if _, err := list.ApplyAll(ops); err != nil {
		return nil, fmt.Errorf("applying stored trust ops: %w", err)
	}
	return list, nil
}

// readAll returns the stored ops, sorted by hash so that the genesis op that
// load picks does not depend on the file system. Temporary files left by an
// interrupted save are removed.
func (s *opStore) readAll() ([]trust.Op, error) {
	entries, err := s.entries()
	if err != nil {
		return nil, err
	}
	var ops []trust.Op
	for _, e := range entries {
		switch {
		case e.IsDir():
		case strings.HasSuffix(e.Name(), tmpExt):
			if err := s.root.Remove(e.Name()); err != nil {
				return nil, fmt.Errorf("removing %s: %w", e.Name(), err)
			}
		case strings.HasSuffix(e.Name(), opExt):
			op, err := s.read(e.Name())
			if err != nil {
				return nil, err
			}
			ops = append(ops, op)
		}
	}
	slices.SortFunc(ops, func(a, b trust.Op) int { return cmp.Compare(a.Hash().String(), b.Hash().String()) })
	return ops, nil
}

// entries lists the trust directory.
func (s *opStore) entries() ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(s.root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("listing trust ops: %w", err)
	}
	return entries, nil
}

func (s *opStore) read(name string) (trust.Op, error) {
	f, err := s.root.Open(name)
	if err != nil {
		return trust.Op{}, fmt.Errorf("reading trust op: %w", err)
	}
	b, err := io.ReadAll(io.LimitReader(f, trust.MaxOpSize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return trust.Op{}, fmt.Errorf("reading trust op %s: %w", name, err)
	}
	op, err := trust.Decode(b)
	if err != nil {
		return trust.Op{}, fmt.Errorf("decoding trust op %s: %w", name, err)
	}
	return op, nil
}

// replace makes ops, the member list of the project the device joins, the
// saved ones in place of those of the project it leaves. ops come parents
// first, as [trust.List.Since] returns them.
func (s *opStore) replace(ops []trust.Op) error {
	if err := s.clear(); err != nil {
		return err
	}
	for _, op := range ops {
		if err := s.save(op); err != nil {
			return err
		}
	}
	return nil
}

// clear removes every saved op.
func (s *opStore) clear() error {
	entries, err := s.entries()
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), opExt) && !strings.HasSuffix(e.Name(), rcvExt) && !strings.HasSuffix(e.Name(), tmpExt)) {
			continue
		}
		if err := s.root.Remove(e.Name()); err != nil {
			return fmt.Errorf("removing %s: %w", e.Name(), err)
		}
	}
	return nil
}

// save writes op, which is then found again by the next load, and notes when
// it was received if that was not noted already.
func (s *opStore) save(op trust.Op) error {
	if err := store.WriteAtomic(s.root, op.Hash().String()+opExt, trust.Encode(op), 0o600); err != nil {
		return fmt.Errorf("saving trust op: %w", err)
	}
	name := op.Hash().String() + rcvExt
	if _, err := s.root.Stat(name); err == nil {
		return nil
	}
	stamp := binary.BigEndian.AppendUint64(nil, uint64(time.Now().UnixNano())) // #nosec G115 -- the clock is after 1970.
	if err := store.WriteAtomic(s.root, name, stamp, 0o600); err != nil {
		return fmt.Errorf("noting the arrival of a trust op: %w", err)
	}
	return nil
}

// receivedAt returns when this device received the op with hash h: the time
// noted when it was saved, which ops do not carry (they have no clock). It is
// the zero time if the op was not saved here.
func (s *opStore) receivedAt(h trust.Hash) time.Time {
	b, err := s.root.ReadFile(h.String() + rcvExt)
	if err != nil || len(b) != 8 {
		return time.Time{}
	}
	return time.Unix(0, int64(binary.BigEndian.Uint64(b))) // #nosec G115 -- written from an int64.
}

func (s *opStore) close() error {
	if err := s.root.Close(); !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}
