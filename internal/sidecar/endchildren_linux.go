package sidecar

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Children returns the pids of the children of every thread of this process. It fails when no
// thread's list can be read (a kernel without them): the caller then does not know.
func Children() ([]string, error) { return childrenOf("/proc/self/task") }

// childrenOf is [Children] on the task directory dir.
func childrenOf(dir string) ([]string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	tasks, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, err
	}
	var out []string
	var read int
	var unread error
	for _, task := range tasks {
		b, err := root.ReadFile(filepath.Join(task.Name(), "children"))
		if err != nil {
			unread = err // the thread ended, unless no list can be read at all
			continue
		}
		read++
		out = append(out, strings.Fields(string(b))...)
	}
	if read == 0 {
		return nil, fmt.Errorf("no thread's children can be read in %s: %w", dir, unread)
	}
	return out, nil
}

// EndChildren ends every child this process still has, and returns once it has none; past
// timeout it fails and names the children that still run. It is the last step before the process
// executes itself again ([ExecSelf]), when its own services are stopped and what remains are
// helpers it does not control (the web view's processes and their sandbox holders): rule N-14 of
// docs/plans/2026-11-14-pc1-project-change-by-restart.md.
//
// The process first makes itself a child subreaper, so that what a dying helper leaves behind
// comes to it instead of to init. It then asks each child to end (SIGTERM), kills it when it still
// runs after grace (SIGKILL), and reaps whatever ended, until no child is left. It reaps every
// child of the process: nothing else may wait for one meanwhile. On success the process stops
// being a subreaper, an attribute the executed image would keep.
func EndChildren(timeout, grace time.Duration) error {
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("becoming the reaper of the remaining processes: %w", err)
	}
	deadline := time.Now().Add(timeout)
	asked := map[int]time.Time{} // when each child was asked to end; a pid stays its child's until reaped
	killed := map[int]bool{}
	for {
		reap()
		kids, err := Children()
		if err != nil {
			return fmt.Errorf("listing children: %w", err)
		}
		if len(kids) == 0 {
			if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 0, 0, 0, 0); err != nil {
				return fmt.Errorf("leaving the reaper role: %w", err)
			}
			return nil
		}
		now := time.Now()
		if now.After(deadline) {
			return fmt.Errorf("%d child processes still running: %s", len(kids), describe(kids))
		}
		for _, kid := range kids {
			pid, err := strconv.Atoi(kid)
			if err != nil || pid <= 0 {
				continue
			}
			at, ok := asked[pid]
			switch {
			case !ok:
				asked[pid] = now
				_ = unix.Kill(pid, unix.SIGTERM) // it may have ended since the listing: reaped next
			case !killed[pid] && now.Sub(at) >= grace:
				killed[pid] = true
				_ = unix.Kill(pid, unix.SIGKILL)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// reap collects every child that has ended, without waiting.
func reap() {
	for {
		pid, err := unix.Wait4(-1, nil, unix.WNOHANG, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if pid <= 0 || err != nil {
			return
		}
	}
}

// describe names the processes pids for a person who reads the log: « [pid (name) …] ».
func describe(pids []string) string {
	out := make([]string, len(pids))
	for i, pid := range pids {
		name, _ := os.ReadFile("/proc/" + pid + "/comm") // #nosec G304 -- a pid listed by the kernel
		out[i] = fmt.Sprintf("%s (%s)", pid, strings.TrimSpace(string(name)))
	}
	return "[" + strings.Join(out, " ") + "]"
}
