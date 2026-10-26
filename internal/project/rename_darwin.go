//go:build darwin

package project

import "golang.org/x/sys/unix"

func noReplace(sfd int, src string, dfd int, dst string) error {
	return unix.RenameatxNp(sfd, src, dfd, dst, unix.RENAME_EXCL)
}
