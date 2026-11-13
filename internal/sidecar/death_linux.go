package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// SandboxDeath is trusted process identity captured before the reap. In a PID
// namespace the host identity comes from the original host procfs, not a PID
// guessed from the namespace init's status.
type SandboxDeath struct {
	PID   int    `json:"pid"`
	NSPID []int  `json:"nspid"`
	Comm  string `json:"comm"`
}

func identityFromStatus(b []byte) SandboxDeath {
	var d SandboxDeath
	for line := range strings.SplitSeq(string(b), "\n") {
		k, v, _ := strings.Cut(line, ":")
		switch k {
		case "Name":
			d.Comm = strings.TrimSpace(v)
		case "Pid":
			d.PID, _ = strconv.Atoi(strings.TrimSpace(v))
		case "NSpid":
			for f := range strings.FieldsSeq(v) {
				n, err := strconv.Atoi(f)
				if err == nil {
					d.NSPID = append(d.NSPID, n)
				}
			}
		}
	}
	return d
}

func recordRunManifest(test string, out Output) error {
	path := os.Getenv("INKWELL_SANDBOX_DEATHS")
	if path == "" || test == "" || out.Signal == 0 {
		return nil
	}
	d := out.Death
	if d == nil || d.PID <= 0 || len(d.NSPID) == 0 || d.Comm == "" {
		return errors.New("sidecar: missing death identity")
	}
	line, err := json.Marshal(map[string]any{"test": test, "pid": d.PID, "nspid": d.NSPID, "comm": d.Comm, "signal": out.Signal, "coredumped": out.CoreDump != CoreDumpNone})
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) // #nosec G304 G703 -- test-only manifest named by the gate runner via INKWELL_SANDBOX_DEATHS.
	if err != nil {
		return err
	}
	n, err := f.Write(append(line, '\n'))
	if err == nil && n != len(line)+1 {
		err = errors.New("short death manifest write")
	}
	return errors.Join(err, f.Close())
}

func captureDeath(cmd *exec.Cmd) *SandboxDeath {
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.PidFD == nil {
		return nil
	}
	fd := *cmd.SysProcAttr.PidFD
	for {
		var info unix.Siginfo
		err := unix.Waitid(unix.P_PIDFD, fd, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil
		}
		break
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", cmd.Process.Pid))
	if err != nil {
		return nil
	}
	d := identityFromStatus(b)
	return &d
}
