package wrapped

import "os/exec"

func Launch() { _ = exec.Command("true").Run() }
