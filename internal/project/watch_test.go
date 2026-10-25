package project

import (
	"context"
	"os"
	"testing"
	"time"
)

// startWatch runs p.Watch and returns the channel of reported files. The
// watcher is stopped, and waited for, when the test ends.
func startWatch(t *testing.T, p *Project) <-chan string {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	got := make(chan string, 16)
	done := make(chan error, 1)
	go func() { done <- p.Watch(ctx, func(rel string) { got <- rel }) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Watch returned %v after cancel", err)
		}
	})
	return got
}

func expect(t *testing.T, got <-chan string, want string) {
	t.Helper()
	select {
	case rel := <-got:
		if rel != want {
			t.Fatalf("reported %q, want %q", rel, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("no report of %q", want)
	}
}

func expectNothing(t *testing.T, got <-chan string, d time.Duration) {
	t.Helper()
	select {
	case rel := <-got:
		t.Fatalf("unexpected report of %q", rel)
	case <-time.After(d):
	}
}

func TestExternalEditNotified(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("= A\n")); err != nil {
		t.Fatal(err)
	}
	got := startWatch(t, p)

	if err := base.WriteFile("p/main.typ", []byte("= A, edited elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)

	// The same content, saved again, is not news.
	if err := base.WriteFile("p/main.typ", []byte("= A, edited elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, got, 4*watchDebounce)

	// A new content is.
	if err := base.WriteFile("p/main.typ", []byte("= A, edited again\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)
}

// TestTruncateThenWriteIsOneEdit covers a writer that truncates the file and
// writes it only after a pause longer than watchDebounce: the empty file in
// between is not reported, and rewriting the same content is not news.
func TestTruncateThenWriteIsOneEdit(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("= A\n")); err != nil {
		t.Fatal(err)
	}
	got := startWatch(t, p)
	// Let Watch add its watches first: a file found empty by its initial check
	// is reported at once (the writer cannot be mid-write if Inkwell was closed).
	time.Sleep(watchDebounce)

	f, err := base.OpenFile("p/main.typ", os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(watchDebounce * 3 / 2)
	if _, err := f.WriteString("= A\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, got, 4*watchDebounce)
}

// TestEmptiedFileIsReported checks that a file really emptied outside Inkwell
// is still reported, one debounce later.
func TestEmptiedFileIsReported(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("= A\n")); err != nil {
		t.Fatal(err)
	}
	got := startWatch(t, p)

	if err := base.WriteFile("p/main.typ", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)
}

// TestExternalEditViaRename covers editors that save by writing a temporary
// file and renaming it over the original.
func TestExternalEditViaRename(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("x")); err != nil {
		t.Fatal(err)
	}
	got := startWatch(t, p)

	if err := base.WriteFile("p/.main.typ.swp", []byte("saved by an editor"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Rename("p/.main.typ.swp", "p/main.typ"); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)
}

func TestExternalEditWhileClosedNotified(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	if err := p.WriteProjection(MainFile, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base.Name()+"/p/main.typ", []byte("edited offline"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, startWatch(t, p), MainFile)
}

func TestProjectionIgnoresOwnWrites(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	got := startWatch(t, p)

	// Prove the watcher is live: an outside edit is reported, then acknowledged.
	if err := base.WriteFile("p/main.typ", []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)
	if err := p.Acknowledge(MainFile); err != nil {
		t.Fatal(err)
	}

	for _, text := range []string{"one", "two", "three", ""} {
		if err := p.WriteProjection(MainFile, []byte(text)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(watchDebounce / 2)
	}
	expectNothing(t, got, 6*watchDebounce)

	// The same text as the one we published earlier, written from outside, is
	// still news once Inkwell has moved on.
	if err := p.WriteProjection(MainFile, []byte("current")); err != nil {
		t.Fatal(err)
	}
	if err := base.WriteFile("p/main.typ", []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, got, MainFile)
}

func TestWatchIgnoresOtherFiles(t *testing.T) {
	base := newBase(t)
	p := create(t, base, "p")
	got := startWatch(t, p)
	if err := base.WriteFile("p/notes.txt", []byte("not ours"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := base.Remove("p/notes.txt"); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, got, 6*watchDebounce)
}
