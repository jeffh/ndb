package ndb

import (
	"bytes"
	"io"
	"os"
	"strings"
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

// MmapFileSystem opens local files like LocalFileSystem, but Open and OpenOne
// memory-map them instead of reading them into the heap. The OS pages file
// data in as it is scanned and can drop those pages again under memory
// pressure, so a large database costs page cache rather than Go heap.
//
// A database opened through MmapFileSystem holds its mappings until
// (*Ndb).Close is called or the *Ndb becomes unreachable. Its records copy
// attribute and value strings out of the mapping, so they stay valid after
// Close. Truncating a file while it is mapped can crash the process with
// SIGBUS, as with any mmap; replace database files by renaming new ones
// into place instead.
//
// On platforms without mmap support, and for files that cannot be mapped,
// it falls back to reading the file like LocalFileSystem.
type MmapFileSystem struct{}

func (fs *MmapFileSystem) Open(filename string) (io.ReadCloser, error) {
	f, err := os.Open(filename)
	if err != nil || !mmapSupported {
		return f, err
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() == 0 || int64(int(st.Size())) != st.Size() {
		return f, nil
	}
	data, err := mmapFile(f, int(st.Size()))
	if err != nil {
		return f, nil
	}
	// The mapping stays valid after the descriptor is closed.
	_ = f.Close()
	m := &mappedFile{data: data}
	m.r.Reset(data)
	return m, nil
}

// mappedFile is what MmapFileSystem.Open returns for a mapped file. It reads
// like any io.Reader; Ndb uses the mapping directly and takes ownership of
// it, releasing it with Close.
type mappedFile struct {
	data []byte
	r    bytes.Reader
	once sync.Once
	err  error
}

func (m *mappedFile) Read(p []byte) (int, error) { return m.r.Read(p) }

func (m *mappedFile) Close() error {
	m.once.Do(func() {
		m.r.Reset(nil)
		m.err = munmap(m.data)
		m.data = nil
	})
	return m.err
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
