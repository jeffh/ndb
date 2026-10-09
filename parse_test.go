package ndb

import (
	"testing"
)

func TestParseOneReportsUnterminatedQuote(t *testing.T) {
	_, err := ParseOne([]byte(`person name="John`))
	if err == nil {
		t.Fatal("expected error for unterminated quote")
	}

	_, err = ParseOne([]byte("ok=1\nperson name=\"nope"))
	if err == nil {
		t.Fatal("expected error when a later record has an unterminated quote")
	}

	_, err = ParseOne([]byte("\xff\xfe=bad"))
	if err == nil {
		t.Fatal("expected error for invalid UTF-8 attribute")
	}

	_, err = ParseOneString(`person name="John`)
	if err == nil {
		t.Fatal("expected ParseOneString error for unterminated quote")
	}
}

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
		longValue := ""
		for i := 0; i < 1000; i++ {
			longValue += "a"
		}
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
