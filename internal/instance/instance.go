// Package instance keeps one Inkwell process per profile folder and hands a later launch's
// request to it (ADR 0037, plan docs/plans/2026-11-11-oi-single-instance-open.md section 4.1;
// model docs/protocol/tla/OISingleInstance.tla).
//
// The process that takes the kernel lock of <profile>/instance.lock is the primary: it removes
// any leftover socket file, listens on a Unix-domain socket beside the lock (or in a private
// runtime folder when that path is too long or cannot be bound), and serves requests until it
// closes. A launch that does not get the lock sends its request to that socket and is done once
// the primary acknowledges it, which the primary does once it has received and queued the request:
// from then on the request is the primary's, whose window shows the file or says why it cannot (a
// crash of the primary in between loses it: an accepted residual). The lock, not the
// socket, decides who is primary, and the kernel releases it when its holder dies: a crash needs
// no cleanup and no staleness heuristic.
//
// Both ends check that the other runs as the same user (peer credentials) before reading or
// writing; nothing is logged.
//
//	p, err := instance.Start(ctx, profile, req, instance.Options{})
//	switch {
//	case err != nil: // ErrNotResponding, ErrOtherVersion, ErrNoLock or an I/O error
//	case p == nil:   // handed over: exit 0
//	default:         // primary: p.Serve(queue.Push); …; p.Closing(); …; p.Close()
//	}
package instance

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	// ErrNotResponding means that the lock is held, but its holder gave no answer before the deadline
	// (R11).
	ErrNotResponding = errors.New("instance: the running Inkwell does not answer")
	// ErrOtherVersion means that the holder speaks another version of the hand-off (R17).
	ErrOtherVersion = errors.New("instance: another version of Inkwell is running")
	// ErrNoLock means that the profile folder cannot be locked (ENOLCK, EOPNOTSUPP): Inkwell must not start,
	// or two processes could share one identity (R18).
	ErrNoLock = errors.New("instance: the profile folder cannot be locked")

	// errHeld is a lock held by another process.
	errHeld = errors.New("instance: lock held")
	// errPeer is a socket whose other end runs as another user.
	errPeer = errors.New("instance: the socket belongs to another user")
	// errRefused is a request the primary called malformed: a bug, not retried.
	errRefused = errors.New("instance: the running Inkwell refused the request as malformed")
)

// Defaults of Options (plan section 4.1, table « Numbers »).
const (
	// DefaultDeadline bounds a hand-off while the lock is held: under the 10 s limit of
	// attention, far above a local round trip.
	DefaultDeadline = 5 * time.Second
	// DefaultClosingWait bounds the wait for a closing primary to release the lock: three times
	// the 5 s that bound its shutdown loops.
	DefaultClosingWait = 15 * time.Second
	// DefaultReadTimeout bounds one connection on the primary's side.
	DefaultReadTimeout = 2 * time.Second

	// maxConns is the connections served at once: Explorer starts at most 15 processes for one
	// selection.
	maxConns = 16
	// recentIDs is how many request IDs the primary remembers to ignore a repeat.
	recentIDs = 64
	// firstBackoff and maxBackoff bound the wait between two tries of the lock.
	firstBackoff = 10 * time.Millisecond
	maxBackoff   = 200 * time.Millisecond
)

// Options tunes Start; the zero value means the defaults. Tests shorten the durations.
type Options struct {
	// Deadline bounds a hand-off to a holder that does not answer (DefaultDeadline).
	Deadline time.Duration
	// ClosingWait bounds the wait once the holder answered « closing » or « busy »: it is alive,
	// closing, or not ready to queue yet (DefaultClosingWait).
	ClosingWait time.Duration
	// ReadTimeout bounds one connection on the primary's side (DefaultReadTimeout).
	ReadTimeout time.Duration

	// lock and peer replace tryLock and peerUID in tests.
	lock func(*os.File) error
	peer func(*net.UnixConn) (int, error)
}

func (o Options) withDefaults() Options {
	if o.Deadline <= 0 {
		o.Deadline = DefaultDeadline
	}
	if o.ClosingWait <= 0 {
		o.ClosingWait = DefaultClosingWait
	}
	if o.ReadTimeout <= 0 {
		o.ReadTimeout = DefaultReadTimeout
	}
	if o.lock == nil {
		o.lock = tryLock
	}
	if o.peer == nil {
		o.peer = peerUID
	}
	return o
}

// sameUser reports whether the other end of c runs as this process's user.
func (o Options) sameUser(c *net.UnixConn) bool {
	uid, err := o.peer(c)
	return err == nil && uid == os.Getuid()
}

// backoff is the wait before try attempt+1 of the lock: 10 ms, doubling, at most 200 ms (rule
// D-RETRY: every try goes back to the lock, so the crash of its holder makes the next try a
// primary).
func backoff(attempt int) time.Duration {
	return min(firstBackoff<<min(max(attempt, 0), 5), maxBackoff)
}

// Start claims the profile folder dir, which the caller created private to the user, or hands
// req to the process that holds it. Exactly one of: (p, nil), this process is the primary and
// must keep p until it exits; (nil, nil), the primary acknowledged req once it queued it; (nil,
// err), err wrapping ErrNotResponding, ErrOtherVersion or ErrNoLock, or another error.
func Start(ctx context.Context, dir string, req Request, o Options) (*Primary, error) {
	o = o.withDefaults()
	frame, err := req.Encode()
	if err != nil {
		return nil, err
	}
	dir, err = canonical(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("instance: %w", err)
	}
	// Go opens every file close-on-exec: typst and tinymist never inherit the lock.
	f, err := root.OpenFile(lockName, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("instance: %w", err), root.Close())
	}
	p, err := claim(ctx, root, f, dir, frame, o)
	if p == nil {
		err = errors.Join(err, f.Close(), root.Close())
	}
	return p, err
}

// canonical returns the absolute path of dir with its symlinks resolved: both ends must name the
// same runtime socket for one profile.
func canonical(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("instance: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("instance: %w", err)
	}
	return resolved, nil
}

// claim is the loop of the model: try the lock; held, hand the frame over and act on the reply.
func claim(ctx context.Context, root *os.Root, f *os.File, dir string, frame []byte, o Options) (*Primary, error) {
	limit := time.Now().Add(o.Deadline)
	answered := false
	for attempt := 0; ; attempt++ {
		err := o.lock(f)
		if err == nil {
			return listen(root, f, dir, o)
		}
		if !errors.Is(err, errHeld) {
			return nil, err
		}
		// The candidates are computed on every try: the private fallback folder may appear once the
		// primary made it, after this launch started.
		st, err := handOff(ctx, socketPaths(dir, runtimeDir(false)), frame, limit, o)
		switch {
		case errors.Is(err, errPeer), ctx.Err() != nil:
			return nil, errors.Join(err, ctx.Err())
		case err != nil:
			// No socket, refused, reset or timed out: the holder may be starting, crashed or hung.
		case st == StatusAccepted:
			return nil, nil
		case st == StatusClosing, st == StatusBusy:
			// The holder answers: it is closing, or not ready to queue (starting, or queue full).
			if !answered {
				answered = true
				limit = time.Now().Add(o.ClosingWait)
			}
		case st == StatusMalformed:
			return nil, errRefused
		default:
			return nil, fmt.Errorf("%w (%v)", ErrOtherVersion, st)
		}
		if !time.Now().Before(limit) {
			return nil, ErrNotResponding
		}
		select {
		case <-time.After(min(backoff(attempt), time.Until(limit))):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// handOff sends frame to the first candidate that accepts a connection and returns the reply.
// An error means no answer: no candidate listens, the connection broke, or limit passed.
func handOff(ctx context.Context, paths []string, frame []byte, limit time.Time, o Options) (Status, error) {
	d := net.Dialer{Deadline: limit}
	var errs []error
	for _, p := range paths {
		c, err := d.DialContext(ctx, "unix", p)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		return exchange(c.(*net.UnixConn), frame, limit, o)
	}
	return 0, errors.Join(append(errs, errors.New("instance: no socket answers"))...)
}

// exchange checks the peer, writes frame and reads the one-byte reply on c, then closes it.
func exchange(c *net.UnixConn, frame []byte, limit time.Time, o Options) (st Status, err error) {
	defer func() { err = errors.Join(err, ignoreClosed(c.Close())) }()
	if !o.sameUser(c) {
		return 0, errPeer
	}
	if err := c.SetDeadline(limit); err != nil {
		return 0, err
	}
	if _, err := c.Write(frame); err != nil {
		return 0, err
	}
	var reply [1]byte
	if _, err := io.ReadFull(c, reply[:]); err != nil {
		return 0, err
	}
	return Status(reply[0]), nil
}

func ignoreClosed(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// Primary is the process that holds the profile lock. It must stay reachable until the process
// exits: the lock file closes, and the lock goes, with it.
type Primary struct {
	root *os.Root
	lock *os.File
	dir  string // the canonical profile folder
	ln   *net.UnixListener
	sock string // the bound socket path
	o    Options

	wg sync.WaitGroup
	mu sync.Mutex // serializes the handler; guards closing, seen, filled and next
	// closing: every request from now on is answered « closing ».
	closing bool
	seen    [recentIDs][16]byte // the last request IDs queued
	filled  int                 // entries of seen in use
	next    int                 // where the next ID goes in seen

	closeOnce sync.Once
	closeErr  error
}

// listen makes this lock holder the primary: it removes both socket candidates, left by a
// crashed holder (only the holder ever binds or unlinks one, rule D-UNLINK), then binds the
// first that works.
func listen(root *os.Root, f *os.File, dir string, o Options) (*Primary, error) {
	paths := socketPaths(dir, runtimeDir(true))
	for _, p := range paths {
		_ = removeSocket(root, dir, p) // a file that stays makes the bind fail: next candidate
	}
	var errs []error
	for _, p := range paths {
		ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: p, Net: "unix"})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		ln.SetUnlinkOnClose(false) // Close unlinks it itself, before the lock goes
		return &Primary{root: root, lock: f, dir: dir, ln: ln, sock: p, o: o}, nil
	}
	return nil, errors.Join(append(errs, errors.New("instance: no socket candidate can be bound"))...)
}

// removeSocket removes the socket file p: through the profile's root when it is the first
// candidate, so that the removal cannot leave the profile folder.
func removeSocket(root *os.Root, dir, p string) error {
	var err error
	if p == filepath.Join(dir, sockName) {
		err = root.Remove(sockName)
	} else {
		err = os.Remove(p)
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Addr is the address the primary listens on.
func (p *Primary) Addr() net.Addr { return p.ln.Addr() }

// Serve starts accepting requests in the background, until Close. handle is called once per new
// request ID, from one goroutine at a time, before the acknowledgement is written: it must only
// queue the request, and return false when it cannot (its queue is full, or the process is not
// ready to queue yet): the launch is told « busy » and tries again. Once acknowledged, the
// request is the primary's: its window shows the file or says why it cannot (rule D-ACK).
func (p *Primary) Serve(handle func(Request) bool) {
	p.wg.Go(func() { p.accept(handle) })
}

func (p *Primary) accept(handle func(Request) bool) {
	slots := make(chan struct{}, maxConns)
	for {
		c, err := p.ln.AcceptUnix()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		if err != nil {
			time.Sleep(firstBackoff) // out of descriptors, for instance: the backlog waits
			continue
		}
		slots <- struct{}{}
		p.wg.Go(func() {
			defer func() { <-slots }()
			p.serveConn(c, handle)
		})
	}
}

// serveConn answers one connection, bounded by ReadTimeout. A peer of another user is closed
// without a byte read or written.
func (p *Primary) serveConn(c *net.UnixConn, handle func(Request) bool) {
	defer func() { _ = c.Close() }()
	if !p.o.sameUser(c) {
		return
	}
	if err := c.SetDeadline(time.Now().Add(p.o.ReadTimeout)); err != nil {
		return
	}
	req, err := readRequest(c)
	st := replyTo(err)
	if err == nil {
		st = p.answer(req, handle)
	}
	_, _ = c.Write([]byte{byte(st)}) // a launch gone meanwhile retries with the same ID
}

// answer decides the reply to a valid request, calling handle for a new one.
func (p *Primary) answer(req Request, handle func(Request) bool) Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case p.closing:
		return StatusClosing
	case p.seenID(req.ID):
		return StatusAccepted
	case !handle(req):
		return StatusBusy
	}
	p.seen[p.next] = req.ID
	p.next = (p.next + 1) % recentIDs
	p.filled = min(p.filled+1, recentIDs)
	return StatusAccepted
}

// seenID reports whether id is one of the last recentIDs queued. The caller holds p.mu.
func (p *Primary) seenID(id [16]byte) bool {
	for i := range p.filled {
		if p.seen[i] == id {
			return true
		}
	}
	return false
}

// Closing makes every later request get « closing »: the launch waits for the lock and becomes
// the next primary, or hands its request to it (R12). No call of the handler starts after Closing
// returns.
func (p *Primary) Closing() {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
}

// Close is the last step of a clean exit, once the identity, the project and the network are
// closed: it stops serving, unlinks the socket, and only then releases the lock (rule D-HOLD).
// Later calls return the first result.
func (p *Primary) Close() error {
	p.closeOnce.Do(func() {
		p.Closing()
		errLn := ignoreClosed(p.ln.Close())
		p.wg.Wait()
		errSock := removeSocket(p.root, p.dir, p.sock)
		errLock := p.lock.Close()
		p.closeErr = errors.Join(errLn, errSock, errLock, p.root.Close())
	})
	return p.closeErr
}
