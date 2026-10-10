package ndb

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestParseOne(t *testing.T) {
	t.Run("simple record", func(t *testing.T) {
		db, err := ParseOne([]byte("person name=John age=30"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "John" {
			t.Fatalf("expected name=John, got %s", records[0].Get("name"))
		}
	})

	t.Run("multiple records", func(t *testing.T) {
		data := `person name=John age=30
person name=Jane age=25`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("multiline record", func(t *testing.T) {
		data := `person name=John
	age=30
	city=NYC`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("city") != "NYC" {
			t.Fatalf("expected city=NYC, got %s", records[0].Get("city"))
		}
	})

	t.Run("with comments", func(t *testing.T) {
		data := `# This is a comment
person name=John # inline comment
	age=30`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("age") != "30" {
			t.Fatalf("expected age=30, got %s", records[0].Get("age"))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		db, err := ParseOne([]byte(""))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.AllSlice()
		if len(records) != 0 {
			t.Fatalf("expected 0 records for empty input, got %d", len(records))
		}
	})

	t.Run("only comments", func(t *testing.T) {
		data := `# Comment 1
# Comment 2
# Comment 3`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.AllSlice()
		if len(records) != 0 {
			t.Fatalf("expected 0 records for comment-only input, got %d", len(records))
		}
	})

	t.Run("quoted values with spaces", func(t *testing.T) {
		data := `person name="John Doe" email="john@example.com"`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "John Doe" {
			t.Fatalf("expected name='John Doe', got %s", records[0].Get("name"))
		}
	})
}

func TestParseOneString(t *testing.T) {
	t.Run("simple string", func(t *testing.T) {
		db, err := ParseOneString("person name=John age=30")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "John" {
			t.Fatalf("expected name=John, got %s", records[0].Get("name"))
		}
	})

	t.Run("multiline string", func(t *testing.T) {
		data := `person name=John
	age=30
person name=Jane
	age=25`
		db, err := ParseOneString(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("empty string", func(t *testing.T) {
		db, err := ParseOneString("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.AllSlice()
		if len(records) != 0 {
			t.Fatalf("expected 0 records for empty string, got %d", len(records))
		}
	})
}

func TestParseEdgeCases(t *testing.T) {
	t.Run("unterminated quote", func(t *testing.T) {
		_, err := ParseOne([]byte(`person name="John`))
		if err == nil {
			t.Fatal("expected error for unterminated quote")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("expected line number in error, got %v", err)
		}
	})

	t.Run("unterminated quote drops tail", func(t *testing.T) {
		_, err := ParseOne([]byte("ok=1\nperson name=\"nope"))
		if err == nil {
			t.Fatal("expected error for unterminated quote after a valid record")
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("expected line 2 in error, got %v", err)
		}
	})

	t.Run("unterminated quote via ParseOneString", func(t *testing.T) {
		_, err := ParseOneString(`person name="John`)
		if err == nil {
			t.Fatal("expected error for unterminated quote")
		}
	})

	t.Run("invalid utf8 in value", func(t *testing.T) {
		data := append([]byte("person name="), 0xff)
		if utf8.Valid(data) {
			t.Fatal("precondition failed: test data is valid utf8")
		}
		_, err := ParseOne(data)
		if err == nil {
			t.Fatal("expected error for invalid utf8")
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Fatalf("expected line number in error, got %v", err)
		}
	})

	t.Run("invalid utf8 after valid record", func(t *testing.T) {
		data := append([]byte("ok=1\nperson name="), 0x80)
		if utf8.Valid(data) {
			t.Fatal("precondition failed: test data is valid utf8")
		}
		_, err := ParseOne(data)
		if err == nil {
			t.Fatal("expected error for invalid utf8")
		}
		if !strings.Contains(err.Error(), "line 2") {
			t.Fatalf("expected line 2 in error, got %v", err)
		}
	})

	t.Run("invalid utf8 via ParseOneString", func(t *testing.T) {
		_, err := ParseOneString("person name=" + string([]byte{0xff}))
		if err == nil {
			t.Fatal("expected error for invalid utf8")
		}
	})

	t.Run("unicode characters", func(t *testing.T) {
		data := `person name=José age=30 emoji=😀`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "José" {
			t.Fatalf("expected name=José, got %s", records[0].Get("name"))
		}
		if records[0].Get("emoji") != "😀" {
			t.Fatalf("expected emoji=😀, got %s", records[0].Get("emoji"))
		}
	})

	t.Run("very long line", func(t *testing.T) {
		longValue := strings.Repeat("a", 1000)
		data := `person name=` + longValue
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != longValue {
			t.Fatalf("expected long value to be preserved")
		}
	})

	t.Run("70KiB value is found", func(t *testing.T) {
		longValue := strings.Repeat("x", 70*1024)
		db, err := ParseOne([]byte("person name=" + longValue))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != longValue {
			t.Fatalf("expected 70KiB value to be preserved")
		}
	})

	t.Run("record after overlong line is found", func(t *testing.T) {
		longValue := strings.Repeat("y", 70*1024)
		data := "person name=" + longValue + "\nperson name=Jane"
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		jane := db.SearchSlice(HasAttrValue("name", "Jane"))
		if len(jane) != 1 {
			t.Fatalf("expected to find record after overlong line, got %d", len(jane))
		}
		all := db.SearchSlice(HasAttr("person"))
		if len(all) != 2 {
			t.Fatalf("expected 2 records, got %d", len(all))
		}
	})

	t.Run("duplicate attributes in same record", func(t *testing.T) {
		data := `person name=John name=Jane name=Bob age=30`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		names := records[0].GetAll("name")
		if len(names) != 3 {
			t.Fatalf("expected 3 names, got %d", len(names))
		}
	})

	t.Run("attribute with equals in quoted value", func(t *testing.T) {
		data := `person equation="a=b+c"`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("equation") != "a=b+c" {
			t.Fatalf("expected equation='a=b+c', got %s", records[0].Get("equation"))
		}
	})

	t.Run("tabs and mixed whitespace", func(t *testing.T) {
		data := "person name=John\tage=30"
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "John" {
			t.Fatalf("expected name=John, got %s", records[0].Get("name"))
		}
		if records[0].Get("age") != "30" {
			t.Fatalf("expected age=30, got %s", records[0].Get("age"))
		}
	})

	t.Run("empty attribute value", func(t *testing.T) {
		data := `person name= age=30`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "" {
			t.Fatalf("expected name to be empty, got %s", records[0].Get("name"))
		}
		if records[0].Get("age") != "30" {
			t.Fatalf("expected age=30, got %s", records[0].Get("age"))
		}
	})

	t.Run("quoted empty value", func(t *testing.T) {
		data := `person name="" age=30`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "" {
			t.Fatalf("expected name to be empty, got %s", records[0].Get("name"))
		}
	})

	t.Run("newline characters in quoted value", func(t *testing.T) {
		data := `person name="John\nDoe"`
		db, err := ParseOne([]byte(data))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		// Should preserve the escaped newline
		name := records[0].Get("name")
		if name != "John\nDoe" {
			t.Fatalf("expected name to contain newline, got %s", name)
		}
	})
}

func TestParseMultilineQuotedValue(t *testing.T) {
	db, err := ParseOneString("person name=\"John\n\tDoe\" age=30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	recs := db.AllSlice()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Get("name") != "John Doe" {
		t.Fatalf("expected joined quoted value 'John Doe', got %q", recs[0].Get("name"))
	}
	if recs[0].Get("age") != "30" {
		t.Fatalf("expected age=30, got %q", recs[0].Get("age"))
	}
	if got := db.SearchSlice(HasAttrValue("name", "John Doe")); len(got) != 1 {
		t.Fatalf("HasAttrValue missed multiline quoted name, got %d", len(got))
	}
}

func TestParseQuotedValueSpanningThreeLines(t *testing.T) {
	in := "k=\"a\n\tb\n\tc\"\n"
	db, err := ParseOne([]byte(in))
	if err != nil {
		t.Fatalf("ParseOne 3-line quote: %v", err)
	}
	recs := db.AllSlice()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d (%v)", len(recs), recs)
	}
	if recs[0].Get("k") != "a b c" {
		t.Fatalf("k=%q want %q", recs[0].Get("k"), "a b c")
	}
	if got := db.SearchSlice(HasAttrValue("k", "a b c")); len(got) != 1 {
		t.Fatalf("HasAttrValue missed 3-line quoted value, got %d", len(got))
	}
}

func TestParseQuotedValueEscapedQuoteThenContinuation(t *testing.T) {
	in := "k=\"abc\\\"\n\tmore\"\n"
	db, err := ParseOne([]byte(in))
	if err != nil {
		t.Fatalf("ParseOne escaped quote + continuation: %v", err)
	}
	recs := db.AllSlice()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	want := `abc" more`
	if recs[0].Get("k") != want {
		t.Fatalf("k=%q want %q", recs[0].Get("k"), want)
	}
	if got := db.SearchSlice(HasAttrValue("k", want)); len(got) != 1 {
		t.Fatalf("HasAttrValue missed escaped-quote continuation, got %d", len(got))
	}
}

func TestParseQuotedValueCRLFMultiline(t *testing.T) {
	in := "k=\"a\r\n\tb\r\n\tc\"\r\n"
	db, err := ParseOne([]byte(in))
	if err != nil {
		t.Fatalf("ParseOne CRLF multiline quote: %v", err)
	}
	recs := db.AllSlice()
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	if recs[0].Get("k") != "a b c" {
		t.Fatalf("k=%q want %q", recs[0].Get("k"), "a b c")
	}
	if got := db.SearchSlice(HasAttrValue("k", "a b c")); len(got) != 1 {
		t.Fatalf("HasAttrValue missed CRLF multiline quote, got %d", len(got))
	}
}

func TestQuoteClosed(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{`"abc"`, true},
		{`"abc`, false},
		{`"abc\"`, false},
		{`"abc\\"`, true},
		{`"abc\"more"`, true},
		{`"`, false},
		{``, false},
		{`abc"`, false},
	}
	for _, tc := range cases {
		if got := quoteClosed([]byte(tc.in)); got != tc.want {
			t.Fatalf("quoteClosed(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseKeepsInchMarkOutsideQuotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want Record
	}{
		{"inch mark then comment", `size=6" # inches`, MakeRecord("size", `6"`)},
		{"inch mark then quoted text in comment", `size=6" # see c="x y`, MakeRecord("size", `6"`)},
		{"quote inside key", `a"b=1 # c`, MakeRecord(`a"b`, "1")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := ParseOne([]byte(tc.in))
			if err != nil {
				t.Fatalf("ParseOne(%q) unexpected error: %v", tc.in, err)
			}
			got := db.AllSlice()
			if len(got) != 1 {
				t.Fatalf("expected 1 record from %q, got %d", tc.in, len(got))
			}
			if len(got[0]) != len(tc.want) {
				t.Fatalf("%q: got %d tuples %q, want %d %q", tc.in, len(got[0]), got[0].String(), len(tc.want), tc.want.String())
			}
			for i := range tc.want {
				if got[0][i] != tc.want[i] {
					t.Fatalf("%q: tuple %d got %+v, want %+v", tc.in, i, got[0][i], tc.want[i])
				}
			}
			if recs := db.SearchSlice(HasAttr(tc.want[0].Attr)); len(recs) != 1 {
				t.Fatalf("Search(%q) dropped the record from %q, got %d", tc.want[0].Attr, tc.in, len(recs))
			}
		})
	}
}

func TestStripComment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"escaped quote", `name="foo\"bar"`, `name="foo\"bar"`},
		{"escaped quote then hash inside quotes", `name="foo\"#bar"`, `name="foo\"#bar"`},
		{"value ending in backslash then comment", `key="a\\" # comment`, `key="a\\" `},
		{"hash immediately after quoted value", `key="v"#c`, `key="v"`},
		{"unterminated quote keeps hash", `name="foo#bar`, `name="foo#bar`},
		{"hash in unquoted value", `name=foo#bar`, `name=foo`},
		{"inch mark is not a quote", `size=6" # inches`, `size=6" `},
		{"inch mark then later equals-quote in comment", `size=6" # see c="x y`, `size=6" `},
		{"quote inside key", `a"b=1 # c`, `a"b=1 `},
		{"equals-quote inside unquoted value", `a=b="c#d"`, `a=b="c`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(stripComment([]byte(tc.in)))
			if got != tc.want {
				t.Fatalf("stripComment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestQuotedValueLen(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"escaped quote", `"foo\"bar"`, len(`"foo\"bar"`)},
		{"escaped quote then hash", `"foo\"#bar"`, len(`"foo\"#bar"`)},
		{"value ending in backslash", `"a\\"`, len(`"a\\"`)},
		{"hash immediately after closer", `"v"#c`, len(`"v"`)},
		{"unterminated", `"foo#bar`, len(`"foo#bar`)},
		{"hash inside quotes", `"foo#bar"`, len(`"foo#bar"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := quotedValueLen([]byte(tc.in))
			if got != tc.want {
				t.Fatalf("quotedValueLen(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestSearchWithSimilarAttributeNames(t *testing.T) {
	data := `provider=openai model=gpt-4
provider=anthropic model=claude`
	db, err := ParseOneString(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("search for provider", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("provider"))
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
	})

	t.Run("search for specific provider value", func(t *testing.T) {
		records := db.SearchSlice(HasAttrValue("provider", "openai"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("model") != "gpt-4" {
			t.Fatalf("expected model=gpt-4, got %s", records[0].Get("model"))
		}
	})
}

func quotedContinuationInput(n int, closed bool) []byte {
	var b strings.Builder
	b.Grow(4 + n*3)
	b.WriteString(`a="`)
	for i := 0; i < n; i++ {
		b.WriteString("\n\tx")
	}
	if closed {
		b.WriteByte('"')
	}
	return []byte(b.String())
}

func TestContinuationJoinScaling(t *testing.T) {
	const n = 100000
	// Non-race is ~20ms here; -race is a few times slower. Quadratic join
	// was 1.3s at 40k lines, so 500ms still fails the old path.
	const bound = 500 * time.Millisecond

	closed := quotedContinuationInput(n, true)
	start := time.Now()
	db, err := ParseOne(closed)
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	got := 0
	for range db.All() {
		got++
	}
	parseAll := time.Since(start)
	if got != 1 {
		t.Fatalf("All()=%d want 1", got)
	}
	if parseAll > bound {
		t.Fatalf("ParseOne+All %d continuations took %s, want <%s", n, parseAll, bound)
	}

	fs := &MemoryFileSystem{Files: map[string]string{"t.ndb": string(closed)}}
	start = time.Now()
	db = mustOpenOne(t, fs, "t.ndb")
	got = 0
	for range db.All() {
		got++
	}
	openOne := time.Since(start)
	if got != 1 {
		t.Fatalf("OpenOne All()=%d want 1", got)
	}
	if openOne > bound {
		t.Fatalf("OpenOne+All %d continuations took %s, want <%s", n, openOne, bound)
	}

	root := "database file=\"" + strings.Repeat("\n\tx", n)
	fs = &MemoryFileSystem{Files: map[string]string{"root.ndb": root}}
	start = time.Now()
	if _, err := Open(fs, "root.ndb"); err != nil {
		t.Fatalf("Open: %v", err)
	}
	openDB := time.Since(start)
	if openDB > bound {
		t.Fatalf("Open database-file quote + %d continuations took %s, want <%s", n, openDB, bound)
	}

	start = time.Now()
	_ = mainAllSlice(closed)
	mainClosed := time.Since(start)
	unterm := quotedContinuationInput(n, false)
	start = time.Now()
	_ = mainValidate(unterm)
	mainUnterm := time.Since(start)
	t.Logf("head ParseOne+All=%s OpenOne+All=%s Open=%s; main AllSlice(closed)=%s Validate(unterm)=%s",
		parseAll, openOne, openDB, mainClosed, mainUnterm)

	timeHeadMain := func(name string, in []byte) {
		t.Helper()
		hs := time.Now()
		_, herr := ParseOne(in)
		hd := time.Since(hs)
		ms := time.Now()
		_ = mainValidate(in)
		md := time.Since(ms)
		t.Logf("%s: head ParseOne=%s (err=%v) main Validate=%s", name, hd, herr, md)
	}
	timeHeadMain("40k closed", quotedContinuationInput(40000, true))
	timeHeadMain("40k unterm", quotedContinuationInput(40000, false))
	bad := append([]byte(`a="\x`), bytesRepeatCont(40000)...)
	timeHeadMain("40k bad-escape", bad)
}

func bytesRepeatCont(n int) []byte {
	return []byte(strings.Repeat("\n\tx", n))
}

func TestContinuationJoinIsLinear(t *testing.T) {
	timeN := func(n int) time.Duration {
		in := quotedContinuationInput(n, true)
		best := time.Duration(1 << 62)
		for i := 0; i < 3; i++ {
			start := time.Now()
			db, err := ParseOne(in)
			if err != nil {
				t.Fatalf("ParseOne n=%d: %v", n, err)
			}
			for range db.All() {
			}
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}

	const n = 25000
	t1 := timeN(n)
	t2 := timeN(2 * n)
	t.Logf("n=%d %s; 2n=%d %s (ratio %.2f)", n, t1, 2*n, t2, float64(t2)/float64(t1))
	if t1 >= 2*time.Millisecond && t2 >= 4*t1 {
		t.Fatalf("doubling %d→%d: %s → %s (≈4x or worse; want closer to 2x)", n, 2*n, t1, t2)
	}
}
