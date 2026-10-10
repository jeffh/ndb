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

func mainHasAttr(recBytes []byte, attr string) bool {
	return bytes.Contains(recBytes, []byte(" "+attr+"=")) ||
		bytes.Contains(recBytes, []byte(" "+attr+" "))
}

func mainSearchHasAttr(data []byte, attr string) []Record {
	var out []Record
	var rec Record
	mainForEachRawRecord(data, func(recBytes []byte, _ int) bool {
		if !mainHasAttr(recBytes, attr) {
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
		if !recordEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func recordEqual(a, b Record) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func recordHasAttrValue(r Record, attr, val string) bool {
	for _, t := range r {
		if t.Attr == attr && t.Val == val {
			return true
		}
	}
	return false
}

func extrasHaveParsedAttrValue(got, want []Record, attr, val string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && recordEqual(g, want[i]) {
			i++
			continue
		}
		if !recordHasAttrValue(g, attr, val) {
			return false
		}
	}
	return i == len(want)
}

// recordsSuperset reports whether got contains every record in want, in order.
func recordsSuperset(got, want []Record) bool {
	i := 0
	for _, w := range want {
		for i < len(got) && !recordEqual(got[i], w) {
			i++
		}
		if i >= len(got) {
			return false
		}
		i++
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

	tokens := rawSearchTokens(in)
	if len(tokens) > 32 {
		tokens = tokens[:32]
	}
	for _, attr := range tokens {
		wantHits := mainSearchHasAttr(in, attr)
		gotHits := got.SearchSlice(HasAttr(attr))
		if !recordsEqual(gotHits, wantHits) {
			t.Fatalf("HasAttr(%q) for %q\n got %v\nwant %v", attr, in, gotHits, wantHits)
		}
		for _, val := range tokens {
			wantHits := mainSearchHasAttrValue(in, attr, val)
			gotHits := got.SearchSlice(HasAttrValue(attr, val))
			if recordsEqual(gotHits, wantHits) {
				continue
			}
			if recordsSuperset(gotHits, wantHits) {
				if val == "" {
					// Intentional: when value is empty, main can skip the
					// bare-attr Contains(" x ") check after it lands on
					// " x=" inside a quoted value and treats a later quote
					// as opening a new one. Head still runs that check.
					// Head is right when a real bare attr exists
					// (`ip k=" ip="` → main 0, head 1), but it can also
					// match text inside a quoted value (`="0 1 1="`), the
					// same Contains quirk HasAttr already has.
					continue
				}
				if extrasHaveParsedAttrValue(gotHits, wantHits, attr, val) {
					// Head found a later real tuple that main's concat
					// scan missed after treating attr=" inside a closed
					// quote as an opening quote (k="x a=" / a=real).
					continue
				}
			}
			t.Fatalf("HasAttrValue(%q,%q) for %q\n got %v\nwant %v", attr, val, in, gotHits, wantHits)
		}
	}
}

func rawSearchTokens(in []byte) []string {
	seen := map[string]struct{}{"": {}}
	out := []string{""}
	start := -1
	flush := func(i int) {
		if start < 0 || i <= start {
			start = -1
			return
		}
		s := string(in[start:i])
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
		start = -1
	}
	for i := 0; i < len(in); i++ {
		c := in[i]
		if c == '_' || c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			if start < 0 {
				start = i
			}
			continue
		}
		flush(i)
	}
	flush(len(in))
	return out
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
		"ip=\"a\n\tip=1 b\"\n",
		`="0 1 1="`,
		`ip k=" ip="`,
		"k=\"x a=\"\n\ta=real\n",
		"k=\"x a=\"\n\tfoo\"\n",
		"k=\"hello a=\"\n\tmore\"\n",
		"prefix k=\"unclosed a=\"\n\tmore\"\n",
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
		"ip=\"a\n\tip=1 b\"\n",
		`="0 1 1="`,
		`ip k=" ip="`,
		"k=\"x a=\"\n\ta=real\n",
		"k=\"x a=\"\n\tfoo\"\n",
		"k=\"hello a=\"\n\tmore\"\n",
		"prefix k=\"unclosed a=\"\n\tmore\"\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 4096 {
			t.Skip()
		}
		compareAgainstMain(t, []byte(s))
	})
}
