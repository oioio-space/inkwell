package app_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
)

func join(t *testing.T, h *app.Present) *previewConn {
	t.Helper()
	c := newPreviewConn(t)
	done := make(chan struct{})
	go func() { defer close(done); h.Serve(c) }()
	t.Cleanup(func() { c.close(); <-done })
	return c
}

// frame waits for the next frame of the type, skipping the others, and returns its text.
func (c *previewConn) frame(t *testing.T, typ string) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case b := <-c.sent:
			var head struct{ Type string }
			if err := json.Unmarshal(b, &head); err != nil {
				t.Fatalf("frame %q is not JSON: %v", b, err)
			}
			if head.Type == typ {
				return string(b)
			}
		case <-timeout:
			t.Fatalf("no %q frame", typ)
		}
	}
}

// roster waits for a roster that counts n windows.
func (c *previewConn) roster(t *testing.T, n int) {
	t.Helper()
	for {
		var r struct{ Windows int }
		if err := json.Unmarshal([]byte(c.frame(t, "roster")), &r); err != nil {
			t.Fatal(err)
		}
		if r.Windows == n {
			return
		}
	}
}

func TestPresentRelaysToTheOtherWindows(t *testing.T) {
	h := app.NewPresent(nil)
	presenter, public := join(t, h), join(t, h)
	presenter.roster(t, 2)
	public.roster(t, 2)

	presenter.ask(t, `{"type":"state","slide":3,"step":1}`)
	if got := public.frame(t, "state"); got != `{"type":"state","slide":3,"step":1}` {
		t.Errorf("public view got %s", got)
	}
	public.ask(t, `{"type":"cmd","op":"next"}`)
	if got := presenter.frame(t, "cmd"); got != `{"type":"cmd","op":"next"}` {
		t.Errorf("presenter got %s", got)
	}
	// Nothing comes back to the window that sent.
	presenter.ask(t, `{"type":"state","slide":4}`)
	public.frame(t, "state")
	select {
	case b := <-presenter.sent:
		var head struct{ Type string }
		if json.Unmarshal(b, &head) == nil && head.Type == "state" {
			t.Errorf("the sender got its own frame back: %s", b)
		}
	case <-time.After(200 * time.Millisecond):
	}
}

func TestPresentTellsWhenAWindowLeaves(t *testing.T) {
	h := app.NewPresent(nil)
	presenter := join(t, h)
	presenter.roster(t, 1)
	public := join(t, h)
	presenter.roster(t, 2)
	public.close()
	presenter.roster(t, 1)
}

func TestPresentDropsWhatIsNotAFrame(t *testing.T) {
	h := app.NewPresent(nil)
	presenter, public := join(t, h), join(t, h)
	public.roster(t, 2)
	for _, bad := range []string{
		``,
		`nope`,
		`[]`,
		`{"slide":1}`,
		`{"type":""}`,
		`{"type":"Upper"}`,
		`{"type":"with space"}`,
		`{"type":"` + strings.Repeat("a", 17) + `"}`,
		`{"type":"big","x":"` + strings.Repeat("x", 8<<20) + `"}`,
	} {
		presenter.ask(t, bad)
	}
	presenter.ask(t, `{"type":"state","slide":1}`) // arrives after the bad ones, so they were dropped before it
	for {
		select {
		case b := <-public.sent:
			var head struct{ Type string }
			if json.Unmarshal(b, &head) != nil {
				t.Fatalf("a malformed frame was relayed: %.80s", b)
			}
			switch head.Type {
			case "roster":
			case "state":
				return
			default:
				t.Fatalf("a bad frame was relayed: %.80s", b)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the good frame never arrived")
		}
	}
}

func TestPresentRefusesTooManyWindows(t *testing.T) {
	h := app.NewPresent(nil)
	var first *previewConn
	for i := range 6 {
		c := join(t, h)
		if i == 0 {
			first = c
		}
	}
	first.roster(t, 6)
	extra := newPreviewConn(t)
	done := make(chan struct{})
	go func() { defer close(done); h.Serve(extra) }()
	select {
	case <-done: // refused: Serve returned at once
	case <-time.After(5 * time.Second):
		t.Fatal("the seventh window was served")
	}
}
