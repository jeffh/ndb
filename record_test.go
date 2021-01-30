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
	if r.Len() != 3 {
		t.Fatalf("Expected 3 tuples, got %v", r.Len())
	}

	{
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
	}
	{
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
	}
	{
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
	}
	{
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
	}
}
