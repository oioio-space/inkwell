package obs

import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/store"
)

// Where the store keeps its files, under INKWELL_HOME (plan §3).
const (
	statsDir     = "stats"
	daysDir      = "stats/v1"
	keyFile      = "stats/key"
	settingsFile = "stats/settings.json"
	keySize      = 32
	dayLayout    = "2006-01-02"
)

// Bounds of a day (rule S-3) and the retention.
const (
	maxRecords = 1024
	maxDaySize = 128 << 10
	// Retention is how many days a day file is kept.
	Retention = 400
	// recordOverhead bounds what a record adds to a day file besides its
	// strings: field names, quotes and two numbers of at most 16 digits.
	recordOverhead = 64
)

// AppKey is the key of an application metric's record.
const AppKey = "app"

// droppedID is the metric that counts the deltas a full day had no room for.
const droppedID = "obs.dropped"

var dropped = Descriptor{ID: droppedID, Kind: KindCounter, Unit: "count", Scope: ScopeApp, Label: "metric.dropped", Max: maxSafe}

var (
	// ErrNoHome is returned by [Open] without a profile folder: the store
	// never falls back to another one.
	ErrNoHome = errors.New("obs: no profile folder")
	// ErrNoChooser is returned by [Store.Export] without a save dialog.
	ErrNoChooser = errors.New("obs: no save dialog to choose where to export")
	// ErrInsideProject is returned for a store or an export inside an open
	// project's folder, which is synchronised with other people.
	ErrInsideProject = errors.New("obs: inside a project folder, which is shared with the project's members")
)

// Record is one metric's sum for one key in one day.
type Record struct {
	// M is the metric's id.
	M string `json:"m"`
	// K is [AppKey] or the pseudonym of a document: HMAC-SHA-256 of its id
	// under the profile's stats key, 8 bytes in hex.
	K string `json:"k"`
	// By is the declared value the sum is split by, if any.
	By  string `json:"by,omitempty"`
	Sum uint64 `json:"sum"`
	// N is the number of measures of a duration.
	N uint64 `json:"n,omitempty"`
}

// Day is the content of one day file.
type Day struct {
	V       int      `json:"v"`
	Day     string   `json:"day"`
	Records []Record `json:"records"`
}

// ExportRecord is one record of an export, with its day.
type ExportRecord struct {
	Day string `json:"day"`
	Record
}

// Export is what [Store.Export] writes: the pseudonymous records of every day.
type Export struct {
	V       int            `json:"v"`
	Records []ExportRecord `json:"records"`
}

// Options tunes [Open]. The zero value is valid.
type Options struct {
	// Now returns the time; nil is time.Now. The day is the local calendar day.
	Now func() time.Time
	// Projects returns the folders of the open projects: neither the store
	// nor an export may be inside one.
	Projects func() []string
	// Meter holds Go's instruments, drained at each flush; nil is none.
	Meter *Meter
	// Choose asks the person, through a native save dialog, where to export,
	// given a suggested file name, and returns the path; "" means they
	// cancelled. Nil: no export (the path is never the webview's to give).
	Choose func(name string) (string, error)
}

// ExportName is the file name an export's dialog suggests.
const ExportName = "statistiques-inkwell.json"

type recordKey struct{ m, k, by string }

// Store is the local daily store of statistics (plan §3). It is safe for
// concurrent use.
type Store struct {
	home   *os.Root
	now    func() time.Time
	roots  func() []string
	meter  *Meter
	choose func(name string) (string, error)

	mu       sync.Mutex
	key      []byte
	keySaved bool
	enabled  bool
	descs    map[string]Descriptor
	day      string
	recs     map[recordKey]*Record
	size     int
	dirty    bool
}

// Open opens the store of the profile folder home (INKWELL_HOME). An empty
// home is [ErrNoHome] and a home inside an open project is
// [ErrInsideProject]. Days older than [Retention] are removed.
func Open(home string, o Options) (*Store, error) {
	if home == "" {
		return nil, ErrNoHome
	}
	s := &Store{now: o.Now, roots: o.Projects, meter: o.Meter, choose: o.Choose, enabled: true}
	if s.now == nil {
		s.now = time.Now
	}
	if s.roots == nil {
		s.roots = func() []string { return nil }
	}
	if inside, err := s.insideProject(home); err != nil || inside {
		return nil, cmp.Or(err, ErrInsideProject)
	}
	r, err := os.OpenRoot(home)
	if err != nil {
		return nil, fmt.Errorf("obs: opening the profile folder: %w", err)
	}
	s.home = r
	s.descs = map[string]Descriptor{droppedID: dropped}
	for _, d := range o.Meter.Descriptors() {
		s.descs[d.ID] = d
	}
	if b, err := r.ReadFile(settingsFile); err == nil {
		var set struct{ Enabled bool }
		if json.Unmarshal(b, &set) == nil {
			s.enabled = set.Enabled
		}
	}
	if b, err := r.ReadFile(keyFile); err == nil && len(b) == keySize {
		s.key, s.keySaved = b, true
	} else {
		s.key = newKey()
	}
	s.startDay(s.today())
	if b, err := r.ReadFile(dayFile(s.day)); err == nil {
		s.load(b)
	}
	s.prune()
	return s, nil
}

// newKey returns a fresh stats key.
func newKey() []byte {
	k := make([]byte, keySize)
	_, _ = rand.Read(k) // never fails (crypto/rand)
	return k
}

// today is the local calendar day of now.
func (s *Store) today() string { return s.now().Format(dayLayout) }

func dayFile(day string) string { return path.Join(daysDir, day+".json") }

func (s *Store) startDay(day string) {
	s.day, s.recs, s.size, s.dirty = day, map[recordKey]*Record{}, 0, false
}

// load takes back the records of today's file, written by an earlier run.
func (s *Store) load(b []byte) {
	var d Day
	if json.Unmarshal(b, &d) != nil || d.V != 1 || d.Day != s.day {
		return
	}
	for _, r := range d.Records {
		if len(s.recs) < maxRecords {
			k := recordKey{r.M, r.K, r.By}
			s.recs[k] = &r
			s.size += recordSize(k)
		}
	}
}

// prune removes the day files older than Retention days.
func (s *Store) prune() {
	entries, err := fs.ReadDir(s.home.FS(), daysDir)
	if err != nil {
		return
	}
	cutoff := s.now().AddDate(0, 0, -Retention).Format(dayLayout)
	for _, e := range entries {
		day, ok := strings.CutSuffix(e.Name(), ".json")
		if _, err := time.Parse(dayLayout, day); ok && err == nil && day < cutoff {
			_ = s.home.Remove(dayFile(day))
		}
	}
}

// roll starts a new day when the local day changed, writing the old one.
func (s *Store) roll() error {
	day := s.today()
	if day == s.day {
		return nil
	}
	err := s.write()
	s.startDay(day)
	// The clock may have gone back across midnight: that day's file is taken back, not replaced.
	if b, rerr := s.home.ReadFile(dayFile(day)); rerr == nil {
		s.load(b)
	}
	s.prune()
	return err
}

// Declare adds the webview's metric declarations, each checked under rule V;
// one bad declaration refuses them all. A metric already declared the same
// way is accepted again; declared otherwise, it is refused.
func (s *Store) Declare(ds []Descriptor) error {
	if len(ds) > maxDescriptors {
		return refuse("%d declarations", len(ds))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fresh := map[string]bool{}
	for _, d := range ds {
		if err := d.Validate(); err != nil {
			return err
		}
		e, ok := s.descs[d.ID]
		if ok && !e.equal(d) {
			return refuse("metric %s already declared otherwise", d.ID)
		}
		fresh[d.ID] = !ok
	}
	added := 0
	for _, isNew := range fresh {
		if isNew {
			added++
		}
	}
	// The bound is on every metric the store holds, not on one call.
	if len(s.descs)+added > maxDescriptors {
		return refuse("%d metrics in all", len(s.descs)+added)
	}
	for _, d := range ds {
		s.descs[d.ID] = d
	}
	return nil
}

// Descriptors returns every declared metric, Go's and the webview's, sorted.
func (s *Store) Descriptors() []Descriptor {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := slices.Collect(func(yield func(Descriptor) bool) {
		for _, d := range s.descs {
			if !yield(d) {
				return
			}
		}
	})
	slices.SortFunc(out, func(a, b Descriptor) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// Add adds a batch of deltas to today, after checking each under rule V
// (a declared metric, a bounded integer, a declared value, a document id
// for a document metric only): one bad delta refuses the batch.
func (s *Store) Add(batch []Delta) error {
	if len(batch) > maxBatch {
		return refuse("batch of %d deltas", len(batch))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range batch {
		d, ok := s.descs[x.M]
		if !ok {
			return refuse("metric %q is not declared", x.M)
		}
		if err := x.check(d); err != nil {
			return err
		}
	}
	err := s.roll()
	for _, x := range batch {
		s.add(x)
	}
	return err
}

// add sums x into today; a new record that the day has no room for is
// dropped and counted.
func (s *Store) add(x Delta) {
	k := recordKey{x.M, AppKey, x.By}
	if x.Doc != "" {
		k.k = pseudonym(s.key, x.Doc) // never the id, never an unkeyed hash of it
	}
	n := uint64(x.N) // explicit: the later parts of a split measure carry none
	r := s.recs[k]
	if r == nil {
		if len(s.recs) >= maxRecords-1 || s.size+recordSize(k) > maxDaySize-recordSize(droppedKey) {
			s.bump(droppedKey, 1, 0)
			return
		}
		r = &Record{M: k.m, K: k.k, By: k.by}
		s.recs[k] = r
		s.size += recordSize(k)
	}
	r.Sum += uint64(x.V)
	r.N += n
	s.dirty = true
}

var droppedKey = recordKey{droppedID, AppKey, ""}

// bump adds to a record that always has room.
func (s *Store) bump(k recordKey, v, n uint64) {
	r := s.recs[k]
	if r == nil {
		r = &Record{M: k.m, K: k.k, By: k.by}
		s.recs[k] = r
		s.size += recordSize(k)
	}
	r.Sum += v
	r.N += n
	s.dirty = true
}

// recordSize bounds the bytes a record takes in a day file.
func recordSize(k recordKey) int { return len(k.m) + len(k.k) + len(k.by) + recordOverhead }

// Flush adds what Go's instruments hold to today and writes today's file if
// it changed and the store is on: called once a minute and on close, so a
// crash loses at most one interval, never a file (store.WriteAtomic).
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.roll()
	for _, x := range s.meter.drain() {
		if d, ok := s.descs[x.M]; ok && x.check(d) == nil {
			s.add(x)
		}
	}
	return errors.Join(err, s.write())
}

// write writes today's file, and the key first if it is not saved yet.
func (s *Store) write() error {
	if !s.enabled || !s.dirty {
		return nil
	}
	if err := s.home.MkdirAll(daysDir, 0o700); err != nil {
		return fmt.Errorf("obs: creating the stats folder: %w", err)
	}
	if !s.keySaved {
		if err := store.WriteAtomic(s.home, keyFile, s.key, 0o600); err != nil {
			return fmt.Errorf("obs: saving the stats key: %w", err)
		}
		s.keySaved = true
	}
	b, err := json.Marshal(Day{V: 1, Day: s.day, Records: s.sorted()})
	if err != nil {
		return err
	}
	if err := store.WriteAtomic(s.home, dayFile(s.day), b, 0o600); err != nil {
		return fmt.Errorf("obs: writing the day: %w", err)
	}
	s.dirty = false
	return nil
}

// sorted returns today's records in a stable order.
func (s *Store) sorted() []Record {
	out := make([]Record, 0, len(s.recs))
	for _, r := range s.recs {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b Record) int {
		return cmp.Or(cmp.Compare(a.M, b.M), cmp.Compare(a.K, b.K), cmp.Compare(a.By, b.By))
	})
	return out
}

// Days returns every stored day, oldest first, today's as it is in memory
// (also when the store is off).
func (s *Store) Days() ([]Day, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Day
	entries, err := fs.ReadDir(s.home.FS(), daysDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		day, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || day == s.day {
			continue
		}
		var d Day
		if b, err := s.home.ReadFile(dayFile(day)); err == nil && json.Unmarshal(b, &d) == nil && d.Day == day {
			out = append(out, d)
		}
	}
	out = append(out, Day{V: 1, Day: s.day, Records: s.sorted()})
	slices.SortFunc(out, func(a, b Day) int { return cmp.Compare(a.Day, b.Day) })
	return out, nil
}

// Export asks the person where to write the pseudonymous records of every
// day (Options.Choose, a native save dialog) and writes them there; it
// returns the path, or "" when they cancelled. A file inside an open project
// is [ErrInsideProject]. Only that file is written.
func (s *Store) Export() (string, error) {
	if s.choose == nil {
		return "", ErrNoChooser
	}
	name, err := s.choose(ExportName)
	if err != nil || name == "" {
		return "", err
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	inside, err := s.insideProject(abs)
	if err != nil || inside {
		return "", cmp.Or(err, ErrInsideProject)
	}
	return abs, s.exportTo(abs)
}

// exportTo writes the export to the absolute path abs.
func (s *Store) exportTo(abs string) error {
	days, err := s.Days()
	if err != nil {
		return err
	}
	e := Export{V: 1, Records: []ExportRecord{}}
	for _, d := range days {
		for _, r := range d.Records {
			e.Records = append(e.Records, ExportRecord{Day: d.Day, Record: r})
		}
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	dir, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return err
	}
	defer func() { _ = dir.Close() }()
	return store.WriteAtomic(dir, filepath.Base(abs), b, 0o600)
}

// insideProject reports whether name, or the folder it would be written in,
// is inside one of the open projects. Links are resolved on both sides.
func (s *Store) insideProject(name string) (bool, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return false, err
	}
	resolved := resolve(abs)
	for _, root := range s.roots() {
		r, err := filepath.Abs(root)
		if err != nil {
			return false, err
		}
		for _, p := range []string{abs, resolved} {
			for _, q := range []string{r, resolve(r)} {
				if rel, err := filepath.Rel(q, p); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// resolve is name with its links resolved, as far as the path exists.
func resolve(name string) string {
	for dir, rest := name, ""; ; {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return name
		}
		dir, rest = parent, filepath.Join(filepath.Base(dir), rest)
	}
}

// Erase removes every stats file and the key: the pseudonyms of the erased
// days can no longer be linked to the next ones. A store switched off stays
// off, and says so in its settings file.
func (s *Store) Erase() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.home.RemoveAll(statsDir); err != nil {
		return fmt.Errorf("obs: erasing: %w", err)
	}
	s.key, s.keySaved = newKey(), false
	s.startDay(s.today())
	if !s.enabled {
		return s.saveSettings()
	}
	return nil
}

// SetEnabled switches the store on or off. Off, no day file and no key is
// written; the values are still summed in memory and read by [Store.Days].
func (s *Store) SetEnabled(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = on
	return s.saveSettings()
}

// Enabled reports whether the store writes its files.
func (s *Store) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

func (s *Store) saveSettings() error {
	if err := s.home.MkdirAll(statsDir, 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{s.enabled})
	return store.WriteAtomic(s.home, settingsFile, b, 0o600)
}

// Close flushes and releases the profile folder.
func (s *Store) Close() error {
	return errors.Join(s.Flush(), s.home.Close())
}
