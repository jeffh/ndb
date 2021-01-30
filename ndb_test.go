package ndb

import (
	"context"
	"testing"
)

func runParse(raw string) (*DB, error) {
	return Parse(raw, "")
}

func TestOpeningMultipleFileDatabase(t *testing.T) {
	{
		fs := &SimulatedFileSystem{
			Files: map[string]string{
				"main.db":  "database file=child.db",
				"child.db": "a=1 sauce=yes",
			},
		}
		ctx := context.Background()
		db, err := OpenWithFS(ctx, fs, "main.db")
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}
		if db.Len() != 2 {
			t.Fatalf("Expected 2 tuples, got: %#v", db.Len())
		}
	}
	t.Run("it opens nested children", func(t *testing.T) {
		fs := &SimulatedFileSystem{
			Files: map[string]string{
				"main.db": "database file=child.db",
				"child.db": `database file=grandchild.db
		a=1 sauce=yes`,
				"grandchild.db": "a=4 sauce=no",
			},
		}
		ctx := context.Background()
		db, err := OpenWithFS(ctx, fs, "main.db")
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}
		if db.Len() != 3 {
			t.Fatalf("Expected 3 tuples, got: %#v", db.Len())
		}
	})
}

func TestParsingRepeatedKey(t *testing.T) {
	db, err := runParse(`a=1 a=2 id=2`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	actual, ok := db.FindFirst("id", "2")
	if !ok {
		t.Fatalf("Failed to find id=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "1", "a", "2", "id", "2")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingOneTuple(t *testing.T) {
	db, err := runParse(`a=1`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "1")
	if !ok {
		t.Fatalf("Failed to find a=1: %#v", db.Records)
	}

	expected := MakeRecord("a", "1")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingOneTuple_NoValue(t *testing.T) {
	{
		db, err := runParse(`a`)
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}

		if db.Len() != 1 {
			t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
		}

		actual, ok := db.FindFirst("a", "")
		if !ok {
			t.Fatalf("Failed to find a=: %#v", db.Records)
		}

		expected := MakeRecord("a", "")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}
	{
		db, err := runParse(`a b=2`)
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}

		if db.Len() != 1 {
			t.Fatalf("Expected num tuples to be 1, got: %v (%v)", db.Len(), db.Records)
		}

		actual, ok := db.FindFirst("a", "")
		if !ok {
			t.Fatalf("Failed to find a=: %#v", db.Records)
		}

		expected := MakeRecord("a", "", "b", "2")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}
	{
		db, err := runParse(`b=2 a`)
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}

		if db.Len() != 1 {
			t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
		}

		actual, ok := db.FindFirst("a", "")
		if !ok {
			t.Fatalf("Failed to find a=: %#v", db.Records)
		}

		expected := MakeRecord("b", "2", "a", "")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}
}

func TestParsingOneTuple_BlankValue(t *testing.T) {
	db, err := runParse(`a=`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "")
	if !ok {
		t.Fatalf("Failed to find a=: %#v", db.Records)
	}

	expected := MakeRecord("a", "")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingTwoTuples(t *testing.T) {
	db, err := runParse(`a=1 b=2`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "1")
	if !ok {
		t.Fatalf("Failed to find a=1: %#v", db.Records)
	}

	expected := MakeRecord("a", "1", "b", "2")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingThreeTuples(t *testing.T) {
	db, err := runParse(`a=1 b=2 foo=bar`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("b", "2")
	if !ok {
		t.Fatalf("Failed to find b=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "1", "b", "2", "foo", "bar")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingMultilineTuple(t *testing.T) {
	db, err := runParse(`a=1 
	b=2 
	foo=bar`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("b", "2")
	if !ok {
		t.Fatalf("Failed to find b=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "1", "b", "2", "foo", "bar")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingTwoRecords(t *testing.T) {
	db, err := runParse(`a=1 b=2
a=2 b=4`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 2 {
		t.Fatalf("Expected num tuples to be 2, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "2")
	if !ok {
		t.Fatalf("Failed to find a=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "2", "b", "4")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingThreeRecords(t *testing.T) {
	db, err := runParse(`a=1 b=2
a=2
	b=4

d=3`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 3 {
		t.Fatalf("Expected num tuples to be 3, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "2")
	if !ok {
		t.Fatalf("Failed to find a=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "2", "b", "4")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestParsingQuotedKey(t *testing.T) {
	db, err := runParse(`"a store"=1 b=2
"a store"=2
	b=4

d=3`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	{
		actual, ok := db.FindFirst("a store", "2")
		if !ok {
			t.Fatalf("Failed to find \"a store\"=2: %#v", db.Records)
		}

		expected := MakeRecord("a store", "2", "b", "4")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}

	{
		actual, ok := db.FindFirst("a store", "1")
		if !ok {
			t.Fatalf("Failed to find \"a store\"=1: %#v", db.Records)
		}

		expected := MakeRecord("a store", "1", "b", "2")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}
}

func TestParsingQuotedValue(t *testing.T) {
	db, err := runParse(`a="store of value" b=2
a="store of values"
	b=4

d=3`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	{
		actual, ok := db.FindFirst("a", "store of values")
		if !ok {
			t.Fatalf("Failed to find a=\"store of values\": %#v", db.Records)
		}

		expected := MakeRecord("a", "store of values", "b", "4")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}

	{
		actual, ok := db.FindFirst("a", "store of value")
		if !ok {
			t.Fatalf("Failed to find a=\"store of value\": %#v", db.Records)
		}

		expected := MakeRecord("a", "store of value", "b", "2")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	}
}

func TestParsingWithComments(t *testing.T) {
	db, err := runParse(`# I am a comment
a=1 b=2
a=2
	b=4

# hello world
d=3`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}

	if db.Len() != 3 {
		t.Fatalf("Expected num tuples to be 3, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "2")
	if !ok {
		t.Fatalf("Failed to find a=2: %#v", db.Records)
	}

	expected := MakeRecord("a", "2", "b", "4")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestEdgeCase_ParseStartingSpaceIsInferred(t *testing.T) {
	db, err := runParse(` a=1`)
	if err != nil {
		t.Fatalf("Failed to parse: %s", err)
	}
	if db.Len() != 1 {
		t.Fatalf("Expected num tuples to be 1, got: %v", db.Len())
	}

	actual, ok := db.FindFirst("a", "1")
	if !ok {
		t.Fatalf("Failed to find a=1: %#v", db.Records)
	}

	expected := MakeRecord("a", "1")
	if !expected.Equal(actual) {
		t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
	}
}

func TestEdgeCase_EqualsAsAKeyIsInvalid(t *testing.T) {
	_, err := runParse(`==foo`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`=`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`a=b =`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`a=b ==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`a=b
	==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	_, err = runParse(`a=b
	=`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeKey) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeKey, got %s", err)
	}

	t.Run("Unless it is quoted", func(t *testing.T) {
		db, err := runParse(`"="=2`)
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}
		if db.Len() != 1 {
			t.Fatalf("Expected num tuples to be 1, got: %v", db)
		}

		actual, ok := db.FindFirst("=", "2")
		if !ok {
			t.Fatalf("Failed to find a=1: %#v", db.Records)
		}

		expected := MakeRecord("=", "2")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	})
}

func TestEdgeCase_EqualsAsAValueIsInvalid(t *testing.T) {
	_, err := runParse(`foo==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeValue) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeValue, got %s", err)
	}

	_, err = runParse(`a=b foo==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeValue) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeValue, got %s", err)
	}

	_, err = runParse(`a=b
	b==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeValue) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeValue, got %s", err)
	}

	_, err = runParse(`a=b
	c==`)
	if !IsErrCode(err, ErrCodeEqualsCannotBeValue) {
		t.Fatalf("Expected error code ErrCodeEqualsCannotBeValue, got %s", err)
	}

	t.Run("Unless it is quoted", func(t *testing.T) {
		db, err := runParse(`a="="`)
		if err != nil {
			t.Fatalf("Failed to parse: %s", err)
		}
		if db.Len() != 1 {
			t.Fatalf("Expected num tuples to be 1, got: %v", db)
		}

		actual, ok := db.FindFirst("a", "=")
		if !ok {
			t.Fatalf("Failed to find a=\"=\": %#v", db.Records)
		}

		expected := MakeRecord("a", "=")
		if !expected.Equal(actual) {
			t.Fatalf("Expected %s, got %s: %#v", expected.String(), actual.String(), db)
		}
	})
}

func TestEdgeCase_UnclosedQuotedKeyIsAnError(t *testing.T) {
	_, err := runParse(`"apple`)
	if !IsErrCode(err, ErrCodeKeyNotClosed) {
		t.Fatalf("Expected error code ErrCodeKeyNotClosed, got %s", err)
	}
	_, err = runParse(`f=1 "apple=sauce`)
	if !IsErrCode(err, ErrCodeKeyNotClosed) {
		t.Fatalf("Expected error code ErrCodeKeyNotClosed, got %s", err)
	}
}

func TestEdgeCase_UnclosedQuotedValueIsAnError(t *testing.T) {
	_, err := runParse(`apple="bees`)
	if !IsErrCode(err, ErrCodeValueNotClosed) {
		t.Fatalf("Expected error code ErrCodeValueNotClosed, got %s", err)
	}
	_, err = runParse(`f=1 apple="sauce`)
	if !IsErrCode(err, ErrCodeValueNotClosed) {
		t.Fatalf("Expected error code ErrCodeValueNotClosed, got %s", err)
	}
}
