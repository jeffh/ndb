package ndb

import (
	"reflect"
	"testing"
)

func TestRecordEquality(t *testing.T) {
	a := MakeRecord("a", "1", "b", "2")
	b := MakeRecord("a", "1", "b", "2")
	c := MakeRecord("a", "1", "b", "3")
	d := MakeRecord("a", "1")
	if !a.Equal(b) {
		t.Fatalf("Expected %s to equal %s", a.String(), b.String())
	}
	if a.Equal(c) {
		t.Fatalf("Expected %s to not equal %s", a.String(), c.String())
	}
	if a.Equal(d) {
		t.Fatalf("Expected %s to not equal %s", a.String(), d.String())
	}
}

func TestRecordAttributeReading(t *testing.T) {
	r := MakeRecord("a", "1", "b", "2", "b", "3")

	t.Run("AsMap", func(t *testing.T) {
		m := r.AsMap()
		expected := map[string][]string{
			"a": []string{"1"},
			"b": []string{"2", "3"},
		}

		if !reflect.DeepEqual(expected, m) {
			t.Fatalf("Expected %#v, got %#v", expected, m)
		}
	})

	t.Run("Len", func(t *testing.T) {
		if r.Len() != 3 {
			t.Fatalf("Expected 3 tuples, got %v", r.Len())
		}
	})

	t.Run("HasKey", func(t *testing.T) {
		if !r.HasKey("a") {
			t.Fatalf("Expected HasKey(%#v) = true", "a")
		}
		if !r.HasKey("b") {
			t.Fatalf("Expected HasKey(%#v) = true", "b")
		}
		if r.HasKey("MISSING_KEY") {
			t.Fatalf("Expected HasKey(%#v) = false", "MISSING_KEY")
		}
	})

	t.Run("GetFirst", func(t *testing.T) {
		key := "a"
		v, ok := r.GetFirst(key)
		if !ok {
			t.Fatalf("GetFirst(%#v) did not find a key", key)
		}
		if v != "1" {
			t.Fatalf("GetFirst(%#v) returned %#v, but expected %#v", key, v, "1")
		}

		key = "b"
		v, ok = r.GetFirst(key)
		if !ok {
			t.Fatalf("GetFirst(%#v) did not find a key", key)
		}
		if v != "2" {
			t.Fatalf("GetFirst(%#v) returned %#v, but expected %#v", key, v, "2")
		}
	})

	t.Run("Keys", func(t *testing.T) {
		expected := []string{"a", "b", "b"}
		actual := r.Keys()
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Expected %#v, got %#v", expected, actual)
		}
		for i, k := range expected {
			if r.KeyAt(i) != k {
				t.Fatalf("Expected %#v at %d, but got %#v", k, i, r.KeyAt(i))
			}
		}
	})
	t.Run("Values", func(t *testing.T) {
		expected := []string{"1", "2", "3"}
		actual := r.Values()
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Expected %#v, got %#v", expected, actual)
		}
		for i, v := range expected {
			if r.ValueAt(i) != v {
				t.Fatalf("Expected %#v at %d, but got %#v", v, i, r.ValueAt(i))
			}
		}
	})
	t.Run("Tuples", func(t *testing.T) {
		expected := []Tuple{
			{"a", "1"},
			{"b", "2"},
			{"b", "3"},
		}
		actual := r.Tuples()
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Expected %#v, got %#v", expected, actual)
		}
		for i, tup := range expected {
			if r.TupleAt(i) != tup {
				t.Fatalf("Expected %#v at %d, but got %#v", tup, i, r.TupleAt(i))
			}
		}
	})

	t.Run("ValuesForKey", func(t *testing.T) {
		expected := []string{"1"}
		actual := r.ValuesForKey("a")
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Expected %#v, got %#v", expected, actual)
		}

		expected = []string{"2", "3"}
		actual = r.ValuesForKey("b")
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("Expected %#v, got %#v", expected, actual)
		}
	})

	t.Run("Copy", func(t *testing.T) {
		a := r.Copy()
		if !r.Equal(a) {
			t.Fatalf("Expected %#v, got %#v", r, a)
		}
	})
}

func TestRecordMutation(t *testing.T) {
	r := MakeRecord("a", "1", "b", "2", "b", "3")

	t.Run("ReplaceAt", func(t *testing.T) {
		a := r.Copy()
		a.ReplaceAt(1, Tuple{"foo", "bar"})

		expected := MakeRecord("a", "1", "foo", "bar", "b", "3")
		if !expected.Equal(a) {
			t.Fatalf("Expected %s, got %s", expected, a)
		}
	})

	t.Run("Replace", func(t *testing.T) {
		a := r.Copy()

		a.Replace("b", "2", "3")
		expected := MakeRecord("a", "1", "b", "3", "b", "3")
		if !expected.Equal(a) {
			t.Fatalf("Expected %s, got %s", expected.String(), a.String())
		}

		a.Replace("b", "3", "4")
		expected = MakeRecord("a", "1", "b", "4", "b", "4")
		if !expected.Equal(a) {
			t.Fatalf("Expected %s, got %s", expected.String(), a.String())
		}
	})

	t.Run("ReplaceTimes", func(t *testing.T) {
		a := r.Copy()
		a.Replace("b", "2", "3")

		a.ReplaceTimes("b", "3", "1", 1)
		expected := MakeRecord("a", "1", "b", "1", "b", "3")
		if !expected.Equal(a) {
			t.Fatalf("Expected %s, got %s", expected.String(), a.String())
		}
	})

	t.Run("Put", func(t *testing.T) {
		a := r.Copy()
		a.Put(Tuple{"another", "value"})

		expected := MakeRecord("a", "1", "b", "2", "b", "3", "another", "value")
		if !expected.Equal(a) {
			t.Fatalf("Expected %s, got %s", expected.String(), a.String())
		}
	})

	t.Run("Delete", func(t *testing.T) {
		a := r.Copy()
		a.Delete("a", 0)

		expected := MakeRecord("b", "2", "b", "3")
		if !a.Equal(expected) {
			t.Fatalf("Expected %#v, got %#v", expected, a)
		}

		a = r.Copy()
		a.Delete("b", 1)

		expected = MakeRecord("a", "1", "b", "3")
		if !a.Equal(expected) {
			t.Fatalf("Expected %#v, got %#v", expected, a)
		}

		a = r.Copy()
		a.Delete("b", 2)

		expected = MakeRecord("a", "1")
		if !a.Equal(expected) {
			t.Fatalf("Expected %#v, got %#v", expected, a)
		}
	})
}
