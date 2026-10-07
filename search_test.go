package ndb

import (
	"testing"
)

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
