package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func openKV(t *testing.T) (*KV, *os.Root) {
	t.Helper()
	r, _ := openTemp(t)
	kv, err := Open(r)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return kv, r
}

func mustSave(t *testing.T, kv *KV, key []string, data string) {
	t.Helper()
	if err := kv.Save(key, []byte(data)); err != nil {
		t.Fatalf("Save(%v): %v", key, err)
	}
}

func TestSaveLoadRemove(t *testing.T) {
	kv, _ := openKV(t)
	key := []string{"doc1", "snapshot", "abc"}

	if _, err := kv.Load(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(missing) error = %v, want ErrNotFound", err)
	}
	mustSave(t, kv, key, "hello")
	got, err := kv.Load(key)
	if err != nil || string(got) != "hello" {
		t.Fatalf("Load = %q, %v; want hello", got, err)
	}
	mustSave(t, kv, key, "")
	if got, err := kv.Load(key); err != nil || len(got) != 0 {
		t.Fatalf("Load(empty value) = %q, %v", got, err)
	}
	if err := kv.Remove(key); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := kv.Remove(key); err != nil {
		t.Fatalf("Remove(absent) = %v, want nil (idempotent)", err)
	}
	if _, err := kv.Load(key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(removed) error = %v, want ErrNotFound", err)
	}
}

// TestKeyAndKeyPrefixCoexist: a key may be a prefix of another, as automerge-repo's
// [id] and [id, "sync-state", s] are.
func TestKeyAndKeyPrefixCoexist(t *testing.T) {
	kv, _ := openKV(t)
	mustSave(t, kv, []string{"a"}, "short")
	mustSave(t, kv, []string{"a", "b"}, "long")
	for key, want := range map[string]string{"a": "short", "a/b": "long"} {
		got, err := kv.Load(strings.Split(key, "/"))
		if err != nil || string(got) != want {
			t.Errorf("Load(%s) = %q, %v; want %q", key, got, err, want)
		}
	}
}

func TestLoadRangeAndRemoveRange(t *testing.T) {
	kv, _ := openKV(t)
	mustSave(t, kv, []string{"d1", "snapshot", "h1"}, "s1")
	mustSave(t, kv, []string{"d1", "incremental", sum("i1")}, "i1")
	mustSave(t, kv, []string{"d1", "incremental", sum("i2")}, "i2")
	mustSave(t, kv, []string{"d2", "snapshot", "h2"}, "s2")
	mustSave(t, kv, []string{"d1"}, "whole")

	got, err := kv.LoadRange([]string{"d1", "incremental"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("LoadRange(d1/incremental) = %d entries, want 2", len(got))
	}
	for _, e := range got {
		if len(e.Key) != 3 || e.Key[1] != "incremental" || e.Key[2] != sum(string(e.Data)) {
			t.Errorf("entry %v / %q is not an incremental chunk", e.Key, e.Data)
		}
	}
	all, err := kv.LoadRange([]string{"d1"})
	if err != nil || len(all) != 4 {
		t.Fatalf("LoadRange(d1) = %d entries, %v; want 4 (the key itself and 3 below)", len(all), err)
	}
	if everything, err := kv.LoadRange(nil); err != nil || len(everything) != 5 {
		t.Fatalf("LoadRange(nil) = %d entries, %v; want 5", len(everything), err)
	}
	if none, err := kv.LoadRange([]string{"zz"}); err != nil || len(none) != 0 {
		t.Fatalf("LoadRange(unknown) = %v, %v; want empty", none, err)
	}

	if err := kv.RemoveRange([]string{"d1", "incremental"}); err != nil {
		t.Fatal(err)
	}
	if left, _ := kv.LoadRange([]string{"d1"}); len(left) != 2 {
		t.Errorf("after RemoveRange, d1 has %d entries, want 2", len(left))
	}
	if err := kv.RemoveRange([]string{"d1", "nothing"}); err != nil {
		t.Errorf("RemoveRange(absent) = %v, want nil", err)
	}
	if err := kv.RemoveRange(nil); !errors.Is(err, ErrBadKey) {
		t.Errorf("RemoveRange(nil) = %v, want ErrBadKey", err)
	}
}

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestKeyComponentRejected(t *testing.T) {
	kv, r := openKV(t)
	long := strings.Repeat("a", 129)
	bad := map[string][]string{
		"empty key":       {},
		"empty component": {"a", ""},
		"dot":             {"."},
		"dotdot":          {"a", ".."},
		"slash":           {"a/b"},
		"backslash":       {`a\b`},
		"traversal":       {"../x"},
		"nul":             {"a\x00b"},
		"space":           {"a b"},
		"tilde":           {"a~"},
		"non ascii":       {"é"},
		"too long":        {long},
		"too deep":        {"a", "b", "c", "d", "e"},
	}
	for name, key := range bad {
		if err := kv.Save(key, []byte("x")); !errors.Is(err, ErrBadKey) {
			t.Errorf("%s: Save(%q) = %v, want ErrBadKey", name, key, err)
		}
		if _, err := kv.Load(key); !errors.Is(err, ErrBadKey) {
			t.Errorf("%s: Load(%q) = %v, want ErrBadKey", name, key, err)
		}
		if err := kv.Remove(key); !errors.Is(err, ErrBadKey) {
			t.Errorf("%s: Remove(%q) = %v, want ErrBadKey", name, key, err)
		}
	}
	if err := kv.Save([]string{strings.Repeat("a", 128), "b", "c", "d"}, nil); err != nil {
		t.Errorf("Save(maximal valid key) = %v", err)
	}
	// Nothing was created outside repo/.
	entries, _ := fs.ReadDir(r.FS(), ".")
	for _, e := range entries {
		if e.Name() != repoDir && e.Name() != quarantineDir {
			t.Errorf("unexpected entry %q after rejected keys", e.Name())
		}
	}
}

func TestValueTooLarge(t *testing.T) {
	kv, _ := openKV(t)
	if err := kv.Save([]string{"k"}, make([]byte, MaxValue+1)); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Save(MaxValue+1 bytes) = %v, want ErrTooLarge", err)
	}
}

func TestIncrementalKeyMustHashData(t *testing.T) {
	kv, _ := openKV(t)
	good := []string{"doc", "incremental", sum("data")}
	if err := kv.Save(good, []byte("data")); err != nil {
		t.Fatalf("Save(matching hash) = %v", err)
	}
	if err := kv.Save([]string{"doc", "incremental", sum("other")}, []byte("data")); !errors.Is(err, ErrKeyHash) {
		t.Errorf("Save(mismatching hash) = %v, want ErrKeyHash", err)
	}
	if err := kv.Save([]string{"doc", "incremental", "xyz"}, []byte("data")); !errors.Is(err, ErrKeyHash) {
		t.Errorf("Save(non-hash key) = %v, want ErrKeyHash", err)
	}
	// Snapshots are keyed by heads, not by content: any name is fine.
	if err := kv.Save([]string{"doc", "snapshot", "heads"}, []byte("data")); err != nil {
		t.Errorf("Save(snapshot) = %v", err)
	}
}

// flipBit flips one bit of the stored file of key.
func flipBit(t *testing.T, r *os.Root, key []string, offset int) {
	t.Helper()
	name := filePath(key)
	b, err := r.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	b[offset] ^= 1
	if err := r.WriteFile(name, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func quarantined(t *testing.T, r *os.Root) int {
	t.Helper()
	entries, err := fs.ReadDir(r.FS(), quarantineDir)
	if err != nil {
		return 0
	}
	return len(entries)
}

func TestBitFlipQuarantined(t *testing.T) {
	for name, offset := range map[string]int{"in data": headerLen + 2, "in checksum": 10, "in magic": 0} {
		t.Run(name, func(t *testing.T) {
			kv, r := openKV(t)
			key := []string{"doc", "snapshot", "h"}
			mustSave(t, kv, key, "precious text")
			flipBit(t, r, key, offset)

			_, err := kv.Load(key)
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("Load(corrupted) = %v, want ErrCorrupt", err)
			}
			if n := quarantined(t, r); n != 1 {
				t.Errorf("%d files in quarantine, want 1", n)
			}
			if _, err := kv.Load(key); !errors.Is(err, ErrNotFound) {
				t.Errorf("Load after quarantine = %v, want ErrNotFound (the bad file is gone)", err)
			}
		})
	}
}

func TestTruncatedValueIsCorrupt(t *testing.T) {
	kv, r := openKV(t)
	key := []string{"k"}
	mustSave(t, kv, key, "some value")
	b, _ := r.ReadFile(filePath(key))
	for _, n := range []int{0, 3, headerLen, len(b) - 1} {
		if err := r.WriteFile(filePath(key), b[:n], 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := kv.Load(key); !errors.Is(err, ErrCorrupt) {
			t.Errorf("Load(truncated to %d) = %v, want ErrCorrupt", n, err)
		}
	}
}

func TestCorruptFallsBackToLatestBackup(t *testing.T) {
	kv, r := openKV(t)
	key := []string{"doc", "snapshot", "h"}
	mustSave(t, kv, key, "v1")
	t0 := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if err := kv.Backup(t0, 10); err != nil {
		t.Fatal(err)
	}
	mustSave(t, kv, key, "v2")
	if err := kv.Backup(t0.Add(time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	mustSave(t, kv, key, "v3")
	flipBit(t, r, key, headerLen)

	got, err := kv.Load(key)
	if err != nil || string(got) != "v2" {
		t.Fatalf("Load(corrupted with backups) = %q, %v; want the latest backup v2", got, err)
	}
	if n := quarantined(t, r); n != 1 {
		t.Errorf("%d files in quarantine, want 1", n)
	}
	if again, err := kv.Load(key); err != nil || string(again) != "v2" {
		t.Errorf("restored value not persisted: %q, %v", again, err)
	}
}

func TestCorruptBackupSkipped(t *testing.T) {
	kv, r := openKV(t)
	key := []string{"k"}
	mustSave(t, kv, key, "v1")
	t0 := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if err := kv.Backup(t0, 10); err != nil {
		t.Fatal(err)
	}
	mustSave(t, kv, key, "v2")
	if err := kv.Backup(t0.Add(time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	// Damage the newest backup and the live file: the older backup still serves.
	newest := "backups/" + t0.Add(time.Hour).Format(backupLayout) + "/" + strings.TrimPrefix(filePath(key), repoDir+"/")
	b, err := r.ReadFile(newest)
	if err != nil {
		t.Fatal(err)
	}
	b[headerLen] ^= 1
	if err := WriteAtomic(r, newest, b, 0o600); err != nil {
		t.Fatal(err)
	}
	flipBit(t, r, key, headerLen)

	got, err := kv.Load(key)
	if err != nil || string(got) != "v1" {
		t.Fatalf("Load = %q, %v; want v1 from the older backup", got, err)
	}
}

func TestLoadRangeSkipsCorrupt(t *testing.T) {
	kv, r := openKV(t)
	mustSave(t, kv, []string{"d", "snapshot", "a"}, "ok")
	mustSave(t, kv, []string{"d", "snapshot", "b"}, "bad")
	flipBit(t, r, []string{"d", "snapshot", "b"}, headerLen)

	got, err := kv.LoadRange([]string{"d", "snapshot"})
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("LoadRange error = %v, want it to report ErrCorrupt", err)
	}
	if len(got) != 1 || string(got[0].Data) != "ok" {
		t.Errorf("LoadRange = %v, want only the intact entry", got)
	}
	if quarantined(t, r) != 1 {
		t.Error("corrupt entry not quarantined")
	}
}

func TestLoadRangeIgnoresStrayFiles(t *testing.T) {
	kv, r := openKV(t)
	mustSave(t, kv, []string{"d", "k"}, "v")
	for _, name := range []string{"repo/d/k~.deadbeef.tmp", "repo/d/stray", "repo/d/bad name~"} {
		if err := r.WriteFile(name, []byte("junk"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := kv.LoadRange([]string{"d"})
	if err != nil || len(got) != 1 {
		t.Fatalf("LoadRange = %v, %v; want the single real entry", got, err)
	}
}

func TestOpenSweepsInterruptedWrites(t *testing.T) {
	r, _ := openTemp(t)
	if err := r.MkdirAll("repo/d", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile("repo/d/k~.x.tmp", []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(r); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Stat("repo/d/k~.x.tmp"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("temporary file survived Open: %v", err)
	}
}

// TestSymlinkEscape: a link inside the store pointing outside it is never
// followed for reads, writes or listings.
func TestSymlinkEscape(t *testing.T) {
	kv, r := openKV(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "k~")
	if err := os.WriteFile(secret, []byte("IKV1"+strings.Repeat("x", 40)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.Symlink(outside, "repo/evil"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := kv.Save([]string{"evil", "k"}, []byte("pwned")); err == nil {
		t.Error("Save through an escaping symlink succeeded")
	}
	if _, err := kv.Load([]string{"evil", "k"}); err == nil {
		t.Error("Load through an escaping symlink succeeded")
	}
	got, _ := kv.LoadRange([]string{"evil"})
	if len(got) != 0 {
		t.Errorf("LoadRange followed the link: %v", got)
	}
	if err := kv.RemoveRange([]string{"evil"}); err != nil {
		t.Logf("RemoveRange(evil) = %v", err)
	}
	b, err := os.ReadFile(secret)
	if err != nil || !bytes.HasPrefix(b, []byte("IKV1")) {
		t.Errorf("file outside the store was modified: %q, %v", b, err)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 1 {
		t.Errorf("outside directory has %d entries, want 1", len(entries))
	}
}

func TestBackupHardLinksAndPrunes(t *testing.T) {
	kv, r := openKV(t)
	key := []string{"d", "snapshot", "h"}
	mustSave(t, kv, key, "v1")
	t0 := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	if err := kv.Backup(t0, 3); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	dir := "backups/" + t0.Format(backupLayout)
	live, err := r.Stat(filePath(key))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := r.Stat(dir + "/d/snapshot/h~")
	if err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	if !os.SameFile(live, saved) {
		t.Error("backup is a copy, want a hard link")
	}
	// An atomic rewrite leaves the backup with the old content.
	mustSave(t, kv, key, "v2")
	if b, err := r.ReadFile(dir + "/d/snapshot/h~"); err != nil || !bytes.HasSuffix(b, []byte("v1")) {
		t.Errorf("backup changed with the live file: %q, %v", b, err)
	}

	for i := 1; i <= 4; i++ {
		if err := kv.Backup(t0.Add(time.Duration(i)*time.Minute), 3); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := fs.ReadDir(r.FS(), "backups")
	if err != nil || len(entries) != 3 {
		t.Fatalf("%d backups kept, want 3 (%v)", len(entries), err)
	}
	if entries[0].Name() != t0.Add(2*time.Minute).Format(backupLayout) {
		t.Errorf("oldest kept = %s, want the 3rd backup", entries[0].Name())
	}
	if err := kv.Backup(t0, 0); err == nil {
		t.Error("Backup(keep=0) succeeded")
	}
	if err := kv.Backup(t0.Add(4*time.Minute), 3); err != nil {
		t.Errorf("repeating a backup in the same second: %v", err)
	}
}

func TestConcurrentSaves(t *testing.T) {
	kv, _ := openKV(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			key := []string{"d", "k"}
			for range 20 {
				if err := kv.Save(key, bytes.Repeat([]byte{byte('a' + i)}, 1000)); err != nil {
					t.Error(err)
					return
				}
				if _, err := kv.Load(key); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
}
