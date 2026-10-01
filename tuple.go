package ndb

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

type Tuple struct {
	Attr, Val string
}

type Record []Tuple

func (r *Record) zero() { *r = (*r)[:0] }

// MapToRecord converts a map of strings to a Record. The keys of the map are the attributes and the values are the values.
// Ordering is by sorted keys.
func MapToRecord(m map[string]string) Record {
	r := make(Record, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		r = append(r, Tuple{k, m[k]})
	}
	return r
}

// MapSliceToRecord converts a map of strings to a Record. The keys of the map are the attributes and the values are the values.
// Ordering is by sorted keys.
func MapSliceToRecord(m map[string][]string) Record {
	r := make(Record, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vs := m[k]
		for _, v := range vs {
			r = append(r, Tuple{k, v})
		}
	}
	return r
}

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

// ParseRecord parses a single record from a string. The string should be a line containing all the attributes and values.
func ParseRecord(line string) (Record, error) {
	var results Record
	err := parseRecord([]byte(line), &results)
	return results, err
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
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '=' || r > 128 {
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

// Tuples returns all tuples in the record
func (r Record) Tuples() []Tuple {
	tuples := make([]Tuple, len(r))
	copy(tuples, r)
	return tuples
}

// Delete deletes N occurrences of a key.
// If n=0, then all tuples with the given key are deleted
func (r *Record) Delete(key string, n int) {
	numDel := 0
	i := 0
	for i < len(*r) {
		if (*r)[i].Attr == key {
			// Remove this element
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

// ReplaceTimes replaces up to n instances of oldValue with newValue for the given key
func (r *Record) ReplaceTimes(key, oldValue, newValue string, n int) {
	if n <= 0 {
		return
	}
	for i := range *r {
		if (*r)[i].Attr == key && (*r)[i].Val == oldValue {
			(*r)[i].Val = newValue
			n--
			if n == 0 {
				return
			}
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

// Equal returns true if two records have the same key-value pairs
func (r Record) Equal(other Record) bool {
	if len(r) != len(other) {
		return false
	}
	rMap := r.AsMap()
	otherMap := other.AsMap()

	if len(rMap) != len(otherMap) {
		return false
	}

	for k, v := range rMap {
		otherV, ok := otherMap[k]
		if !ok {
			return false
		}
		if len(v) != len(otherV) {
			return false
		}
		// Check all values match (order-independent)
		for _, val := range v {
			found := false
			for _, otherVal := range otherV {
				if val == otherVal {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}
