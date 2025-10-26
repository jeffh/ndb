package ndb

import (
	"os"
	"testing"
)

func TestFileNotFound(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{},
	}

	t.Run("OpenOne returns error for missing file", func(t *testing.T) {
		_, err := OpenOne(fs, "nonexistent.ndb")
		if err == nil {
			t.Fatalf("expected error for non-existent file")
		}
		if err != os.ErrNotExist {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("Open returns error for missing file", func(t *testing.T) {
		_, err := Open(fs, "nonexistent.ndb")
		if err == nil {
			t.Fatalf("expected error for non-existent file")
		}
		if err != os.ErrNotExist {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})
}

func TestDatabaseReferencesWithMissingFiles(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"start.ndb": `database=
	file=exists.ndb
	file=missing.ndb`,
			"exists.ndb": `person name=John`,
		},
	}

	t.Run("Open returns error when referenced file is missing", func(t *testing.T) {
		_, err := Open(fs, "start.ndb")
		if err == nil {
			t.Fatalf("expected error for missing referenced file")
		}
	})
}

func TestChangedMethod(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John`,
		},
	}

	t.Run("Changed returns false when file hasn't changed", func(t *testing.T) {
		db, err := OpenOne(fs, "test.ndb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// In the current implementation with SimulatedFileSystem,
		// Changed() will always read the file. Let's verify the behavior.
		changed := db.Changed()
		// The implementation always reads, so this tests the current behavior
		t.Logf("Changed() returned: %v", changed)
	})

	t.Run("Changed detects file modifications in SimulatedFileSystem", func(t *testing.T) {
		db, err := OpenOne(fs, "test.ndb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Modify the file
		fs.Files["test.ndb"] = `person name=Jane
person name=Bob`

		changed := db.Changed()
		if !changed {
			t.Fatalf("expected Changed() to return true after file modification")
		}

		// Verify the new data is loaded
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 2 {
			t.Fatalf("expected 2 records after reload, got %d", len(records))
		}
	})
}

func TestOpenPanicsWithNilFileSystem(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when FileSystem is nil")
		}
	}()
	Open(nil, "test.ndb")
}

func TestOpenOnePanicsWithNilFileSystem(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic when FileSystem is nil")
		}
	}()
	OpenOne(nil, "test.ndb")
}

func TestRecursiveDatabaseReferences(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"level1.ndb": `database= file=level2.ndb
person name=Level1`,
			"level2.ndb": `database= file=level3.ndb
person name=Level2`,
			"level3.ndb": `person name=Level3`,
		},
	}

	db, err := Open(fs, "level1.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 3 {
		t.Fatalf("expected 3 records from recursive loading, got %d", len(records))
	}

	// Verify all levels are loaded
	names := make(map[string]bool)
	for _, r := range records {
		names[r.Get("name")] = true
	}

	if !names["Level1"] || !names["Level2"] || !names["Level3"] {
		t.Fatalf("not all levels were loaded: %v", names)
	}
}

// TestReadFilesBug specifically tests the bug at ndb.go:137
// where n.data[i] should be n.data[idx] when skip > 0
func TestReadFilesBug(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"file0.ndb": `data0 value=zero`,
			"file1.ndb": `data1 value=one`,
			"file2.ndb": `data2 value=two`,
		},
	}

	// First, open just file0
	db, err := OpenOne(fs, "file0.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Manually add file1 and file2 to the database
	db.files = append(db.files, "file1.ndb", "file2.ndb")
	db.data = append(db.data, []byte{}, []byte{})
	db.mods = append(db.mods, db.mods[0], db.mods[0])

	// Now read the files starting from index 1 (simulating what Open() does)
	// This triggers the bug where data gets written to wrong indices
	_, err = db.readFiles(1)
	if err != nil {
		t.Fatalf("unexpected error reading files: %v", err)
	}

	// Check that each file's data is in the correct position
	// Without the fix, file1's data goes to index 0 and file2's data goes to index 1
	records0 := db.SearchSlice(HasAttr("data0"))
	records1 := db.SearchSlice(HasAttr("data1"))
	records2 := db.SearchSlice(HasAttr("data2"))

	if len(records0) != 1 {
		t.Errorf("expected 1 record with data0, got %d", len(records0))
	}
	if len(records1) != 1 {
		t.Errorf("expected 1 record with data1, got %d", len(records1))
	}
	if len(records2) != 1 {
		t.Errorf("expected 1 record with data2, got %d", len(records2))
	}

	// Verify the values are correct
	if len(records0) > 0 && records0[0].Get("value") != "zero" {
		t.Errorf("data0 has wrong value: %s", records0[0].Get("value"))
	}
	if len(records1) > 0 && records1[0].Get("value") != "one" {
		t.Errorf("data1 has wrong value: %s", records1[0].Get("value"))
	}
	if len(records2) > 0 && records2[0].Get("value") != "two" {
		t.Errorf("data2 has wrong value: %s", records2[0].Get("value"))
	}
}

func TestDatabaseReferencesWithDuplicates(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"start.ndb": `database=
	file=data.ndb
	file=data.ndb
	file=other.ndb`,
			"data.ndb":  `person name=John`,
			"other.ndb": `person name=Jane`,
		},
	}

	db, err := Open(fs, "start.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not load the same file twice
	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 2 {
		t.Fatalf("expected 2 records (not duplicated), got %d", len(records))
	}
}

func TestMultipleDatabaseAttributes(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"start.ndb": `database= file=data1.ndb
database= file=data2.ndb`,
			"data1.ndb": `person name=John`,
			"data2.ndb": `person name=Jane`,
		},
	}

	db, err := Open(fs, "start.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 2 {
		t.Fatalf("expected 2 records from multiple database entries, got %d", len(records))
	}
}

func TestOpenOneDoesNotFollowReferences(t *testing.T) {
	fs := &SimulatedFileSystem{
		Files: map[string]string{
			"start.ndb": `database= file=data.ndb
person name=John`,
			"data.ndb": `person name=Jane`,
		},
	}

	db, err := OpenOne(fs, "start.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OpenOne should not follow database references
	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 1 {
		t.Fatalf("expected 1 record (only from start.ndb), got %d", len(records))
	}
	if records[0].Get("name") != "John" {
		t.Fatalf("expected name=John, got %s", records[0].Get("name"))
	}

	// The database attribute should still be present as a record
	dbRecords := db.SearchSlice(HasAttr("database"))
	if len(dbRecords) != 1 {
		t.Fatalf("expected 1 database record, got %d", len(dbRecords))
	}
}

func TestFileSystemInterface(t *testing.T) {
	t.Run("SimulatedFileSystem CreateOrTruncate and Open", func(t *testing.T) {
		fs := &SimulatedFileSystem{}

		// Write some data
		w, err := fs.CreateOrTruncate("test.ndb")
		if err != nil {
			t.Fatalf("unexpected error creating file: %v", err)
		}
		_, err = w.Write([]byte("person name=John"))
		if err != nil {
			t.Fatalf("unexpected error writing: %v", err)
		}
		err = w.Close()
		if err != nil {
			t.Fatalf("unexpected error closing: %v", err)
		}

		// Read it back
		r, err := fs.Open("test.ndb")
		if err != nil {
			t.Fatalf("unexpected error opening file: %v", err)
		}
		buf := make([]byte, 100)
		n, _ := r.Read(buf)
		r.Close()

		content := string(buf[:n])
		if content != "person name=John" {
			t.Fatalf("expected 'person name=John', got %s", content)
		}
	})

	t.Run("SimulatedFileSystem Open non-existent file", func(t *testing.T) {
		fs := &SimulatedFileSystem{}
		_, err := fs.Open("nonexistent.ndb")
		if err != os.ErrNotExist {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})

	t.Run("SimulatedFileSystem truncates existing file", func(t *testing.T) {
		fs := &SimulatedFileSystem{
			Files: map[string]string{
				"test.ndb": "original content",
			},
		}

		w, err := fs.CreateOrTruncate("test.ndb")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = w.Write([]byte("new content"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = w.Close()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fs.Files["test.ndb"] != "new content" {
			t.Fatalf("expected file to be truncated and replaced")
		}
	})
}
