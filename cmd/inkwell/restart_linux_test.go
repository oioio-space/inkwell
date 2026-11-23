package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/sidecar"
)

func init() {
	childModes["exec"] = execChild
	childModes["barrier"] = barrierChild
}

// imageLine is what a helper image prints about itself.
func imageLine() string {
	return fmt.Sprintf("image %s pid %d args %q after %q", (&app.Session{}).StartupID(), os.Getpid(), os.Args, os.Getenv(afterTakeOverEnv))
}

// execChild prints its image line; unless it is the restarted image, it waits for a line on its
// standard input, then restarts on $INKWELL_CHILD_ARGS.
func execChild() int {
	fmt.Println(imageLine())
	if os.Getenv(afterTakeOverEnv) == "1" {
		return 0
	}
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	err := restartImage(os.Getenv("INKWELL_CHILD_ARGS"))
	fmt.Println("restart failed:", err)
	return 1
}

var imageRE = regexp.MustCompile(`^image ([0-9a-f]{32}) pid (\d+) args (.*) after "(.*)"$`)

// TestRestartExecsTheRunningImage: the restart executes the running image itself, even when its
// file was replaced, then deleted, by an upgrade: the same pid runs a new image, with a new
// startup id, the rebuilt arguments and the marker (rule N-12).
func TestRestartExecsTheRunningImage(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "inkwell")
	self, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, self, 0o700); err != nil { // #nosec G306 -- an executable for the test
		t.Fatal(err)
	}
	folder := t.TempDir()
	cmd := exec.Command(bin, "-test.run=^$") // #nosec G204 -- a copy of the test binary
	cmd.Env = cleanEnv(childEnv+"=exec", "INKWELL_CHILD_ARGS="+folder, afterTakeOverEnv+"=stale")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	lines := bufio.NewScanner(stdout)
	first := nextImage(t, lines)

	// The upgrade: another file takes the path, then the path goes.
	if err := os.WriteFile(bin+".new", []byte("#!/bin/sh\necho not the image\n"), 0o700); err != nil { // #nosec G306
		t.Fatal(err)
	}
	if err := os.Rename(bin+".new", bin); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bin); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(stdin, "restart\n"); err != nil {
		t.Fatal(err)
	}
	second := nextImage(t, lines)
	if second[1] == first[1] || second[2] != first[2] {
		t.Errorf("N-12 the running image restarts in place: before id %s pid %s, after id %s pid %s", first[1], first[2], second[1], second[2])
	}
	if want := fmt.Sprintf("%q", []string{"inkwell", folder}); second[3] != want || second[4] != "1" {
		t.Errorf("N-12 the restarted image: args %s after %q, want %s after \"1\"", second[3], second[4], want)
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("the restarted image: %v", err)
	}
}

func nextImage(t *testing.T, lines *bufio.Scanner) []string {
	t.Helper()
	for lines.Scan() {
		if m := imageRE.FindStringSubmatch(lines.Text()); m != nil {
			return m
		}
		t.Logf("child: %s", lines.Text())
	}
	t.Fatalf("N-12 the running image restarts in place: no image line (%v)", lines.Err())
	return nil
}

// census counts the children of this process, and its sockets and descriptors beyond the standard
// three; with inherited, only those without close-on-exec, which an image inherited: Go opens
// every descriptor of its own with it (the runtime's poller and cgroup file of a fresh image are
// its own). It lists /proc/self/fd with raw system calls and skips its own descriptor.
func census(inherited bool) (kids, sockets, fds int, names []string) {
	k, _ := sidecar.Children()
	dir, err := syscall.Open("/proc/self/fd", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return len(k), -1, -1, nil
	}
	defer func() { _ = syscall.Close(dir) }()
	buf := make([]byte, 1<<16)
	var all []string
	for {
		n, err := syscall.ReadDirent(dir, buf)
		if err != nil || n <= 0 {
			break
		}
		_, _, all = syscall.ParseDirent(buf[:n], -1, all)
	}
	for _, name := range all {
		fd, err := strconv.Atoi(name)
		if err != nil || fd <= 2 || fd == dir {
			continue
		}
		target, _ := os.Readlink("/proc/self/fd/" + name)
		if inherited && closeOnExec(name) {
			continue
		}
		if strings.HasPrefix(target, "socket:") {
			sockets++
		}
		fds++
		names = append(names, target)
	}
	return len(k), sockets, fds, names
}

// closeOnExec reports whether the descriptor named fd has O_CLOEXEC in its fdinfo flags.
func closeOnExec(fd string) bool {
	b, err := os.ReadFile("/proc/self/fdinfo/" + fd)
	if err != nil {
		return false
	}
	for line := range strings.Lines(string(b)) {
		if v, ok := strings.CutPrefix(line, "flags:"); ok {
			flags, err := strconv.ParseUint(strings.TrimSpace(v), 8, 64)
			return err == nil && flags&syscall.O_CLOEXEC != 0
		}
	}
	return false
}

// lspConn is the page's end of the language stream: the test writes frames to in, and the
// service's frames arrive on out. It ends when closed.
type lspConn struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     chan []byte
	out    chan []byte
}

func (c *lspConn) Send(b []byte) error {
	select {
	case c.out <- b:
	default: // the census needs only the first answer
	}
	return nil
}
func (c *lspConn) Context() context.Context { return c.ctx }
func (c *lspConn) Close() error             { c.cancel(); return nil }
func (c *lspConn) Receive() ([]byte, error) {
	select {
	case b := <-c.in:
		return b, nil
	case <-c.ctx.Done():
		return nil, io.EOF
	}
}

// barrierChild is a real launch (live deps) whose window starts the language service, prints the
// census once its child runs, starts a helper the session does not control, and returns with a
// recorded change; it prints what the shutdown left to the barrier, and the restarted image
// prints its own census at its start and exits.
func barrierChild() int {
	if os.Getenv(afterTakeOverEnv) == "1" {
		kids, sockets, fds, names := census(true)
		fmt.Printf("after: children %d sockets %d descriptors %d %q\n", kids, sockets, fds, names)
		return 0
	}
	d := liveDeps(nil)
	d.window = func(_ context.Context, s *session) int {
		// As a Wails stream, the language stream lives apart from the window's context: only the
		// shutdown ends it. tinymist runs once it answered « initialize ».
		ctx, cancel := context.WithCancel(context.Background())
		c := &lspConn{ctx: ctx, cancel: cancel, in: make(chan []byte, 1), out: make(chan []byte, 1)}
		go s.language.Serve(c)
		c.in <- []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"capabilities":{}}}`)
		select {
		case b := <-c.out:
			fmt.Printf("language service answered %d bytes\n", len(b))
		case <-time.After(30 * time.Second):
			fmt.Println("language service did not answer")
		}
		kids, sockets, fds, _ := census(false)
		pids, _ := sidecar.Children()
		for _, pid := range pids {
			b, _ := os.ReadFile("/proc/" + pid + "/cmdline")
			fmt.Printf("child %s: %q\n", pid, strings.ReplaceAll(string(b), "\x00", " "))
		}
		_, _, open, names := census(true)
		fmt.Printf("before: children %d sockets %d descriptors %d (without close-on-exec: %d %q)\n", kids, sockets, fds, open, names)
		// A helper the session does not control, like the web view's: it ends when asked and
		// leaves a process behind that ignores the request, like a sandbox holder and its workload.
		helper := exec.Command("/bin/sh", "-c", `(trap "" TERM; exec sleep 600) & echo $!; trap 'exit 0' TERM; echo ready; wait`) // #nosec G204 -- a fixed test script
		lines, err := helper.StdoutPipe()
		if err == nil {
			err = helper.Start()
		}
		if err != nil {
			fmt.Println("no foreign helper:", err)
			return exitError
		}
		sc := bufio.NewScanner(lines)
		sc.Scan()
		left := sc.Text()
		sc.Scan() // « ready »: its trap is set
		fmt.Printf("foreign: helper %d left %s\n", helper.Process.Pid, left)
		return exitOK
	}
	d.restartChange = func(*app.Runtime) string { return app.ChangeTakeOver }
	restart := d.restart
	d.restart = func(dir string) error {
		pids, _ := sidecar.Children()
		fmt.Printf("at the barrier: children %v\n", pids) // what the shutdown left running
		return restart(dir)
	}
	return startup(nil, d)
}

var censusRE = regexp.MustCompile(`(?m)^(before|after): children (\d+) sockets (\d+) descriptors (\d+)`)

var (
	foreignRE   = regexp.MustCompile(`(?m)^foreign: helper (\d+) left (\d+)$`)
	atBarrierRE = regexp.MustCompile(`(?m)^at the barrier: children \[([\d ]*)\]$`)
)

// TestRestartBarrier: a real launch with the language service (its tinymist child), the network
// listeners, the instance socket and a helper it does not control restarts. The shutdown ends the
// session's own services; the barrier ends what remains, the process the helper leaves behind
// included; the new image starts with no descendant, no socket and no inherited descriptor beyond
// the standard three (rule N-14).
func TestRestartBarrier(t *testing.T) {
	home, docs := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$") // #nosec G204 -- the test binary itself
	cmd.Env = cleanEnv(childEnv+"=barrier", "INKWELL_HOME="+home, "INKWELL_DOCS="+docs, "INKWELL_RENDEZVOUS=off")
	out, err := cmd.CombinedOutput()
	got := map[string][]int{}
	for _, m := range censusRE.FindAllStringSubmatch(string(out), -1) {
		for _, n := range m[2:] {
			v, _ := strconv.Atoi(n)
			got[m[1]] = append(got[m[1]], v)
		}
	}
	before, after := got["before"], got["after"]
	t.Logf("census (children, sockets, descriptors): before %v, after the restart %v\n%s", before, after, out)
	if len(before) != 3 || before[0] == 0 || before[1] == 0 || !strings.Contains(string(out), "language service answered") {
		t.Fatalf("the launch did not start its language service and listeners: census %v (exit %v)\n%s", before, err, out)
	}
	foreign := foreignRE.FindStringSubmatch(string(out))
	if foreign == nil {
		t.Fatalf("the launch did not start its foreign helper (exit %v)\n%s", err, out)
	}
	t.Cleanup(func() { // whatever the outcome, the helper does not outlive the test
		for _, pid := range foreign[1:] {
			if n, err := strconv.Atoi(pid); err == nil {
				_ = syscall.Kill(n, syscall.SIGKILL)
			}
		}
	})
	// The session's own services ended in the shutdown: only the helper it does not control is
	// left to the barrier.
	if m := atBarrierRE.FindStringSubmatch(string(out)); m == nil || m[1] != foreign[1] {
		t.Fatalf("N-14 no child survives: the shutdown left %v to the barrier, want only the foreign helper %s\n%s", m, foreign[1], out)
	}
	if strings.Contains(string(out), "still running") {
		t.Fatalf("N-14 remaining children are ended: the barrier gave up (exit %v)\n%s", err, out)
	}
	if err != nil || len(after) != 3 {
		t.Fatalf("N-14 no child survives: no restarted image (exit %v)\n%s", err, out)
	}
	for _, pid := range foreign[1:] {
		if _, err := os.Stat("/proc/" + pid); err == nil {
			t.Errorf("N-14 remaining children are ended: process %s outlived the image\n%s", pid, out)
		}
	}
	if after[0] != 0 || after[1] != 0 || after[2] != 0 {
		t.Errorf("N-14 no child survives: the new image inherited children %d, sockets %d, descriptors %d\n%s", after[0], after[1], after[2], out)
	}
}

func init() { childModes["firstjoin"] = firstJoinChild }

// firstJoinChild is a real launch on a new profile whose window, in place of the page, accepts
// the invitation of $INKWELL_CHILD_TICKET and finishes the join; the image the join restarts
// prints what it loaded and ends.
func firstJoinChild() int {
	restarted := os.Getenv(afterTakeOverEnv) == "1"
	d := liveDeps(nil)
	d.window = func(ctx context.Context, s *session) int {
		sess := s.rt.Session
		line := func(when string) {
			pending := sess.PendingJoin()
			fmt.Printf("%s: image %s pid %d members %d waiting %v inviter %q folder %q\n", when, sess.StartupID(), os.Getpid(), len(sess.Members()), pending.Waiting, pending.Inviter, s.proj.Path())
		}
		if restarted {
			line("second")
			return exitOK
		}
		line("first")
		quit := make(chan struct{})
		s.rt.SetQuit(func() { close(quit) })
		res, err := sess.Join(ctx, os.Getenv("INKWELL_CHILD_TICKET"))
		fmt.Printf("join: restart %v err %v\n", res.Restart, err)
		line("held")
		if err := sess.FinishJoin(ctx); err != nil {
			fmt.Println("finish:", err)
			return exitError
		}
		select {
		case <-quit:
			return exitOK
		case <-time.After(10 * time.Second):
			fmt.Println("finish: the application was not asked to quit")
			return exitError
		}
	}
	if restarted {
		return startup(os.Args[1:], d) // the folder the restart gave
	}
	return startup(nil, d)
}

var firstJoinRE = regexp.MustCompile(`(?m)^(first|held|second): image ([0-9a-f]{32}) pid (\d+) members (\d+) waiting (true|false) inviter "(.*)" folder "(.*)"$`)

// TestFirstLaunchJoinRestarts: a profile with no project accepts an invitation at its first
// launch. Nothing is adopted in that image: its member list stays its own until the end; the
// process restarts in place, with another startup id, on the folder of the joined document,
// beside the one the first launch created, and waits there for the document of the inviter
// (rule J-3: one rule, no live case).
func TestFirstLaunchJoinRestarts(t *testing.T) {
	hostHome, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hostHome.Close() }()
	hostDocs, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = hostDocs.Close() }()
	proj, err := project.Create(hostDocs, "Roman", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proj.Close() }()
	host, err := app.Start(t.Context(), hostHome, app.Options{Project: proj, RendezvousURL: app.NoCodes})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = host.Close() }()
	inv, err := host.Session.Invite("writer")
	if err != nil || inv.Text == "" {
		t.Fatalf("Invite = %+v, %v", inv, err)
	}

	home, docs := t.TempDir(), t.TempDir() // a new profile and an empty documents folder
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$") // #nosec G204 -- the test binary itself
	cmd.Env = cleanEnv(childEnv+"=firstjoin", "INKWELL_HOME="+home, "INKWELL_DOCS="+docs, "INKWELL_RENDEZVOUS=off", "INKWELL_CHILD_TICKET="+inv.Text)
	out, err := cmd.CombinedOutput()
	got := map[string][]string{}
	for _, m := range firstJoinRE.FindAllStringSubmatch(string(out), -1) {
		got[m[1]] = m[2:]
	}
	first, held, second := got["first"], got["held"], got["second"]
	if err != nil || first == nil || held == nil || second == nil || !strings.Contains(string(out), "join: restart true err <nil>") {
		t.Fatalf("J-3 first launch restarts: no restarted image after the join (exit %v)\n%s", err, out)
	}
	// image, pid, members, waiting, inviter, folder
	if held[0] != first[0] || held[2] != "1" || held[3] != "false" || held[5] != first[5] {
		t.Errorf("J-3 first launch restarts: the first image adopted the project: members %s, waiting %s, folder %q after the Welcome\n%s", held[2], held[3], held[5], out)
	}
	if second[0] == first[0] || second[1] != first[1] {
		t.Errorf("J-3 first launch restarts: the startup id changes in the same process: before id %s pid %s, after id %s pid %s", first[0], first[1], second[0], second[1])
	}
	want := filepath.Join(docs, "Roman")
	if second[5] != want || second[5] == first[5] || second[2] != "2" || second[3] != "true" || second[4] != host.Session.LocalPeer() {
		t.Errorf("J-3 first launch restarts: the restarted image is on %q with %s members, waiting %s for %q; want %q, 2 members, waiting for the inviter\n%s", second[5], second[2], second[3], second[4], want, out)
	}
	entries, err := os.ReadDir(docs)
	if err != nil || len(entries) != 2 {
		t.Errorf("J-3 first launch restarts: the documents folder holds %v (%v), want the first folder and the joined one", entries, err)
	}
}
