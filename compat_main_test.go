package ndb

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"
	"unicode"
	"unicode/utf8"
)

// Reference implementation copied from main (9fc49af / #15) so tests can
// compare the optimized walker against the concat+parse behavior that shipped.

func mainForEachLine(data []byte, fn func(line []byte, lineNo int) bool) {
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

func mainForEachRawRecord(data []byte, fn func(rec []byte, startLine int) bool) {
	recBytes := []byte{}
	startLine := 0
	stopped := false
	mainForEachLine(data, func(line []byte, lineNo int) bool {
		line = mainStripComment(line)
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

func mainStripComment(line []byte) []byte {
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '\r' || line[i] == '\n') {
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
		if line[i] == ' ' || line[i] == '\t' || line[i] == '\r' || line[i] == '\n' {
			continue
		}
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

func mainParseTuple(p []byte) (Tuple, int, error) {
	end := bytes.IndexAny(p, "= \t\r\n")
	if end == -1 {
		return Tuple{string(p), ""}, len(p), nil
	}
	attr := string(p[:end])
	if p[end] != '=' {
		return Tuple{attr, ""}, end, nil
	}
	valueStart := end + 1
	firstValue, _ := utf8.DecodeRune(p[valueStart:])
	if firstValue == '"' {
		length := quotedValueLen(p[valueStart:])
		actualValue, err := strconv.Unquote(string(p[valueStart : valueStart+length]))
		if err != nil {
			return Tuple{}, valueStart + length, err
		}
		return Tuple{attr, actualValue}, valueStart + length, nil
	}
	length := bytes.IndexAny(p[valueStart:], " \t\r\n")
	if length == -1 {
		length = len(p) - valueStart
	}
	return Tuple{attr, string(p[valueStart : valueStart+length])}, valueStart + length, nil
}

func mainParseRecord(recBytes []byte, results *Record) error {
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
		tup, n, err := mainParseTuple(r)
		if err != nil {
			return err
		}
		*results = append(*results, tup)
		r = r[n:]
	}
	return nil
}

func mainValidate(data []byte) error {
	var rec Record
	var firstErr error
	mainForEachRawRecord(data, func(recBytes []byte, startLine int) bool {
		rec.zero()
		if err := mainParseRecord(recBytes, &rec); err != nil {
			firstErr = fmt.Errorf("line %d: %w", startLine, err)
			return false
		}
		return true
	})
	return firstErr
}

func mainAllSlice(data []byte) []Record {
	var out []Record
	var rec Record
	mainForEachRawRecord(data, func(recBytes []byte, _ int) bool {
		rec.zero()
		if err := mainParseRecord(recBytes, &rec); err == nil {
			out = append(out, rec.Copy())
		}
		return true
	})
	return out
}

func mainHasAttrVal(recBytes []byte, attr, value string) bool {
	attrKey := []byte(" " + attr + "=")
	off := 0
	for off < len(recBytes) {
		idx := bytes.Index(recBytes[off:], attrKey)
		if idx == -1 {
			return len(value) == 0 && bytes.Contains(recBytes, []byte(" "+attr+" "))
		}
		off += idx + 1
		tup, n, err := mainParseTuple(recBytes[off:])
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

func mainSearchHasAttrValue(data []byte, attr, value string) []Record {
	var out []Record
	var rec Record
	mainForEachRawRecord(data, func(recBytes []byte, _ int) bool {
		if !mainHasAttrVal(recBytes, attr, value) {
			return true
		}
		rec.zero()
		if err := mainParseRecord(recBytes, &rec); err == nil {
			out = append(out, rec.Copy())
		}
		return true
	})
	return out
}

func recordsEqual(a, b []Record) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func compareAgainstMain(t *testing.T, in []byte) {
	t.Helper()
	mainErr := mainValidate(in)
	got, err := ParseOne(in)

	if mainErr != nil {
		if err == nil {
			t.Fatalf("ParseOne(%q) succeeded, main rejected: %v", in, mainErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("ParseOne(%q) failed %v, main accepted", in, err)
	}

	wantRecs := mainAllSlice(in)
	gotRecs := got.AllSlice()
	if !recordsEqual(gotRecs, wantRecs) {
		t.Fatalf("AllSlice mismatch for %q\n got %v\nwant %v", in, gotRecs, wantRecs)
	}

	seen := map[[2]string]struct{}{}
	for _, rec := range wantRecs {
		for _, tup := range rec {
			key := [2]string{tup.Attr, tup.Val}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			wantHits := mainSearchHasAttrValue(in, tup.Attr, tup.Val)
			gotHits := got.SearchSlice(HasAttrValue(tup.Attr, tup.Val))
			if !recordsEqual(gotHits, wantHits) {
				t.Fatalf("HasAttrValue(%q,%q) for %q\n got %v\nwant %v", tup.Attr, tup.Val, in, gotHits, wantHits)
			}
		}
	}
}

func TestCompatVsMainSeeded(t *testing.T) {
	seeds := []string{
		`k="a` + "\n\tb\n\tc\"\n",
		"k=\"abc\\\"\n\tmore\"\n",
		`a="x a=1 "`,
		"k=\"a\r\n\tb\r\n\tc\"\r\n",
		`person name=John age=30`,
		"person name=John\n\tage=30\n",
		`note=" sys=foo " extra=1`,
		`name="foo#bar"`,
		`size=6" # inches`,
		`sys=a ip=1.2.3.4 ip=5.6.7.8`,
		"ok=1\nperson name=Jane\n",
		`# comment only`,
		``,
		`dhcp`,
		`name=`,
		`name=""`,
		"k=\"a\n\tb\" extra=1\n",
		"a=1\n\tb=\"x\n\ty\n\tz\"\n",
	}
	for i, s := range seeds {
		t.Run(fmt.Sprintf("seed%d", i), func(t *testing.T) {
			compareAgainstMain(t, []byte(s))
		})
	}
}

func FuzzParseCompat(f *testing.F) {
	for _, s := range []string{
		`k="a` + "\n\tb\n\tc\"\n",
		"k=\"abc\\\"\n\tmore\"\n",
		`a="x a=1 "`,
		"k=\"a\r\n\tb\r\n\tc\"\r\n",
		`person name=John age=30`,
		"person name=John\n\tage=30\n",
		`note=" sys=foo " extra=1`,
		`name="foo#bar"`,
		`size=6" # inches`,
		`sys=a ip=1.2.3.4 ip=5.6.7.8`,
		`dhcp`,
		`name=""`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}
		in := []byte(s)
		mainErr := mainValidate(in)
		got, err := ParseOne(in)
		if mainErr != nil {
			if err == nil {
				t.Fatalf("ParseOne accepted %q; main rejected: %v", s, mainErr)
			}
			return
		}
		if err != nil {
			t.Fatalf("ParseOne rejected %q (%v); main accepted", s, err)
		}
		wantRecs := mainAllSlice(in)
		gotRecs := got.AllSlice()
		if !recordsEqual(gotRecs, wantRecs) {
			t.Fatalf("AllSlice mismatch for %q\n got %v\nwant %v", s, gotRecs, wantRecs)
		}
	})
}
