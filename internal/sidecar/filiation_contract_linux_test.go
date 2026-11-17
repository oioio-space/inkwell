package sidecar

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// sb2Parent is a disposable supervising process. Its stdout is an explicit
// phase barrier; the test kills it only after observing the requested phase.
func sb2Parent(arg string) int {
	parts := strings.Split(arg, "|")
	phase, dir := parts[0], parts[1]
	tr := tierF
	if len(parts) > 2 && parts[2] == "N" {
		tr = tierN
	}
	plan := testPlan(tr)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := Spec{Path: os.Args[0], Dir: dir, Env: helperEnv("target-sleep"), tier: tr}
	if phase == "before" {
		s.armDelay = 500 * time.Millisecond
	}
	if len(parts) > 3 {
		s.landlockABI, _ = strconv.Atoi(parts[3])
	}
	c := baseConfig(s, plan)
	if phase == "mismatch" {
		c.ParentPID = os.Getpid() + 1000000
	}
	path, args, err := sandboxPrefix(c)
	if err != nil {
		fmt.Println("error", err)
		return 2
	}
	args = append(args, s.Path)
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	cmd.Env = s.Env
	cmd.Stdout = os.Stdout
	setProcAttr(cmd)
	if phase == "before" {
		cmd.SysProcAttr.Pdeathsig = 0
	}
	tierNamespaces(cmd, plan)
	h, err := newHandshake(cmd, plan)
	if err != nil {
		return 2
	}
	defer h.close()
	if phase == "armed" {
		h.afterReport = func() { fmt.Println("armed", cmd.Process.Pid); time.Sleep(time.Hour) }
	}
	if phase == "running" {
		h.afterReport = func() { fmt.Println("running", cmd.Process.Pid) }
	}
	if err := cmd.Start(); err != nil {
		fmt.Println("error", err)
		return 2
	}
	if phase == "before" {
		fmt.Println("before", cmd.Process.Pid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = h.await(ctx)
	if phase == "mismatch" {
		if err == nil {
			_ = stopLaunch(cmd, fmt.Errorf("unexpected report"))
			return 3
		}
		_ = cmd.Wait()
		if cmd.ProcessState.ExitCode() != parentDiedExit {
			return 4
		}
		fmt.Println("parent mismatch refused")
		return 0
	}
	if err != nil {
		_ = stopLaunch(cmd, err)
		fmt.Println("error", err)
		return 2
	}
	_ = cmd.Wait()
	return 0
}

func TestFiliation(t *testing.T) {
	t.Run("disarm", func(t *testing.T) {
		if !requireLaunch(t) {
			return
		}
		s := spec("disarm")
		s.tier = tierF
		out, err := Run(t.Context(), s)
		if err != nil || out.ExitCode != 0 || string(out.Stdout) != "disarm: EPERM\n" {
			t.Fatalf("S-4 prctl: disarm: %+v %v", out, err)
		}
	})
	for _, tc := range []struct{ name, phase string }{{"parent-dies-before-arming", "before"}, {"parent-dies-after-arming", "armed"}, {"parent-dies-during-run", "running"}} {
		t.Run(tc.name, func(t *testing.T) {
			if !requireLaunch(t) {
				return
			}
			if tc.phase == "before" {
				// Even if the inherited handshake descriptors remain open, a stale
				// parent identity must terminate the launcher before the target execs.
				cmd := helperCmd("sb2-parent:mismatch|" + t.TempDir())
				data, err := cmd.CombinedOutput()
				if err != nil || string(data) != "parent mismatch refused\n" {
					t.Fatalf("S-4 parent check: stale parent accepted: %q %v", data, err)
				}
			}
			dir := t.TempDir()
			cmd := helperCmd("sb2-parent:" + tc.phase + "|" + dir)
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("S-4 parent check: pipe: %v", err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatalf("S-4 parent check: parent start: %v", err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
			reader := bufio.NewReader(pipe)
			text, _ := readLine(t, reader)
			pid := phasePID(text, tc.phase)
			if pid == 0 {
				t.Fatalf("S-4 parent check: bad phase %q", text)
			}
			t.Cleanup(func() {
				if alive(pid) {
					p, _ := os.FindProcess(pid)
					_ = p.Kill()
				}
			})
			if tc.phase == "running" {
				readTargetStarted(t, reader)
			}
			_ = cmd.Process.Kill()
			if tc.phase != "running" {
				readTargetEOF(t, reader)
			}
			_ = cmd.Wait()
			if !waitGone(pid, time.Second) {
				t.Fatalf("S-4 parent check: sidecar %d survived parent at %s", pid, tc.phase)
			}
		})
	}
}

// Keep one reader across phase and target lines: a read may buffer both.
func readTargetStarted(t *testing.T, r *bufio.Reader) {
	t.Helper()
	line, err := readLine(t, r)
	if err != nil || phasePID(line, "target") == 0 {
		t.Fatalf("S-4 parent check: target did not execute: %q %v", line, err)
	}
}

func readTargetEOF(t *testing.T, r *bufio.Reader) {
	t.Helper()
	line, err := readLine(t, r)
	if line != "" || !errors.Is(err, io.EOF) {
		t.Fatalf("S-4 parent check: target started before proceed: %q %v", line, err)
	}
}

// readLine reads one line of a helper's stdout within 5 s.
func readLine(t *testing.T, r *bufio.Reader) (string, error) {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	go func() { line, err := r.ReadString('\n'); done <- result{line, err} }()
	select {
	case v := <-done:
		return v.line, v.err
	case <-time.After(5 * time.Second):
		t.Fatal("S-4 parent check: helper stdout deadline")
		return "", nil
	}
}
