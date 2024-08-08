//go:build !windows

package extract_test

import "golang.org/x/sys/unix"

func UnixUmaskZero() int {
	return unix.Umask(0)
}

func UnixUmask(userUmask int) {
	unix.Umask(userUmask)
}

func OsFilePerms(unixPerms uint64) uint64 {
	return unixPerms
}

func OsDirPerms(unixPerms uint64) uint64 {
	return unixPerms
}
