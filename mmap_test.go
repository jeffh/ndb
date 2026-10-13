package ndb

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func recordStrings(recs []Record) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.String()
	}
	return out
}

func TestMmapFileSystemMatchesLocal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.ndb")
	if _, err := writeBenchFixture(path, 256<<10); err != nil {
		t.Fatal(err)
	}
	local := mustOpenOne(t, &LocalFileSystem{}, path)
	mapped := mustOpenOne(t, &MmapFileSystem{}, path)
	defer mapped.Close()

	if got, want := recordStrings(mapped.AllSlice()), recordStrings(local.AllSlice()); !slices.Equal(got, want) {
		t.Fatalf("All differs: %d vs %d records", len(got), len(want))
	}
	for _, preds := range [][]SearchPredicate{
		{HasAttr("sys")},
		{HasAttr("dhcp")},
		{HasAttr("no-such-attr")},
		{HasAttrValue("sys", benchTargetSys)},
		{HasAttrValue("note", benchQuotedNote)},
		{HasAttr("owner"), HasAttrValue("owner", "alice")},
	} {
		got := recordStrings(mapped.SearchSlice(preds...))
		want := recordStrings(local.SearchSlice(preds...))
		if !slices.Equal(got, want) {
			t.Fatalf("Search(%v) differs: %d vs %d records", preds, len(got), len(want))
		}
	}
}

func TestMmapRecordsOutliveClose(t *testing.T) {
	path := writeTestFile(t, t.TempDir(), "db.ndb",
		"sys=alpha ip=10.0.0.1 note=\"quoted value\"\n\towner=alice\nsys=beta ip=10.0.0.2\n")
	db := mustOpenOne(t, &MmapFileSystem{}, path)

	var held []Record
	for rec := range db.All() {
		held = append(held, rec.Copy())
	}
	first := db.First("sys", "beta")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Overwrite the file too: values must not come from the old mapping.
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 128)), 0o644); err != nil {
		t.Fatal(err)
	}

	if got, want := recordStrings(held), []string{
		`sys=alpha ip=10.0.0.1 note="quoted value" owner=alice`,
		`sys=beta ip=10.0.0.2`,
	}; !slices.Equal(got, want) {
		t.Fatalf("held records = %q, want %q", got, want)
	}
	if first.Get("ip") != "10.0.0.2" {
		t.Fatalf("First after Close: ip=%q", first.Get("ip"))
	}
	if n := len(db.AllSlice()); n != 0 {
		t.Fatalf("closed db yielded %d records", n)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestMmapFileSystemEdgeCases(t *testing.T) {
	dir := t.TempDir()

	t.Run("empty file", func(t *testing.T) {
		db := mustOpenOne(t, &MmapFileSystem{}, writeTestFile(t, dir, "empty.ndb", ""))
		defer db.Close()
		if n := len(db.AllSlice()); n != 0 {
			t.Fatalf("empty file yielded %d records", n)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := OpenOne(&MmapFileSystem{}, filepath.Join(dir, "missing.ndb")); !os.IsNotExist(err) {
			t.Fatalf("err = %v, want not-exist", err)
		}
	})

	t.Run("invalid record", func(t *testing.T) {
		path := writeTestFile(t, dir, "bad.ndb", "ok=1\nbad=\"\\q\"\n")
		if _, err := OpenOne(&MmapFileSystem{}, path); err == nil {
			t.Fatal("expected invalid record error")
		}
	})

	t.Run("database references", func(t *testing.T) {
		one := writeTestFile(t, dir, "one.ndb", "data1 value=one\n")
		two := writeTestFile(t, dir, "two.ndb", "data2 value=two\n")
		start := writeTestFile(t, dir, "start.ndb",
			"database= file=\""+one+"\" file=\""+two+"\"\ndata0 value=zero\n")
		db := mustOpen(t, &MmapFileSystem{}, start)
		defer db.Close()
		for attr, want := range map[string]string{"data0": "zero", "data1": "one", "data2": "two"} {
			recs := db.SearchSlice(HasAttr(attr))
			if len(recs) != 1 || recs[0].Get("value") != want {
				t.Fatalf("%s: %v", attr, recs)
			}
		}
	})

	t.Run("Close on unmapped db", func(t *testing.T) {
		db, err := ParseOneString("a=1\n")
		must(t, err)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if n := len(db.AllSlice()); n != 0 {
			t.Fatalf("closed db yielded %d records", n)
		}
	})
}
