package sidecar

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

const bootstrapRefusalReport = 255

const signalReportEnv = "INKWELL_INTERNAL_SIGNAL_FD"

// The trusted namespace init owns the write end, closed on target exec.
// This channel remains reliable even when stderr has reached MaxOutput. It
// carries a refusal marker or two bytes: the target's signal and its [CoreDump]. The child's own
// pidfd answers for a target that is the child itself.
func signalReport(cmd *exec.Cmd) (func(*Output), func(), error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Env = append(cmd.Env, signalReportEnv+"="+strconv.Itoa(3+len(cmd.ExtraFiles)))
	cmd.ExtraFiles = append(cmd.ExtraFiles, w)
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	pidfd := -1
	cmd.SysProcAttr.PidFD = &pidfd
	closeReport := func() {
		_ = r.Close()
		_ = w.Close()
		if pidfd >= 0 {
			_ = unix.Close(pidfd)
		}
	}
	finish := func(out *Output) {
		_ = w.Close()
		out.CoreDump = coreDumpOf(pidfd)
		b, _ := io.ReadAll(io.LimitReader(r, 8192))
		n := len(b)
		out.bootstrapRefused = n == 1 && b[0] == bootstrapRefusalReport
		if n >= 1 && b[0] > 0 && b[0] < 65 && out.ExitCode == 128+int(b[0]) {
			out.Signal, out.ExitCode = int(b[0]), -1
			if n >= 2 && b[1] <= byte(CoreDumpStarted) && out.CoreDump != CoreDumpStarted {
				out.CoreDump = CoreDump(b[1])
			}
		}
		if n > 2 {
			var d SandboxDeath
			if json.Unmarshal(b[2:], &d) == nil {
				out.Death = &d
			}
		}
	}
	return finish, closeReport, nil
}
