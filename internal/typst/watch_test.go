package typst

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/sidecar"
)

type watched struct {
	res *Result
	err error
}

// startWatch runs Watch and returns the results it delivers and a stop
// function that cancels it and checks that it returned nil.
func startWatch(t *testing.T, e *Engine, p Project, main string) (<-chan watched, func()) {
	t.Helper()
	results := make(chan watched, 32)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- e.Watch(ctx, p, main, func(res *Result, err error) { results <- watched{res, err} })
	}()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Watch returned %v after cancellation, want nil", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("Watch did not return after cancellation")
		}
	}
	t.Cleanup(stop)
	return results, stop
}

func next(t *testing.T, results <-chan watched, within time.Duration) watched {
	t.Helper()
	select {
	case w := <-results:
		return w
	case <-time.After(within):
		t.Fatalf("no result within %v", within)
		return watched{}
	}
}

func quiet(t *testing.T, results <-chan watched, d time.Duration) {
	t.Helper()
	select {
	case w := <-results:
		t.Fatalf("unexpected result %+v", w)
	case <-time.After(d):
	}
}

func rewrite(t *testing.T, p Project, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(p.Root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const (
	onePage   = "= One\n"
	threePage = "= One\n#pagebreak()\nTwo\n#pagebreak()\nThree\n"
	endless   = "#for i in range(3000000000) { let x = i * 2 }\n"
)

func TestWatch(t *testing.T) {
	e := newEngine(t, nil)

	t.Run("TYP-21 compiles at once, then after each change", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": onePage})
		results, _ := startWatch(t, e, p, "main.typ")

		first := next(t, results, 60*time.Second)
		if first.err != nil || len(first.res.Pages) != 1 {
			t.Fatalf("first result = %v, %d pages; want 1 page", first.err, len(first.res.Pages))
		}
		rewrite(t, p, "main.typ", threePage)
		second := next(t, results, 30*time.Second)
		if second.err != nil || len(second.res.Pages) != 3 {
			t.Fatalf("after the change: %v, %d pages; want 3", second.err, len(second.res.Pages))
		}
	})

	t.Run("TYP-21 a burst of writes gives one compilation", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": onePage})
		results, _ := startWatch(t, e, p, "main.typ")
		next(t, results, 60*time.Second)

		for range 10 {
			rewrite(t, p, "main.typ", onePage)
		}
		rewrite(t, p, "main.typ", threePage)
		got := next(t, results, 30*time.Second)
		if got.err != nil || len(got.res.Pages) != 3 {
			t.Fatalf("result = %v, %d pages; want the last version, 3 pages", got.err, len(got.res.Pages))
		}
		quiet(t, results, 1500*time.Millisecond)
	})

	t.Run("TYP-21 a change cancels the compilation under way", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": endless})
		results, _ := startWatch(t, e, p, "main.typ")
		time.Sleep(700 * time.Millisecond) // the endless compilation is running
		rewrite(t, p, "main.typ", threePage)

		got := next(t, results, 30*time.Second)
		if got.err != nil || len(got.res.Pages) != 3 {
			t.Fatalf("first delivered result = %v; want the new version only (the old one was cancelled)", got.err)
		}
	})

	t.Run("an error in the document is a result, not the end of Watch", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "#oops(\n"})
		results, _ := startWatch(t, e, p, "main.typ")
		bad := next(t, results, 60*time.Second)
		if !errors.Is(bad.err, ErrCompile) {
			t.Fatalf("first result = %v, want ErrCompile", bad.err)
		}
		rewrite(t, p, "main.typ", onePage)
		if good := next(t, results, 30*time.Second); good.err != nil {
			t.Fatalf("after the fix: %v", good.err)
		}
	})

	t.Run("a folder created later is watched", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": "#include \"sub/inc.typ\"\n"})
		results, _ := startWatch(t, e, p, "main.typ")
		if first := next(t, results, 60*time.Second); first.err == nil {
			t.Fatal("the missing include compiled")
		}
		if err := os.Mkdir(filepath.Join(p.Root, "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		next(t, results, 30*time.Second) // the folder itself changed the project
		time.Sleep(300 * time.Millisecond)
		rewrite(t, p, "sub/inc.typ", "Included\n")
		deadline := time.Now().Add(30 * time.Second)
		for {
			got := next(t, results, time.Until(deadline))
			if got.err == nil {
				return
			}
		}
	})

	t.Run("a main file that appears later", func(t *testing.T) {
		p := writeProject(t, map[string]string{"other.typ": "x"})
		results, _ := startWatch(t, e, p, "main.typ")
		if first := next(t, results, 30*time.Second); !errors.Is(first.err, ErrInvalidMain) {
			t.Fatalf("first result = %v, want ErrInvalidMain", first.err)
		}
		rewrite(t, p, "main.typ", onePage)
		if got := next(t, results, 30*time.Second); got.err != nil {
			t.Fatalf("after the file appeared: %v", got.err)
		}
	})

	t.Run("Inkwell's own folders do not trigger a compilation", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": onePage, ".inkwell/repo/x": "1"})
		results, _ := startWatch(t, e, p, "main.typ")
		next(t, results, 60*time.Second)
		rewrite(t, p, ".inkwell/repo/x", "2")
		rewrite(t, p, ".inkwell/new", "3")
		quiet(t, results, 1200*time.Millisecond)
	})

	t.Run("TYP-22 cancelling stops Watch, even mid-compilation, leaving no process", func(t *testing.T) {
		p := writeProject(t, map[string]string{"main.typ": endless})
		_, stop := startWatch(t, e, p, "main.typ")
		time.Sleep(700 * time.Millisecond)
		begin := time.Now()
		stop()
		if took := time.Since(begin); took > 8*time.Second {
			t.Errorf("stopping took %v", took)
		}
		bin, _ := sharedStore.Resolve(sidecar.Typst)
		deadline := time.Now().Add(3 * time.Second)
		for len(typstProcesses(t, bin)) > 0 && time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
		}
		if left := typstProcesses(t, bin); len(left) > 0 {
			t.Errorf("typst processes left running: %v", left)
		}
	})

	t.Run("a bad project folder is an error", func(t *testing.T) {
		if err := e.Watch(t.Context(), Project{Root: filepath.Join(t.TempDir(), "nope")}, "main.typ", func(*Result, error) {}); err == nil {
			t.Fatal("Watch on a missing folder = nil, want an error")
		}
	})
}
