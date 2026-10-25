//go:build linux

package project

import "golang.org/x/sys/unix"

func noReplace(sfd int, src string, dfd int, dst string) error {
	return unix.Renameat2(sfd, src, dfd, dst, unix.RENAME_NOREPLACE)
}
