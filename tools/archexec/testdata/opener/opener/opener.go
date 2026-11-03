package opener

import "os/exec"

func Open(path string) { _ = exec.Command("open", path).Run() }
