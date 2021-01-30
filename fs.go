package ndb

import (
	"bytes"
	"io"
	"os"
)

type FileSystem interface {
	Open(filename string) (io.ReadWriteCloser, error)
}

type LocalFileSystem struct{}

var DefaultFileSystem FileSystem = &LocalFileSystem{}

func (fs *LocalFileSystem) Open(filename string) (io.ReadWriteCloser, error) {
	f, err := os.Open(filename)
	return f, err
}

type SimulatedFileSystem struct {
	Files map[string]string
}

func (fs *SimulatedFileSystem) Open(filename string) (io.ReadWriteCloser, error) {
	data, ok := fs.Files[filename]
	if !ok {
		return nil, os.ErrNotExist
	}
	buf := bytes.NewBufferString(data)
	return &buffer{*buf}, nil
}

type buffer struct {
	bytes.Buffer
}

func (b *buffer) Read(p []byte) (int, error)  { return b.Buffer.Read(p) }
func (b *buffer) Write(p []byte) (int, error) { return b.Buffer.Write(p) }
func (b *buffer) Close() error                { return nil }
