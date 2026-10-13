//go:build linux

package ndb

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func mappedInProcess(t *testing.T, path string) bool {
	t.Helper()
	maps, err := os.ReadFile("/proc/self/maps")
	if err != nil {
		t.Skip("no /proc/self/maps: ", err)
	}
	return strings.Contains(string(maps), path)
}

func TestMmapReleasedByCloseAndCleanup(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "close.ndb", "a=1\n")

	db := mustOpenOne(t, &MmapFileSystem{}, path)
	if !mappedInProcess(t, path) {
		t.Fatal("file not mapped after OpenOne")
	}
	must(t, db.Close())
	if mappedInProcess(t, path) {
		t.Fatal("file still mapped after Close")
	}

	// An abandoned db releases its mapping once the GC collects it.
	path = writeTestFile(t, dir, "abandoned.ndb", "a=1\n")
	func() {
		db := mustOpenOne(t, &MmapFileSystem{}, path)
		if len(db.AllSlice()) != 1 {
			t.Fatal("expected one record")
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for mappedInProcess(t, path) {
		if time.Now().After(deadline) {
			t.Fatal("abandoned db still mapped after GC")
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}

	// A failed open releases what it mapped.
	path = writeTestFile(t, dir, "bad.ndb", "ok=1\nbad=\"\\q\"\n")
	if _, err := OpenOne(&MmapFileSystem{}, path); err == nil {
		t.Fatal("expected invalid record error")
	}
	if mappedInProcess(t, filepath.Clean(path)) {
		t.Fatal("failed open left the file mapped")
	}
}
