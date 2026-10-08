package ndb

import (
	"testing"
)

func TestRecordDelete(t *testing.T) {
	t.Run("delete all occurrences", func(t *testing.T) {
		r := MakeRecord("name", "John", "name", "Jane", "age", "30")
		r.Delete("name", 0)
		if len(r) != 1 {
			t.Fatalf("expected 1 tuple, got %d", len(r))
		}
		if r.Get("age") != "30" {
			t.Fatalf("expected age=30, got %s", r.Get("age"))
		}
		if r.Get("name") != "" {
			t.Fatalf("expected name to be deleted, got %s", r.Get("name"))
		}
	})

	t.Run("delete n occurrences", func(t *testing.T) {
		r := MakeRecord("name", "John", "name", "Jane", "name", "Bob", "age", "30")
		r.Delete("name", 2)
		if len(r) != 2 {
			t.Fatalf("expected 2 tuples, got %d", len(r))
		}
		names := r.GetAll("name")
		if len(names) != 1 {
			t.Fatalf("expected 1 name remaining, got %d", len(names))
		}
		if names[0] != "Bob" {
			t.Fatalf("expected Bob to remain, got %s", names[0])
		}
	})

	t.Run("delete non-existent key", func(t *testing.T) {
		r := MakeRecord("name", "John", "age", "30")
		r.Delete("email", 0)
		if len(r) != 2 {
			t.Fatalf("expected 2 tuples unchanged, got %d", len(r))
		}
	})
}

func TestRecordReplace(t *testing.T) {
	t.Run("replace all values", func(t *testing.T) {
		r := MakeRecord("name", "John", "name", "John", "age", "30")
		r.Replace("name", "John", "Jane")
		names := r.GetAll("name")
		if len(names) != 2 {
			t.Fatalf("expected 2 names, got %d", len(names))
		}
		for _, name := range names {
			if name != "Jane" {
				t.Fatalf("expected all names to be Jane, got %s", name)
			}
		}
	})

	t.Run("replace no match", func(t *testing.T) {
		r := MakeRecord("name", "John", "age", "30")
		r.Replace("name", "Jane", "Bob")
		if r.Get("name") != "John" {
			t.Fatalf("expected name to remain John, got %s", r.Get("name"))
		}
	})
}

func TestRecordReplaceAt(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30")
	r.ReplaceAt(0, Tuple{"name", "Jane"})
	if r.Get("name") != "Jane" {
		t.Fatalf("expected name to be Jane, got %s", r.Get("name"))
	}
}

func TestRecordPut(t *testing.T) {
	t.Run("replace existing key", func(t *testing.T) {
		r := MakeRecord("name", "John", "name", "Jane", "age", "30")
		r.Put(Tuple{"name", "Bob"})
		names := r.GetAll("name")
		if len(names) != 1 {
			t.Fatalf("expected 1 name, got %d", len(names))
		}
		if names[0] != "Bob" {
			t.Fatalf("expected name to be Bob, got %s", names[0])
		}
	})

	t.Run("add new key", func(t *testing.T) {
		r := MakeRecord("name", "John", "age", "30")
		r.Put(Tuple{"email", "john@example.com"})
		if r.Get("email") != "john@example.com" {
			t.Fatalf("expected email to be added, got %s", r.Get("email"))
		}
	})
}

func TestRecordAdd(t *testing.T) {
	r := MakeRecord("name", "John")
	r.Add(Tuple{"age", "30"})
	r.Add(Tuple{"name", "Jane"})
	if len(r) != 3 {
		t.Fatalf("expected 3 tuples, got %d", len(r))
	}
	if r.Get("age") != "30" {
		t.Fatalf("expected age=30, got %s", r.Get("age"))
	}
	names := r.GetAll("name")
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
}

func TestRecordCopy(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30")
	r2 := r.Copy()

	if len(r) != len(r2) || r.Get("name") != r2.Get("name") || r.Get("age") != r2.Get("age") {
		t.Fatalf("expected copy to be equal")
	}

	// Modify the copy
	r2.Put(Tuple{"name", "Jane"})

	// Verify original is unchanged
	if r.Get("name") != "John" {
		t.Fatalf("expected original to be unchanged, got %s", r.Get("name"))
	}
	if r2.Get("name") != "Jane" {
		t.Fatalf("expected copy to be changed, got %s", r2.Get("name"))
	}
}

func TestRecordKeys(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30", "name", "Jane")
	keys := r.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 unique keys, got %d", len(keys))
	}
	hasName := false
	hasAge := false
	for _, k := range keys {
		if k == "name" {
			hasName = true
		} else if k == "age" {
			hasAge = true
		}
	}
	if !hasName || !hasAge {
		t.Fatalf("expected keys to contain 'name' and 'age', got %v", keys)
	}
}

func TestRecordValues(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30", "name", "Jane")
	values := r.Values()
	if len(values) != 3 {
		t.Fatalf("expected 3 values, got %d", len(values))
	}
}

func TestRecordHasKey(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30")
	if !r.HasKey("name") {
		t.Fatalf("expected record to have key 'name'")
	}
	if !r.HasKey("age") {
		t.Fatalf("expected record to have key 'age'")
	}
	if r.HasKey("email") {
		t.Fatalf("expected record not to have key 'email'")
	}
}

func TestRecordLookup(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30")

	val, ok := r.Lookup("name")
	if !ok {
		t.Fatalf("expected to find 'name'")
	}
	if val != "John" {
		t.Fatalf("expected value to be 'John', got %s", val)
	}

	val, ok = r.Lookup("email")
	if ok {
		t.Fatalf("expected not to find 'email'")
	}
	if val != "" {
		t.Fatalf("expected empty value for missing key, got %s", val)
	}
}

func TestRecordAsMap(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30", "name", "Jane")
	m := r.AsMap()

	if len(m) != 2 {
		t.Fatalf("expected 2 keys in map, got %d", len(m))
	}

	names, ok := m["name"]
	if !ok {
		t.Fatalf("expected 'name' key in map")
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	ages, ok := m["age"]
	if !ok {
		t.Fatalf("expected 'age' key in map")
	}
	if len(ages) != 1 || ages[0] != "30" {
		t.Fatalf("expected age to be ['30'], got %v", ages)
	}
}

func TestMakeRecordPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for odd number of arguments")
		}
	}()
	MakeRecord("name", "John", "age")
}

func TestRecordString(t *testing.T) {
	t.Run("simple record", func(t *testing.T) {
		r := MakeRecord("name", "John", "age", "30")
		expected := "name=John age=30"
		if r.String() != expected {
			t.Fatalf("expected %q, got %q", expected, r.String())
		}
	})

	t.Run("quoted values", func(t *testing.T) {
		r := MakeRecord("name", "John Doe", "age", "30")
		expected := `name="John Doe" age=30`
		if r.String() != expected {
			t.Fatalf("expected %q, got %q", expected, r.String())
		}
	})
}

func TestRecordIndexAccessors(t *testing.T) {
	r := MakeRecord("name", "John", "age", "30")

	if r.KeyAt(0) != "name" {
		t.Fatalf("expected key at 0 to be 'name', got %s", r.KeyAt(0))
	}
	if r.ValueAt(0) != "John" {
		t.Fatalf("expected value at 0 to be 'John', got %s", r.ValueAt(0))
	}

	tup := r.TupleAt(1)
	if tup.Attr != "age" || tup.Val != "30" {
		t.Fatalf("expected tuple at 1 to be {age 30}, got %+v", tup)
	}
}
