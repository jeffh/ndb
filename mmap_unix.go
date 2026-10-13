//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package ndb

import (
	"os"
	"syscall"
)

const mmapSupported = true

// mmapFile maps f read-only. The mapping is MAP_SHARED, so it reads the page
// cache directly and pages in on first touch.
func mmapFile(f *os.File, size int) ([]byte, error) {
	return syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
}

func munmap(b []byte) error {
	return syscall.Munmap(b)
}
