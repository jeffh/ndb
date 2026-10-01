package ndb

import (
	"os"
	"testing"
)

func TestFileNotFound(t *testing.T) {
	fs := &MemoryFileSystem{
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
	fs := &MemoryFileSystem{
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
	fs := &MemoryFileSystem{
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

// TestReadFilesBug tests that database file references are loaded correctly.
// This indirectly tests the fix for a bug where readFiles(skip) would write
// data to incorrect indices when skip > 0, which occurs during recursive
// database reference processing in Open().
func TestReadFilesBug(t *testing.T) {
	fs := &MemoryFileSystem{
		Files: map[string]string{
			// start.ndb references file1 and file2
			"start.ndb": `database= file=file1.ndb file=file2.ndb
data0 value=zero`,
			"file1.ndb": `data1 value=one`,
			"file2.ndb": `data2 value=two`,
		},
	}

	// Open() will:
	// 1. Read start.ndb (index 0)
	// 2. Find database references, add file1.ndb and file2.ndb
	// 3. Call readFiles(1) to read the newly added files
	// Without the bug fix, data would be written to wrong indices
	db, err := Open(fs, "start.ndb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that data from each file is correctly accessible
	// If the bug exists, file data would be in wrong positions and searches would fail
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
	fs := &MemoryFileSystem{
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
	fs := &MemoryFileSystem{
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
	fs := &MemoryFileSystem{
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
	t.Run("MemoryFileSystem Open non-existent file", func(t *testing.T) {
		fs := &MemoryFileSystem{}
		_, err := fs.Open("nonexistent.ndb")
		if err != os.ErrNotExist {
			t.Fatalf("expected os.ErrNotExist, got %v", err)
		}
	})
}
