package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/version"
)

// childEnv selects a helper process mode of the test binary (TestMain): « primary » is a launch
// whose window is replaced by printing the requests it gets; « second » is a real launch (the
// effects of the shipped binary) with the arguments of $INKWELL_CHILD_ARGS, one per line.
const childEnv = "INKWELL_CMD_CHILD"

// childModes are the helper modes registered by the platform's test files.
var childModes = map[string]func() int{}

func TestMain(m *testing.M) {
	switch os.Getenv(childEnv) {
	case "primary":
		os.Exit(primaryChild())
	case "second":
		os.Exit(startup(strings.Split(os.Getenv("INKWELL_CHILD_ARGS"), "\n"), liveDeps(nil)))
	}
	if mode, ok := childModes[os.Getenv(childEnv)]; ok {
		os.Exit(mode())
	}
	os.Exit(m.Run())
}

// primaryChild claims the profile of $INKWELL_HOME like Inkwell does, prints « ready » where
// Inkwell would open the identity, then prints each request it is handed until its standard input
// closes.
func primaryChild() int {
	d := liveDeps(nil)
	d.openHome = func() (*os.Root, error) {
		return nil, errors.New("stopped") // never reached: the queue is read below
	}
	var q *openQueue
	d.start = func(ctx context.Context, dir string, req instance.Request) (*instance.Primary, error) {
		p, err := instance.Start(ctx, dir, req, instance.Options{})
		if p == nil {
			return p, err
		}
		// Serve before startup does, into a queue of our own: the test reads what was handed over.
		q = newOpenQueue()
		q.open()
		p.Serve(q.push)
		fmt.Println("ready")
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); os.Exit(0) }()
		g := q.connect()
		for {
			changed := q.changed()
			if e, ok := q.take(g); ok {
				r := e.req
				fmt.Printf("got %q cwd=%s token=%s\n", r.Args, r.Cwd, r.Activation)
				q.handled(g, e.seq)
				continue
			}
			<-changed
		}
	}
	return startup(nil, d)
}

// cleanEnv is this process's environment without the variables a test sets for its child.
func cleanEnv(extra ...string) []string {
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool {
		for _, k := range []string{"XDG_ACTIVATION_TOKEN=", "DESKTOP_STARTUP_ID=", "INKWELL_HOME=", "INKWELL_DOCS=", childEnv + "=", "INKWELL_CHILD_ARGS="} {
			if strings.HasPrefix(kv, k) {
				return true
			}
		}
		return false
	})
	return append(env, extra...)
}

func child(mode string, env ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^$") // #nosec G204 -- the test binary itself
	cmd.Env = cleanEnv(append(env, childEnv+"="+mode)...)
	return cmd
}

// snapshot lists every file under the folders, with its size, mode and modification time.
func snapshot(t *testing.T, dirs ...string) []string {
	t.Helper()
	var out []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			out = append(out, fmt.Sprintf("%s %v %d %d", p, info.Mode(), info.Size(), info.ModTime().UnixNano()))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// TestVersionDoesNotClaim: « inkwell --version » prints the version and takes no lock (R15).
func TestVersionDoesNotClaim(t *testing.T) {
	var out bytes.Buffer
	d := deps{
		stdout:  &out,
		homeDir: func() (string, error) { t.Fatal("profile folder opened for --version"); return "", nil },
	}
	if code := startup([]string{"--version"}, d); code != exitOK {
		t.Errorf("exit code %d", code)
	}
	if got, want := strings.TrimSpace(out.String()), fmt.Sprint(version.Current()); got != want {
		t.Errorf("printed %q, want %q", got, want)
	}
}

// TestSecondLaunchTouchesNothing: with a primary running, a second launch with a file exits 0
// within 5 s and leaves the profile and Docs folders exactly as they were: no identity, no project,
// nothing (R10).
func TestSecondLaunchTouchesNothing(t *testing.T) {
	home, docs := t.TempDir(), t.TempDir()
	file := filepath.Join(t.TempDir(), "rapport.typ")
	if err := os.WriteFile(file, []byte("= Rapport"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := child("primary", "INKWELL_HOME="+home, "INKWELL_DOCS="+docs)
	stdin, err := primary.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := primary.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := primary.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stdin.Close(); _ = primary.Process.Kill(); _ = primary.Wait() })
	lines := make(chan string, 8)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
	}()
	next := func() string {
		select {
		case l := <-lines:
			return l
		case <-time.After(20 * time.Second):
			t.Fatal("no line from the primary within 20 s")
		}
		return ""
	}
	if got := next(); got != "ready" {
		t.Fatalf("primary: %q", got)
	}
	before := snapshot(t, home, docs)

	second := child("second", "INKWELL_HOME="+home, "INKWELL_DOCS="+docs, "INKWELL_CHILD_ARGS="+file)
	var stderr bytes.Buffer
	second.Stderr = &stderr
	start := time.Now()
	if err := second.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- second.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second launch: %v\n%s", err, &stderr)
		}
	case <-time.After(5 * time.Second):
		_ = second.Process.Kill()
		t.Fatal("second launch still running after 5 s")
	}
	t.Logf("second launch exited 0 after %v", time.Since(start))
	if got, want := next(), fmt.Sprintf("got %q cwd=%s token=", []string{file}, mustWd(t)); got != want {
		t.Errorf("primary got %q, want %q", got, want)
	}
	if after := snapshot(t, home, docs); !slices.Equal(after, before) {
		t.Errorf("the second launch changed the folders\nbefore %q\n after %q", before, after)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "instance.lock" && e.Name() != "instance.sock" {
			t.Errorf("profile holds %q: only the primary's lock and socket belong there", e.Name())
		}
	}
}

func mustWd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

// TestSecondLaunchForwardsArgsAndCwd: the arguments go over verbatim with the second launch's
// working directory, and the activation token from XDG_ACTIVATION_TOKEN, else DESKTOP_STARTUP_ID
// (R10, F17).
func TestSecondLaunchForwardsArgsAndCwd(t *testing.T) {
	home := t.TempDir()
	p, err := instance.Start(t.Context(), home, instance.NewRequest(nil, "/", ""), instance.Options{})
	if err != nil || p == nil {
		t.Fatalf("primary: %v, %v", p, err)
	}
	defer func() { _ = p.Close() }()
	var mu sync.Mutex
	var got []instance.Request
	p.Serve(func(r instance.Request) bool { mu.Lock(); defer mu.Unlock(); got = append(got, r); return true })

	cwd, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"a.typ", "../b.md", "/tmp/c d.csv", "été.typ"}
	cases := []struct {
		env  []string
		want string
	}{
		{[]string{"XDG_ACTIVATION_TOKEN=xdg-1", "DESKTOP_STARTUP_ID=dsi-1"}, "xdg-1"},
		{[]string{"DESKTOP_STARTUP_ID=dsi-2"}, "dsi-2"},
		{nil, ""},
	}
	for i, tc := range cases {
		cmd := child("second", append(tc.env, "INKWELL_HOME="+home, "INKWELL_CHILD_ARGS="+strings.Join(args, "\n"))...)
		cmd.Dir = cwd
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("second launch %d: %v\n%s", i, err, out)
		}
		mu.Lock()
		if len(got) != i+1 {
			t.Fatalf("primary got %d requests, want %d", len(got), i+1)
		}
		r := got[i]
		mu.Unlock()
		if !slices.Equal(r.Args, args) {
			t.Errorf("Args = %q, want %q", r.Args, args)
		}
		if resolved, _ := filepath.EvalSymlinks(r.Cwd); resolved != cwd {
			t.Errorf("Cwd = %q, want %q", r.Cwd, cwd)
		}
		if r.Activation != tc.want {
			t.Errorf("Activation = %q, want %q", r.Activation, tc.want)
		}
	}
}

// fakeDeps are deps whose effects fail the test unless the test replaces them.
func fakeDeps(t *testing.T, env map[string]string) deps {
	return deps{
		stdout:   io.Discard,
		log:      slog.New(slog.DiscardHandler),
		getenv:   func(k string) string { return env[k] },
		unsetenv: func(string) error { return nil },
		getwd:    func() (string, error) { return "/home/ana", nil },
		homeDir: func() (string, error) {
			return t.TempDir(), nil
		},
		start: func(context.Context, string, instance.Request) (*instance.Primary, error) {
			t.Fatal("unexpected claim")
			return nil, nil
		},
		notify:      func(m message) { t.Fatalf("unexpected notification %q", m.title) },
		openHome:    func() (*os.Root, error) { t.Fatal("identity opened"); return nil, nil },
		openProject: func(instance.Request) (*project.Project, error) { t.Fatal("project opened"); return nil, nil },
		startNetwork: func(context.Context, *os.Root, *project.Project, bool) (*app.Runtime, error) {
			t.Fatal("network started")
			return nil, nil
		},
		window:           func(context.Context, *session) int { t.Fatal("window opened"); return 0 },
		restartRequested: func(*app.Runtime) bool { t.Fatal("restart asked"); return false },
		restart:          func(string) error { t.Fatal("restarted"); return nil },
	}
}

// TestUnresponsiveTellsPerson: a primary that does not answer, of another version, or a profile
// that cannot be locked: the person gets the notification of the decision table in their language,
// and the launch exits 3 (R11, R17) or 4 (R18), touching nothing.
func TestUnresponsiveTellsPerson(t *testing.T) {
	cases := []struct {
		err   error
		env   map[string]string
		code  int
		title string
		body  string
	}{
		{instance.ErrNotResponding, map[string]string{"LANG": "fr_FR.UTF-8"}, exitNoAnswer, "Inkwell ne répond pas", "Inkwell est déjà ouvert avec ce profil mais ne répond pas. Fermez-le puis réessayez."},
		{fmt.Errorf("wrapped: %w", instance.ErrNotResponding), map[string]string{"LANG": "en_US.UTF-8"}, exitNoAnswer, "Inkwell is not responding", "Inkwell is already open with this profile but is not responding. Close it, then try again."},
		{instance.ErrOtherVersion, map[string]string{"LC_MESSAGES": "fr_CA.UTF-8", "LANG": "en_US"}, exitNoAnswer, "Inkwell est déjà ouvert", "Une autre version d'Inkwell est ouverte. Fermez-la puis réessayez."},
		{instance.ErrOtherVersion, map[string]string{"LC_ALL": "C", "LC_MESSAGES": "fr_FR"}, exitNoAnswer, "Inkwell is already open", "Another version of Inkwell is open. Close it, then try again."},
		{instance.ErrNoLock, map[string]string{"LC_ALL": "fr_FR.UTF-8"}, exitNoLock, "Inkwell ne démarre pas", "Inkwell ne peut pas s'assurer d'être seul à utiliser son profil ; il ne démarre pas."},
		{instance.ErrNoLock, nil, exitNoLock, "Inkwell does not start", "Inkwell cannot make sure it is the only one using its profile; it does not start."},
	}
	for _, tc := range cases {
		d := fakeDeps(t, tc.env)
		d.start = func(context.Context, string, instance.Request) (*instance.Primary, error) { return nil, tc.err }
		var got []message
		d.notify = func(m message) { got = append(got, m) }
		if code := startup([]string{"a.typ"}, d); code != tc.code {
			t.Errorf("%v: exit code %d, want %d", tc.err, code, tc.code)
		}
		if want := []message{{tc.title, tc.body}}; !slices.Equal(got, want) {
			t.Errorf("%v %v: notified %q, want %q", tc.err, tc.env, got, want)
		}
	}
	d := fakeDeps(t, nil)
	d.start = func(context.Context, string, instance.Request) (*instance.Primary, error) {
		return nil, errors.New("disk on fire")
	}
	if code := startup(nil, d); code != exitError {
		t.Errorf("other error: exit code %d, want 1", code)
	}
	d.start = func(context.Context, string, instance.Request) (*instance.Primary, error) { return nil, nil }
	if code := startup(nil, d); code != exitOK {
		t.Errorf("handed over: exit code %d, want 0", code)
	}
	d.homeDir = func() (string, error) { return "", errors.New("no config dir") }
	if code := startup(nil, d); code != exitError {
		t.Errorf("no profile folder: exit code %d, want 1", code)
	}
}

// primaryDeps are deps that claim a real profile and record the order of the primary's steps:
// « serve » when a hand-off to it already succeeds as the identity is opened.
func primaryDeps(t *testing.T, steps *[]string) deps {
	t.Helper()
	profile := t.TempDir()
	docs, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = docs.Close() })
	opts := instance.Options{Deadline: 2 * time.Second}
	d := fakeDeps(t, nil)
	d.homeDir = func() (string, error) { return profile, nil }
	var addr string
	d.start = func(ctx context.Context, dir string, req instance.Request) (*instance.Primary, error) {
		*steps = append(*steps, "claim")
		p, err := instance.Start(ctx, dir, req, opts)
		if p != nil {
			addr = p.Addr().String()
		}
		return p, err
	}
	d.openHome = func() (*os.Root, error) {
		if answers(t, addr) {
			*steps = append(*steps, "serve")
		}
		*steps = append(*steps, "home")
		return os.OpenRoot(profile)
	}
	d.openProject = func(instance.Request) (*project.Project, error) {
		*steps = append(*steps, "project")
		return project.Create(docs, "Essai", "")
	}
	d.startNetwork = func(context.Context, *os.Root, *project.Project, bool) (*app.Runtime, error) {
		*steps = append(*steps, "network")
		return nil, errors.New("stop here")
	}
	return d
}

// answers reports whether a primary listens on addr and answers there: a frame of another version
// gets « unsupported » at once, without queueing anything.
func answers(t *testing.T, addr string) bool {
	t.Helper()
	if addr == "" {
		return false
	}
	c, err := net.Dial("unix", addr)
	if err != nil {
		return false
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte{0, 0, 0, 1, 2}); err != nil {
		return false
	}
	var reply [1]byte
	_, err = io.ReadFull(c, reply[:])
	return err == nil && instance.Status(reply[0]) == instance.StatusUnsupported
}

// TestPrimaryClaimsBeforeIdentity: the primary claims the profile and serves the hand-off before
// it opens the identity, the project and the network (rule D-ORDER), and lets the lock go when it
// ends.
func TestPrimaryClaimsBeforeIdentity(t *testing.T) {
	var steps []string
	d := primaryDeps(t, &steps)
	if code := startup([]string{"a.typ"}, d); code != exitError {
		t.Errorf("exit code %d, want 1 (the network was made to fail)", code)
	}
	if want := []string{"claim", "serve", "home", "project", "network"}; !slices.Equal(steps, want) {
		t.Errorf("steps %v, want %v", steps, want)
	}
	dir, _ := d.homeDir()
	p, err := instance.Start(t.Context(), dir, instance.NewRequest(nil, "/", ""), instance.Options{Deadline: time.Second})
	if err != nil || p == nil {
		t.Fatalf("after the launch ended: %v, %v; want the lock free", p, err)
	}
	_ = p.Close()
}

// TestPrimaryHeldUntilExit: garbage collection while the primary runs does not release the lock
// (F13).
func TestPrimaryHeldUntilExit(t *testing.T) {
	var steps []string
	d := primaryDeps(t, &steps)
	dir, _ := d.homeDir()
	d.openHome = func() (*os.Root, error) {
		for range 5 {
			runtime.GC()
			time.Sleep(10 * time.Millisecond)
		}
		// Still held: no primary here; the holder holds the request (nothing applies it while it
		// starts), so this launch ends at its short deadline.
		o := instance.Options{Deadline: 300 * time.Millisecond, ClosingWait: 300 * time.Millisecond}
		p, err := instance.Start(t.Context(), dir, instance.NewRequest(nil, "/", ""), o)
		if p != nil {
			_ = p.Close()
			t.Error("the lock went with garbage collection")
		}
		if err != nil && !errors.Is(err, instance.ErrNotResponding) {
			t.Errorf("hand-off after collection: %v", err)
		}
		return nil, errors.New("stop here")
	}
	if code := startup(nil, d); code != exitError {
		t.Errorf("exit code %d", code)
	}
}

// TestShutdownStopsHandoffFirst: shutdown answers « closing » first, closes the network and the
// project, and releases the lock last (rules D-CLOSING, D-HOLD).
func TestShutdownStopsHandoffFirst(t *testing.T) {
	var steps []string
	step := func(name string) func() error { return func() error { steps = append(steps, name); return nil } }
	ctx, stopLoops := context.WithCancel(t.Context())
	var loops shutdownLoops
	loops.Go("watch", func() { <-ctx.Done(); steps = append(steps, "loop") })
	cancel := func() { steps = append(steps, "cancel"); stopLoops() }
	var logs bytes.Buffer
	stopAll(func() { steps = append(steps, "closing") }, cancel, &loops,
		step("network"), step("project"), func() error { steps = append(steps, "lock"); return errors.New("busy disk") },
		slog.New(slog.NewTextHandler(&logs, nil)))
	if want := []string{"closing", "cancel", "loop", "network", "project", "lock"}; !slices.Equal(steps, want) {
		t.Errorf("steps %v, want %v", steps, want)
	}
	if !strings.Contains(logs.String(), "releasing the profile") {
		t.Errorf("lock error not logged: %s", &logs)
	}
}

// TestOpenQueueBounded: the queue refuses launches (« busy ») until the primary opened it, takes the
// primary's own request first whatever its state, and holds at most 64 requests counting those sent
// to the window and not yet handled: a request keeps its slot until the window reports it handled,
// so refilling while requests are in flight never exceeds the bound; a window that connects again
// gets the unhandled ones again.
func TestOpenQueueBounded(t *testing.T) {
	q := newOpenQueue()
	if q.push(instance.Request{Cwd: "early"}) {
		t.Fatal("a launch was queued before the primary could show it")
	}
	q.own(instance.Request{Cwd: "own"})
	q.open()
	for i := range openQueueSize - 1 {
		if !q.push(instance.Request{Cwd: fmt.Sprint(i)}) {
			t.Fatalf("push %d refused", i)
		}
	}
	if q.push(instance.Request{Cwd: "65"}) {
		t.Fatal("65th request queued")
	}
	g := q.connect()
	first, ok := q.take(g)
	if !ok || first.req.Cwd != "own" {
		t.Fatalf("take = %q, %v; want the primary's own first", first.req.Cwd, ok)
	}
	var sent []*openEntry
	sent = append(sent, first)
	for range maxInFlight - 1 {
		e, ok := q.take(g)
		if !ok {
			t.Fatal("take refused below the in-flight bound")
		}
		sent = append(sent, e)
	}
	if _, ok := q.take(g); ok {
		t.Fatalf("more than %d requests in flight", maxInFlight)
	}
	// Taken is not handled: the slots stay reserved.
	if q.push(instance.Request{Cwd: "refill"}) {
		t.Fatal("a request was queued while 64 were waiting or in flight")
	}
	if !q.handled(g, sent[0].seq) || q.handled(g, sent[0].seq) {
		t.Fatal("handled must remove a request exactly once")
	}
	if !q.push(instance.Request{Cwd: "refill"}) {
		t.Fatal("push refused after a request was handled")
	}
	if q.push(instance.Request{Cwd: "over"}) {
		t.Fatal("the bound was exceeded after a refill")
	}
	// The window connected again: what it had not handled is sent again, first, in order.
	again, _ := q.take(q.connect())
	if again.seq != sent[1].seq {
		t.Errorf("after resend, took %d, want %d", again.seq, sent[1].seq)
	}
	// The drain at close counts what is left, and returns at its bound.
	start := time.Now()
	if left := q.waitHandled(100 * time.Millisecond); left != openQueueSize || time.Since(start) < 100*time.Millisecond {
		t.Errorf("waitHandled = %d after %v, want %d after the 100 ms bound", left, time.Since(start), openQueueSize)
	}
}

// fakeWindow is the webview's end of the open stream: it records each frame Go sends and sends
// back what the test puts in acks; Go closing it ends it like a Wails stream (its context is done).
type fakeWindow struct {
	ctx    context.Context
	cancel context.CancelFunc
	fail   bool
	mu     sync.Mutex
	sent   []string
	closed bool
	acks   chan []byte
}

func newFakeWindow(parent context.Context) *fakeWindow {
	ctx, cancel := context.WithCancel(parent)
	return &fakeWindow{ctx: ctx, cancel: cancel, acks: make(chan []byte, 1024)}
}

func (w *fakeWindow) Close() error {
	w.mu.Lock()
	w.closed = true
	w.mu.Unlock()
	w.cancel()
	return nil
}

func (w *fakeWindow) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *fakeWindow) Send(b []byte) error {
	if w.fail {
		return errors.New("window gone")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.sent = append(w.sent, string(b))
	return nil
}

func (w *fakeWindow) Receive() ([]byte, error) {
	select {
	case b := <-w.acks:
		return b, nil
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	}
}

func (w *fakeWindow) Context() context.Context { return w.ctx }

// waitFrames waits until n frames were sent.
func (w *fakeWindow) waitFrames(t *testing.T, n int) []string {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		w.mu.Lock()
		f := slices.Clone(w.sent)
		w.mu.Unlock()
		if len(f) >= n {
			return f
		}
	}
	t.Fatalf("fewer than %d frames sent", n)
	return nil
}

// projectDocs makes a project folder Cur with ch/a.typ in a fresh Docs folder.
func projectDocs(t *testing.T) (docs, cur string) {
	t.Helper()
	docs = t.TempDir()
	cur = filepath.Join(docs, "Cur")
	for _, f := range []string{filepath.Join(cur, ".inkwell", "project.json"), filepath.Join(cur, "ch", "a.typ")} {
		if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return docs, cur
}

// TestOpenerServe: each queued request is resolved against its own working directory, the window
// raised with its token, the outcome sent as JSON with its number; a request stays queued until the
// window reports it handled; a request not handled when the window went away is sent to the next.
func TestOpenerServe(t *testing.T) {
	docs, cur := projectDocs(t)
	q := newOpenQueue()
	q.open()
	var mu sync.Mutex
	var raised []string
	o := &opener{queue: q, env: openEnv(cur), raise: func(tok string) { mu.Lock(); raised = append(raised, tok); mu.Unlock() }, log: slog.New(slog.DiscardHandler)}
	q.push(instance.Request{Cwd: filepath.Join(cur, "ch"), Activation: "tok", Args: []string{"a.typ"}})

	gone, stop := context.WithCancel(t.Context())
	w := newFakeWindow(gone)
	done := make(chan struct{})
	go func() { o.Serve(w); close(done) }()
	w.waitFrames(t, 1)
	stop() // went away without handling it
	<-done
	if q.pending() != 1 {
		t.Fatalf("%d requests left, want the unhandled one", q.pending())
	}
	ctx, cancel := context.WithCancel(t.Context())
	w = newFakeWindow(ctx)
	done = make(chan struct{})
	go func() { o.Serve(w); close(done) }()
	q.push(instance.Request{Cwd: "/", Args: []string{filepath.Join(docs, "nothing.typ")}})
	got := w.waitFrames(t, 2)
	want := []string{`{"id":1,"show":"ch/a.typ"}`, `{"id":2,"notices":[{"kind":"refused","reason":"missing","name":"nothing.typ"}]}`}
	if !slices.Equal(got, want) {
		t.Errorf("sent %q, want %q", got, want)
	}
	w.acks <- []byte(`{"handled":2}`)
	w.acks <- []byte(`not json`)
	w.acks <- []byte(`{"handled":1}`)
	q.waitHandled(2 * time.Second)
	if n := q.pending(); n != 0 {
		t.Errorf("%d requests left after both were handled", n)
	}
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"tok", "tok", ""}; !slices.Equal(raised, want) {
		t.Errorf("raised with %q, want %q", raised, want)
	}
}

// TestCloseWaitsForHandled: closing the window right after a launch was acknowledged keeps the
// process until the window reported the request handled (its file shown or its toast up), within
// the bound; a window that cannot answer while closing is logged after the bound (the documented
// residual), never waited on for ever.
func TestCloseWaitsForHandled(t *testing.T) {
	_, cur := projectDocs(t)
	q := newOpenQueue()
	q.open()
	o := &opener{queue: q, env: openEnv(cur), raise: func(string) {}, log: slog.New(slog.DiscardHandler)}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w := newFakeWindow(ctx)
	go o.Serve(w)
	q.push(instance.Request{Cwd: cur, Args: []string{"ch/a.typ"}}) // acknowledged to its launch
	closed := false
	go func() {
		w.waitFrames(t, 1)
		time.Sleep(200 * time.Millisecond) // the window shows it, then says so
		w.acks <- []byte(`{"handled":1}`)
	}()
	var logs bytes.Buffer
	start := time.Now()
	drainOnClose(q, func() { closed = true }, 2*time.Second, slog.New(slog.NewTextHandler(&logs, nil)))
	if d := time.Since(start); d < 200*time.Millisecond || d > 2*time.Second {
		t.Errorf("close waited %v, want until the window handled it (about 200 ms)", d)
	}
	if !closed || q.pending() != 0 || logs.Len() != 0 {
		t.Errorf("closing %v, %d left, logs %q", closed, q.pending(), logs.String())
	}

	q.push(instance.Request{Cwd: cur, Args: []string{"ch/a.typ"}}) // the window will not answer
	w.waitFrames(t, 2)
	start = time.Now()
	drainOnClose(q, func() {}, 300*time.Millisecond, slog.New(slog.NewTextHandler(&logs, nil)))
	if d := time.Since(start); d < 300*time.Millisecond || d > 2*time.Second {
		t.Errorf("close with a silent window waited %v, want its 300 ms bound", d)
	}
	if !strings.Contains(logs.String(), "requests=1") {
		t.Errorf("residual not logged: %q", logs.String())
	}
}

// TestStartupFailureAcksNothing: a launch handed over while the primary is still opening is told
// « busy » (the queue opens only once the primary can show a file); when the primary then fails to
// start, that launch becomes the primary itself instead of exiting 0 with its file never opened.
func TestStartupFailureAcksNothing(t *testing.T) {
	var steps []string
	d := primaryDeps(t, &steps)
	dir, _ := d.homeDir()
	type result struct {
		p   *instance.Primary
		err error
	}
	second := make(chan result, 1)
	d.openHome = func() (*os.Root, error) {
		go func() {
			p, err := instance.Start(context.Background(), dir, instance.NewRequest([]string{"x.typ"}, "/", ""), instance.Options{Deadline: 2 * time.Second, ClosingWait: 5 * time.Second})
			second <- result{p, err}
		}()
		time.Sleep(300 * time.Millisecond) // the second launch is connected and told « busy »
		return os.OpenRoot(dir)
	}
	if code := startup(nil, d); code != exitError {
		t.Fatalf("exit code %d, want 1 (the network was made to fail)", code)
	}
	select {
	case r := <-second:
		if r.err != nil || r.p == nil {
			t.Fatalf("second launch = %v, %v; want it to become the primary", r.p, r.err)
		}
		_ = r.p.Close()
	case <-time.After(5 * time.Second):
		t.Fatal("second launch still waiting 5 s after the primary failed")
	}
}
