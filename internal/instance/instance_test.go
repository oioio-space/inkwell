//go:build linux

package instance

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// childEnv selects the helper process mode of the test binary (TestMain).
const childEnv = "INKWELL_INSTANCE_CHILD"

func TestMain(m *testing.M) {
	if mode := os.Getenv(childEnv); mode != "" {
		os.Exit(runChild(mode))
	}
	os.Exit(m.Run())
}

// runChild is a launch of Inkwell reduced to the hand-off: « claim » starts on $INKWELL_CHILD_DIR with the
// argument $INKWELL_CHILD_ARG; as the primary it prints « primary », then « got <arg> » per request, until its
// standard input closes; as a second launch it prints « handed ». « sleep » only waits for its
// standard input to close (a typst stand-in).
func runChild(mode string) int {
	if mode == "sleep" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0
	}
	deadline, _ := time.ParseDuration(os.Getenv("INKWELL_CHILD_DEADLINE"))
	req := NewRequest([]string{os.Getenv("INKWELL_CHILD_ARG")}, "/", "")
	p, err := Start(context.Background(), os.Getenv("INKWELL_CHILD_DIR"), req, Options{Deadline: deadline})
	if err != nil {
		fmt.Printf("error %v\n", err)
		return 3
	}
	if p == nil {
		fmt.Println("handed")
		return 0
	}
	var mu sync.Mutex
	fmt.Println("primary")
	p.Serve(func(r Request) bool {
		mu.Lock()
		defer mu.Unlock()
		fmt.Printf("got %s\n", strings.Join(r.Args, " "))
		return true
	})
	_, _ = io.Copy(io.Discard, os.Stdin)
	if err := p.Close(); err != nil {
		fmt.Printf("error %v\n", err)
		return 3
	}
	return 0
}

func randomID() [16]byte {
	var id [16]byte
	_, _ = rand.Read(id[:])
	return id
}

// child is a helper process and the lines it printed.
type child struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
}

func startChild(t *testing.T, mode, dir, arg string) *child {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$") // #nosec G204 -- the test binary itself
	cmd.Env = append(os.Environ(), childEnv+"="+mode, "INKWELL_CHILD_DIR="+dir, "INKWELL_CHILD_ARG="+arg, "INKWELL_CHILD_DEADLINE=30s")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := &child{cmd: cmd, stdin: stdin, lines: make(chan string, 64)}
	go func() {
		defer close(c.lines)
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
	}()
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return c
}

// line returns the next line the child printed, or fails after 20 s.
func (c *child) line(t *testing.T) string {
	t.Helper()
	select {
	case l, ok := <-c.lines:
		if !ok {
			t.Fatal("child exited without a line")
		}
		return l
	case <-time.After(20 * time.Second):
		t.Fatal("no line from the child within 20 s")
	}
	return ""
}

// profile returns a fresh private profile folder. Its path is short (t.TempDir names the test):
// the socket beside it must fit in sun_path, as for a real profile.
func profile(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "oi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if dir, err = filepath.EvalSymlinks(dir); err != nil {
		t.Fatal(err)
	}
	if len(filepath.Join(dir, sockName)) > maxSunPath {
		t.Fatalf("temporary folder %q too long for a socket: set TMPDIR to a shorter folder", dir)
	}
	return dir
}

func request(args ...string) Request {
	return Request{ID: randomID(), Cwd: "/home/ana", Activation: "token-1", Args: args}
}

// recorder is a handler that keeps the requests it queued.
type recorder struct {
	mu   sync.Mutex
	got  []Request
	busy int // refuse this many calls first
	wait time.Duration
}

func (r *recorder) handle(req Request) bool {
	time.Sleep(r.wait)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy > 0 {
		r.busy--
		return false
	}
	r.got = append(r.got, req)
	return true
}

func (r *recorder) requests() []Request {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.got)
}

// primary starts the primary of dir and serves into a new recorder.
func primary(t *testing.T, dir string, o Options) (*Primary, *recorder) {
	t.Helper()
	p, err := Start(t.Context(), dir, request(), o)
	if err != nil || p == nil {
		t.Fatalf("Start(first) = %v, %v; want the primary", p, err)
	}
	t.Cleanup(func() { _ = p.Close() })
	rec := &recorder{}
	p.Serve(rec.handle)
	return p, rec
}

// fast is the options of a test that expects no wait at all.
var fast = Options{Deadline: 2 * time.Second, ClosingWait: 2 * time.Second, ReadTimeout: time.Second}

func TestClaimFirstIsPrimary(t *testing.T) {
	dir := profile(t)
	p, _ := primary(t, dir, fast)
	info, err := os.Stat(filepath.Join(dir, lockName))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("lock file: %v, mode %v; want 0600", err, info.Mode().Perm())
	}
	if got := p.Addr().String(); got != filepath.Join(dir, sockName) {
		t.Errorf("listens on %q, want the first candidate", got)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, sockName)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("socket file after Close: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	// A socket file someone removed meanwhile is no error at Close.
	gone, _ := primary(t, profile(t), fast)
	if err := os.Remove(gone.Addr().String()); err != nil {
		t.Fatal(err)
	}
	if err := gone.Close(); err != nil {
		t.Errorf("Close without its socket file: %v", err)
	}
	again, err := Start(t.Context(), dir, request(), fast)
	if err != nil || again == nil {
		t.Fatalf("Start after Close = %v, %v; want the primary", again, err)
	}
	_ = again.Close()
}

func TestClaimSecondIsNotPrimary(t *testing.T) {
	dir := profile(t)
	_, rec := primary(t, dir, fast)
	c := startChild(t, "claim", dir, "second.typ")
	if got := c.line(t); got != "handed" {
		t.Fatalf("second process: %q, want handed", got)
	}
	if err := c.cmd.Wait(); err != nil {
		t.Errorf("second process exit: %v", err)
	}
	if got := rec.requests(); len(got) != 1 || !slices.Equal(got[0].Args, []string{"second.typ"}) {
		t.Errorf("primary got %+v", got)
	}
	// In the same process too: a second descriptor of the lock file is a second claim.
	p, err := Start(t.Context(), dir, request("third.typ"), fast)
	if p != nil || err != nil {
		t.Fatalf("Start(second) = %v, %v; want handed over", p, err)
	}
}

// TestClaimSeparateProfiles: another INKWELL_HOME is another instance (R14).
func TestClaimSeparateProfiles(t *testing.T) {
	a, b := profile(t), profile(t)
	pa, ra := primary(t, a, fast)
	pb, rb := primary(t, b, fast)
	if pa.Addr().String() == pb.Addr().String() {
		t.Fatal("two profiles share one socket")
	}
	if p, err := Start(t.Context(), b, request("b.typ"), fast); p != nil || err != nil {
		t.Fatalf("Start(b) = %v, %v", p, err)
	}
	if len(ra.requests()) != 0 || len(rb.requests()) != 1 {
		t.Errorf("requests: a %d, b %d; want 0, 1", len(ra.requests()), len(rb.requests()))
	}
}

func TestClaimSameProfileThroughSymlink(t *testing.T) {
	dir := profile(t)
	link := filepath.Join(profile(t), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	_, rec := primary(t, dir, fast)
	if p, err := Start(t.Context(), link, request("via-link.typ"), fast); p != nil || err != nil {
		t.Fatalf("Start(symlink) = %v, %v; want handed over", p, err)
	}
	if len(rec.requests()) != 1 {
		t.Errorf("primary got %d requests, want 1", len(rec.requests()))
	}
}

// TestClaimReleasedByCrash: the kernel releases the lock of a killed primary; the next launch is
// the primary at once and removes the socket file left behind (R13, F2, F4).
func TestClaimReleasedByCrash(t *testing.T) {
	dir := profile(t)
	c := startChild(t, "claim", dir, "")
	if got := c.line(t); got != "primary" {
		t.Fatalf("child: %q, want primary", got)
	}
	if err := c.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = c.cmd.Wait()
	if _, err := os.Lstat(filepath.Join(dir, sockName)); err != nil {
		t.Fatalf("the crash left no socket file: %v", err)
	}
	start := time.Now()
	p, err := Start(t.Context(), dir, request(), Options{Deadline: 5 * time.Second})
	if err != nil || p == nil {
		t.Fatalf("Start after the crash = %v, %v; want the primary", p, err)
	}
	defer func() { _ = p.Close() }()
	if d := time.Since(start); d > time.Second {
		t.Errorf("primary after %v, want within 1 s", d)
	}
	p.Serve((&recorder{}).handle)
	if q, err := Start(t.Context(), dir, request(), fast); q != nil || err != nil {
		t.Errorf("hand-off to the new primary = %v, %v", q, err)
	}
}

// TestClaimNotInheritedByChild: a process the primary starts (typst, tinymist) does not hold the
// lock (F12).
func TestClaimNotInheritedByChild(t *testing.T) {
	dir := profile(t)
	p, err := Start(t.Context(), dir, request(), fast)
	if err != nil || p == nil {
		t.Fatalf("Start = %v, %v", p, err)
	}
	c := startChild(t, "sleep", dir, "")
	if fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", c.cmd.Process.Pid)); err == nil {
		for _, fd := range fds {
			target, _ := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", c.cmd.Process.Pid, fd.Name()))
			if strings.HasSuffix(target, lockName) {
				t.Errorf("child holds descriptor %s on %s", fd.Name(), target)
			}
		}
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	q, err := Start(t.Context(), dir, request(), Options{Deadline: 500 * time.Millisecond})
	if err != nil || q == nil {
		t.Fatalf("Start while the child runs = %v, %v; want the primary (lock not inherited)", q, err)
	}
	_ = q.Close()
}

// TestClaimNoLock: a filesystem without locks stops Inkwell (R18, F18); other errors are not
// mistaken for it.
func TestClaimNoLock(t *testing.T) {
	for _, errno := range []error{unix.ENOLCK, unix.EOPNOTSUPP} {
		dir := profile(t)
		o := fast
		o.lock = func(*os.File) error { return lockError(errno) }
		p, err := Start(t.Context(), dir, request(), o)
		if p != nil || !errors.Is(err, ErrNoLock) {
			t.Errorf("Start with %v = %v, %v; want ErrNoLock", errno, p, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, sockName)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a socket was made without the lock: %v", err)
		}
		if q, err := Start(t.Context(), dir, request(), fast); err != nil || q == nil {
			t.Errorf("Start after the refusal = %v, %v; want the primary (lock file closed)", q, err)
		} else {
			_ = q.Close()
		}
	}
	tests := []struct {
		in   error
		want error
	}{
		{nil, nil},
		{unix.EWOULDBLOCK, errHeld},
		{unix.ENOLCK, ErrNoLock},
		{unix.EOPNOTSUPP, ErrNoLock},
	}
	for _, tt := range tests {
		if got := lockError(tt.in); !errors.Is(got, tt.want) || (tt.want == nil) != (got == nil) {
			t.Errorf("lockError(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
	if got := lockError(unix.EIO); got == nil || errors.Is(got, ErrNoLock) || errors.Is(got, errHeld) {
		t.Errorf("lockError(EIO) = %v, want another error", got)
	}
	o := fast
	o.lock = func(*os.File) error { return lockError(unix.EIO) }
	if p, err := Start(t.Context(), profile(t), request(), o); p != nil || !errors.Is(err, unix.EIO) {
		t.Errorf("Start with EIO = %v, %v", p, err)
	}
}

// TestHandoffDelivers: the request arrives as sent, and is queued before the acknowledgement
// (rule D-ACK: exit 0 means taken).
func TestHandoffDelivers(t *testing.T) {
	dir := profile(t)
	_, rec := primary(t, dir, fast)
	rec.wait = 50 * time.Millisecond
	req := Request{ID: randomID(), Cwd: "/home/zoë/Documents", Activation: "gnome-shell/Inkwell/1-0_TIME9", Dropped: 3, Args: []string{"été.typ", "../b.md"}}
	p, err := Start(t.Context(), dir, req, fast)
	if p != nil || err != nil {
		t.Fatalf("Start = %v, %v; want handed over", p, err)
	}
	got := rec.requests() // no wait: acknowledged means already queued
	if len(got) != 1 || !equalRequest(got[0], req) {
		t.Fatalf("queued %+v, want %+v", got, req)
	}
}

// TestHandoffBeforeListen: between taking the lock and listening, a launch retries and is
// delivered once the primary listens.
func TestHandoffBeforeListen(t *testing.T) {
	dir := profile(t)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryLock(f); err != nil {
		t.Fatal(err)
	}
	type result struct {
		p   *Primary
		err error
	}
	done := make(chan result, 1)
	go func() {
		p, err := Start(context.Background(), dir, request("early.typ"), Options{Deadline: 5 * time.Second})
		done <- result{p, err}
	}()
	time.Sleep(150 * time.Millisecond)
	select {
	case r := <-done:
		t.Fatalf("Start returned %v, %v before the primary listened", r.p, r.err)
	default:
	}
	p, err := listen(root, f, dir, fast.withDefaults())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	rec := &recorder{}
	p.Serve(rec.handle)
	if r := <-done; r.p != nil || r.err != nil {
		t.Fatalf("Start = %v, %v; want handed over", r.p, r.err)
	}
	if got := rec.requests(); len(got) != 1 || got[0].Args[0] != "early.typ" {
		t.Errorf("primary got %+v", got)
	}
}

// staleSocket leaves a socket file at path with nothing listening, as a crash does.
func staleSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	_ = ln.Close()
}

// TestHandoffStaleSocket: a socket file left by a crash is removed by the next lock holder only;
// a second launch never removes the primary's live socket (rule D-UNLINK, F4).
func TestHandoffStaleSocket(t *testing.T) {
	runtime := profile(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	dir := profile(t)
	paths := socketPaths(dir, runtime)
	if len(paths) != 2 {
		t.Fatalf("candidates %v", paths)
	}
	for _, p := range paths {
		staleSocket(t, p)
	}
	_, rec := primary(t, dir, fast)
	if _, err := os.Lstat(paths[1]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale runtime socket kept: %v", err)
	}
	before, err := os.Lstat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if q, err := Start(t.Context(), dir, request(strconv.Itoa(i)), fast); q != nil || err != nil {
			t.Fatalf("hand-off %d = %v, %v", i, q, err)
		}
	}
	after, err := os.Lstat(paths[0])
	if err != nil || !os.SameFile(before, after) {
		t.Errorf("the primary's socket was replaced or removed: %v", err)
	}
	if len(rec.requests()) != 3 {
		t.Errorf("primary got %d requests, want 3", len(rec.requests()))
	}
}

// TestHandoffDuringClosing: a launch while the primary closes waits for the lock, past the
// ordinary deadline, and becomes the next primary (R12, rule D-CLOSING, F6).
func TestHandoffDuringClosing(t *testing.T) {
	dir := profile(t)
	p, rec := primary(t, dir, fast)
	p.Closing()
	type result struct {
		p   *Primary
		err error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		q, err := Start(context.Background(), dir, request("late.typ"), Options{Deadline: 200 * time.Millisecond, ClosingWait: 5 * time.Second})
		done <- result{q, err}
	}()
	time.Sleep(600 * time.Millisecond) // three times the deadline
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil || r.p == nil {
		t.Fatalf("Start during closing = %v, %v; want the next primary", r.p, r.err)
	}
	defer func() { _ = r.p.Close() }()
	if d := time.Since(start); d < 600*time.Millisecond {
		t.Errorf("became primary after %v, before the first one closed", d)
	}
	if len(rec.requests()) != 0 {
		t.Errorf("a closing primary queued %+v", rec.requests())
	}
	// The ClosingWait bounds the wait: a primary that never finishes closing is given up on.
	q, rec2 := primary(t, profile(t), fast)
	q.Closing()
	start = time.Now()
	_, err := Start(t.Context(), q.dir, request(), Options{Deadline: 100 * time.Millisecond, ClosingWait: 400 * time.Millisecond})
	if d := time.Since(start); !errors.Is(err, ErrNotResponding) || d < 400*time.Millisecond || d > 900*time.Millisecond {
		t.Errorf("stuck closing: %v after %v; want ErrNotResponding after 400 ms", err, d)
	}
	if len(rec2.requests()) != 0 {
		t.Error("a closing primary queued a request")
	}
}

// holdLock takes the lock of dir in this process, as a primary that never serves would.
func holdLock(t *testing.T, dir string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, lockName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := tryLock(f); err != nil {
		t.Fatal(err)
	}
}

// TestHandoffUnresponsive: a holder that does not answer is given up on at the deadline, not
// before, and within 500 ms after (R11, F5); no second copy starts.
func TestHandoffUnresponsive(t *testing.T) {
	const deadline = 400 * time.Millisecond
	cases := map[string]func(t *testing.T, dir string){
		"listening, never answering": func(t *testing.T, dir string) {
			ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, sockName), Net: "unix"})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = ln.Close() })
		},
		"no socket": func(*testing.T, string) {},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := profile(t)
			holdLock(t, dir)
			setup(t, dir)
			start := time.Now()
			p, err := Start(t.Context(), dir, request(), Options{Deadline: deadline})
			d := time.Since(start)
			if p != nil || !errors.Is(err, ErrNotResponding) {
				t.Fatalf("Start = %v, %v; want ErrNotResponding", p, err)
			}
			if d < deadline || d > deadline+500*time.Millisecond {
				t.Errorf("gave up after %v, want in [%v, %v]", d, deadline, deadline+500*time.Millisecond)
			}
		})
	}
}

// TestHandoffConcurrentLaunches: 15 processes at once on a free profile (Explorer's limit for one
// selection), 10 rounds: exactly one primary, the 14 others delivered once each (F1, F10).
func TestHandoffConcurrentLaunches(t *testing.T) {
	for round := range 10 {
		dir := profile(t)
		children := make([]*child, 15)
		for i := range children {
			children[i] = startChild(t, "claim", dir, fmt.Sprintf("r%d-%d.typ", round, i))
		}
		var prim *child
		var handed []string
		for i, c := range children {
			switch got := c.line(t); got {
			case "primary":
				if prim != nil {
					t.Fatalf("round %d: two primaries", round)
				}
				prim = c
			case "handed":
				handed = append(handed, fmt.Sprintf("r%d-%d.typ", round, i))
			default:
				t.Fatalf("round %d, child %d: %q", round, i, got)
			}
		}
		if prim == nil || len(handed) != 14 {
			t.Fatalf("round %d: primary %v, %d handed", round, prim != nil, len(handed))
		}
		var got []string
		for range handed {
			l := prim.line(t)
			arg, ok := strings.CutPrefix(l, "got ")
			if !ok {
				t.Fatalf("round %d: primary printed %q", round, l)
			}
			got = append(got, arg)
		}
		slices.Sort(got)
		slices.Sort(handed)
		if !slices.Equal(got, handed) {
			t.Fatalf("round %d: delivered %v, want %v", round, got, handed)
		}
		_ = prim.stdin.Close()
		if err := prim.cmd.Wait(); err != nil {
			t.Fatalf("round %d: primary exit: %v", round, err)
		}
		if l, ok := <-prim.lines; ok {
			t.Errorf("round %d: primary printed %q after the last request", round, l)
		}
	}
}

// TestHandoffDuplicateIgnored: a request retried with the same ID is acknowledged but queued once
// (F11); the primary remembers the last 64 IDs.
func TestHandoffDuplicateIgnored(t *testing.T) {
	dir := profile(t)
	_, rec := primary(t, dir, fast)
	req := request("once.typ")
	for range 3 {
		if p, err := Start(t.Context(), dir, req, fast); p != nil || err != nil {
			t.Fatalf("Start = %v, %v", p, err)
		}
	}
	if n := len(rec.requests()); n != 1 {
		t.Fatalf("queued %d times, want once", n)
	}
	for i := range recentIDs - 1 {
		if p, err := Start(t.Context(), dir, request(strconv.Itoa(i)), fast); p != nil || err != nil {
			t.Fatalf("Start %d = %v, %v", i, p, err)
		}
	}
	// 64 IDs seen: the first is still remembered…
	if p, err := Start(t.Context(), dir, req, fast); p != nil || err != nil || len(rec.requests()) != recentIDs {
		t.Fatalf("repeat after 63 others: %v, %v, %d queued", p, err, len(rec.requests()))
	}
	// …until a 65th pushes it out.
	if _, err := Start(t.Context(), dir, request("65"), fast); err != nil {
		t.Fatal(err)
	}
	if _, err := Start(t.Context(), dir, req, fast); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.requests()); n != recentIDs+2 {
		t.Errorf("queued %d, want %d (the oldest ID forgotten)", n, recentIDs+2)
	}
}

// TestHandoffBusy: a full queue answers « busy »; the launch backs off, retries, and is queued
// once room is made, under its own ID.
func TestHandoffBusy(t *testing.T) {
	dir := profile(t)
	_, rec := primary(t, dir, fast)
	rec.busy = 3
	req := request("busy.typ")
	start := time.Now()
	if p, err := Start(t.Context(), dir, req, fast); p != nil || err != nil {
		t.Fatalf("Start = %v, %v", p, err)
	}
	if d := time.Since(start); d < backoff(0)+backoff(1)+backoff(2) {
		t.Errorf("accepted after %v, faster than the back-off", d)
	}
	if got := rec.requests(); len(got) != 1 || got[0].ID != req.ID {
		t.Errorf("queued %+v", got)
	}
	// Busy for ever: an answering primary earns the closing wait, then the launch gives up.
	rec.busy = 1 << 30
	start = time.Now()
	_, err := Start(t.Context(), dir, request(), Options{Deadline: 100 * time.Millisecond, ClosingWait: 400 * time.Millisecond})
	if d := time.Since(start); !errors.Is(err, ErrNotResponding) || d < 400*time.Millisecond || d > 900*time.Millisecond {
		t.Errorf("always busy: %v after %v; want ErrNotResponding after 400 ms", err, d)
	}
}

// TestHandoffAcrossTakeover: three launches. B arrives while A closes and is told « closing »; a
// third launch C takes the lock before B (B's lock is held back to make the order certain); B then
// hands its request, unchanged, to C and exits 0 once C queued it (takeover with retry, R12).
func TestHandoffAcrossTakeover(t *testing.T) {
	dir := profile(t)
	a, recA := primary(t, dir, fast)
	a.Closing()
	gate := make(chan struct{})
	o := Options{Deadline: 2 * time.Second, ClosingWait: 10 * time.Second}
	o.lock = func(f *os.File) error {
		select {
		case <-gate:
			return tryLock(f)
		default:
			return errHeld
		}
	}
	reqB := Request{ID: randomID(), Cwd: "/home/ana/Documents", Args: []string{"Projet B/notes.md"}}
	done := make(chan error, 1)
	go func() {
		p, err := Start(context.Background(), dir, reqB, o)
		if p != nil {
			err = errors.New("B became the primary")
		}
		done <- err
	}()
	time.Sleep(100 * time.Millisecond) // B told « closing », waiting for the lock
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	_, recC := primary(t, dir, fast) // the third contender wins the lock
	close(gate)
	if err := <-done; err != nil {
		t.Fatalf("B across the takeover: %v", err)
	}
	if got := recC.requests(); len(got) != 1 || !equalRequest(got[0], reqB) {
		t.Fatalf("C got %+v, want B's request unchanged %+v", got, reqB)
	}
	if n := len(recA.requests()); n != 0 {
		t.Errorf("closing A queued %d requests", n)
	}
}

// TestHandoffFallbackCreatedLate: a launch that finds no socket candidate yet (a profile path too
// long for sun_path, no XDG_RUNTIME_DIR, the private fallback folder not made yet) computes them
// again on each try, and reaches the primary once it made the folder and listens.
func TestHandoffFallbackCreatedLate(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", profile(t)) // no inkwell-<uid> in it yet
	dir := filepath.Join(profile(t), strings.Repeat("p", maxSunPath))
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := socketPaths(dir, runtimeDir(false)); len(got) != 0 {
		t.Fatalf("candidates before the primary listens: %q, want none", got)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := tryLock(f); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		p, err := Start(context.Background(), dir, request("late.typ"), Options{Deadline: 5 * time.Second})
		if p != nil {
			err = errors.New("became the primary")
		}
		done <- err
	}()
	time.Sleep(150 * time.Millisecond)
	p, err := listen(root, f, dir, fast.withDefaults())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	rec := &recorder{}
	p.Serve(rec.handle)
	if err := <-done; err != nil {
		t.Fatalf("launch racing the primary's fallback folder: %v", err)
	}
	if len(rec.requests()) != 1 {
		t.Errorf("primary got %d requests", len(rec.requests()))
	}
}

// fakePrimary holds the lock of dir and answers every connection with the byte reply after
// reading the frame.
func fakePrimary(t *testing.T, dir string, reply byte) {
	t.Helper()
	holdLock(t, dir)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, sockName), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = readRequest(c)
			_, _ = c.Write([]byte{reply})
			_ = c.Close()
		}
	}()
}

// send writes raw bytes to the primary of dir, ends the write side, and returns the reply bytes. A
// primary that refuses the peer may have closed the connection before the write: that is no error.
func send(t *testing.T, dir string, raw []byte) []byte {
	t.Helper()
	c, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: filepath.Join(dir, sockName), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = c.Write(raw)
	_ = c.CloseWrite()
	b, _ := io.ReadAll(c)
	return b
}

// TestHandoffRejectsMalformed: every malformed frame gets « malformed » (« unsupported » for
// another version), the handler is not called and the connection is closed (F9); a launch told
// « malformed » stops at once instead of retrying.
func TestHandoffRejectsMalformed(t *testing.T) {
	dir := profile(t)
	p, err := Start(t.Context(), dir, request(), fast)
	if err != nil || p == nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	p.Serve(func(r Request) bool { t.Errorf("handler called with %+v", r); return true })
	for _, tc := range loadVectors(t).Rejects {
		if strings.HasSuffix(tc.Name, "after the frame") {
			continue // on a connection, the primary reads one frame and replies: what follows is not read
		}
		want := byte(StatusMalformed)
		if tc.Status == "unsupported" {
			want = byte(StatusUnsupported)
		}
		if got := send(t, dir, mustHex(t, tc.Hex)); len(got) != 1 || got[0] != want {
			t.Errorf("%s: reply %x, want %02x", tc.Name, got, want)
		}
	}
	other := profile(t)
	fakePrimary(t, other, byte(StatusMalformed))
	start := time.Now()
	if q, err := Start(t.Context(), other, request(), fast); q != nil || !errors.Is(err, errRefused) {
		t.Errorf("Start against « malformed » = %v, %v; want errRefused", q, err)
	}
	if d := time.Since(start); d > fast.Deadline/2 {
		t.Errorf("« malformed » was retried for %v", d)
	}
}

// settle waits up to 2 s for the number of goroutines to come back to n.
func settle(n int) int {
	got := runtime.NumGoroutine()
	for end := time.Now().Add(2 * time.Second); got > n && time.Now().Before(end); got = runtime.NumGoroutine() {
		time.Sleep(10 * time.Millisecond)
	}
	return got
}

// TestHandoffSlowClient: a connection that sends nothing, or half a frame, is closed after the
// read timeout without calling the handler, and leaves no goroutine behind (F9).
func TestHandoffSlowClient(t *testing.T) {
	base := runtime.NumGoroutine()
	dir := profile(t)
	p, err := Start(t.Context(), dir, request(), Options{ReadTimeout: 200 * time.Millisecond})
	if err != nil || p == nil {
		t.Fatal(err)
	}
	var called atomic.Int32
	p.Serve(func(Request) bool { called.Add(1); return true })
	serving := runtime.NumGoroutine()
	frame, _ := request("x.typ").Encode()
	for _, raw := range [][]byte{nil, frame[:3], frame[:len(frame)-1]} {
		// The clock starts before Dial: the primary arms its deadline after Accept, which can run
		// before a descheduled client reads the clock once Dial has returned (CI run 45138932780
		// measured 199.92 ms). Before Dial, start precedes the arming, so the bound is exact.
		start := time.Now()
		c, err := net.Dial("unix", filepath.Join(dir, sockName))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = c.Write(raw)
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		b, _ := io.ReadAll(c)
		_ = c.Close()
		if d := time.Since(start); d < 200*time.Millisecond || d > 1500*time.Millisecond {
			t.Errorf("%d bytes sent: closed after %v, want after the 200 ms read timeout", len(raw), d)
		}
		if len(b) > 1 {
			t.Errorf("reply %x", b)
		}
	}
	if got := settle(serving); got > serving {
		t.Errorf("goroutines: %d while serving, %d after the slow clients", serving, got)
	}
	if called.Load() != 0 {
		t.Error("handler called for an incomplete frame")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if got := settle(base); got > base {
		t.Errorf("goroutines: %d before Start, %d after Close", base, got)
	}
}

// TestHandoffPeerCheck: each end refuses a peer of another user before reading or writing a byte
// (F8): the primary closes the connection, the launch does not send its request.
func TestHandoffPeerCheck(t *testing.T) {
	other := func(*net.UnixConn) (int, error) { return os.Getuid() + 1, nil }
	failing := func(*net.UnixConn) (int, error) { return 0, errors.New("no credentials") }
	for name, peer := range map[string]func(*net.UnixConn) (int, error){"other user": other, "unreadable": failing} {
		t.Run("primary refuses, "+name, func(t *testing.T) {
			dir := profile(t)
			o := fast
			o.peer = peer
			_, rec := primary(t, dir, o)
			p, err := Start(t.Context(), dir, request(), Options{Deadline: 300 * time.Millisecond})
			if p != nil || !errors.Is(err, ErrNotResponding) {
				t.Errorf("Start = %v, %v; want ErrNotResponding", p, err)
			}
			if got := send(t, dir, nil); len(got) != 0 {
				t.Errorf("primary wrote %x to a refused peer", got)
			}
			if len(rec.requests()) != 0 {
				t.Error("handler called for a refused peer")
			}
		})
		t.Run("launch refuses, "+name, func(t *testing.T) {
			dir := profile(t)
			_, rec := primary(t, dir, fast)
			o := fast
			o.peer = peer
			start := time.Now()
			p, err := Start(t.Context(), dir, request(), o)
			if p != nil || !errors.Is(err, errPeer) {
				t.Errorf("Start = %v, %v; want errPeer", p, err)
			}
			if d := time.Since(start); d > fast.Deadline/2 {
				t.Errorf("refused after %v: retried", d)
			}
			if len(rec.requests()) != 0 {
				t.Error("request sent to a refused peer")
			}
		})
	}
	// The real credentials of a connection of this process are this user's.
	dir := profile(t)
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "s"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	c, err := net.DialUnix("unix", nil, ln.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	s, err := ln.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	for _, end := range []*net.UnixConn{c, s} {
		if uid, err := peerUID(end); err != nil || uid != os.Getuid() {
			t.Errorf("peerUID = %d, %v; want %d", uid, err, os.Getuid())
		}
	}
}

// TestHandoffOtherVersion: a primary of another version answers « unsupported », which stops the
// launch with ErrOtherVersion (R17, F14), as does a reply this version does not know.
func TestHandoffOtherVersion(t *testing.T) {
	dir := profile(t)
	_, rec := primary(t, dir, fast)
	frame, _ := request("v2.typ").Encode()
	frame[4] = 2
	if got := send(t, dir, frame); len(got) != 1 || got[0] != byte(StatusUnsupported) {
		t.Errorf("reply to version 2: %x", got)
	}
	if len(rec.requests()) != 0 {
		t.Error("a version 2 request was queued")
	}
	for _, reply := range []byte{byte(StatusUnsupported), 9} {
		other := profile(t)
		fakePrimary(t, other, reply)
		if p, err := Start(t.Context(), other, request(), fast); p != nil || !errors.Is(err, ErrOtherVersion) {
			t.Errorf("reply %d: Start = %v, %v; want ErrOtherVersion", reply, p, err)
		}
	}
}

// TestHandoffListensUnixOnly: the primary listens on a Unix-domain socket file in the profile,
// never on a network port (ADR 0037).
func TestHandoffListensUnixOnly(t *testing.T) {
	dir := profile(t)
	p, _ := primary(t, dir, fast)
	if got := p.Addr().Network(); got != "unix" {
		t.Errorf("listens on %q, want unix", got)
	}
	info, err := os.Lstat(p.Addr().String())
	if err != nil || info.Mode().Type() != os.ModeSocket {
		t.Errorf("socket file: %v, %v", info, err)
	}
	if !strings.HasPrefix(p.Addr().String(), dir+string(filepath.Separator)) {
		t.Errorf("socket %q outside the profile", p.Addr())
	}
}

// TestSocketPathFallback: a profile path too long for sun_path uses the runtime folder; a runtime
// folder that is not private to the user is skipped (F7, F8).
func TestSocketPathFallback(t *testing.T) {
	runtime := profile(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	long := filepath.Join(profile(t), strings.Repeat("p", maxSunPath))
	if err := os.Mkdir(long, 0o700); err != nil {
		t.Fatal(err)
	}
	p, rec := primary(t, long, fast)
	if got, want := p.Addr().String(), socketPaths(long, runtime)[0]; got != want || !strings.HasPrefix(got, runtime) {
		t.Errorf("listens on %q, want %q in the runtime folder", got, want)
	}
	if q, err := Start(t.Context(), long, request("long.typ"), fast); q != nil || err != nil {
		t.Fatalf("hand-off through the runtime folder = %v, %v", q, err)
	}
	if len(rec.requests()) != 1 {
		t.Errorf("primary got %d requests", len(rec.requests()))
	}

	uid := os.Getuid()
	tmp := profile(t)
	private := profile(t)
	open := profile(t)
	if err := os.Chmod(open, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(profile(t), "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(tmp, "inkwell-"+strconv.Itoa(uid))
	cases := []struct {
		name   string
		xdg    string
		uid    int
		create bool
		want   string
	}{
		{"private XDG_RUNTIME_DIR", private, uid, false, private},
		{"open XDG_RUNTIME_DIR, no fallback yet", open, uid, false, ""},
		{"open XDG_RUNTIME_DIR, fallback made", open, uid, true, fallback},
		{"fallback already there", "", uid, false, fallback},
		{"symlinked XDG_RUNTIME_DIR", link, uid, false, fallback},
		{"relative XDG_RUNTIME_DIR", "run/user", uid, false, fallback},
		{"another user's folders", private, uid + 1, true, ""},
	}
	for _, tc := range cases {
		if got := runtimeDirFrom(tc.xdg, tmp, tc.uid, tc.create); got != tc.want {
			t.Errorf("%s: runtimeDirFrom = %q, want %q", tc.name, got, tc.want)
		}
	}
	if info, err := os.Lstat(fallback); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("fallback folder: %v, %v", info, err)
	}
	// A squatted fallback (open, or a symlink) is never used.
	for _, squat := range []func(string) error{
		func(p string) error { return os.Chmod(p, 0o755) },
		func(p string) error { _ = os.Remove(p); return os.Symlink(private, p) },
	} {
		if err := squat(fallback); err != nil {
			t.Fatal(err)
		}
		if got := runtimeDirFrom("", tmp, uid, true); got != "" {
			t.Errorf("squatted fallback used: %q", got)
		}
	}
	// A relative TMPDIR would make the socket depend on the working directory: never used, even
	// when the folder exists there.
	t.Chdir(profile(t))
	if err := os.MkdirAll(filepath.Join("rel", "inkwell-"+strconv.Itoa(uid)), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := runtimeDirFrom("", "rel", uid, true); got != "" {
		t.Errorf("relative temporary folder used: %q", got)
	}
	if got := runtimeDir(false); got != runtime {
		t.Errorf("runtimeDir = %q, want $XDG_RUNTIME_DIR %q", got, runtime)
	}
}

// unbindable makes path impossible to bind or to remove, whoever runs the test (root included): a
// folder that is not empty, where a socket file should go.
func unbindable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o700); err != nil {
		t.Fatal(err)
	}
}

// TestSocketBindFallback: when the profile folder cannot hold a socket (a network or FUSE
// filesystem), the primary binds the runtime candidate and launches find it there (F7); when no
// candidate can be bound, there is no primary and the lock is let go.
func TestSocketBindFallback(t *testing.T) {
	runtime := profile(t)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	dir := profile(t)
	paths := socketPaths(dir, runtime)
	unbindable(t, paths[0])
	p, rec := primary(t, dir, fast)
	if got := p.Addr().String(); got != paths[1] {
		t.Errorf("listens on %q, want the runtime candidate %q", got, paths[1])
	}
	if q, err := Start(t.Context(), dir, request("fallback.typ"), fast); q != nil || err != nil {
		t.Fatalf("hand-off = %v, %v", q, err)
	}
	if len(rec.requests()) != 1 {
		t.Errorf("primary got %d requests", len(rec.requests()))
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(paths[1]); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("runtime socket left after Close: %v", err)
	}
	unbindable(t, paths[1])
	for range 2 {
		if q, err := Start(t.Context(), dir, request(), fast); q != nil || err == nil {
			t.Errorf("Start with no socket possible = %v, %v; want an error", q, err)
		}
	}
}

// TestSocketPathVectors pins the candidate rule both ends compute (testdata/handoff-v1.json).
func TestSocketPathVectors(t *testing.T) {
	for _, tc := range loadVectors(t).Sockets {
		if got := socketPaths(tc.Profile, tc.Runtime); !slices.Equal(got, tc.Candidates) {
			t.Errorf("socketPaths(%q, %q) = %q, want %q", tc.Profile, tc.Runtime, got, tc.Candidates)
		}
	}
}

// TestBackoffSchedule: 10 ms, doubling, at most 200 ms; never zero, never negative.
func TestBackoffSchedule(t *testing.T) {
	ms := time.Millisecond
	want := []time.Duration{10 * ms, 20 * ms, 40 * ms, 80 * ms, 160 * ms, 200 * ms, 200 * ms}
	for i, w := range want {
		if got := backoff(i); got != w {
			t.Errorf("backoff(%d) = %v, want %v", i, got, w)
		}
	}
	for _, n := range []int{-1, 63, 64, 1 << 30} {
		if got := backoff(n); got < 10*ms || got > 200*ms {
			t.Errorf("backoff(%d) = %v", n, got)
		}
	}
	if backoff(-1) != 10*ms {
		t.Errorf("backoff(-1) = %v", backoff(-1))
	}
	var total time.Duration
	tries := 0
	for total < DefaultDeadline {
		total += backoff(tries)
		tries++
	}
	if tries > 30 {
		t.Errorf("%d tries in the 5 s deadline, want at most 30", tries)
	}
	d := Options{}.withDefaults()
	if d.Deadline != 5*time.Second || d.ClosingWait != 15*time.Second || d.ReadTimeout != 2*time.Second {
		t.Errorf("defaults %v %v %v", d.Deadline, d.ClosingWait, d.ReadTimeout)
	}
	o := Options{Deadline: time.Second, ClosingWait: 2 * time.Second, ReadTimeout: 3 * time.Second}.withDefaults()
	if o.Deadline != time.Second || o.ClosingWait != 2*time.Second || o.ReadTimeout != 3*time.Second {
		t.Errorf("set options overridden: %v %v %v", o.Deadline, o.ClosingWait, o.ReadTimeout)
	}
}
