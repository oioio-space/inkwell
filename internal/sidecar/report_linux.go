package sidecar

import (
	"io"
	"os"
	"os/exec"
	"strconv"
)

const signalReportEnv = "INKWELL_INTERNAL_SIGNAL_FD"

// The trusted namespace init owns the write end, closed on target exec.
// This channel remains reliable even when stderr has reached MaxOutput.
func signalReport(cmd *exec.Cmd) (func(*Output), func(), error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Env = append(cmd.Env, signalReportEnv+"="+strconv.Itoa(3+len(cmd.ExtraFiles)))
	cmd.ExtraFiles = append(cmd.ExtraFiles, w)
	closeReport := func() { _ = r.Close(); _ = w.Close() }
	finish := func(out *Output) {
		_ = w.Close()
		var b [1]byte
		if n, _ := io.ReadFull(r, b[:]); n == 1 && b[0] > 0 && b[0] < 65 && out.ExitCode == 128+int(b[0]) {
			out.Signal, out.ExitCode = int(b[0]), -1
		}
	}
	return finish, closeReport, nil
}
