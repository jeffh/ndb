package ndb

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// Search skips ahead to records whose raw text contains the predicate's
// attribute or value. These cases put that text in places the skip logic
// has to get right: escapes, comments, continuation lines, CRLF, and the
// edges of 32-byte scan blocks.

func searchStrings(db *Ndb, preds ...SearchPredicate) []string {
	var out []string
	for rec := range db.Search(preds...) {
		out = append(out, rec.String())
	}
	return out
}

func TestSearchSkipFindsEscapedValue(t *testing.T) {
	// "\x62ench" unquotes to "bench" although the raw text never says bench.
	in := strings.Repeat("pad=1 filler=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n", 20) +
		`sys="\x62ench" ip=1` + "\n" +
		strings.Repeat("pad=2 filler=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\n", 20) +
		`sys="be\nch" ip=2` + "\n"
	db, err := ParseOneString(in)
	must(t, err)
	got := db.SearchSlice(HasAttrValue("sys", "bench"))
	if len(got) != 1 || got[0].Get("ip") != "1" {
		t.Fatalf("HasAttrValue(sys, bench) = %v", got)
	}
	got = db.SearchSlice(HasAttrValue("sys", "be\nch"))
	if len(got) != 1 || got[0].Get("ip") != "2" {
		t.Fatalf("HasAttrValue(sys, be\\nch) = %v", got)
	}
}

func TestSearchSkipHitOnContinuationLine(t *testing.T) {
	in := "a=1\n" +
		"# sys=comment-only, not part of any tuple\n" +
		"\n" +
		"\tb=2\n" +
		"# comment between lines of one record\n" +
		"\tsys=target\n" +
		"c=3 # sys=in a trailing comment\n" +
		"d=4\n" +
		"\tsys=other\n"
	db, err := ParseOneString(in)
	must(t, err)
	if got, want := searchStrings(db, HasAttr("sys")), []string{"a=1 b=2 sys=target", "d=4 sys=other"}; !slices.Equal(got, want) {
		t.Fatalf("HasAttr(sys) = %q, want %q", got, want)
	}
	if got, want := searchStrings(db, HasAttrValue("sys", "target")), []string{"a=1 b=2 sys=target"}; !slices.Equal(got, want) {
		t.Fatalf("HasAttrValue(sys, target) = %q, want %q", got, want)
	}
	if got := searchStrings(db, HasAttrValue("sys", "comment-only,")); len(got) != 0 {
		t.Fatalf("matched comment text: %q", got)
	}
}

func TestSearchSkipLeadingContinuationAndCRLF(t *testing.T) {
	in := "\tlead=1\r\n\tsys=first\r\n" +
		"x=1\r\n" +
		"y=2\r\n\tsys=second\r\n"
	db, err := ParseOneString(in)
	must(t, err)
	if got, want := searchStrings(db, HasAttr("sys")), []string{"lead=1 sys=first", "y=2 sys=second"}; !slices.Equal(got, want) {
		t.Fatalf("HasAttr(sys) = %q, want %q", got, want)
	}
	if got, want := searchStrings(db, HasAttrValue("sys", "second")), []string{"y=2 sys=second"}; !slices.Equal(got, want) {
		t.Fatalf("HasAttrValue(sys, second) = %q, want %q", got, want)
	}
}

func TestSearchSkipStopsEarly(t *testing.T) {
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "id=%d\n\tfiller=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx\n", i)
		if i%50 == 7 {
			fmt.Fprintf(&b, "id=%d hit=yes\n", 1000+i)
		}
	}
	db, err := ParseOneString(b.String())
	must(t, err)
	n := 0
	for rec := range db.Search(HasAttrValue("hit", "yes")) {
		n++
		if rec.Get("id") != "1007" {
			t.Fatalf("first hit id=%s", rec.Get("id"))
		}
		break
	}
	if n != 1 {
		t.Fatalf("got %d records before break", n)
	}
}

// TestSearchSkipMatchesFullScan compares Search against filtering All() on
// generated files with plain tokens, where HasAttr/HasAttrValue mean exactly
// "the record has this key / key=value". Record and token lengths vary so
// hits land at every offset of a 32-byte block, including the file's end.
func TestSearchSkipMatchesFullScan(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	attrs := []string{"a", "sys", "ip", "dom", "host", "x-long-attribute-name"}
	vals := []string{"1", "bench", "10.0.0.1", "bench-target", "v", "zz"}
	for iter := range 300 {
		var b strings.Builder
		nl := "\n"
		if iter%5 == 0 {
			nl = "\r\n"
		}
		nrec := 1 + rng.Intn(40)
		for r := range nrec {
			if rng.Intn(6) == 0 {
				b.WriteString("# " + attrs[rng.Intn(len(attrs))] + "=" + vals[rng.Intn(len(vals))] + nl)
			}
			lines := 1 + rng.Intn(3)
			for l := range lines {
				if l > 0 {
					b.WriteString("\t")
				} else {
					fmt.Fprintf(&b, "rec=%d ", r)
				}
				for range 1 + rng.Intn(3) {
					attr := attrs[rng.Intn(len(attrs))]
					switch rng.Intn(4) {
					case 0:
						b.WriteString(attr + " ")
					default:
						b.WriteString(attr + "=" + vals[rng.Intn(len(vals))] + " ")
					}
				}
				b.WriteString(strings.Repeat("p", rng.Intn(40)))
				if rng.Intn(5) == 0 {
					b.WriteString(" # " + attrs[rng.Intn(len(attrs))])
				}
				if r < nrec-1 || l < lines-1 || rng.Intn(2) == 0 {
					b.WriteString(nl)
				}
			}
		}
		in := b.String()
		db, err := ParseOneString(in)
		must(t, err)
		all := db.AllSlice()
		for _, attr := range attrs {
			var want []string
			for _, rec := range all {
				if rec.HasKey(attr) {
					want = append(want, rec.String())
				}
			}
			if got := searchStrings(db, HasAttr(attr)); !slices.Equal(got, want) {
				t.Fatalf("HasAttr(%q) on %q\n got %q\nwant %q", attr, in, got, want)
			}
			for _, val := range vals {
				var want []string
				for _, rec := range all {
					if slices.Contains(rec.GetAll(attr), val) {
						want = append(want, rec.String())
					}
				}
				if got := searchStrings(db, HasAttrValue(attr, val)); !slices.Equal(got, want) {
					t.Fatalf("HasAttrValue(%q, %q) on %q\n got %q\nwant %q", attr, val, in, got, want)
				}
			}
		}
	}
}
