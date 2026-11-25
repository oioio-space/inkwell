package obs_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/obs"
)

// docA and docB look like Automerge document ids.
const (
	docA = "3dxpJ7Vv6fHQm2BRq1xUqMr8KiW5"
	docB = "4Qn8XwKz2cUeYr9Lb7Tq5PdHs3Ja"
)

// The webview's metrics of the activity extension, as it declares them.
var (
	words  = obs.Descriptor{ID: "activity.words", Kind: obs.KindVolume, Unit: "words", Scope: obs.ScopeDocument, Label: "stats.words", Max: 1 << 40}
	active = obs.Descriptor{ID: "activity.active", Kind: obs.KindDuration, Unit: "ms", Scope: obs.ScopeDocument, Label: "stats.active", Max: 1 << 40}
	opened = obs.Descriptor{ID: "activity.opened", Kind: obs.KindCounter, Unit: "count", Scope: obs.ScopeApp, Label: "stats.opened", Max: 1000}
	fails  = obs.Descriptor{ID: "preview.failures", Kind: obs.KindCounter, Unit: "count", Scope: obs.ScopeApp, Label: "stats.failures", Max: 1000, Values: []string{"syntax", "timeout"}}
)

// clock is a settable time.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time  { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) set(t time.Time) { c.mu.Lock(); c.t = t; c.mu.Unlock() }

// open opens a store in home with the four metrics declared.
func open(t *testing.T, home string, o obs.Options) *obs.Store {
	t.Helper()
	s, err := obs.Open(home, o)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Declare([]obs.Descriptor{words, active, opened, fails}); err != nil {
		t.Fatalf("Declare: %v", err)
	}
	return s
}

func mustAdd(t *testing.T, s *obs.Store, batch ...obs.Delta) {
	t.Helper()
	if err := s.Add(batch); err != nil {
		t.Fatalf("Add(%+v): %v", batch, err)
	}
}

func mustFlush(t *testing.T, s *obs.Store) {
	t.Helper()
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

// dayFile reads the day file of day under home.
func dayFile(t *testing.T, home, day string) obs.Day {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, "stats", "v1", day+".json"))
	if err != nil {
		t.Fatalf("day file %s: %v", day, err)
	}
	var d obs.Day
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("day file %s is not whole: %v", day, err)
	}
	return d
}

// files lists the files under dir, relative to it.
func files(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err == nil && !e.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(out)
	return out
}

func pseudonym(key []byte, doc string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(doc))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

func recordOf(d obs.Day, m, k, by string) (obs.Record, bool) {
	i := slices.IndexFunc(d.Records, func(r obs.Record) bool { return r.M == m && r.K == k && r.By == by })
	if i < 0 {
		return obs.Record{}, false
	}
	return d.Records[i], true
}

// TestOBS1V2DeclareAndBatchValueRule proves rule V-2: Go refuses a declaration
// or a batch that breaks rule V, whole, and trusts nothing the webview sends.
func TestOBS1V2DeclareAndBatchValueRule(t *testing.T) {
	s := open(t, t.TempDir(), obs.Options{})
	badDecl := map[string]obs.Descriptor{
		"id without extension": {ID: "words", Kind: "volume", Unit: "words", Scope: "app", Label: "stats.words", Max: 1},
		"id with free text":    {ID: "activity.Mots écrits", Kind: "volume", Unit: "words", Scope: "app", Label: "stats.words", Max: 1},
		"unknown kind":         {ID: "activity.x", Kind: "text", Unit: "words", Scope: "app", Label: "stats.x", Max: 1},
		"inconsistent unit":    {ID: "activity.x", Kind: "counter", Unit: "ms", Scope: "app", Label: "stats.x", Max: 1},
		"unknown scope":        {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "peer", Label: "stats.x", Max: 1},
		"free-text label":      {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "Mots écrits aujourd'hui", Max: 1},
		"unbounded":            {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.x"},
		"bound past 2^53":      {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.x", Max: 1 << 54},
		"free-text value":      {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.x", Max: 1, Values: []string{"a file name.typ"}},
		"repeated value":       {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.x", Max: 1, Values: []string{"a", "a"}},
		"taken otherwise":      {ID: words.ID, Kind: "volume", Unit: "bytes", Scope: "document", Label: "stats.words", Max: 1 << 40},
	}
	for name, d := range badDecl {
		// A good declaration with a bad one is refused as a whole.
		fresh := obs.Descriptor{ID: "activity.fresh", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.fresh", Max: 1}
		if err := s.Declare([]obs.Descriptor{fresh, d}); !errors.Is(err, obs.ErrRefused) {
			t.Errorf("Declare(%s) = %v, want a refusal", name, err)
		}
		if slices.ContainsFunc(s.Descriptors(), func(x obs.Descriptor) bool { return x.ID == fresh.ID }) {
			t.Fatalf("Declare(%s) kept the good declaration of a refused batch", name)
		}
	}
	if err := s.Declare(make([]obs.Descriptor, 257)); !errors.Is(err, obs.ErrRefused) {
		t.Errorf("Declare of 257 metrics = %v, want a refusal", err)
	}
	// The bound is on every metric declared, not on each call.
	many := func(from, n int) []obs.Descriptor {
		out := make([]obs.Descriptor, n)
		for i := range out {
			out[i] = obs.Descriptor{ID: fmt.Sprintf("bulk.m%d", from+i), Kind: "counter", Unit: "count", Scope: "app", Label: "metric.x", Max: 1}
		}
		return out
	}
	if err := s.Declare(many(0, 200)); err != nil {
		t.Fatalf("Declare of 200 metrics: %v", err)
	}
	if err := s.Declare(many(200, 100)); !errors.Is(err, obs.ErrRefused) {
		t.Errorf("Declare of 100 more, %d in all = %v, want a refusal", len(s.Descriptors())+100, err)
	}
	if err := s.Declare(many(0, 200)); err != nil {
		t.Errorf("declaring the same 200 again = %v, want nil: they are not new", err)
	}
	if err := s.Declare([]obs.Descriptor{words}); err != nil {
		t.Errorf("declaring a metric again the same way = %v, want nil", err)
	}

	badDelta := map[string]obs.Delta{
		"undeclared metric":    {M: "activity.typed", V: 1},
		"free text as metric":  {M: "canary text", V: 1},
		"NaN":                  {M: opened.ID, V: math.NaN()},
		"infinite":             {M: opened.ID, V: math.Inf(1)},
		"negative":             {M: opened.ID, V: -1},
		"not an integer":       {M: opened.ID, V: 1.5},
		"over its bound":       {M: opened.ID, V: 1001},
		"value not declared":   {M: fails.ID, By: "oom", V: 1},
		"free text as value":   {M: fails.ID, By: "main.typ", V: 1},
		"value on a plain one": {M: opened.ID, By: "syntax", V: 1},
		"document on app":      {M: opened.ID, Doc: docA, V: 1},
		"document missing":     {M: words.ID, V: 1},
		"name as document":     {M: words.ID, Doc: "Mémoire-final.typ", V: 1},
		"path as document":     {M: words.ID, Doc: "chapitres/un/notes.typ", V: 1},
		"measures on counter":  {M: opened.ID, V: 1, N: 1},
		"negative measures":    {M: active.ID, Doc: docA, V: 1, N: -1},
	}
	for name, x := range badDelta {
		good := obs.Delta{M: opened.ID, V: 7}
		if err := s.Add([]obs.Delta{good, x}); !errors.Is(err, obs.ErrRefused) {
			t.Errorf("Add(%s) = %v, want a refusal", name, err)
		}
	}
	if err := s.Add(make([]obs.Delta, 1025)); !errors.Is(err, obs.ErrRefused) {
		t.Errorf("Add of 1025 deltas = %v, want a refusal", err)
	}
	days, err := s.Days()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(days[len(days)-1].Records); n != 0 {
		t.Errorf("refused batches left %d records, want none: %+v", n, days)
	}
	mustAdd(t, s, obs.Delta{M: opened.ID, V: 1000}, obs.Delta{M: fails.ID, By: "timeout", V: 2},
		obs.Delta{M: words.ID, Doc: docA, V: 12}, obs.Delta{M: active.ID, Doc: docA, V: 60000, N: 3})
}

// TestOBS1S1DailyAggregation proves rule S-1: a day sums a counter and a
// volume, sums and counts a duration, per local calendar day.
func TestOBS1S1DailyAggregation(t *testing.T) {
	home := t.TempDir()
	zone := time.FixedZone("UTC+14", 14*3600)
	c := &clock{t: time.Date(2026, 11, 24, 23, 30, 0, 0, zone)} // already the 25th in UTC+14 only
	s := open(t, home, obs.Options{Now: c.now})
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 10}, obs.Delta{M: words.ID, Doc: docB, V: 4},
		obs.Delta{M: opened.ID, V: 1}, obs.Delta{M: active.ID, Doc: docA, V: 1500, N: 2})
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 5}, obs.Delta{M: opened.ID, V: 2},
		obs.Delta{M: active.ID, Doc: docA, V: 500, N: 1}, obs.Delta{M: fails.ID, By: "syntax", V: 1})
	mustFlush(t, s)
	key, err := os.ReadFile(filepath.Join(home, "stats", "key"))
	if err != nil || len(key) != 32 {
		t.Fatalf("stats key: %d bytes, %v", len(key), err)
	}
	d := dayFile(t, home, "2026-11-24")
	want := []obs.Record{
		{M: active.ID, K: pseudonym(key, docA), Sum: 2000, N: 3},
		{M: opened.ID, K: obs.AppKey, Sum: 3},
		{M: fails.ID, K: obs.AppKey, By: "syntax", Sum: 1},
		{M: words.ID, K: pseudonym(key, docA), Sum: 15},
		{M: words.ID, K: pseudonym(key, docB), Sum: 4},
	}
	for _, w := range want {
		if r, ok := recordOf(d, w.M, w.K, w.By); !ok || r != w {
			t.Errorf("record %s/%s/%s = %+v, want %+v", w.M, w.K, w.By, r, w)
		}
	}
	if len(d.Records) != len(want) || d.V != 1 || d.Day != "2026-11-24" {
		t.Errorf("day file = %+v, want version 1, day 2026-11-24 and %d records", d, len(want))
	}

	// The local day changes: the next deltas go to a new file, the old one stays.
	c.set(time.Date(2026, 11, 25, 0, 0, 1, 0, zone))
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 7})
	mustFlush(t, s)
	next := dayFile(t, home, "2026-11-25")
	if r, ok := recordOf(next, words.ID, pseudonym(key, docA), ""); !ok || r.Sum != 7 || len(next.Records) != 1 {
		t.Errorf("the next day = %+v, want only 7 words for docA", next)
	}
	if r, _ := recordOf(dayFile(t, home, "2026-11-24"), words.ID, pseudonym(key, docA), ""); r.Sum != 15 {
		t.Errorf("the previous day changed: %+v", r)
	}
	days, _ := s.Days()
	if len(days) != 2 || days[0].Day != "2026-11-24" || days[1].Day != "2026-11-25" {
		t.Errorf("Days = %+v, want the two days in order", days)
	}

	// The clock goes back across midnight: the day already written is taken back, not replaced.
	c.set(time.Date(2026, 11, 24, 23, 59, 0, 0, zone))
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 1})
	mustFlush(t, s)
	if r, _ := recordOf(dayFile(t, home, "2026-11-24"), words.ID, pseudonym(key, docA), ""); r.Sum != 16 {
		t.Errorf("after the clock went back the day holds %d words, want 15 + 1", r.Sum)
	}
	if r, _ := recordOf(dayFile(t, home, "2026-11-24"), opened.ID, obs.AppKey, ""); r.Sum != 3 {
		t.Errorf("after the clock went back the day lost its openings: %+v", r)
	}

	// A measure longer than one delta's bound is one measure: a 30 h opening, split in two
	// deltas by the webview or by Go's drain, is stored with N = 1.
	c.set(time.Date(2026, 11, 26, 12, 0, 0, 0, zone))
	const thirty = 30 * 3600_000
	day := obs.Descriptor{ID: "activity.open-time", Kind: obs.KindDuration, Unit: "ms", Scope: obs.ScopeDocument, Label: "metric.openTime", Max: 86_400_000}
	if err := s.Declare([]obs.Descriptor{day}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, obs.Delta{M: day.ID, Doc: docA, V: 86_400_000, N: 1}, obs.Delta{M: day.ID, Doc: docA, V: thirty - 86_400_000})
	var m obs.Meter
	goTime := m.Duration(obs.Descriptor{ID: "test.open", Scope: obs.ScopeApp, Label: "metric.openTime", Max: 86_400_000})
	withMeter := open(t, t.TempDir(), obs.Options{Now: c.now, Meter: &m})
	goTime.Observe(thirty * time.Millisecond)
	mustFlush(t, withMeter)
	mustFlush(t, s)
	for _, got := range []struct {
		s    *obs.Store
		m, k string
	}{{s, day.ID, pseudonym(key, docA)}, {withMeter, "test.open", obs.AppKey}} {
		days, _ := got.s.Days()
		if r, _ := recordOf(days[len(days)-1], got.m, got.k, ""); r.Sum != thirty || r.N != 1 {
			t.Errorf("%s: a 30 h measure is stored as %+v, want Sum %d and N 1", got.m, r, thirty)
		}
	}
}

// TestOBS1S2CrashLosesAtMostOneInterval proves rule S-2: what was flushed
// survives a crash, what was not is the only loss, and the file stays whole.
func TestOBS1S2CrashLosesAtMostOneInterval(t *testing.T) {
	home := t.TempDir()
	c := &clock{t: time.Date(2026, 11, 25, 10, 0, 0, 0, time.Local)}
	s, err := obs.Open(home, obs.Options{Now: c.now})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Declare([]obs.Descriptor{words, opened}); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 100}, obs.Delta{M: opened.ID, V: 1})
	mustFlush(t, s)
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 50}) // never flushed: the process dies here
	// A temporary file of a write cut short is beside it, as a kill during WriteAtomic leaves it.
	dir := filepath.Join(home, "stats", "v1")
	if err := os.WriteFile(filepath.Join(dir, "2026-11-25.json.cut.tmp"), []byte(`{"v":1,"day":"2026-11-25","rec`), 0o600); err != nil {
		t.Fatal(err)
	}

	again := open(t, home, obs.Options{Now: c.now})
	d := dayFile(t, home, "2026-11-25")
	key, _ := os.ReadFile(filepath.Join(home, "stats", "key"))
	if r, _ := recordOf(d, words.ID, pseudonym(key, docA), ""); r.Sum != 100 {
		t.Errorf("after the crash the file holds %d words, want the 100 flushed", r.Sum)
	}
	// The next run takes the day back and adds to it.
	mustAdd(t, again, obs.Delta{M: words.ID, Doc: docA, V: 1}, obs.Delta{M: opened.ID, V: 1})
	mustFlush(t, again)
	d = dayFile(t, home, "2026-11-25")
	if r, _ := recordOf(d, words.ID, pseudonym(key, docA), ""); r.Sum != 101 {
		t.Errorf("the next run holds %d words, want 101", r.Sum)
	}
	if r, _ := recordOf(d, opened.ID, obs.AppKey, ""); r.Sum != 2 {
		t.Errorf("the next run holds %d openings, want 2", r.Sum)
	}
}

// TestOBS1S3RetentionAndBound proves rule S-3: days older than 400 are
// removed, and a day holds at most 1024 records and 128 KiB, the excess
// dropped and counted.
func TestOBS1S3RetentionAndBound(t *testing.T) {
	home := t.TempDir()
	c := &clock{t: time.Date(2026, 11, 25, 12, 0, 0, 0, time.Local)}
	dir := filepath.Join(home, "stats", "v1")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	old := map[string]bool{ // day → kept
		c.t.AddDate(0, 0, -401).Format("2006-01-02"): false,
		c.t.AddDate(0, 0, -400).Format("2006-01-02"): true,
		c.t.AddDate(0, 0, -1).Format("2006-01-02"):   true,
	}
	for day := range old {
		if err := os.WriteFile(filepath.Join(dir, day+".json"), []byte(`{"v":1,"day":"`+day+`","records":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s := open(t, home, obs.Options{Now: c.now})
	for day, kept := range old {
		if _, err := os.Stat(filepath.Join(dir, day+".json")); (err == nil) != kept {
			t.Errorf("day %s: present %v, want %v", day, err == nil, kept)
		}
	}
	// At a day change too: tomorrow the day 400 days old goes.
	c.set(c.t.AddDate(0, 0, 1))
	mustAdd(t, s, obs.Delta{M: opened.ID, V: 1})
	if _, err := os.Stat(filepath.Join(dir, c.t.AddDate(0, 0, -401).Format("2006-01-02")+".json")); err == nil {
		t.Error("a day turned 401 days old at a day change is still there")
	}

	// 1500 documents in a day: the records past the bound are dropped and counted, by number with
	// a short metric id, by size with the longest one.
	long := words
	long.ID = "activity." + strings.Repeat("w", 55)
	if err := s.Declare([]obs.Descriptor{long}); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{words.ID, long.ID} {
		c.set(c.t.AddDate(0, 0, 1))
		const docs = 1500
		var batch []obs.Delta
		for i := range docs {
			batch = append(batch, obs.Delta{M: m, Doc: docID(i), V: 1})
			if len(batch) == 1000 || i == docs-1 {
				mustAdd(t, s, batch...)
				batch = nil
			}
		}
		mustFlush(t, s)
		b, err := os.ReadFile(filepath.Join(dir, c.t.Format("2006-01-02")+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var d obs.Day
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		drop, _ := recordOf(d, "obs.dropped", obs.AppKey, "")
		kept := 0
		for _, r := range d.Records {
			if r.M == m {
				kept += int(r.Sum)
			}
		}
		if len(d.Records) > 1024 || len(b) > 128<<10 {
			t.Errorf("%s: day holds %d records in %d bytes, want at most 1024 and 128 KiB", m, len(d.Records), len(b))
		}
		if kept == docs || kept+int(drop.Sum) != docs {
			t.Errorf("%s: kept %d and dropped %d of %d deltas, want some dropped and all accounted for", m, kept, drop.Sum, docs)
		}
	}
}

// docID is the i-th of a series of document ids.
func docID(i int) string {
	const b58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	id := []byte(docA[:20])
	for range 4 {
		id = append(id, b58[i%58])
		i /= 58
	}
	return string(id)
}
