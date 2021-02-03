package ndb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
)

type PredicateFunc func(r *Record) bool

func FindKeyThen(r *Record, key string, pred func(fieldIndex int) bool) bool {
	for i, size := 0, r.Len(); i < size; i++ {
		if r.KeyAt(i) == key && pred(i) {
			return true
		}
	}
	return false
}

func OpenWithFS(ctx context.Context, fs FileSystem, filename string) (*DB, error) {
	f, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	db, err := ParseBytes(buf, filename)
	if err != nil {
		return nil, err
	}
	err = db.OpenChildren(ctx, fs)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Open(ctx context.Context, filename string) (*DB, error) {
	return OpenWithFS(ctx, DefaultFileSystem, filename)
}

// Parse returns a DB from parsing a raw string
func Parse(raw, filename string) (*DB, error) {
	recs, err := parseRecords(raw, filename)
	if err != nil {
		return nil, err
	}
	db := &DB{Records: recs}
	return db, nil
}

func ParseBytes(raw []byte, filename string) (*DB, error) { return Parse(string(raw), filename) }

type DB struct {
	Filename string

	M        sync.RWMutex
	Records  []Record
	Children []*DB
}

type Record struct {
	keys   []string
	values []string
}

type Tuple struct {
	Key, Value string
}

func MakeDB(r []Record) *DB {
	return &DB{Records: r}
}

func (db *DB) OpenChildren(ctx context.Context, fs FileSystem) error {
	seen := make(map[string]struct{})
	return db.openChildren(ctx, fs, seen)
}

func (db *DB) openChildren(ctx context.Context, fs FileSystem, seen map[string]struct{}) error {
	db.M.Lock()
	defer db.M.Unlock()
	it := db.searchNoLock("database", "")

	type result struct {
		i   int
		db  *DB
		err error
	}

	res := make(chan result, 10)

	subctx, cancel := context.WithCancel(ctx)
	defer cancel()

	numFiles := 0
	for it.Next() {
		r := it.Record()
		files := r.ValuesForKey("file")
		numFiles += len(files)
		for i, file := range files {
			if _, ok := seen[file]; !ok {
				seen[file] = struct{}{}
				go func(i int, file string) {
					d, err := OpenWithFS(subctx, fs, file)
					out := result{
						i:   i,
						db:  d,
						err: err,
					}

					select {
					case res <- out:
					case <-subctx.Done():
					}
				}(i, file)
			}
		}
	}
	it.Close()

	if numFiles > 0 {
		children := make([]*DB, numFiles)
		for i := 0; i < numFiles; i++ {
			select {
			case res := <-res:
				if res.err != nil {
					return res.err
				}
				children[res.i] = res.db
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		for i, c := range children {
			if c == nil {
				copy(children[i:], children[i+1:])
			}
		}
		db.Children = children
	}
	return it.Err()
}

func (db *DB) WithFilename(filename string) *DB {
	db.Filename = filename
	return db
}

func (db *DB) WithChildren(children ...*DB) *DB {
	db.Children = children
	return db
}

func (db *DB) Save(ctx context.Context, fs FileSystem) error {
	if db.Filename == "" {
		return errors.New("Filename not specified for DB")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	f, err := fs.CreateOrTruncate(db.Filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for i, record := range db.Records {
		if i != 0 {
			if _, err = f.Write([]byte("\n")); err != nil {
				return err
			}
		}
		if err = record.write(f); err != nil {
			return err
		}
	}

	if len(db.Children) > 0 {
		out := make(chan error, len(db.Children))
		count := int32(1)
		for i := range db.Children {
			child := db.Children[i]
			atomic.AddInt32(&count, 1)
			go func() {
				if err := child.Save(ctx, fs); err != nil {
					out <- err
				}
				res := atomic.AddInt32(&count, -1)
				if res == 0 {
					close(out)
				}
			}()
		}
		res := atomic.AddInt32(&count, -1)
		if res == 0 {
			close(out)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-out:
			if !ok {
				return nil
			}
			return err
		}
	}

	return nil
}

// Len returns the number of records in the database, including children
func (db *DB) Len() int {
	db.M.RLock()
	defer db.M.RUnlock()
	n := len(db.Records)
	for _, child := range db.Children {
		n += child.Len()
	}
	return n
}

func (db *DB) searchNoLock(key, value string) *searchIterator {
	return makeSearchIterator(context.Background(), db, false, key, value)
}

// SearchRoot only searches for records in the current database (and not any children)
func (db *DB) SearchRoot(key, value string) Iterator {
	return makeSearchIterator(context.Background(), db, true, key, value)
}

// Search returns an iterator of records that have the given key,value pair
func (db *DB) Search(key, value string) Iterator {
	it := makeSearchIterator(context.Background(), db, true, key, value)
	if len(db.Children) == 0 {
		return it
	}

	ch := make(chan Iterator, 1)
	ch <- it

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		db.M.RLock()
		defer db.M.RUnlock()
		for _, child := range db.Children {
			select {
			case <-ctx.Done():
				return
			default:
				it := makeSearchIterator(ctx, child, true, key, value)
				select {
				case ch <- it:
				case <-ctx.Done():
					return
				}
			}
		}

		close(ch)
	}()

	return &lazyIterator{ctx: ctx, cancel: cancel, ch: ch}
}

// SearchPredicate filters over the database satisfies the predicate
// value. pred is assumed to be a pure function that is safe to run in
// multiple goroutines.
func (db *DB) SearchPredicate(pred PredicateFunc) Iterator {
	it := makePredicateSearchIterator(context.Background(), db, true, pred)
	if len(db.Children) == 0 {
		return it
	}

	ch := make(chan Iterator, 4)
	ch <- it

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		db.M.RLock()
		defer db.M.RUnlock()
		for _, child := range db.Children {
			select {
			case <-ctx.Done():
				return
			default:
				it := makePredicateSearchIterator(ctx, child, true, pred)
				select {
				case ch <- it:
				case <-ctx.Done():
					return
				}
			}
		}

		close(ch)
	}()

	return &lazyIterator{ctx: ctx, cancel: cancel, ch: ch}
}

// Find behaves like Search, but returns a slice of all results that match the
// given key value pair.
func (db *DB) Find(key, value string) []Record {
	return toSlice(db.Search(key, value))
}

// FindPredicate behaves like SearchPredicate, but returns a slice of all
// results that match the given key and predicate.
func (db *DB) FindPredicate(pred PredicateFunc) []Record {
	return toSlice(db.SearchPredicate(pred))
}

// FindFirst returns the first record that matches the given key value pair.
func (db *DB) FindFirst(key, value string) (Record, bool) {
	return first(db.Search(key, value))
}

// FindFirstPredicate behaves like SearchPredicate, but returns a slice of all
// results that match the given key and predicate.
func (db *DB) FindFirstPredicate(pred PredicateFunc) (Record, bool) {
	return first(db.SearchPredicate(pred))
}

func parseRecords(raw, filename string) ([]Record, error) {
	const debug = false
	// state
	var (
		isBeginningOfLine bool = true
		parsingState      int  = parsingStart
		startIndex        int
		currTuple         Tuple

		col, line int

		records []Record
		record  Record
	)

	if debug {
		fmt.Printf("------------ PARSE\n")
	}

	for i, r := range raw {
		col++
		if isBeginningOfLine {
			line++
			col = 1
		}
		if debug {
			fmt.Printf("[%d: %#v] Step: (isBeginningOfLine=%v, state=%s, line=%d, col=%d)\n", i, string(r), isBeginningOfLine, stateString(parsingState), line, col)
		}
		switch parsingState {
		case parsingStart:
			if isBeginningOfLine && r == '#' {
				parsingState = parsingComment
				startIndex = i
			} else {
				if isBeginningOfLine && !unicode.IsSpace(r) {
					// new tuple, commit the one we're holding
					if record.Len() != 0 {
						records = append(records, record)
						record = Record{}
					}
				}
				if !unicode.IsSpace(r) {
					startIndex = i
					if r == '"' {
						parsingState = parsingKeyQuoted
					} else {
						parsingState = parsingKey
					}
				}
			}
		case parsingKey:
			isSpace := unicode.IsSpace(r)
			if isSpace || r == '=' {
				key := raw[startIndex:i]
				if len(key) > 0 {
					if key == "=" {
						return nil, mkE(ErrCodeEqualsCannotBeKey, line, col, filename, "'=' is an invalid key. Please surround it in double quotes if you want to have it as a key.")
					}
					currTuple.Key = key
					startIndex = i
					if isSpace {
						record.Add(currTuple)
						currTuple = Tuple{}
						parsingState = parsingStart
					} else {
						parsingState = parsingEqual
					}
				}
			}
		case parsingKeyQuoted:
			if r == '"' {
				numEscapes := 0
				for j := i - 1; j >= 0; j-- {
					if raw[j] != '\\' {
						numEscapes = i - j - 1
						break
					}
				}
				if numEscapes%2 == 0 {
					key := strings.ReplaceAll(raw[startIndex+1:i], "\\\\", "\\")
					if len(key) > 0 {
						currTuple.Key = key
						parsingState = parsingEqual
						startIndex = i + 1
					} else {
						return nil, mkE(ErrCodeKeyCannotBeEmpty, line, col, filename, "Keys cannot be empty.")
					}
				}
			}
		case parsingEqual:
			if r == '=' {
				if raw[startIndex:i] == "=" {
					return nil, mkE(ErrCodeEqualsCannotBeValue, line, col, filename, "'=' is an invalid value. Please surround it in double quotes if you want to have it as a value.")
				}
			} else if unicode.IsSpace(r) {
				record.Add(currTuple)
				currTuple = Tuple{}
				parsingState = parsingStart
				startIndex = i + 1
			} else {
				startIndex = i
				if r == '"' {
					parsingState = parsingValueQuoted
				} else {
					parsingState = parsingValue
				}
			}
		case parsingValue:
			if unicode.IsSpace(r) {
				currTuple.Value = raw[startIndex:i]
				if currTuple.Value == "=" {
					return nil, mkE(ErrCodeEqualsCannotBeValue, line, col, filename, "'=' is an invalid value. Please surround it in double quotes if you want to have it as a value.")
				}
				record.Add(currTuple)
				currTuple = Tuple{}
				parsingState = parsingStart
				startIndex = i + 1
			}
		case parsingValueQuoted:
			if r == '"' {
				numEscapes := 0
				for j := i - 1; j >= 0; j-- {
					if raw[j] != '\\' {
						numEscapes = i - j - 1
						break
					}
				}
				if numEscapes%2 == 0 {
					currTuple.Value = raw[startIndex+1 : i]
					record.Add(currTuple)
					currTuple = Tuple{}
					parsingState = parsingStart
					startIndex = i + 1
				}
			}
		case parsingComment:
			if r == '\n' {
				parsingState = parsingStart
				startIndex = i + 1
			}
		}
		isBeginningOfLine = r == '\n'
	}

	if debug {
		fmt.Printf("[%d: $END] Step: (isBeginningOfLine=%v, state=%s, line=%d, col=%d)\n", len(raw), isBeginningOfLine, stateString(parsingState), line, col)
	}

	switch parsingState {
	case parsingKey:
		currTuple = Tuple{Key: raw[startIndex:]}
		if currTuple.Key != "" {
			if currTuple.Key == "=" {
				return nil, mkE(ErrCodeEqualsCannotBeKey, line, col, filename, "'=' cannot be a key")
			}
			record.Add(currTuple)
			records = append(records, record)
		}
	case parsingKeyQuoted:
		return nil, mkE(ErrCodeKeyNotClosed, line, col, filename, "quote not closed for key")
	case parsingEqual:
		if currTuple.Key != "" {
			record.Add(currTuple)
			records = append(records, record)
		}
	case parsingValue:
		if currTuple.Key != "" {
			currTuple.Value = raw[startIndex:]
			if currTuple.Value == "=" {
				return nil, mkE(ErrCodeEqualsCannotBeValue, line, col, filename, "'=' is an invalid value. Please surround it in double quotes if you want to have it as a value.")
			}
			record.Add(currTuple)
			records = append(records, record)
		}
	case parsingValueQuoted:
		return nil, mkE(ErrCodeValueNotClosed, line, col, filename, "quote not closed for value")
	default:
		if record.Len() != 0 {
			records = append(records, record)
		}
	}

	return records, nil
}

const (
	parsingStart = iota
	parsingKey
	parsingKeyQuoted
	parsingEqual
	parsingValue
	parsingValueQuoted
	parsingComment
)

func stateString(s int) string {
	switch s {
	case parsingStart:
		return "start"
	case parsingKey:
		return "key"
	case parsingKeyQuoted:
		return "key-quoted"
	case parsingEqual:
		return "equal"
	case parsingValue:
		return "value"
	case parsingValueQuoted:
		return "value-quoted"
	case parsingComment:
		return "comment"
	default:
		panic("Unreachable")
	}
}

////////////////////////////////

func MakeRecord(kvs ...string) Record {
	size := len(kvs)
	if size%2 != 0 {
		panic("kvs should be even number of items")
	}
	end := size / 2
	r := Record{
		keys:   make([]string, 0, end),
		values: make([]string, 0, end),
	}
	for i := 0; i < size; i += 2 {
		r.keys = append(r.keys, kvs[i])
		r.values = append(r.values, kvs[i+1])
	}
	return r
}

func MakeRecordFromMap(m map[string][]string) Record {
	size := len(m)
	r := Record{
		keys:   make([]string, 0, size),
		values: make([]string, 0, size),
	}
	for k, v := range m {
		r.keys = append(r.keys, k)
		if len(v) == 0 {
			r.values = append(r.values, "")
		} else {
			r.values = append(r.values, v...)
		}
	}
	return r
}

func (a Record) write(w io.Writer) error {
	for i, k := range a.keys {
		k = quoteIfNeeded(k)
		v := quoteIfNeeded(a.values[i])
		var spacing string
		if i != 0 {
			spacing = " "
		}
		_, err := w.Write([]byte(fmt.Sprintf("%s%s=%s", spacing, k, v)))
		if err != nil {
			return err
		}
	}
	return nil
}

func (a Record) Len() int { return len(a.keys) }
func (a Record) Keys() []string {
	out := make([]string, len(a.keys))
	copy(out, a.keys)
	return out
}
func (a Record) Values() []string {
	out := make([]string, len(a.values))
	copy(out, a.values)
	return out
}
func (a Record) KeyAt(index int) string   { return a.keys[index] }
func (a Record) ValueAt(index int) string { return a.values[index] }
func (a Record) ValuesForKey(key string) []string {
	var out []string
	for i, k := range a.keys {
		if k == key {
			out = append(out, a.values[i])
		}
	}
	return out
}
func (a Record) TupleAt(index int) Tuple {
	return Tuple{
		Key:   a.keys[index],
		Value: a.values[index],
	}
}
func (a Record) Tuples() []Tuple {
	out := make([]Tuple, a.Len())
	for i, k := range a.keys {
		out[i] = Tuple{
			Key:   k,
			Value: a.values[i],
		}
	}
	return out
}

// Delete deletes N number of occurrences of a key.
// If n=0, then all tuples of the given key is deleted
func (a *Record) Delete(key string, n int) {
	numDel := 0
	for i, k := range a.keys {
		if k == key {
			copy(a.keys[i:], a.keys[i+1:])
			copy(a.values[i:], a.values[i+1:])
			a.keys = a.keys[:len(a.keys)-1]
			a.values = a.values[:len(a.values)-1]
			numDel++
			if n != 0 && n == numDel {
				return
			}
		}
	}
}

func (a *Record) Replace(key, oldValue, newValue string) {
	for i, k := range a.keys {
		if k == key && a.values[i] == oldValue {
			a.values[i] = newValue
		}
	}
}

// Put replaces the all keys found with the one tuple given
func (a *Record) Put(t Tuple) {
	a.Delete(t.Key, 0)
	a.keys = append(a.keys, t.Key)
	a.values = append(a.values, t.Value)
}

// Add adds a new key-value pair to the record
func (a *Record) Add(t Tuple) {
	a.keys = append(a.keys, t.Key)
	a.values = append(a.values, t.Value)
}

func (a Record) String() string {
	var sb []string
	for i, k := range a.keys {
		k = quoteIfNeeded(k)
		v := quoteIfNeeded(a.values[i])
		sb = append(sb, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(sb, " ")
}

func (a Record) Map() map[string][]string {
	m := make(map[string][]string)
	for i, k := range a.keys {
		out, _ := m[k]
		m[k] = append(out, a.values[i])
	}
	return m
}

// TODO: optimize??
func (a Record) Equal(b Record) bool { return reflect.DeepEqual(a.Map(), b.Map()) }

func valueNeedsQuoting(x string) bool {
	return len(x) == 0 || strings.ContainsAny(x, "\t\n\v\f\r =\"")
}

var escaper *strings.Replacer

func init() {
	escaper = strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
	)
}

func quoteIfNeeded(x string) string {
	if valueNeedsQuoting(x) {
		var buf bytes.Buffer
		buf.Write([]byte("\""))
		escaper.WriteString(&buf, x)
		buf.Write([]byte("\""))
		return buf.String()
	}
	return x
}

//////////////////////////////////////

func first(it Iterator) (Record, bool) {
	defer it.Close()
	if it.Next() {
		return *it.Record(), true
	} else {
		return Record{}, false
	}
}

func toSlice(it Iterator) []Record {
	var out []Record
	defer it.Close()
	for it.Next() {
		out = append(out, *it.Record())
	}
	if it.Err() != nil {
		return nil
	}
	return out
}
