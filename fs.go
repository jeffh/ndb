package ndb

import (
	"bytes"
	"io"
	"os"
	"sync"
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
	M     sync.Mutex
	Files map[string]string
}

func (fs *MemoryFileSystem) Open(filename string) (io.ReadCloser, error) {
	fs.M.Lock()
	defer fs.M.Unlock()
	if fs.Files == nil {
		fs.Files = make(map[string]string)
	}
	data, ok := fs.Files[filename]
	if !ok {
		return nil, os.ErrNotExist
	}
	buf := bytes.NewBufferString(data)
	return &readBuffer{*buf}, nil
}

type readBuffer struct {
	bytes.Buffer
}

func (b *readBuffer) Read(p []byte) (int, error) { return b.Buffer.Read(p) }
func (b *readBuffer) Close() error               { return nil }
