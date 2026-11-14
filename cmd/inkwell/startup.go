package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/version"
)

// Exit codes of a launch (plan docs/plans/2026-11-11-oi-single-instance-open.md, rows R10-R18).
const (
	exitOK    = 0
	exitError = 1
	// exitNoAnswer: an Inkwell of this profile runs but does not answer, or is of another version
	// (R11, R17).
	exitNoAnswer = 3
	// exitNoLock: the profile folder cannot be locked, so Inkwell does not start (R18).
	exitNoLock = 4
)

// deps are the effects of a launch, injected so that the order of startup is tested.
type deps struct {
	stdout io.Writer
	log    *slog.Logger
	getenv func(string) string
	getwd  func() (string, error)
	// homeDir returns the profile folder, created private to the user.
	homeDir func() (string, error)
	// start claims the profile or hands the request over (instance.Start).
	start func(ctx context.Context, dir string, req instance.Request) (*instance.Primary, error)
	// notify tells the person something went wrong, where no window exists.
	notify func(m message)
	// openHome, openProject and startNetwork open the identity, the project and the network: only
	// the primary reaches them, after the claim.
	openHome     func() (*os.Root, error)
	openProject  func(own instance.Request) (*project.Project, error)
	startNetwork func(ctx context.Context, home *os.Root, proj *project.Project) (*app.Runtime, error)
	// window runs the window until it closes, once the network is up.
	window func(ctx context.Context, s *session) int
}

// session is what the primary has open while its window runs.
type session struct {
	cancel context.CancelFunc
	proj   *project.Project
	rt     *app.Runtime
	queue  *openQueue
	// closing and closeInstance are the primary's Closing and Close: the first and the last step of
	// shutdown.
	closing       func()
	closeInstance func() error
}

// startup runs a launch with the arguments args (the command line without the program name) and
// returns its exit code. The order is fixed (plan section 4.3): the version, then the claim of the
// profile, and only then, in the primary, the identity, the project, the network and the window.
// A launch that hands its request over opens none of them.
func startup(args []string, d deps) int {
	if versionRequested(args) {
		_, _ = fmt.Fprintln(d.stdout, version.Current())
		return exitOK
	}
	dir, err := d.homeDir()
	if err != nil {
		d.log.Error("opening the profile folder", "err", err)
		return exitError
	}
	cwd, err := d.getwd()
	if err != nil {
		cwd = "" // relative arguments are then not found; absolute ones still open
	}
	req := instance.NewRequest(args, cwd, cmp.Or(d.getenv("XDG_ACTIVATION_TOKEN"), d.getenv("DESKTOP_STARTUP_ID")))
	ctx := context.Background()
	p, err := d.start(ctx, dir, req)
	switch {
	case errors.Is(err, instance.ErrNotResponding):
		d.notify(messageFor(msgNotResponding, d.getenv))
		return exitNoAnswer
	case errors.Is(err, instance.ErrOtherVersion):
		d.notify(messageFor(msgOtherVersion, d.getenv))
		return exitNoAnswer
	case errors.Is(err, instance.ErrNoLock):
		d.notify(messageFor(msgNoLock, d.getenv))
		return exitNoLock
	case err != nil:
		d.log.Error("claiming the profile", "err", err)
		return exitError
	case p == nil:
		return exitOK // handed over: the running Inkwell shows it (R10)
	}
	// The lock lives as long as p: it must stay reachable until the very end (F13).
	defer runtime.KeepAlive(p)
	queue := newOpenQueue()
	p.Serve(queue.push)
	queue.own(req) // the primary's own arguments take the same path as forwarded ones
	code := runPrimary(ctx, d, req, queue, p)
	if err := p.Close(); err != nil { // already done by a clean shutdown; a no-op then
		d.log.Warn("releasing the profile", "err", err)
	}
	return code
}

// runPrimary opens the identity, the project and the network, then runs the window.
func runPrimary(ctx context.Context, d deps, own instance.Request, queue *openQueue, p *instance.Primary) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	home, err := d.openHome()
	if err != nil {
		d.log.Error("opening identity directory", "err", err)
		return exitError
	}
	proj, err := d.openProject(own)
	if err != nil {
		d.log.Error("opening project", "err", err)
		_ = home.Close()
		return exitError
	}
	rt, err := d.startNetwork(ctx, home, proj)
	if err := home.Close(); err != nil {
		d.log.Warn("closing identity directory", "err", err)
	}
	if err != nil {
		d.log.Error("starting network", "err", err)
		_ = proj.Close()
		return exitError
	}
	queue.open() // ready to show a file: launches are acknowledged from now on (« busy » until then)
	return d.window(ctx, &session{cancel: cancel, proj: proj, rt: rt, queue: queue, closing: p.Closing, closeInstance: p.Close})
}

// stopAll is the shutdown of the primary, in this order: answer « closing » to any new launch, so
// that it waits for the lock instead of handing over to a process that is going away; stop the
// loops and close the network and the project; and only then let the lock go (rule D-HOLD).
func stopAll(closing func(), cancel context.CancelFunc, loops *shutdownLoops, closeNetwork, closeProject, closeInstance func() error, log *slog.Logger) {
	closing()
	shutdown(cancel, loops, closeNetwork, closeProject, log)
	if err := closeInstance(); err != nil {
		log.Warn("releasing the profile", "err", err)
	}
}

// message is a notification of a launch that has no window: a title and a body.
type message struct{ title, body string }

type messageID int

const (
	msgNotResponding messageID = iota // R11
	msgOtherVersion                   // R17
	msgNoLock                         // R18
)

// messages are the only texts of Go for people (plan section 2): no webview exists in the process
// that shows them. French first, English second.
var messages = map[messageID][2]message{
	msgNotResponding: {
		{"Inkwell ne répond pas", "Inkwell est déjà ouvert avec ce profil mais ne répond pas. Fermez-le puis réessayez."},
		{"Inkwell is not responding", "Inkwell is already open with this profile but is not responding. Close it, then try again."},
	},
	msgOtherVersion: {
		{"Inkwell est déjà ouvert", "Une autre version d'Inkwell est ouverte. Fermez-la puis réessayez."},
		{"Inkwell is already open", "Another version of Inkwell is open. Close it, then try again."},
	},
	msgNoLock: {
		{"Inkwell ne démarre pas", "Inkwell ne peut pas s'assurer d'être seul à utiliser son profil ; il ne démarre pas."},
		{"Inkwell does not start", "Inkwell cannot make sure it is the only one using its profile; it does not start."},
	},
}

// messageFor returns the text of id in the person's language: French when LC_ALL, else
// LC_MESSAGES, else LANG starts with "fr"; English otherwise.
func messageFor(id messageID, getenv func(string) string) message {
	lang := cmp.Or(getenv("LC_ALL"), getenv("LC_MESSAGES"), getenv("LANG"))
	if strings.HasPrefix(lang, "fr") {
		return messages[id][0]
	}
	return messages[id][1]
}

// notifyTimeout bounds the desktop notification, so that a stuck bus never holds a launch.
const notifyTimeout = 2 * time.Second

// tell prints m on standard error and shows it as a desktop notification when one can be sent.
func tell(m message) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %s\n", m.title, m.body)
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	_ = desktopNotify(ctx, m) // standard error already has it
}
