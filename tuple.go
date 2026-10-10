package ndb

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Tuple struct {
	Attr, Val string
}

type Record []Tuple

func (r *Record) zero() { *r = (*r)[:0] }

// MakeRecord creates a Record from a list of attribute-value pairs. The number of arguments must be even.
func MakeRecord(avPairs ...string) Record {
	if len(avPairs)%2 == 1 {
		panic("MakeRecord() requires an even number of arguments")
	}
	r := make(Record, len(avPairs)/2)
	j := 0
	for i := 0; i < len(avPairs); i += 2 {
		r[j] = Tuple{avPairs[i], avPairs[i+1]}
		j++
	}
	return r
}

func (r Record) Keys() []string {
	keys := make([]string, 0, len(r))
	for _, t := range r {
		if !slices.Contains(keys, t.Attr) {
			keys = append(keys, t.Attr)
		}
	}
	return keys
}

func (r Record) AsMap() map[string][]string {
	m := make(map[string][]string, len(r))
	for _, t := range r {
		m[t.Attr] = append(m[t.Attr], t.Val)
	}
	return m
}

func (r Record) Lookup(attr string) (string, bool) {
	for _, t := range r {
		if t.Attr == attr {
			return t.Val, true
		}
	}
	return "", false
}

func (r Record) Get(attr string) string {
	for _, t := range r {
		if t.Attr == attr {
			return t.Val
		}
	}
	return ""
}

func (r Record) GetAll(attr string) []string {
	var results []string
	for _, t := range r {
		if t.Attr == attr {
			results = append(results, t.Val)
		}
	}
	return results
}

func (r Record) String() string {
	var sb strings.Builder
	for i, t := range r {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(t.Attr)
		sb.WriteString("=")
		if needsQuote(t.Val) {
			sb.WriteString(strconv.Quote(t.Val))
		} else {
			sb.WriteString(t.Val)
		}
	}
	return sb.String()
}

func needsQuote(s string) bool {
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '=' ||
			r == '"' || r == '\\' || r == '#' || r >= utf8.RuneSelf {
			return true
		}
	}
	return false
}

// Len returns the number of tuples in the record
func (r Record) Len() int { return len(r) }

// Values returns all values in the record (may contain duplicates)
func (r Record) Values() []string {
	values := make([]string, len(r))
	for i, t := range r {
		values[i] = t.Val
	}
	return values
}

// HasKey returns true if the record has the given key
func (r Record) HasKey(key string) bool {
	for _, t := range r {
		if t.Attr == key {
			return true
		}
	}
	return false
}

// KeyAt returns the key at the given index
func (r Record) KeyAt(index int) string { return r[index].Attr }

// ValueAt returns the value at the given index
func (r Record) ValueAt(index int) string { return r[index].Val }

// TupleAt returns the tuple at the given index
func (r Record) TupleAt(index int) Tuple {
	return r[index]
}

// Delete deletes N occurrences of a key.
// If n=0, then all tuples with the given key are deleted
func (r *Record) Delete(key string, n int) {
	numDel := 0
	i := 0
	for i < len(*r) {
		if (*r)[i].Attr == key {
			*r = append((*r)[:i], (*r)[i+1:]...)
			numDel++
			if n != 0 && n == numDel {
				return
			}
		} else {
			i++
		}
	}
}

// ReplaceAt replaces the tuple at the given index
func (r *Record) ReplaceAt(i int, t Tuple) {
	(*r)[i] = t
}

// Replace replaces all instances of oldValue with newValue for the given key
func (r *Record) Replace(key, oldValue, newValue string) {
	for i := range *r {
		if (*r)[i].Attr == key && (*r)[i].Val == oldValue {
			(*r)[i].Val = newValue
		}
	}
}

// Put replaces all keys found with the one tuple given
func (r *Record) Put(t Tuple) {
	r.Delete(t.Attr, 0)
	*r = append(*r, t)
}

// Add adds a new tuple to the record
func (r *Record) Add(t Tuple) {
	*r = append(*r, t)
}

// Copy creates a deep copy of the record
func (r Record) Copy() Record {
	newRecord := make(Record, len(r))
	copy(newRecord, r)
	return newRecord
}
