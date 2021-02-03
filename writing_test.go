package ndb

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestWritingSingleRecord(t *testing.T) {
	fs := &SimulatedFileSystem{}
	db := MakeDB([]Record{
		MakeRecord("given_name", "John", "family_name", "Doe"),
	}).WithFilename("main.db")

	ctx := context.Background()
	if err := db.Save(ctx, fs); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	actual := fs.Files["main.db"]
	expected := "given_name=John family_name=Doe"
	if expected != actual {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestWritingTwoRecords(t *testing.T) {
	fs := &SimulatedFileSystem{}
	db := MakeDB([]Record{
		MakeRecord("given_name", "John", "family_name", "Doe"),
		MakeRecord("given_name", "Joe", "family_name", "Smith"),
	}).WithFilename("main.db")

	ctx := context.Background()
	if err := db.Save(ctx, fs); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	actual := fs.Files["main.db"]
	expected := "given_name=John family_name=Doe\ngiven_name=Joe family_name=Smith"
	if expected != actual {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestWritingRecordsWithRepeatedKeys(t *testing.T) {
	fs := &SimulatedFileSystem{}
	db := MakeDB([]Record{
		MakeRecord("given_name", "John", "family_name", "Doe", "tag", "a", "tag", "b"),
		MakeRecord("given_name", "Joe", "family_name", "Smith", "tag", "a", "tag", "b"),
	}).WithFilename("main.db")

	ctx := context.Background()
	if err := db.Save(ctx, fs); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	actual := fs.Files["main.db"]
	expected := "given_name=John family_name=Doe tag=a tag=b\ngiven_name=Joe family_name=Smith tag=a tag=b"
	if expected != actual {
		t.Fatalf("expected %#v, got %#v", expected, actual)
	}
}

func TestWritingRecordsWithSubdatabases(t *testing.T) {
	fs := &SimulatedFileSystem{}
	db := MakeDB([]Record{
		MakeRecord("given_name", "John", "family_name", "Doe"),
	}).WithFilename("main.db").WithChildren(
		MakeDB([]Record{
			MakeRecord("given_name", "Joe", "family_name", "Smith"),
		}).WithFilename("child1.db"),
		MakeDB([]Record{
			MakeRecord("given_name", "Jane", "family_name", "Appleseed"),
		}).WithFilename("child2.db"),
	)

	ctx := context.Background()
	if err := db.Save(ctx, fs); err != nil {
		t.Fatalf("Failed to save: %s", err)
	}

	t.Run("Validating the main db", func(t *testing.T) {
		actual := fs.Files["main.db"]
		expected := "given_name=John family_name=Doe"
		if expected != actual {
			t.Fatalf("expected %#v, got %#v", expected, actual)
		}
	})

	t.Run("Validating the child db 1", func(t *testing.T) {
		actual := fs.Files["child1.db"]
		expected := "given_name=Joe family_name=Smith"
		if expected != actual {
			t.Fatalf("expected %#v, got %#v", expected, actual)
		}
	})

	t.Run("Validating the child db 2", func(t *testing.T) {
		actual := fs.Files["child2.db"]
		expected := "given_name=Jane family_name=Appleseed"
		if expected != actual {
			t.Fatalf("expected %#v, got %#v", expected, actual)
		}
	})
}

func TestWritingAndReading(t *testing.T) {
	notEmpty := func(x map[string][]string) bool {
		delete(x, "") // empty key strings are not allowed
		return len(x) > 0
	}

	properties := gopter.NewProperties(gopter.DefaultTestParameters())
	properties.Property("Can read what's written", prop.ForAll(
		func(m map[string][]string) bool {
			ctx := context.Background()
			fs := &SimulatedFileSystem{}

			record := MakeRecordFromMap(m)
			expectedDB := MakeDB([]Record{record}).WithFilename("main.db")
			if err := expectedDB.Save(ctx, fs); err != nil {
				// t.Fatalf("Failed to save: %s", err)
				return false
			}

			actualDB, err := OpenWithFS(ctx, fs, "main.db")
			if err != nil {
				// t.Fatalf("Failed to read: %s:\n%#v\n", err, fs.Files["main.db"])
				return false
			}
			// fmt.Printf("MAP: %#v\n%#v\n-> %#v\n", m, fs.Files["main.db"], actualDB.Records[0].String())

			if expectedDB.Len() != actualDB.Len() {
				// t.Fatalf("Expected len %d, got %d", expectedDB.Len(), actualDB.Len())
				return false
			}

			return true
		},
		gen.MapOf(gen.AnyString(), gen.SliceOf(gen.AnyString())).SuchThat(notEmpty),
	))

	properties.TestingRun(t)
}
