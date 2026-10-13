//go:build !(linux && (amd64 || arm64))

package ndb

import "errors"

func dropPageCache(string) error { return errors.ErrUnsupported }
