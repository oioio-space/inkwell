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
	"sync"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/language"
	"github.com/oioio-space/inkwell/internal/project"
	"github.com/oioio-space/inkwell/internal/sidecar"
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

// afterTakeOverEnv is set to "1" in the environment of the image a project change restarts
// (rule N-12): that image reads it once, removes it, and says how the change went.
const afterTakeOverEnv = "INKWELL_AFTER_TAKEOVER"

// deps are the effects of a launch, injected so that the order of startup is tested.
type deps struct {
	stdout   io.Writer
	log      *slog.Logger
	getenv   func(string) string
	unsetenv func(string) error
	getwd    func() (string, error)
	// homeDir returns the profile folder, created private to the user.
	homeDir func() (string, error)
	// start claims the profile or hands the request over (instance.Start).
	start func(ctx context.Context, dir string, req instance.Request) (*instance.Primary, error)
	// notify tells the person something went wrong, where no window exists.
	notify func(m message)
	// openHome, openProject and startNetwork open the identity, the project and the network: only
	// the primary reaches them, after the claim.
	openHome    func() (*os.Root, error)
	openProject func(own instance.Request) (*project.Project, error)
	// afterChange is set when this image was started by the restart of a project change.
	startNetwork func(ctx context.Context, home *os.Root, proj *project.Project, afterChange bool) (*app.Runtime, error)
	// window runs the window until it closes, once the network is up.
	window func(ctx context.Context, s *session) int
	// restartChange returns the kind of the project change rt recorded, "" when none
	// ([app.Runtime.RestartChange]).
	restartChange func(rt *app.Runtime) string
	// restartFolder returns the folder the restart opens when it is not the open project's: the
	// one a join installed ([app.Runtime.RestartFolder]); "" otherwise.
	restartFolder func(rt *app.Runtime) string
	// restart replaces this process image by a new launch on the project folder dir (rule N-12);
	// it returns only when it failed.
	restart func(dir string) error
}

// session is what the primary has open while its window runs.
type session struct {
	cancel context.CancelFunc
	proj   *project.Project
	rt     *app.Runtime
	queue  *openQueue
	// language is the language service of the window; shutdown waits until its child has exited
	// (rule N-14).
	language *language.Service
	// closing and closeInstance are the primary's Closing and Close: the first and the last step of
	// shutdown.
	closing       func()
	closeInstance func() error
	// loops are the window's loops, registered by it before it runs; shutdown waits for them.
	loops    shutdownLoops
	stopOnce sync.Once
	forced   bool // set by the desktop shutdown callback before the native loop returns
}

// stop is the shutdown of the session ([stopAll]), once: the window's OnShutdown runs it, and so
// does runPrimary after the window returned, in case the window ended without it.
func (s *session) stop(log *slog.Logger) {
	if s.forced {
		return
	}
	s.stopOnce.Do(func() {
		s.stopResources(log)
	})
}

func (s *session) stopResources(log *slog.Logger) {
	closeNetwork := func() error {
		s.language.Close() // its child ends before the network and the lock (rule N-14)
		err := s.rt.Close()
		sidecar.Shutdown() // and so does the sandbox holder, once the previews are closed
		return err
	}
	stopAll(s.closing, s.cancel, &s.loops, closeNetwork, s.proj.Close, s.closeInstance, log)
}

// stopAfterForce allows resource cleanup to finish when it can, while a store
// operation that never returns cannot keep the process alive.
func (s *session) stopAfterForce(log *slog.Logger) {
	s.forced = true
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.stopResources(log)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		log.Warn("leaving after an explicit force close with cleanup still pending")
	}
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
	// Read once and removed, so that nothing started from this image inherits it (rule N-12).
	afterChange := d.getenv(afterTakeOverEnv) == "1"
	if err := d.unsetenv(afterTakeOverEnv); err != nil {
		d.log.Warn("clearing the restart marker", "err", err)
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
	code, change, restartDir := runPrimary(ctx, d, req, queue, p, dir, afterChange)
	if err := p.Close(); err != nil { // already done by a clean shutdown; a no-op then
		d.log.Warn("releasing the profile", "err", err)
	}
	if restartDir == "" {
		return code
	}
	// The session is closed and the lock released: the change is on disk, the next image loads it.
	// Its slog time opens the restart window of the network proof (rule J-7).
	d.log.Info("restarting", "change", change, "folder", restartDir)
	err = d.restart(restartDir)
	d.log.Error("restarting after a project change", "err", err)
	// No image got the restart marker: the next launch on this folder says how the change went.
	if err := leaveRestartMark(dir, restartDir); err != nil {
		d.log.Warn("recording the failed restart", "err", err)
	}
	d.notify(messageFor(msgRestartFailed, d.getenv))
	return exitError
}

// runPrimary opens the identity, the project and the network, then runs the window. Once the
// window returned and the session is stopped, it returns the kind of the project change this
// image recorded and the folder to restart on (rule N-12; the joined folder for a join, rule J-6),
// else "" twice. profile is the profile folder.
func runPrimary(ctx context.Context, d deps, own instance.Request, queue *openQueue, p *instance.Primary, profile string, afterChange bool) (code int, change, restartDir string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	home, err := d.openHome()
	if err != nil {
		d.log.Error("opening identity directory", "err", err)
		return exitError, "", ""
	}
	proj, err := d.openProject(own)
	if err != nil {
		d.log.Error("opening project", "err", err)
		_ = home.Close()
		return exitError, "", ""
	}
	// A change on this folder whose restart failed is told like one that restarted (rule N-12).
	if takeRestartMark(profile, proj.Path()) {
		afterChange = true
	}
	rt, err := d.startNetwork(ctx, home, proj, afterChange)
	if err := home.Close(); err != nil {
		d.log.Warn("closing identity directory", "err", err)
	}
	if err != nil {
		d.log.Error("starting network", "err", err)
		_ = proj.Close()
		return exitError, "", ""
	}
	queue.open() // ready to show a file: launches are acknowledged from now on (« busy » until then)
	s := &session{cancel: cancel, proj: proj, rt: rt, queue: queue, language: language.New(proj.Path()), closing: p.Closing, closeInstance: p.Close}
	code = d.window(ctx, s)
	s.stop(d.log) // a no-op after the window's OnShutdown
	if change = d.restartChange(rt); change == "" {
		return code, "", ""
	}
	// A join restarts on the folder of the joined document, a take-over on the same folder.
	return code, change, cmp.Or(d.restartFolder(rt), proj.Path())
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
	msgRestartFailed                  // N-12
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
	msgRestartFailed: {
		{"Inkwell ne redémarre pas", "Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau."},
		{"Inkwell does not restart", "Inkwell closed without being able to restart. Open it again."},
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
