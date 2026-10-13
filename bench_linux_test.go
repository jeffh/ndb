//go:build linux && (amd64 || arm64)

package ndb

import (
	"os"
	"syscall"
)

// dropPageCache asks the kernel to evict path's clean pages from the page
// cache (POSIX_FADV_DONTNEED), so the next read comes from storage.
func dropPageCache(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	const fadvDontNeed = 4
	if _, _, errno := syscall.Syscall6(syscall.SYS_FADVISE64, f.Fd(), 0, 0, fadvDontNeed, 0, 0); errno != 0 {
		return errno
	}
	return nil
}
