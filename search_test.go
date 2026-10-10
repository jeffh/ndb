package ndb

import (
	"fmt"
	"strings"
	"testing"
)

func TestSearchKeepsHashInsideQuotes(t *testing.T) {
	db, err := ParseOne([]byte(`person name="foo#bar"`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Get("name") != "foo#bar" {
		t.Fatalf("expected name=foo#bar, got %s", records[0].Get("name"))
	}
}

func TestSearchWithHasAttr(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John age=30
company name=Acme founded=1990
person name=Jane age=25 city=NYC`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("find all with person attribute", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 2 {
			t.Fatalf("expected 2 person records, got %d", len(records))
		}
	})

	t.Run("find all with name attribute", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("name"))
		if len(records) != 3 {
			t.Fatalf("expected 3 records with name, got %d", len(records))
		}
	})

	t.Run("find records with city attribute", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("city"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record with city, got %d", len(records))
		}
		if records[0].Get("city") != "NYC" {
			t.Fatalf("expected city=NYC, got %s", records[0].Get("city"))
		}
	})

	t.Run("find records with non-existent attribute", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("email"))
		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})
}

func TestSearchWithMultiplePredicates(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John age=30 city=NYC
person name=Jane age=25 city=LA
person name=Bob age=30 city=SF
company name=Acme city=NYC`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("person AND age=30", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("person"), HasAttrValue("age", "30"))
		if len(records) != 2 {
			t.Fatalf("expected 2 records, got %d", len(records))
		}
		for _, r := range records {
			if !r.HasKey("person") {
				t.Fatalf("expected all records to have person attribute")
			}
			if r.Get("age") != "30" {
				t.Fatalf("expected all records to have age=30")
			}
		}
	})

	t.Run("person AND city=NYC", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("person"), HasAttrValue("city", "NYC"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if records[0].Get("name") != "John" {
			t.Fatalf("expected name=John, got %s", records[0].Get("name"))
		}
	})

	t.Run("multiple predicates with no matches", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("person"), HasAttrValue("city", "Tokyo"))
		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})
}

func TestSearchEdgeCases(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `personage=50
person age=30
superperson age=25`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("search for person should not match personage", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("person"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record with person, got %d", len(records))
		}
		if records[0].Get("age") != "30" {
			t.Fatalf("expected the exact match for 'person', got %s", records[0].String())
		}
	})

	t.Run("search for age value should not match partial values", func(t *testing.T) {
		records := db.SearchSlice(HasAttrValue("age", "5"))
		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})
}

func TestAll(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John
person name=Jane
company name=Acme`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	count := 0
	for range db.All() {
		count++
	}
	if count != 3 {
		t.Fatalf("expected 3 records from All(), got %d", count)
	}

	allRecords := db.AllSlice()
	if len(allRecords) != 3 {
		t.Fatalf("expected 3 records from AllSlice(), got %d", len(allRecords))
	}
}

func TestFirst(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John age=30
person name=Jane age=25
person name=Bob age=30`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("find first person", func(t *testing.T) {
		record := db.First("person", "")
		if record == nil {
			t.Fatalf("expected to find a record")
		}
		if record.Get("name") != "John" {
			t.Fatalf("expected first record to be John, got %s", record.Get("name"))
		}
	})

	t.Run("find first with age=30", func(t *testing.T) {
		record := db.First("age", "30")
		if record == nil {
			t.Fatalf("expected to find a record")
		}
		if record.Get("name") != "John" {
			t.Fatalf("expected first record with age=30 to be John, got %s", record.Get("name"))
		}
	})

	t.Run("find first non-existent", func(t *testing.T) {
		record := db.First("email", "")
		if record != nil {
			t.Fatalf("expected no record, got %v", record)
		}
	})
}

func TestIteratorEarlyTermination(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John
person name=Jane
person name=Bob
person name=Alice
person name=Charlie`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("stop iteration after 2 records", func(t *testing.T) {
		count := 0
		for range db.Search(HasAttr("person")) {
			count++
			if count >= 2 {
				break
			}
		}
		if count != 2 {
			t.Fatalf("expected to stop at 2 records, got %d", count)
		}
	})

	t.Run("early termination should work with All", func(t *testing.T) {
		count := 0
		for range db.All() {
			count++
			if count >= 3 {
				break
			}
		}
		if count != 3 {
			t.Fatalf("expected to stop at 3 records, got %d", count)
		}
	})
}

func TestSearchWithAttributeSubstring(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `name="John Doe" email="john@example.com"
username="johndoe" fullname="John Doe"`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	t.Run("search for 'name' should not match 'username' or 'fullname'", func(t *testing.T) {
		records := db.SearchSlice(HasAttr("name"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
		if !records[0].HasKey("email") {
			t.Fatalf("expected the record to have email attribute")
		}
	})

	t.Run("search for value 'John' should not match 'John Doe'", func(t *testing.T) {
		records := db.SearchSlice(HasAttrValue("name", "John"))
		if len(records) != 0 {
			t.Fatalf("expected 0 records, got %d", len(records))
		}
	})

	t.Run("search for exact value 'John Doe'", func(t *testing.T) {
		records := db.SearchSlice(HasAttrValue("name", "John Doe"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}
	})
}

func TestSearchDuplicateAttrLaterValue(t *testing.T) {
	t.Run("unquoted later value", func(t *testing.T) {
		db, err := ParseOneString("sys=a ip=1.2.3.4 ip=5.6.7.8")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttrValue("ip", "5.6.7.8"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record for ip=5.6.7.8, got %d", len(records))
		}
		if records[0].Get("sys") != "a" {
			t.Fatalf("expected sys=a, got %s", records[0].Get("sys"))
		}
	})

	t.Run("quoted later value", func(t *testing.T) {
		db, err := ParseOneString(`sys=a ip="1.2.3.4" ip="5.6.7.8"`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		records := db.SearchSlice(HasAttrValue("ip", "5.6.7.8"))
		if len(records) != 1 {
			t.Fatalf("expected 1 record for quoted ip=5.6.7.8, got %d", len(records))
		}
		if records[0].Get("sys") != "a" {
			t.Fatalf("expected sys=a, got %s", records[0].Get("sys"))
		}
	})
}

func TestHeldRecordStableAcrossIteration(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "ip=10.0.0.%d sys=host%d\n", i, i)
	}
	db := mustOpenOne(t, &MemoryFileSystem{Files: map[string]string{"t.ndb": b.String()}}, "t.ndb")

	var held Record
	var heldIP, heldSys string
	n := 0
	for rec := range db.All() {
		if n == 0 {
			held = rec.Copy()
			heldIP = rec.Get("ip")
			heldSys = rec.Get("sys")
		}
		n++
	}
	if n != 200 {
		t.Fatalf("All()=%d want 200", n)
	}
	if held.Get("ip") != "10.0.0.0" || heldIP != "10.0.0.0" {
		t.Fatalf("held ip corrupted: rec=%q str=%q", held.Get("ip"), heldIP)
	}
	if held.Get("sys") != "host0" || heldSys != "host0" {
		t.Fatalf("held sys corrupted: rec=%q str=%q", held.Get("sys"), heldSys)
	}

	later := db.SearchSlice(HasAttrValue("sys", "host199"))
	if len(later) != 1 || later[0].Get("ip") != "10.0.0.199" {
		t.Fatalf("later search broken: %+v", later)
	}
	if held.Get("ip") != "10.0.0.0" || heldIP != "10.0.0.0" {
		t.Fatalf("held values changed after later search: %q %q", held.Get("ip"), heldIP)
	}
}

func TestParseOneHeldRecordIndependentOfInputMutation(t *testing.T) {
	data := []byte("name=John age=30")
	db, err := ParseOne(data)
	if err != nil {
		t.Fatal(err)
	}
	held := db.AllSlice()
	if len(held) != 1 || held[0].Get("name") != "John" {
		t.Fatalf("setup: %+v", held)
	}
	name := held[0].Get("name")

	copy(data, []byte("name=XXXX age=30"))
	if name != "John" || held[0].Get("name") != "John" {
		t.Fatalf("held ParseOne record aliased caller buffer: name=%q rec=%q", name, held[0].Get("name"))
	}

	again := db.AllSlice()
	if len(again) != 1 || again[0].Get("name") != "XXXX" {
		t.Fatalf("re-parse should see mutated ParseOne buffer, got %+v", again)
	}
	if held[0].Get("name") != "John" {
		t.Fatalf("already-held record should stay John, got %q", held[0].Get("name"))
	}
}

func TestHasAttrValueDoesNotRescanContinuationInsideMultilineQuote(t *testing.T) {
	in := "ip=\"a\n\tip=1 b\"\n"
	db, err := ParseOne([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	if got := db.SearchSlice(HasAttrValue("ip", "1")); len(got) != 0 {
		t.Fatalf("HasAttrValue(ip, 1) should not match inside a multiline quoted value, got %d (%q)", len(got), got[0].String())
	}
	if got := db.SearchSlice(HasAttrValue("ip", "a ip=1 b")); len(got) != 1 {
		t.Fatalf("HasAttrValue(ip, exact multiline value) got %d", len(got))
	}
}

func TestHasAttrValueDoesNotScanInsideParsedValue(t *testing.T) {
	db, err := ParseOneString(`a="x a=1 "`)
	if err != nil {
		t.Fatal(err)
	}
	if got := db.SearchSlice(HasAttrValue("a", "1")); len(got) != 0 {
		t.Fatalf("HasAttrValue(a, 1) should not match inside quoted value, got %d", len(got))
	}
	if got := db.SearchSlice(HasAttrValue("a", "x a=1 ")); len(got) != 1 {
		t.Fatalf("HasAttrValue(a, exact quoted value) got %d", len(got))
	}
}

func TestHasAttrMatchesQuotedValueContains(t *testing.T) {
	db, err := ParseOneString(`note=" sys=foo " extra=1`)
	if err != nil {
		t.Fatal(err)
	}
	// Concat-style search is a substring scan, so a quoted value can still match.
	if got := db.SearchSlice(HasAttr("sys")); len(got) != 1 {
		t.Fatalf("HasAttr(sys) in quoted value: got %d", len(got))
	}
	if got := db.SearchSlice(HasAttrValue("sys", "foo")); len(got) != 1 {
		t.Fatalf("HasAttrValue(sys, foo) in quoted value: got %d", len(got))
	}
}

func TestSearchHasAttrMatchesHasKey(t *testing.T) {
	data := `person name=John
	age=30
company name=Acme
dhcp
bare
sys=a ip=1.2.3.4 ip=5.6.7.8
note="a b # c" other=z`
	db, err := ParseOneString(data)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]struct{}{}
	for rec := range db.All() {
		for i := 0; i < rec.Len(); i++ {
			keys[rec.KeyAt(i)] = struct{}{}
		}
	}
	for key := range keys {
		want := 0
		for rec := range db.All() {
			if rec.HasKey(key) {
				want++
			}
		}
		got := len(db.SearchSlice(HasAttr(key)))
		if got != want {
			t.Fatalf("HasAttr(%q)=%d want %d (HasKey filter)", key, got, want)
		}
	}
}

func TestSearchSlice(t *testing.T) {
	m := &MemoryFileSystem{
		Files: map[string]string{
			"test.ndb": `person name=John
person name=Jane`,
		},
	}

	db := mustOpenOne(t, m, "test.ndb")

	records := db.SearchSlice(HasAttr("person"))
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify that modifying one record doesn't affect the other
	records[0].Put(Tuple{"age", "30"})
	if records[1].HasKey("age") {
		t.Fatalf("modifying one record should not affect another")
	}
}
