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
	"unsafe"
)

type Ndb struct {
	data  [][]byte
	files []string
	sys   FileSystem
	// copyStrings forces Attr/Val to be copied out of file buffers. ParseOne
	// sets this so callers who mutate the input after Open still see stable
	// records they already held. Open/OpenOne own their ReadAll buffers, so
	// unquoted strings may alias that immutable data.
	copyStrings bool
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
//
// Attribute and value strings from Search/All may alias the in-memory file
// buffer. They stay valid after iteration, but keeping those strings alive
// also keeps the whole file buffer alive.
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
			// HasAttrValue is a substring prefilter; require a real key.
			if !record.HasKey("database") {
				continue
			}
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
//
// Attribute and value strings from Search/All may alias the in-memory file
// buffer. They stay valid after iteration, but keeping those strings alive
// also keeps the whole file buffer alive.
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
		files:       []string{"inline"},
		data:        [][]byte{p},
		sys:         nil,
		copyStrings: true,
	}, nil
}

func ParseOneString(s string) (*Ndb, error) {
	return ParseOne([]byte(s))
}

func validateRecords(data []byte) error {
	var rec Record
	var firstErr error
	forEachRecordLines(data, func(lines [][]byte, startLine int) bool {
		rec.zero()
		if err := parseRecordLines(lines, &rec, true); err != nil {
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
		if err := validateRecords(buf); err != nil {
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
// The Record yielded on each iteration is reused; call Record.Copy to retain it
// beyond the current step.
//
// Use Search to find matching records instead.
func (n *Ndb) All() iter.Seq[Record] {
	return n.byPredicate(nil)
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
// The Record yielded on each iteration is reused; call Record.Copy to retain it
// beyond the current step.
func (n *Ndb) Search(preds ...SearchPredicate) iter.Seq[Record] {
	if len(preds) == 0 {
		return n.byPredicate(nil)
	}
	return n.byPredicate(func(lines [][]byte) bool {
		for _, pred := range preds {
			if !pred.matchLines(lines) {
				return false
			}
		}
		return true
	})
}

// SearchPredicate selects records during Search. The interface is sealed:
// only predicates from this package (HasAttr, HasAttrValue) implement it.
type SearchPredicate interface {
	matchLines(lines [][]byte) bool
}

type attrPred struct {
	eq     []byte // " attr="
	sp     []byte // " attr "
	val    string
	hasVal bool
}

func (p attrPred) matchLines(lines [][]byte) bool {
	if !p.hasVal {
		return hasAttrLines(lines, p.eq, p.sp)
	}
	return hasAttrValLines(lines, p.eq, p.sp, p.val)
}

// HasAttr returns a predicate that matches records with the given attribute.
func HasAttr(attr string) SearchPredicate {
	return attrPred{
		eq: []byte(" " + attr + "="),
		sp: []byte(" " + attr + " "),
	}
}

// HasAttrValue returns a predicate that matches records with the given attribute and value.
// In multi-line records it can also match attr=value text inside another
// attribute's continued quoted value.
// For example, `b="multi\n\ta=1 z"` matches HasAttrValue("a", "1").
func HasAttrValue(attr, value string) SearchPredicate {
	return attrPred{
		eq:     []byte(" " + attr + "="),
		sp:     []byte(" " + attr + " "),
		val:    value,
		hasVal: true,
	}
}

func (n *Ndb) byPredicate(allow func(lines [][]byte) bool) iter.Seq[Record] {
	copyStr := n.copyStrings
	return func(yield func(Record) bool) {
		var results Record
		for i := range n.data {
			cont := true
			forEachRecordLines(n.data[i], func(lines [][]byte, _ int) bool {
				if allow != nil && !allow(lines) {
					return true
				}
				results.zero()
				if err := parseRecordLines(lines, &results, copyStr); err == nil {
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
		line := data
		found := false
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			line = data[:i]
			data = data[i+1:]
			found = true
		} else {
			data = nil
		}
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}
		if !fn(line, lineNo) {
			return
		}
		if !found {
			return
		}
		lineNo++
	}
}

// forEachRecordLines yields each record as comment-stripped, trimmed lines
// that still point into data. The lines slice is reused; callers must finish
// with a record before returning.
func forEachRecordLines(data []byte, fn func(lines [][]byte, startLine int) bool) {
	var lines [][]byte
	startLine := 0
	stopped := false
	forEachLine(data, func(line []byte, lineNo int) bool {
		line = stripComment(line)
		if len(bytes.TrimSpace(line)) == 0 {
			return true
		}
		if isRecordStart(line) && len(lines) > 0 {
			if !fn(lines, startLine) {
				stopped = true
				return false
			}
			lines = lines[:0]
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			return true
		}
		if len(lines) == 0 {
			startLine = lineNo
		}
		lines = append(lines, line)
		return true
	})
	if !stopped && len(lines) > 0 {
		fn(lines, startLine)
	}
}

func isRecordStart(line []byte) bool {
	if len(line) == 0 {
		return false
	}
	if line[0] < utf8.RuneSelf {
		return !isASCIISpace(line[0])
	}
	first, _ := utf8.DecodeRune(line)
	return !unicode.IsSpace(first)
}

func isASCIISpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r'
}

// stripComment cuts an unquoted '#' comment through end of line, matching
// parseTuple's quote rule: a quoted value starts only when '"' is the first
// byte after '='. Quoted spans are measured with quotedValueLen so both
// helpers share the same \" escape scan.
func stripComment(line []byte) []byte {
	if len(line) == 0 || bytes.IndexByte(line, '#') < 0 {
		return line
	}
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
		end := indexByte6(line[i:], '=', '#', ' ', '\t', '\r', '\n')
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
		n := indexByte5(line[i:], '#', ' ', '\t', '\r', '\n')
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

func hasAttrLines(lines [][]byte, attrEq, attrSp []byte) bool {
	for _, line := range lines {
		if spaceFramedContains(line, attrEq) || spaceFramedContains(line, attrSp) {
			return true
		}
	}
	return false
}

func hasAttrValKeys(recBytes, attrEq, attrSp []byte, value string) bool {
	off := 0
	for off < len(recBytes) {
		idx := bytes.Index(recBytes[off:], attrEq)
		if idx == -1 {
			return len(value) == 0 && bytes.Contains(recBytes, attrSp)
		}
		off += idx + 1
		tup, n, err := parseTuple(recBytes[off:], false)
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

func hasAttrValLines(lines [][]byte, attrEq, attrSp []byte, value string) bool {
	joinedChecked := false
	joinedMatch := false
	for i, line := range lines {
		from := 0
		for {
			idx := indexFramed(line, attrEq, from)
			if idx < 0 {
				break
			}
			p := line[idx:]
			if needJoinForQuote(p) && i+1 < len(lines) {
				if !joinedChecked {
					joinedMatch = hasAttrValKeys(joinFrom(lines), attrEq, attrSp, value)
					joinedChecked = true
				}
				if joinedMatch {
					return true
				}
				if isTupleStart(line, idx) {
					// Real multiline quote: do not rescan later
					// lines as new tuples (ip="a" / ip=1 inside).
					return false
				}
				from = idx + 1
				continue
			}
			tup, n, err := parseTuple(p, false)
			if err == nil && tup.Val == value {
				return true
			}
			if n <= 0 {
				break
			}
			from = idx + n
		}
	}
	if len(value) == 0 {
		for _, line := range lines {
			if spaceFramedContains(line, attrSp) {
				return true
			}
		}
	}
	return false
}

// spaceFramedContains reports whether key occurs in (" " + line + " ")
// without allocating that framed copy. Keys used here always start with space.
func spaceFramedContains(line, key []byte) bool {
	if len(key) == 0 {
		return true
	}
	if bytes.Contains(line, key) {
		return true
	}
	if key[0] == ' ' && bytes.HasPrefix(line, key[1:]) {
		return true
	}
	if key[len(key)-1] == ' ' && bytes.HasSuffix(line, key[:len(key)-1]) {
		return true
	}
	if key[0] == ' ' && key[len(key)-1] == ' ' && bytes.Equal(line, key[1:len(key)-1]) {
		return true
	}
	return false
}

// indexFramed finds key in (" " + line + " ") and returns the line index of
// the attribute name (the byte after key's leading space). key must start
// with space (as in " attr="). from is a line index to resume after a hit.
func indexFramed(line, key []byte, from int) int {
	if from <= 0 && key[0] == ' ' && bytes.HasPrefix(line, key[1:]) {
		return 0
	}
	start := from
	if start < 0 {
		start = 0
	}
	if start < len(line) {
		if i := bytes.Index(line[start:], key); i >= 0 {
			return start + i + 1 // skip the space in key
		}
	}
	return -1
}

func needJoinForQuote(p []byte) bool {
	end := indexByte5(p, '=', ' ', '\t', '\r', '\n')
	if end < 0 || p[end] != '=' {
		return false
	}
	vs := end + 1
	if vs >= len(p) || p[vs] != '"' {
		return false
	}
	q := p[vs:]
	n := quotedValueLen(q)
	if n < len(q) {
		return false
	}
	if n < 2 || q[n-1] != '"' {
		return true
	}
	esc := 0
	for i := n - 2; i >= 1 && q[i] == '\\'; i-- {
		esc++
	}
	return esc%2 == 1
}

func joinFrom(lines [][]byte) []byte {
	var buf []byte
	buf = append(buf, ' ')
	buf = append(buf, lines[0]...)
	buf = append(buf, ' ')
	for _, line := range lines[1:] {
		buf = append(buf, line...)
		buf = append(buf, ' ')
	}
	return buf
}

func skipLeadingSpace(p []byte) ([]byte, error) {
	for len(p) > 0 {
		if p[0] < utf8.RuneSelf {
			if isASCIISpace(p[0]) {
				p = p[1:]
				continue
			}
			return p, nil
		}
		ch, size := utf8.DecodeRune(p)
		if ch == utf8.RuneError {
			return nil, fmt.Errorf("invalid utf8 rune")
		}
		if unicode.IsSpace(ch) {
			p = p[size:]
			continue
		}
		return p, nil
	}
	return p, nil
}

// isTupleStart reports whether idx is the start of a top-level tuple on line,
// not a match inside an already-closed quoted value.
func isTupleStart(line []byte, idx int) bool {
	if idx < 0 || idx > len(line) {
		return false
	}
	p := line
	for len(p) > 0 {
		var err error
		p, err = skipLeadingSpace(p)
		if err != nil || len(p) == 0 {
			return false
		}
		off := len(line) - len(p)
		if off == idx {
			return true
		}
		if off > idx {
			return false
		}
		_, n, err := parseTuple(p, false)
		if n <= 0 {
			return false
		}
		_ = err
		p = p[n:]
	}
	return false
}

func joinRest(head []byte, rest [][]byte) []byte {
	n := len(head)
	for _, line := range rest {
		n += 1 + len(line)
	}
	buf := make([]byte, 0, n)
	buf = append(buf, head...)
	for _, line := range rest {
		buf = append(buf, ' ')
		buf = append(buf, line...)
	}
	return buf
}

func parseJoined(p []byte, results *Record) error {
	if !utf8.Valid(p) {
		return fmt.Errorf("invalid utf8 rune")
	}
	for len(p) > 0 {
		var err error
		p, err = skipLeadingSpace(p)
		if err != nil {
			return err
		}
		if len(p) == 0 {
			return nil
		}
		tup, n, err := parseTuple(p, true)
		if err != nil {
			return err
		}
		if n <= 0 {
			return fmt.Errorf("invalid syntax")
		}
		*results = append(*results, tup)
		p = p[n:]
	}
	return nil
}

func parseRecordLines(lines [][]byte, results *Record, copyStr bool) error {
	if *results == nil {
		*results = make(Record, 0, 10)
	}
	for i, line := range lines {
		if !utf8.Valid(line) {
			return fmt.Errorf("invalid utf8 rune")
		}
		p := line
		for len(p) > 0 {
			var err error
			p, err = skipLeadingSpace(p)
			if err != nil {
				return err
			}
			if len(p) == 0 {
				break
			}
			if i+1 < len(lines) && needJoinForQuote(p) {
				return parseJoined(joinRest(p, lines[i+1:]), results)
			}
			var tup Tuple
			var n int
			tup, n, err = parseTuple(p, copyStr)
			if err != nil {
				if i+1 < len(lines) {
					return parseJoined(joinRest(p, lines[i+1:]), results)
				}
				return err
			}
			*results = append(*results, tup)
			p = p[n:]
		}
	}
	return nil
}

func parseTuple(p []byte, copyStr bool) (Tuple, int, error) {
	end := indexByte5(p, '=', ' ', '\t', '\r', '\n')
	if end == -1 {
		return Tuple{cloneString(p, copyStr), ""}, len(p), nil
	}
	attr := cloneString(p[:end], copyStr)
	if p[end] != '=' {
		return Tuple{attr, ""}, end, nil
	}
	valueStart := end + 1
	if valueStart < len(p) && p[valueStart] == '"' {
		length := quotedValueLen(p[valueStart:])
		q := p[valueStart : valueStart+length]
		if length >= 2 && q[length-1] == '"' && bytes.IndexByte(q, '\\') < 0 {
			return Tuple{attr, cloneString(q[1:length-1], copyStr)}, valueStart + length, nil
		}
		actualValue, err := strconv.Unquote(cloneString(q, false))
		if err != nil {
			return Tuple{}, valueStart + length, err
		}
		return Tuple{attr, actualValue}, valueStart + length, nil
	}
	length := indexByte4(p[valueStart:], ' ', '\t', '\r', '\n')
	if length == -1 {
		length = len(p) - valueStart
	}
	return Tuple{attr, cloneString(p[valueStart:valueStart+length], copyStr)}, valueStart + length, nil
}

func cloneString(b []byte, copyStr bool) string {
	if len(b) == 0 {
		return ""
	}
	if copyStr {
		return string(b)
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func indexByte4(p []byte, a, b, c, d byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d {
			return i
		}
	}
	return -1
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d || x == e {
			return i
		}
	}
	return -1
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d || x == e || x == f {
			return i
		}
	}
	return -1
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
