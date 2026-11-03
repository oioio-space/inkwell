package execabs

import "os/exec"

func Command(name string) *exec.Cmd { return exec.Command(name) }
