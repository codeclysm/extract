//go:build windows

package extract_test

func UnixUmaskZero() int {
	return 0
}

func UnixUmask(userUmask int) {
}

func OsFilePerms(unixPerms uint64) uint64 {
	// Go on Windows just uses 666/444 for files depending on whether "read only" is set
	globalPerms := unixPerms >> 6
	return globalPerms | (globalPerms << 3) | (globalPerms << 6)
}

func OsDirPerms(unixPerms uint64) uint64 {
	// Go on Windows just uses 777 for directories
	return 0777
}
