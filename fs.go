package ndb

import (
	"bytes"
	"io"
	"os"
	"sync"
)

type FileSystem interface {
	// Opens a file for reading
	Open(filename string) (io.ReadCloser, error)
	CreateOrTruncate(filename string) (io.WriteCloser, error)
}

type LocalFileSystem struct{}

var DefaultFileSystem FileSystem = &LocalFileSystem{}

func (fs *LocalFileSystem) Open(filename string) (io.ReadCloser, error) {
	f, err := os.Open(filename)
	return f, err
}

func (fs *LocalFileSystem) CreateOrTruncate(filename string) (io.WriteCloser, error) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	return f, err
}

type SimulatedFileSystem struct {
	M     sync.Mutex
	Files map[string]string
}

func (fs *SimulatedFileSystem) Open(filename string) (io.ReadCloser, error) {
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

func (fs *SimulatedFileSystem) CreateOrTruncate(filename string) (io.WriteCloser, error) {
	fs.M.Lock()
	defer fs.M.Unlock()
	if fs.Files == nil {
		fs.Files = make(map[string]string)
	}

	buf := &writeBuffer{
		filename: filename,
		out:      fs,
	}

	return buf, nil
}

type writeBuffer struct {
	bytes.Buffer

	filename string
	out      *SimulatedFileSystem
}

func (b *writeBuffer) Write(p []byte) (int, error) { return b.Buffer.Write(p) }
func (b *writeBuffer) Close() error {
	b.out.M.Lock()
	defer b.out.M.Unlock()
	b.out.Files[b.filename] = b.String()
	return nil
}

type readBuffer struct {
	bytes.Buffer
}

func (b *readBuffer) Read(p []byte) (int, error) { return b.Buffer.Read(p) }
func (b *readBuffer) Close() error               { return nil }
