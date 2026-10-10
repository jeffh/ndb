package ndb

import (
	"bytes"
	"fmt"
	"io"
	"iter"
	"slices"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type Ndb struct {
	data  [][]byte
	files []string
	sys   FileSystem
}

// Open opens a new Ndb database from the given file path. It will recursively resolve any
// reference databases in the filepath.
//
// Referenced databases can be done with a database attribute followed by file
// attributes in one entry:
//
// ```
// database file="other.ndb" file="another.ndb" file="more.ndb"
// ```
func Open(sys FileSystem, filepath string) (*Ndb, error) {
	if sys == nil {
		panic("sys is required")
	}
	db := &Ndb{
		files: []string{filepath},
		data:  make([][]byte, 1),
		sys:   sys,
	}
	count := 0
	for {
		n, err := db.readFiles(count)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			break
		}
		count += n
		for record := range db.Search(HasAttrValue("database", "")) {
			for _, file := range record.GetAll("file") {
				if !slices.Contains(db.files, file) {
					db.files = append(db.files, file)
					db.data = append(db.data, []byte{})
				}
			}
		}
	}
	return db, nil
}

// OpenOne opens a single file and returns a database. It will not recursively
// open other database references.
func OpenOne(sys FileSystem, filepath string) (*Ndb, error) {
	if sys == nil {
		panic("sys is required")
	}
	db := &Ndb{
		files: []string{filepath},
		data:  make([][]byte, 1),
		sys:   sys,
	}
	if _, err := db.readFiles(0); err != nil {
		return nil, err
	}
	return db, nil
}

func ParseOne(p []byte) (*Ndb, error) {
	if err := validateRecords(p); err != nil {
		return nil, err
	}
	return &Ndb{
		files: []string{"inline"},
		data:  [][]byte{p},
		sys:   nil,
	}, nil
}

func ParseOneString(s string) (*Ndb, error) {
	return ParseOne([]byte(s))
}

func validateRecords(data []byte) error {
	var rec Record
	var firstErr error
	forEachRawRecord(data, func(recBytes []byte, startLine int) bool {
		rec.zero()
		if err := parseRecord(recBytes, &rec); err != nil {
			firstErr = fmt.Errorf("line %d: %w", startLine, err)
			return false
		}
		return true
	})
	return firstErr
}

func (n *Ndb) readFile(fileToRead string) ([]byte, error) {
	f, err := n.sys.Open(fileToRead)
	if err != nil {
		return nil, err
	}
	buf, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		return nil, err
	}
	return buf, nil
}

func (n *Ndb) readFiles(skip int) (int, error) {
	if n.sys == nil {
		return 0, nil
	}
	count := 0
	for i, fileToRead := range n.files[skip:] {
		idx := i + skip
		buf, err := n.readFile(fileToRead)
		if err != nil {
			return count, err
		}
		n.data[idx] = buf
		count++
	}
	return count, nil
}

// All returns an iterator that yields all records in the database.
// This isn't particularly efficient to use in production, but may be useful when
// debugging issues.
//
// Use Search to find matching records instead.
func (n *Ndb) All() iter.Seq[Record] {
	return n.byPredicate(func(rec []byte) bool { return true })
}

// AllSlice returns a slice of all records in the database. This isn't
// efficient to use in production, but may be useful when debugging.
//
// Use SearchSlice to find matching records instead.
func (n *Ndb) AllSlice() []Record { return toSlice(n.All()) }

// SearchSlice returns a slice of records matching the given predicates.
func (n *Ndb) SearchSlice(preds ...SearchPredicate) []Record { return toSlice(n.Search(preds...)) }

// First returns the first record that matches the given attribute and value.
func (n *Ndb) First(attr, val string) Record {
	return first(n.Search(HasAttrValue(attr, val)))
}

// Search returns an iterator that yields records matching the given predicates.
func (n *Ndb) Search(preds ...SearchPredicate) iter.Seq[Record] {
	return n.byPredicate(func(rec []byte) bool {
		for _, pred := range preds {
			if !pred.match(rec) {
				return false
			}
		}
		return true
	})
}

type SearchPredicate interface{ match(rec []byte) bool }
type searchPredicate func(rec []byte) bool

func (sp searchPredicate) match(rec []byte) bool { return sp(rec) }

// HasAttr returns a predicate that matches records with the given attribute.
func HasAttr(attr string) SearchPredicate {
	return searchPredicate(func(rec []byte) bool {
		return hasAttr(rec, attr)
	})
}

// HasAttrValue returns a predicate that matches records with the given attribute and value.
func HasAttrValue(attr, value string) SearchPredicate {
	return searchPredicate(func(rec []byte) bool {
		return hasAttrVal(rec, attr, value)
	})
}

func (n *Ndb) byPredicate(allow func(rec []byte) bool) iter.Seq[Record] {
	var results Record
	return func(yield func(Record) bool) {
		for i := range n.data {
			cont := true
			forEachRawRecord(n.data[i], func(recBytes []byte, _ int) bool {
				if !allow(recBytes) {
					return true
				}
				results.zero()
				if err := parseRecord(recBytes, &results); err == nil {
					if !yield(results) {
						cont = false
						return false
					}
				}
				return true
			})
			if !cont {
				return
			}
		}
	}
}

func forEachLine(data []byte, fn func(line []byte, lineNo int) bool) {
	lineNo := 1
	for len(data) > 0 {
		line, rest, found := bytes.Cut(data, []byte{'\n'})
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if !fn(line, lineNo) {
			return
		}
		if !found {
			return
		}
		data = rest
		lineNo++
	}
}

func forEachRawRecord(data []byte, fn func(rec []byte, startLine int) bool) {
	recBytes := []byte{}
	startLine := 0
	stopped := false
	forEachLine(data, func(line []byte, lineNo int) bool {
		line = stripComment(line)
		if len(line) == 0 {
			return true
		}
		first, _ := utf8.DecodeRune(line)
		if !unicode.IsSpace(first) {
			if len(recBytes) > 0 {
				if !fn(recBytes, startLine) {
					stopped = true
					return false
				}
			}
			recBytes = recBytes[:0]
			startLine = lineNo
		}
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			if len(recBytes) == 0 {
				recBytes = append(recBytes, ' ')
				startLine = lineNo
			}
			recBytes = append(recBytes, line...)
			recBytes = append(recBytes, ' ')
		}
		return true
	})
	if !stopped && len(recBytes) > 0 {
		fn(recBytes, startLine)
	}
}

// stripComment cuts an unquoted '#' comment through end of line, matching
// parseTuple's quote rule: a quoted value starts only when '"' is the first
// byte after '='. Quoted spans are measured with quotedValueLen so both
// helpers share the same \" escape scan.
func stripComment(line []byte) []byte {
	i := 0
	for i < len(line) {
		for i < len(line) && isTupleSpace(line[i]) {
			i++
		}
		if i >= len(line) {
			return line
		}
		if line[i] == '#' {
			return line[:i]
		}
		end := bytes.IndexAny(line[i:], "=# \t\r\n")
		if end == -1 {
			return line
		}
		i += end
		if line[i] == '#' {
			return line[:i]
		}
		if isTupleSpace(line[i]) {
			continue
		}
		// '=' — only '"' immediately after it opens a quote, as in parseTuple.
		i++
		if i < len(line) && line[i] == '"' {
			i += quotedValueLen(line[i:])
			continue
		}
		if i >= len(line) {
			return line
		}
		n := bytes.IndexAny(line[i:], "# \t\r\n")
		if n == -1 {
			return line
		}
		if line[i+n] == '#' {
			return line[:i+n]
		}
		i += n
	}
	return line
}

func isTupleSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func hasAttr(recBytes []byte, attr string) bool {
	attrKey := []byte(" " + attr + "=")
	if bytes.Contains(recBytes, attrKey) {
		return true
	}
	return bytes.Contains(recBytes, []byte(" "+attr+" "))
}

func hasAttrVal(recBytes []byte, attr, value string) bool {
	attrKey := []byte(" " + attr + "=")
	off := 0
	for off < len(recBytes) {
		idx := bytes.Index(recBytes[off:], attrKey)
		if idx == -1 {
			return len(value) == 0 && bytes.Contains(recBytes, []byte(" "+attr+" "))
		}
		off += idx + 1
		tup, n, err := parseTuple(recBytes[off:])
		if err == nil && tup.Val == value {
			return true
		}
		if n <= 0 {
			return false
		}
		off += n
	}
	return false
}

func parseRecord(recBytes []byte, results *Record) error {
	if !utf8.Valid(recBytes) {
		return fmt.Errorf("invalid utf8 rune")
	}
	if *results == nil {
		*results = make(Record, 0, 10)
	}
	r := recBytes
	for len(r) > 0 {
		ch, size := utf8.DecodeRune(r)
		if ch == utf8.RuneError {
			return fmt.Errorf("invalid utf8 rune")
		}
		if unicode.IsSpace(ch) {
			r = r[size:]
			continue
		}
		tup, n, err := parseTuple(r)
		if err != nil {
			return err
		}
		*results = append(*results, tup)
		r = r[n:]
	}
	return nil
}

func parseTuple(p []byte) (Tuple, int, error) {
	end := bytes.IndexAny(p, "= \t\r\n")
	if end == -1 {
		return Tuple{string(p), ""}, len(p), nil
	}
	attr := string(p[:end])
	if p[end] != '=' {
		return Tuple{attr, ""}, end, nil
	} else {
		valueStart := end + 1
		firstValue, _ := utf8.DecodeRune(p[valueStart:])
		if firstValue == '"' {
			length := quotedValueLen(p[valueStart:])
			actualValue, err := strconv.Unquote(string(p[valueStart : valueStart+length]))
			if err != nil {
				return Tuple{}, valueStart + length, err
			}
			return Tuple{attr, actualValue}, valueStart + length, nil
		} else {
			length := bytes.IndexAny(p[valueStart:], " \t\r\n")
			if length == -1 {
				length = len(p) - valueStart
			}

			return Tuple{attr, string(p[valueStart : valueStart+length])}, valueStart + length, nil
		}
	}
}

// quotedValueLen returns the length of a strconv-quoted value starting at p,
// including both quotes. p[0] is the opening '"'. An escaped \" does not
// close the quote; if the closer is missing the result is len(p).
// stripComment uses this for the same escape rules.
func quotedValueLen(p []byte) int {
	escape := false
	for i := 1; i < len(p); i++ {
		c := p[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			return i + 1
		}
	}
	return len(p)
}

func toSlice(it iter.Seq[Record]) []Record {
	var results []Record
	for rec := range it {
		newRecord := make(Record, len(rec))
		copy(newRecord, rec)
		results = append(results, newRecord)
	}
	return results
}

func first(it iter.Seq[Record]) Record {
	for rec := range it {
		return rec
	}
	return nil
}
