package app

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/oioio-space/inkwell/internal/trust"
)

// TODO(Task 9): the store package replaces this file: trust ops move into the
// project directory it manages, written by its atomic key-value store.

const (
	projectsDir = "projects"
	trustDir    = "trust"
	opExt       = ".op"
	// currentFile names the project this device works on: its ID in hexadecimal.
	currentFile = "current"
)

// opStore keeps the ops of one project's member list, one file per op named
// after its hash, in <home>/projects/<ProjectID>/trust.
type opStore struct{ root *os.Root }

// openProject loads the local project of the device whose directory is home,
// or creates it, titled title and signed by s, on the first run. It returns
// the member list, filled with the ops found on disk, and the store that keeps
// it. The caller closes the store.
func openProject(home *os.Root, s trust.Signer, title string) (*trust.List, *opStore, error) {
	if err := home.MkdirAll(projectsDir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("creating projects directory: %w", err)
	}
	id, found, err := currentProject(home)
	if err != nil {
		return nil, nil, err
	}
	var genesis *trust.Op
	if !found {
		g, gid := trust.Genesis(s, title, "")
		id, genesis = gid, &g
	}
	dir := projectsDir + "/" + id.String() + "/" + trustDir
	if err := home.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, fmt.Errorf("creating project directory: %w", err)
	}
	root, err := home.OpenRoot(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("opening project directory: %w", err)
	}
	st := &opStore{root: root}
	list, err := st.load(id, genesis)
	if err != nil {
		return nil, nil, errors.Join(err, root.Close())
	}
	return list, st, nil
}

// currentProject returns the project this device works on: the one it joined
// last, if home remembers it and still has it, otherwise the first project
// directory of home, by name.
func currentProject(home *os.Root) (trust.ProjectID, bool, error) {
	if b, err := readSmall(home, currentFile); err == nil {
		var id trust.ProjectID
		if raw, err := hex.DecodeString(strings.TrimSpace(string(b))); err == nil && len(raw) == len(id) {
			copy(id[:], raw)
			if _, err := home.Stat(projectsDir + "/" + id.String() + "/" + trustDir); err == nil {
				return id, true, nil
			}
		}
	}
	return firstProject(home)
}

// readSmall returns the content of a small file of root.
func readSmall(root *os.Root, name string) ([]byte, error) {
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, 256))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return b, err
}

// adoptProject stores the ops of project id, which this device joined, in
// home and makes it the current project. It returns the store that keeps it;
// the caller closes the store.
func adoptProject(home *os.Root, id trust.ProjectID, ops []trust.Op) (*opStore, error) {
	dir := projectsDir + "/" + id.String() + "/" + trustDir
	if err := home.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating project directory: %w", err)
	}
	root, err := home.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("opening project directory: %w", err)
	}
	st := &opStore{root: root}
	for _, op := range ops {
		if err := st.save(op); err != nil {
			return nil, errors.Join(err, root.Close())
		}
	}
	if err := writeFileAtomic(home, currentFile, []byte(id.String())); err != nil {
		return nil, errors.Join(fmt.Errorf("remembering the current project: %w", err), root.Close())
	}
	return st, nil
}

// firstProject returns the first project directory of home, by name.
func firstProject(home *os.Root) (trust.ProjectID, bool, error) {
	d, err := home.Open(projectsDir)
	if err != nil {
		return trust.ProjectID{}, false, fmt.Errorf("listing projects: %w", err)
	}
	entries, err := d.ReadDir(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return trust.ProjectID{}, false, fmt.Errorf("listing projects: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	for _, n := range names {
		var id trust.ProjectID
		if b, err := hex.DecodeString(n); err == nil && len(b) == len(id) {
			copy(id[:], b)
			return id, true, nil
		}
	}
	return trust.ProjectID{}, false, nil
}

// load builds the list of project id from the stored ops, after saving
// genesis if it is not nil.
func (s *opStore) load(id trust.ProjectID, genesis *trust.Op) (*trust.List, error) {
	if genesis != nil {
		if err := s.save(*genesis); err != nil {
			return nil, err
		}
	}
	d, err := s.root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("reading trust ops: %w", err)
	}
	entries, err := d.ReadDir(-1)
	if cerr := d.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, fmt.Errorf("reading trust ops: %w", err)
	}
	var ops []trust.Op
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), opExt) {
			continue // also skips the temporary files of an interrupted save
		}
		op, err := s.read(e.Name())
		if err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	// Parents first: the Lamport number of an op exceeds those of its parents.
	slices.SortFunc(ops, func(a, b trust.Op) int { return cmp.Compare(a.Lamport, b.Lamport) })
	list := trust.New(id)
	if _, err := list.ApplyAll(ops); err != nil {
		return nil, fmt.Errorf("applying stored trust ops: %w", err)
	}
	if list.Len() == 0 {
		return nil, fmt.Errorf("project %s has no genesis op", id)
	}
	return list, nil
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

// save writes op, which is then found again by the next load.
func (s *opStore) save(op trust.Op) error {
	if err := writeFileAtomic(s.root, op.Hash().String()+opExt, trust.Encode(op)); err != nil {
		return fmt.Errorf("saving trust op: %w", err)
	}
	return nil
}

func (s *opStore) close() error {
	if err := s.root.Close(); !errors.Is(err, os.ErrClosed) {
		return err
	}
	return nil
}

// writeFileAtomic replaces name in root by data, private to the user: a crash
// leaves either the old file or the new one, never a partial one.
func writeFileAtomic(root *os.Root, name string, data []byte) error {
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.Join(err, root.Remove(tmp))
	}
	return root.Rename(tmp, name)
}
