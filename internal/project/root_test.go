package project

import (
	"errors"
	"os"
	"testing"
)

// RT1 (docs/plans/2026-11-25-rt1-root-kept.md): a project's root is replaced only when the storage proves its
// document absent (rules B-1, B-2, B-3).

const (
	rootA = "automerge:2akvofn6bxdksK3uVG6qU6m9fyEP"
	rootB = "automerge:3Pq7Hn9bYdkXvR2mWcT4sLzE8uAf"
)

// storeChunk saves one snapshot chunk of the document url, as the page's storage adapter does.
func storeChunk(t *testing.T, p *Project, url string) {
	t.Helper()
	if err := p.Save([]string{url[len("automerge:"):], "snapshot", "h1"}, []byte("the saved document")); err != nil {
		t.Fatal(err)
	}
}

// recorded reopens the project folder of p in base and returns the root its project.json holds.
func recorded(t *testing.T, base *os.Root, p *Project) string {
	t.Helper()
	q, err := Open(base, p.Dir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = q.Close() }()
	return q.Meta().RootURL
}

func TestRT1B1SetRootURLNeverReplaces(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if err := p.SetRootURL(rootA); err != nil {
		t.Fatalf("B-1 first root: SetRootURL = %v", err)
	}
	if err := p.SetRootURL(rootA); err != nil {
		t.Fatalf("B-1 same root: SetRootURL = %v", err)
	}
	// Stored or not, another root never goes through SetRootURL.
	for _, stored := range []bool{false, true} {
		if stored {
			storeChunk(t, p, rootA)
		}
		if err := p.SetRootURL(rootB); !errors.Is(err, ErrRootRecorded) {
			t.Errorf("B-1 another root (stored %v): SetRootURL = %v, want ErrRootRecorded", stored, err)
		}
		if got := recorded(t, base, p); got != rootA {
			t.Errorf("B-1 project.json root = %q after a refused SetRootURL, want %q", got, rootA)
		}
	}
}

func TestRT1B2ReplaceMissingRootOnlyWhenAbsent(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", rootA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })

	storeChunk(t, p, rootA)
	if err := p.ReplaceMissingRoot(rootA, rootB); !errors.Is(err, ErrRootNotMissing) {
		t.Errorf("B-2 stored root: ReplaceMissingRoot = %v, want ErrRootNotMissing", err)
	}
	if err := p.ReplaceMissingRoot("automerge:other", rootB); !errors.Is(err, ErrRootNotMissing) {
		t.Errorf("B-2 another old root: ReplaceMissingRoot = %v, want ErrRootNotMissing", err)
	}
	if got := recorded(t, base, p); got != rootA {
		t.Fatalf("B-2 project.json root = %q after refusals, want %q", got, rootA)
	}

	q, err := Create(base, "q", rootA) // the same root, nothing of it stored in this folder
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Close() })
	if err := q.ReplaceMissingRoot(rootA, rootB); err != nil {
		t.Fatalf("B-2 absent root: ReplaceMissingRoot = %v", err)
	}
	if got := recorded(t, base, q); got != rootB {
		t.Errorf("B-2 project.json root = %q after the replacement, want %q", got, rootB)
	}

	// An invalid recorded root holds nothing: it is absent and replaced.
	r, err := Create(base, "r", "automerge:not/valid")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	if err := r.ReplaceMissingRoot("automerge:not/valid", rootB); err != nil {
		t.Errorf("B-2 invalid root: ReplaceMissingRoot = %v", err)
	}
}

func TestRT1B3UnknownIsNeverAbsent(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", rootA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	for url, want := range map[string]RootState{"": RootAbsent, "automerge:": RootAbsent, "automerge:a/b": RootAbsent, rootB: RootAbsent} {
		if got := p.RootState(url); got != want {
			t.Errorf("B-3 RootState(%q) = %q, want %q", url, got, want)
		}
	}
	storeChunk(t, p, rootA)
	if got := p.RootState(rootA); got != RootStored {
		t.Fatalf("B-3 RootState(stored) = %q, want stored", got)
	}
	// The one chunk is damaged and has no backup: the storage cannot tell, which is never « absent ».
	name := p.Dir() + "/.inkwell/repo/" + rootA[len("automerge:"):] + "/snapshot/h1~"
	data, err := base.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := base.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := p.RootState(rootA); got != RootUnknown {
		t.Errorf("B-3 RootState(corrupt) = %q, want unknown", got)
	}
}

// The start's own order (RT1 review M1): the page loads the document first, which quarantines its one damaged chunk
// and answers with what is left (nothing); only then does the start ask RootState. A document whose chunks went to
// quarantine is not absent, so it is never replaced.
func TestRT1B3QuarantinedRootIsNeverAbsent(t *testing.T) {
	base := newBase(t)
	p, err := Create(base, "p", rootA)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	storeChunk(t, p, rootA)
	name := p.Dir() + "/.inkwell/repo/" + rootA[len("automerge:"):] + "/snapshot/h1~"
	data, err := base.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xff
	if err := base.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// The storage adapter's loadRange, as automerge-repo's find() sends it.
	if entries, _ := p.LoadRange([]string{rootA[len("automerge:"):], "snapshot"}); len(entries) != 0 {
		t.Fatalf("setup: the damaged chunk was read: %d entries", len(entries))
	}
	for i := range 2 {
		if got := p.RootState(rootA); got != RootUnknown {
			t.Errorf("B-3 RootState after the start's load, call %d = %q, want unknown", i+1, got)
		}
	}
	if err := p.ReplaceMissingRoot(rootA, rootB); !errors.Is(err, ErrRootNotMissing) {
		t.Errorf("B-3 ReplaceMissingRoot over a quarantined root = %v, want ErrRootNotMissing", err)
	}
	if got := recorded(t, base, p); got != rootA {
		t.Errorf("B-3 project.json root = %q, want %q", got, rootA)
	}
}
