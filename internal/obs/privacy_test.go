package obs_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/obs"
)

// The privacy criterion of gates/OBS1.toml draws the seeds and reads what the
// seeded session wrote; without them the test draws its own and keeps its
// output in a temporary folder.
var (
	seedsFile  = flag.String("obs1-seeds", "", "JSON file of the seeds of TestOBS1P1SeededSessionHoldsNothingPrivate")
	privacyOut = flag.String("obs1-privacy-out", "", "folder the seeded session writes its profile, export and debug snapshot to")
)

// seeds are the private values a session handles and must never store.
type seeds struct {
	Text        string `json:"text"`
	Keystrokes  string `json:"keystrokes"`
	FileName    string `json:"file_name"`
	Path        string `json:"path"`
	DocumentID  string `json:"document_id"`
	PeerID      string `json:"peer_id"`
	ExpectCount uint64 `json:"expect_count"`
}

func loadSeeds(t *testing.T) seeds {
	t.Helper()
	if *seedsFile == "" {
		return seeds{
			Text: "canary " + rand.Text(), Keystrokes: "kqzmwxyvbn", FileName: "Mémoire-" + rand.Text()[:10] + ".typ",
			Path: "chapitres/" + rand.Text()[:10] + "/notes.typ", DocumentID: docA, PeerID: "12D3KooW" + rand.Text() + rand.Text(),
			ExpectCount: 829511,
		}
	}
	b, err := os.ReadFile(*seedsFile)
	if err != nil {
		t.Fatal(err)
	}
	var s seeds
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestOBS1P1SeededSessionHoldsNothingPrivate proves rule P-1: a session that
// handles text, keystrokes, a file name, a path, a document id and a peer id
// stores and exports none of them, raw or hashed, nor the key outside its
// file; a document is keyed by HMAC(key, id)[:8] only.
func TestOBS1P1SeededSessionHoldsNothingPrivate(t *testing.T) {
	seed := loadSeeds(t)
	out := *privacyOut
	if out == "" {
		out = t.TempDir()
	}
	home := filepath.Join(out, "profile")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	export := filepath.Join(out, "export.json")
	s := open(t, home, obs.Options{Choose: func(string) (string, error) { return export, nil }})
	// What a careless or hostile webview could try to slip in is refused.
	for name, x := range map[string]obs.Delta{
		"text as a value":         {M: fails.ID, By: seed.Text, V: 1},
		"keystrokes as a value":   {M: fails.ID, By: seed.Keystrokes, V: 1},
		"file name as a document": {M: words.ID, Doc: seed.FileName, V: 1},
		"path as a document":      {M: words.ID, Doc: seed.Path, V: 1},
		"peer id as a metric":     {M: seed.PeerID, V: 1},
		"text as a metric":        {M: seed.Text, V: 1},
	} {
		if err := s.Add([]obs.Delta{x}); !errors.Is(err, obs.ErrRefused) {
			t.Errorf("Add(%s) = %v, want a refusal", name, err)
		}
	}
	for name, d := range map[string]obs.Descriptor{
		"text as a label": {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: seed.Text, Max: 1},
		"file as a value": {ID: "activity.x", Kind: "counter", Unit: "count", Scope: "app", Label: "stats.x", Max: 1, Values: []string{seed.FileName}},
	} {
		if err := s.Declare([]obs.Descriptor{d}); !errors.Is(err, obs.ErrRefused) {
			t.Errorf("Declare(%s) = %v, want a refusal", name, err)
		}
	}
	half := seed.ExpectCount / 2
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: seed.DocumentID, V: float64(half)},
		obs.Delta{M: active.ID, Doc: seed.DocumentID, V: 61000, N: 2}, obs.Delta{M: opened.ID, V: 1})
	mustFlush(t, s)
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: seed.DocumentID, V: float64(seed.ExpectCount - half)})
	mustFlush(t, s)
	if got, err := s.Export(); err != nil || got != export {
		t.Fatalf("Export() = %q, %v; want %q", got, err, export)
	}
	snap, err := json.Marshal(obs.Network(newSalt(), []obs.LinkState{{Peer: seed.PeerID, Kind: obs.LinkDirect, RTT: 42 * time.Millisecond, Sent: 1 << 20}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "debug.json"), snap, 0o600); err != nil {
		t.Fatal(err)
	}

	key, err := os.ReadFile(filepath.Join(home, "stats", "key"))
	if err != nil || len(key) != 32 {
		t.Fatalf("stats key: %d bytes, %v", len(key), err)
	}
	sum := sha256.Sum256([]byte(seed.DocumentID))
	private := map[string][]byte{
		"text": []byte(seed.Text), "keystrokes": []byte(seed.Keystrokes), "file name": []byte(seed.FileName),
		"path": []byte(seed.Path), "document id": []byte(seed.DocumentID), "peer id": []byte(seed.PeerID),
		"sha256(document id)": sum[:8], "key": key,
	}
	written := append(files(t, home), "../export.json", "../debug.json")
	for _, name := range written {
		data, err := os.ReadFile(filepath.Join(home, name))
		if err != nil {
			t.Fatal(err)
		}
		for what, v := range private {
			if what == "key" && name == "stats/key" {
				continue
			}
			for _, form := range [][]byte{v, []byte(hex.EncodeToString(v)), []byte(base64.StdEncoding.EncodeToString(v)), []byte(base64.RawURLEncoding.EncodeToString(v))} {
				if len(form) >= 6 && bytes.Contains(data, form) {
					t.Errorf("%s holds the %s", name, what)
				}
			}
		}
	}
	var e obs.Export
	b, _ := os.ReadFile(export)
	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	want := pseudonym(key, seed.DocumentID)
	total := uint64(0)
	for _, r := range e.Records {
		if r.K != obs.AppKey && r.K != want {
			t.Errorf("export key %q is not HMAC(key, id)[:8] = %s", r.K, want)
		}
		if r.M == words.ID {
			total += r.Sum
		}
	}
	if total != seed.ExpectCount {
		t.Errorf("the export holds %d words, want the %d of the session", total, seed.ExpectCount)
	}
	if !bytes.Contains(b, []byte(strconv.FormatUint(seed.ExpectCount, 10))) {
		t.Error("the export does not hold the seeded count")
	}
}

func newSalt() []byte {
	salt := make([]byte, 32)
	_, _ = rand.Read(salt)
	return salt
}

// TestOBS1P2StoreOutsideProjects proves rule P-2: the store lives under the
// profile folder and outside every open project; an empty one is refused.
func TestOBS1P2StoreOutsideProjects(t *testing.T) {
	if _, err := obs.Open("", obs.Options{}); !errors.Is(err, obs.ErrNoHome) {
		t.Errorf(`Open("") = %v, want ErrNoHome`, err)
	}
	project := t.TempDir()
	roots := obs.Options{Projects: func() []string { return []string{project} }}
	inside := filepath.Join(project, ".inkwell", "home")
	if err := os.MkdirAll(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "home-link")
	if err := os.Symlink(inside, link); err != nil {
		t.Fatal(err)
	}
	for _, home := range []string{project, inside, link} {
		if s, err := obs.Open(home, roots); !errors.Is(err, obs.ErrInsideProject) {
			if s != nil {
				_ = s.Close()
			}
			t.Errorf("Open(%s) = %v, want ErrInsideProject", home, err)
		}
	}
	home := t.TempDir()
	s := open(t, home, roots)
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 3})
	mustFlush(t, s)
	got := files(t, home)
	if len(got) != 2 || got[0] != "stats/key" || !slices.Contains(got, "stats/v1/"+time.Now().Format("2006-01-02")+".json") {
		t.Errorf("profile holds %v, want the key and today's file under stats/", got)
	}
	if f := files(t, project); len(f) != 0 {
		t.Errorf("the project holds %v, want nothing of the stats", f)
	}
	for _, name := range append(got, "stats", "stats/v1") {
		info, err := os.Stat(filepath.Join(home, name))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("%s is %v, want no access for others", name, perm)
		}
	}
}

// TestOBS1P4ExportOnlyAskedOutsideProjects proves rule P-4: an export writes
// the asked file only, and is refused inside an open project.
func TestOBS1P4ExportOnlyAskedOutsideProjects(t *testing.T) {
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, "chapitres"), 0o700); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	var chosen string // what the person picks in the save dialog
	asked := 0
	s := open(t, home, obs.Options{
		Projects: func() []string { return []string{project} },
		Choose: func(suggested string) (string, error) {
			asked++
			if suggested == "" || filepath.Base(suggested) != suggested {
				t.Errorf("the dialog is offered %q, want a bare file name", suggested)
			}
			return chosen, nil
		},
	})
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 3})
	mustFlush(t, s)
	before := files(t, home)
	elsewhere := t.TempDir()
	link := filepath.Join(t.TempDir(), "into-project")
	if err := os.Symlink(filepath.Join(project, "chapitres"), link); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(project, "stats.json"),
		filepath.Join(project, "chapitres", "stats.json"),
		filepath.Join(link, "stats.json"),
		project,
	} {
		chosen = name
		if _, err := s.Export(); !errors.Is(err, obs.ErrInsideProject) {
			t.Errorf("Export(%s) = %v, want ErrInsideProject", name, err)
		}
	}
	if f := files(t, project); len(f) != 0 {
		t.Errorf("refused exports wrote %v into the project", f)
	}
	// Cancelled: nothing is written, and it is no error.
	chosen = ""
	if got, err := s.Export(); got != "" || err != nil {
		t.Errorf("a cancelled export = %q, %v; want nothing", got, err)
	}
	dest := filepath.Join(elsewhere, "mes-stats.json")
	chosen = dest
	if got, err := s.Export(); err != nil || got != dest {
		t.Fatalf("Export() = %q, %v", got, err)
	}
	if asked != 6 {
		t.Errorf("the dialog was opened %d times, want once per export (6)", asked)
	}
	// Without a dialog there is no export: the path is never someone else's to give.
	bare := open(t, t.TempDir(), obs.Options{})
	if _, err := bare.Export(); !errors.Is(err, obs.ErrNoChooser) {
		t.Errorf("Export without a dialog = %v, want ErrNoChooser", err)
	}
	if got := files(t, elsewhere); !reflect.DeepEqual(got, []string{"mes-stats.json"}) {
		t.Errorf("the export folder holds %v, want only the asked file", got)
	}
	if after := files(t, home); !reflect.DeepEqual(after, before) {
		t.Errorf("an export changed the profile: %v, then %v", before, after)
	}
	var e obs.Export
	b, _ := os.ReadFile(dest)
	if err := json.Unmarshal(b, &e); err != nil || e.V != 1 || len(e.Records) != 1 || e.Records[0].Sum != 3 {
		t.Errorf("export = %s (%v), want version 1 and the one record", b, err)
	}
}

// TestOBS1P5EraseRemovesKey proves rule P-5: erasing removes every stats
// file and the key, so a document's next pseudonym is another.
func TestOBS1P5EraseRemovesKey(t *testing.T) {
	home := t.TempDir()
	s := open(t, home, obs.Options{})
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 3})
	mustFlush(t, s)
	key, _ := os.ReadFile(filepath.Join(home, "stats", "key"))
	if err := s.Erase(); err != nil {
		t.Fatalf("Erase: %v", err)
	}
	if got := files(t, home); len(got) != 0 {
		t.Errorf("after Erase the profile holds %v, want nothing", got)
	}
	if days, _ := s.Days(); len(days) != 1 || len(days[0].Records) != 0 {
		t.Errorf("after Erase Days = %+v, want an empty today", days)
	}
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 1})
	mustFlush(t, s)
	next, _ := os.ReadFile(filepath.Join(home, "stats", "key"))
	if len(next) != 32 || hmac.Equal(next, key) {
		t.Fatal("the key after Erase is the old one")
	}
	d := dayFile(t, home, time.Now().Format("2006-01-02"))
	if _, ok := recordOf(d, words.ID, pseudonym(key, docA), ""); ok {
		t.Error("the old pseudonym of the document is used after Erase")
	}
	if r, ok := recordOf(d, words.ID, pseudonym(next, docA), ""); !ok || r.Sum != 1 {
		t.Errorf("after Erase the day = %+v, want 1 word under the new pseudonym", d)
	}
}

// TestOBS1P6SwitchOffWritesNothing proves rule P-6: switched off, the store
// writes no day file and no key, and its values still read from memory.
func TestOBS1P6SwitchOffWritesNothing(t *testing.T) {
	home := t.TempDir()
	c := &clock{t: time.Date(2026, 11, 25, 23, 59, 0, 0, time.Local)}
	s := open(t, home, obs.Options{Now: c.now})
	if err := s.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, obs.Delta{M: words.ID, Doc: docA, V: 9}, obs.Delta{M: opened.ID, V: 1})
	mustFlush(t, s)
	c.set(c.t.Add(2 * time.Minute)) // the day changes: the old one would be written
	mustAdd(t, s, obs.Delta{M: opened.ID, V: 2})
	mustFlush(t, s)
	if got := files(t, home); !reflect.DeepEqual(got, []string{"stats/settings.json"}) {
		t.Errorf("switched off, the profile holds %v, want only the setting", got)
	}
	days, _ := s.Days()
	if r, ok := recordOf(days[len(days)-1], opened.ID, obs.AppKey, ""); !ok || r.Sum != 2 || s.Enabled() {
		t.Errorf("switched off, today reads %+v (enabled %v), want 2 openings in memory", days, s.Enabled())
	}
	_ = s.Close()
	again := open(t, home, obs.Options{Now: c.now})
	if again.Enabled() {
		t.Error("the setting is not kept: the store is on again")
	}
	if err := again.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	mustAdd(t, again, obs.Delta{M: opened.ID, V: 1})
	mustFlush(t, again)
	if got := files(t, home); len(got) != 3 {
		t.Errorf("switched on again, the profile holds %v, want the setting, the key and today", got)
	}
}

// TestOBS1V3DebugSnapshotRedacted proves rule V-3: the network debug snapshot
// holds only safe fields, and a peer only as its pseudonym for the session.
func TestOBS1V3DebugSnapshotRedacted(t *testing.T) {
	peer := "12D3KooWQyQ3dJ7vbgd2wXfMk2qPqH3qTVzMbWb2ScbLqQw5Wf9a"
	links := []obs.LinkState{
		{Peer: peer, Kind: obs.LinkRelayed, RTT: 180 * time.Millisecond, Downs: 2, Resyncs: 1, LastDown: 3 * time.Second, Sent: 10, Received: 20, Opened: 3},
		{Peer: "12D3KooWother", Kind: "/ip4/192.168.1.7/tcp/4001", Downs: -1},
		{Peer: "12D3KooWthird", Kind: obs.LinkLAN, RTT: -time.Second, Downs: -5},
	}
	salt := newSalt()
	snap := obs.Network(salt, links)
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{peer, "12D3KooW", "192.168", "/ip4/", "tcp"} {
		if bytes.Contains(b, []byte(private)) {
			t.Errorf("snapshot holds %q: %s", private, b)
		}
	}
	if len(snap.Links) != 2 {
		t.Fatalf("snapshot has %d links, want 2 (the one without a known kind dropped): %s", len(snap.Links), b)
	}
	var raw struct{ Links []map[string]any }
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	fields := []string{"downs", "kind", "lastDownMs", "opened", "peer", "received", "resyncs", "rttMs", "sent"}
	for _, l := range raw.Links {
		var keys []string
		for k, v := range l {
			keys = append(keys, k)
			switch k {
			case "peer":
			case "kind":
				if !slices.Contains([]string{"lan", "direct", "relayed"}, v.(string)) {
					t.Errorf("kind %v is not an enum value", v)
				}
			default:
				if n, ok := v.(float64); !ok || n < 0 {
					t.Errorf("%s = %v, want a number >= 0", k, v)
				}
			}
		}
		slices.Sort(keys)
		if !slices.Equal(keys, fields) {
			t.Errorf("link fields %v, want %v", keys, fields)
		}
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write([]byte(peer))
	want := hex.EncodeToString(mac.Sum(nil)[:8])
	if !slices.ContainsFunc(snap.Links, func(l obs.LinkDebug) bool { return l.Peer == want && l.Kind == "relayed" && l.RTTMs == 180 }) {
		t.Errorf("no link has the pseudonym %s of the peer: %+v", want, snap.Links)
	}
	if other := obs.Network(newSalt(), links[:1]); other.Links[0].Peer == want {
		t.Error("the pseudonym is the same under another session's salt")
	}
}
