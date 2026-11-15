package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oioio-space/inkwell/internal/app"
	"github.com/oioio-space/inkwell/internal/instance"
	"github.com/oioio-space/inkwell/internal/project"
)

// restartDeps are the deps of a primary that starts the real runtime on a new project, whose
// window returns at once, and whose runtime reports requested as its recorded change.
func restartDeps(t *testing.T, steps *[]string, requested bool) (d deps, profile string, dir *string) {
	d, profile, dir, _ = restartDepsProject(t, steps, requested)
	return d, profile, dir
}

// restartDepsProject is restartDeps that also gives the project the launch opens.
func restartDepsProject(t *testing.T, steps *[]string, requested bool) (d deps, profile string, dir *string, proj **project.Project) {
	t.Helper()
	d = primaryDeps(t, steps)
	profile, _ = d.homeDir()
	dir, proj = new(string), new(*project.Project)
	openProject := d.openProject
	d.openProject = func(r instance.Request) (*project.Project, error) {
		p, err := openProject(r)
		if p != nil {
			*dir, *proj = p.Path(), p
		}
		return p, err
	}
	d.startNetwork = func(ctx context.Context, home *os.Root, proj *project.Project, afterChange bool) (*app.Runtime, error) {
		*steps = append(*steps, "network")
		if afterChange {
			*steps = append(*steps, "after-change")
		}
		return app.Start(ctx, home, app.Options{Project: proj, RendezvousURL: app.NoCodes, AfterChange: afterChange, CanRestart: true})
	}
	d.window = func(context.Context, *session) int { *steps = append(*steps, "window"); return exitOK }
	d.restartRequested = func(*app.Runtime) bool { return requested }
	d.restart = func(string) error { t.Fatal("restarted"); return nil }
	return d, profile, dir, proj
}

// errExecuted stands for an execution that replaced the image: the fake restart returns it.
var errExecuted = errors.New("executed")

// lockFree reports whether the profile lock of dir can be taken now.
func lockFree(t *testing.T, dir string) bool {
	t.Helper()
	p, err := instance.Start(t.Context(), dir, instance.NewRequest(nil, "/", ""), instance.Options{Deadline: 300 * time.Millisecond, ClosingWait: 300 * time.Millisecond})
	if p == nil {
		return false
	}
	_ = p.Close()
	return err == nil
}

// TestRestartArgvAndEnvRebuilt: the restarted image gets a fixed program name and the absolute
// project folder only, whatever this image was given, and the current environment without any
// stale restart marker, plus the marker once (rule N-12).
func TestRestartArgvAndEnvRebuilt(t *testing.T) {
	environ := []string{"PATH=/bin", afterTakeOverEnv + "=0", "HOME=/home/ana", afterTakeOverEnv + "=1; rm -rf /", "LANG=fr_FR.UTF-8"}
	argv, env := restartCommand("/home/ana/Documents/Inkwell/Mon livre", environ)
	if want := []string{"inkwell", "/home/ana/Documents/Inkwell/Mon livre"}; !slices.Equal(argv, want) {
		t.Errorf("N-12 argv rebuilt: argv = %q, want %q", argv, want)
	}
	if want := []string{"PATH=/bin", "HOME=/home/ana", "LANG=fr_FR.UTF-8", afterTakeOverEnv + "=1"}; !slices.Equal(env, want) {
		t.Errorf("N-12 env rebuilt: env = %q, want %q", env, want)
	}
	if environ[1] != afterTakeOverEnv+"=0" {
		t.Error("restartCommand changed its input")
	}
}

// TestRestartOnlyAfterARequestAndOnce: a primary restarts only when its runtime recorded a
// change, once, on its project folder; the image started by that restart reads the marker once,
// removes it from its environment, passes it to the runtime, and does not restart again: a
// consumed marker never restarts (rule N-12, RestartOnce). A take-over and a join record the same
// change (Session.requestChange), so one request stands for both kinds.
func TestRestartOnlyAfterARequestAndOnce(t *testing.T) {
	for _, requested := range []bool{false, true} {
		var steps []string
		d, _, dir := restartDeps(t, &steps, requested)
		var restarts []string
		d.restart = func(dir string) error { restarts = append(restarts, dir); return errExecuted }
		d.notify = func(message) {}
		code := startup(nil, d)
		switch {
		case !requested && (len(restarts) != 0 || code != exitOK):
			t.Errorf("N-12 restart only after a request: no request, restarts %q, exit %d", restarts, code)
		case requested && !slices.Equal(restarts, []string{*dir}):
			t.Errorf("N-12 a request restarts once, on its folder: restarts %q, want [%q]", restarts, *dir)
		}
	}

	var steps, unset []string
	d, _, _ := restartDeps(t, &steps, false)
	d.getenv = func(k string) string {
		if k == afterTakeOverEnv && !slices.Contains(unset, k) {
			return "1"
		}
		return ""
	}
	d.unsetenv = func(k string) error { unset = append(unset, k); return nil }
	if code := startup(nil, d); code != exitOK {
		t.Fatalf("the restarted image: exit %d", code)
	}
	if !slices.Contains(steps, "after-change") || !slices.Equal(unset, []string{afterTakeOverEnv}) {
		t.Errorf("N-12 the marker is read once and removed: steps %v, unset %v", steps, unset)
	}
}

// TestRestartAfterStopAllReturned: the restart runs only once the window returned and the whole
// shutdown is done: the project closed and the profile lock, its last step, released (rule N-12).
func TestRestartAfterStopAllReturned(t *testing.T) {
	var steps []string
	d, profile, _, proj := restartDepsProject(t, &steps, true)
	d.notify = func(message) {}
	d.restart = func(string) error {
		steps = append(steps, "restart")
		if _, err := (*proj).List(); err == nil {
			t.Error("N-12 restart after the outcome: the project is still open at the restart")
		}
		if !lockFree(t, profile) {
			t.Error("N-12 restart after the outcome: the profile is still locked at the restart")
		}
		return errExecuted
	}
	startup(nil, d)
	if i, j := slices.Index(steps, "window"), slices.Index(steps, "restart"); i < 0 || j < i {
		t.Errorf("N-12 restart after the outcome: steps %v", steps)
	}
}

// TestRestartFailureIsReported: when the execution fails, the runtime is already closed: the
// person is told, in their language, that Inkwell closed without restarting, and the process
// exits with the error code (rule N-12).
func TestRestartFailureIsReported(t *testing.T) {
	var steps []string
	d, _, _ := restartDeps(t, &steps, true)
	d.getenv = func(k string) string {
		if k == "LANG" {
			return "fr_FR.UTF-8"
		}
		return ""
	}
	d.restart = func(string) error { return errors.New("exec format error") }
	var told []message
	d.notify = func(m message) { told = append(told, m) }
	if code := startup(nil, d); code != exitError {
		t.Errorf("N-12 a failed restart exits with the error code: exit %d", code)
	}
	want := message{"Inkwell ne redémarre pas", "Inkwell s'est fermé sans pouvoir redémarrer. Ouvrez-le de nouveau."}
	if !slices.Equal(told, []message{want}) {
		t.Errorf("N-12 a failed restart is reported: told %q, want %q", told, want)
	}
}

// TestRestartFailedThenLaunchedByHandSaysOnce: the change is written, the restart fails, and the
// person opens Inkwell again by hand: no restart marker is in that launch's environment, yet the
// image that opens the changed folder says how the change went, once; a launch on another folder
// in between says nothing and does not consume it (rule N-12, UX-PC1a-8).
func TestRestartFailedThenLaunchedByHandSaysOnce(t *testing.T) {
	var steps []string
	d, _, dir, _ := restartDepsProject(t, &steps, true)
	d.restart = func(string) error { return errors.New("exec format error") }
	d.notify = func(message) {}
	if code := startup(nil, d); code != exitError {
		t.Fatalf("the failed restart: exit %d", code)
	}
	changed := *dir

	// Launched by hand: nothing restarts, and the window asks for the notice twice.
	d.restartRequested = func(*app.Runtime) bool { return false }
	var said []string
	d.window = func(_ context.Context, s *session) int {
		said = append(said, s.rt.Session.StartupNotice(), s.rt.Session.StartupNotice())
		return exitOK
	}
	other := d.openProject // creates another folder
	same := func(instance.Request) (*project.Project, error) {
		base, err := os.OpenRoot(filepath.Dir(changed))
		if err != nil {
			return nil, err
		}
		defer func() { _ = base.Close() }()
		return project.Open(base, filepath.Base(changed))
	}
	for _, launch := range []struct {
		name string
		open func(instance.Request) (*project.Project, error)
		want []string
	}{
		{"another folder", other, []string{"", ""}},
		{"the changed folder", same, []string{"foreign.takeOver.done", ""}},
		{"the changed folder again", same, []string{"", ""}},
	} {
		said, d.openProject = nil, launch.open
		if code := startup(nil, d); code != exitOK {
			t.Fatalf("%s: exit %d", launch.name, code)
		}
		if !slices.Equal(said, launch.want) {
			t.Errorf("N-12 a failed restart is told at the next launch on its folder, once: %s: notices %q, want %q", launch.name, said, launch.want)
		}
	}
}

// TestRestartLosesLockToAnotherLaunch: another launch takes the profile between the release of
// the lock and its re-acquisition by the restarted image; that image then hands its request over
// and exits, and the primary that won gets the changed folder to open (residual R-7, OI rule R10).
func TestRestartLosesLockToAnotherLaunch(t *testing.T) {
	var steps []string
	d, profile, _ := restartDeps(t, &steps, true)
	d.notify = func(message) {}
	var mu sync.Mutex
	var got []instance.Request
	var nextOut, folder string
	d.restart = func(dir string) error {
		folder = dir
		other, err := instance.Start(t.Context(), profile, instance.NewRequest(nil, "/", ""), instance.Options{})
		if err != nil || other == nil {
			t.Fatalf("the other launch could not take the free profile: %v, %v", other, err)
		}
		t.Cleanup(func() { _ = other.Close() })
		other.Serve(func(r instance.Request) bool { mu.Lock(); defer mu.Unlock(); got = append(got, r); return true })
		argv, env := restartCommand(dir, nil)
		cmd := child("second", append(env, "INKWELL_HOME="+profile, "INKWELL_CHILD_ARGS="+strings.Join(argv[1:], "\n"))...)
		out, err := cmd.CombinedOutput()
		nextOut = string(out)
		if err != nil {
			t.Errorf("R-7 the restarted image hands over and exits: %v\n%s", err, out)
		}
		return errExecuted
	}
	startup(nil, d)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || !slices.Equal(got[0].Args, []string{folder}) || !filepath.IsAbs(folder) {
		t.Fatalf("R-7 the primary that won gets the changed folder: got %+v (restarted image said %q)", got, nextOut)
	}
}

// TestRestartWaitsForTheWindowLoops: a window that returned without its own shutdown leaves its
// loops to the session's: they are cancelled and waited for before the restart, like the rest of
// the shutdown (rule N-12).
func TestRestartWaitsForTheWindowLoops(t *testing.T) {
	var steps []string
	d, _, _ := restartDeps(t, &steps, true)
	d.notify = func(message) {}
	d.window = func(ctx context.Context, s *session) int {
		s.loops.Go("watch", func() { <-ctx.Done(); steps = append(steps, "loop ended") })
		return exitOK
	}
	d.restart = func(string) error { steps = append(steps, "restart"); return errExecuted }
	startup(nil, d)
	if i, j := slices.Index(steps, "loop ended"), slices.Index(steps, "restart"); i < 0 || j < i {
		t.Errorf("N-12 restart after the outcome: the window's loops were not stopped before the restart: steps %v", steps)
	}
}
