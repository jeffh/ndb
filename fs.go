package ndb

import (
	"io"
	"os"
	"strings"
)

type FileSystem interface {
	Open(filename string) (io.ReadCloser, error)
}

type LocalFileSystem struct{}

func (fs *LocalFileSystem) Open(filename string) (io.ReadCloser, error) {
	f, err := os.Open(filename)
	return f, err
}

type MemoryFileSystem struct {
	Files map[string]string
}

func (fs *MemoryFileSystem) Open(filename string) (io.ReadCloser, error) {
	data, ok := fs.Files[filename]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(strings.NewReader(data)), nil
}
