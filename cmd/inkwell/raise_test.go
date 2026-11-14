package main

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/oioio-space/inkwell/internal/instance"
)

// TestSecondLaunchTokenReachesPresenter proves row R10 and failure F17 up to the toolkit: the
// activation token of a second launch (a real process, started as a desktop starts it, with
// $XDG_ACTIVATION_TOKEN) travels with its request to the primary, through the open queue and the
// opener, and is given with the native window to the platform's presenter. A launch without a
// token presents the window with none. A window that does not exist is never presented.
func TestSecondLaunchTokenReachesPresenter(t *testing.T) {
	home := t.TempDir()
	p, err := instance.Start(t.Context(), home, instance.NewRequest(nil, "/", ""), instance.Options{})
	if err != nil || p == nil {
		t.Fatalf("primary: %v, %v", p, err)
	}
	defer func() { _ = p.Close() }()
	q := newOpenQueue()
	q.open()
	p.Serve(q.push)

	type call struct {
		window unsafe.Pointer
		token  string
	}
	var (
		mu     sync.Mutex
		calls  []call
		native int // its address stands for the GtkWindow
		window = unsafe.Pointer(&native)
	)
	r := raiser{
		invoke: func(f func()) { f() },
		window: func() unsafe.Pointer { mu.Lock(); defer mu.Unlock(); return window },
		present: func(w unsafe.Pointer, token string) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, call{w, token})
		},
	}
	_, cur := projectDocs(t)
	o := &opener{queue: q, env: openEnv(cur), raise: r.raise, log: slog.New(slog.DiscardHandler)}
	ctx, cancel := context.WithCancel(t.Context())
	w := newFakeWindow(ctx)
	done := make(chan struct{})
	go func() { o.Serve(w); close(done) }()
	defer func() { cancel(); <-done }()

	launch := func(env ...string) {
		t.Helper()
		cmd := child("second", append(env, "INKWELL_HOME="+home, "INKWELL_CHILD_ARGS="+strings.Join([]string{"a.typ"}, "\n"))...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("second launch: %v\n%s", err, out)
		}
	}
	launch("XDG_ACTIVATION_TOKEN=gnome-shell/Files/1234-5-host_TIME678")
	w.waitFrames(t, 1)
	launch()
	w.waitFrames(t, 2)
	mu.Lock()
	window = nil // destroyed
	mu.Unlock()
	launch("XDG_ACTIVATION_TOKEN=late")
	w.waitFrames(t, 3)

	mu.Lock()
	defer mu.Unlock()
	want := []call{{unsafe.Pointer(&native), "gnome-shell/Files/1234-5-host_TIME678"}, {unsafe.Pointer(&native), ""}}
	if len(calls) != len(want) {
		t.Fatalf("presented %d times (%v), want %d", len(calls), calls, len(want))
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("presentation %d = %v, want %v", i, calls[i], want[i])
		}
	}
}

// TestRaiseDoesNotWait: the raise is handed to the main thread and returns: it never waits on the
// window (the opener calls it while the window may be closing).
func TestRaiseDoesNotWait(t *testing.T) {
	var queued []func()
	presented := 0
	r := raiser{
		invoke:  func(f func()) { queued = append(queued, f) },
		window:  func() unsafe.Pointer { return unsafe.Pointer(&presented) },
		present: func(unsafe.Pointer, string) { presented++ },
	}
	r.raise("tok")
	if presented != 0 || len(queued) != 1 {
		t.Fatalf("presented %d times with %d queued, want 0 and 1: raise ran the presentation itself", presented, len(queued))
	}
	queued[0]()
	if presented != 1 {
		t.Errorf("presented %d times once the main thread ran it, want 1", presented)
	}
}
