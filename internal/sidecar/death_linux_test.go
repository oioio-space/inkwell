package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestNamespaceDeathIdentityAndManifest(t *testing.T) {
	d := identityFromStatus([]byte("Name:\tactual-name\nPid:\t14000\nNSpid:\t14000\t2\n"))
	if d.PID != 14000 || d.Comm != "actual-name" || len(d.NSPID) != 2 || d.NSPID[1] != 2 {
		t.Fatal(d)
	}
	path := filepath.Join(t.TempDir(), "deaths.jsonl")
	t.Setenv("INKWELL_SANDBOX_DEATHS", path)
	if e := recordRunManifest(t.Name(), Output{Death: &d, Signal: 31, CoreDump: CoreDumpNone}); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var got map[string]any
	if e = json.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got["pid"] != float64(14000) || got["coredumped"] != false || got["comm"] != "actual-name" {
		t.Fatal(got)
	}
	if e = recordRunManifest(t.Name(), Output{Signal: 31, CoreDump: CoreDumpNone}); e == nil {
		t.Fatal("missing identity silently accepted")
	}
}

// holdMainThread locks a goroutine to the process's main thread until the test
// ends. Go never destroys that thread (a goroutine that exits locked to it only
// parks it), so a test that retires threads keeps the work it observes off it.
// Each probe that lands elsewhere exits locked and destroys its thread, so the
// pool of idle threads shrinks until a probe gets the main one.
func holdMainThread(t *testing.T) {
	t.Helper()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	for range 10000 {
		onMain := make(chan bool)
		go func() {
			runtime.LockOSThread()
			if unix.Gettid() != unix.Getpid() {
				onMain <- false
				return // exits locked: this thread is destroyed
			}
			onMain <- true
			<-release
			runtime.UnlockOSThread()
		}()
		if <-onMain {
			return
		}
	}
	t.Fatal("no goroutine reached the main thread")
}

// SIDE-26: the parent-death signal follows the thread that forked the child,
// so runOnce keeps that thread until the child is reaped, whoever calls it.
// With one P, each goroutine of the burst locks a thread and blocks, so the
// runtime hands the P to an idle thread, then to a new one: the burst takes
// every thread not locked to a goroutine, the creating one included unless
// runOnce holds it, and exiting locked destroys each of them.
func TestSupervisorRunOnceKeepsCreatingThread(t *testing.T) {
	if !requireLaunch(t) {
		return
	}
	watchdog(t, supervisorDeadline)
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	holdMainThread(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started, killed := false, false
	sup := NewSupervisor(SupervisorConfig{
		Name:    "thread",
		Command: func(context.Context) (*exec.Cmd, error) { return helperCmd("sleep"), nil },
		OnEvent: func(e Event) {
			if e.Kind != EventStart {
				return
			}
			started = true
			threads, err := os.ReadDir("/proc/self/task")
			if err != nil {
				t.Error(err)
			}
			var ready, retired sync.WaitGroup
			release := make(chan struct{})
			for range len(threads) + 8 {
				ready.Add(1)
				retired.Go(func() {
					runtime.LockOSThread() // never unlocked: the thread ends with the goroutine
					ready.Done()
					<-release
				})
			}
			ready.Wait()
			close(release)
			retired.Wait()
			killed = waitGone(e.PID, time.Second)
			cancel()
		},
	})
	done := make(chan struct{})
	go func() { _, _ = sup.runOnce(ctx); close(done) }()
	<-done
	if !started {
		t.Fatal("the sidecar never started")
	}
	if killed {
		t.Fatal("the sidecar died with a thread its supervisor no longer held: parent-death signal fired early")
	}
}
