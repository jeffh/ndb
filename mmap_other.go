//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package ndb

import (
	"errors"
	"os"
)

const mmapSupported = false

func mmapFile(*os.File, int) ([]byte, error) {
	return nil, errors.ErrUnsupported
}

func munmap([]byte) error { return nil }
