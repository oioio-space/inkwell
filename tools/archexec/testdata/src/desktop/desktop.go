//go:build !server

package desktop

import "syscall"

var launch = syscall.Exec // want "forbidden launch or syscall syscall.Exec"
