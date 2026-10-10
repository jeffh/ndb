//go:build goexperiment.simd

package ndb

import (
	"bytes"
	"fmt"
	"testing"
)

func TestSIMDIndexEnabledOnThisRunner(t *testing.T) {
	t.Logf("simdIndexEnabled=%v", simdIndexEnabled())
	if !simdIndexEnabled() {
		t.Skip("no amd64 AVX2/AVX512; SIMD index path not active")
	}
}

func TestSIMDIndexParityWithScalar(t *testing.T) {
	if !simdIndexEnabled() {
		t.Skip("SIMD index path not active")
	}
	sizes := []int{0, 1, 16, 31, 32, 33, 48, 63, 64, 65, 80, 96, 127, 128, 200, 256, 1024}
	for _, set := range indexSets {
		for _, n := range sizes {
			for off := -1; off <= n; off++ {
				h := bytes.Repeat([]byte{'z'}, n)
				if off >= 0 && off < n {
					h[off] = set.set[off%len(set.set)]
				}
				got := indexN(h, set.set)
				want := indexNScalar(h, set.set)
				if got != want {
					t.Fatalf("set=%s n=%d off=%d: simd=%d scalar=%d", set.name, n, off, got, want)
				}
			}
		}
	}
}

func TestSIMDIndexDirectKernels(t *testing.T) {
	if !simdIndexEnabled() {
		t.Skip("SIMD index path not active")
	}
	cases := [][]byte{
		bytes.Repeat([]byte{'x'}, 32),
		append(bytes.Repeat([]byte{'x'}, 31), '='),
		append(bytes.Repeat([]byte{'x'}, 32), ' ', 'y'),
		append(bytes.Repeat([]byte{'x'}, 63), '#'),
		append(bytes.Repeat([]byte{'x'}, 80), '\t'),
		[]byte("ip=10.0.0.1 sys=host00000000 dom=host00000000.example.net extra=0"),
	}
	for i, h := range cases {
		if got, want := indexByte5SIMD(h, '=', ' ', '\t', '\r', '\n'), indexByte5Scalar(h, '=', ' ', '\t', '\r', '\n'); got != want {
			t.Errorf("5[%d]: simd=%d scalar=%d", i, got, want)
		}
		if got, want := indexByte4SIMD(h, ' ', '\t', '\r', '\n'), indexByte4Scalar(h, ' ', '\t', '\r', '\n'); got != want {
			t.Errorf("4[%d]: simd=%d scalar=%d", i, got, want)
		}
		if got, want := indexByte6SIMD(h, '=', '#', ' ', '\t', '\r', '\n'), indexByte6Scalar(h, '=', '#', ' ', '\t', '\r', '\n'); got != want {
			t.Errorf("6[%d]: simd=%d scalar=%d", i, got, want)
		}
	}
}

func FuzzSIMDIndexParity(f *testing.F) {
	f.Add([]byte("ip=1 sys=h"), uint8(5))
	f.Add(bytes.Repeat([]byte("x"), 70), uint8(4))
	f.Add(append(bytes.Repeat([]byte("x"), 64), '#'), uint8(6))
	f.Fuzz(func(t *testing.T, p []byte, which uint8) {
		set := indexSets[int(which)%len(indexSets)]
		if got, want := indexN(p, set.set), indexNScalar(p, set.set); got != want {
			t.Fatalf("impl=%d scalar=%d set=%s p=%q", got, want, set.name, p)
		}
		if simdIndexEnabled() && len(p) >= simdMin {
			var simd int
			switch len(set.set) {
			case 4:
				simd = indexByte4SIMD(p, set.set[0], set.set[1], set.set[2], set.set[3])
			case 5:
				simd = indexByte5SIMD(p, set.set[0], set.set[1], set.set[2], set.set[3], set.set[4])
			default:
				simd = indexByte6SIMD(p, set.set[0], set.set[1], set.set[2], set.set[3], set.set[4], set.set[5])
			}
			if simd != indexNScalar(p, set.set) {
				t.Fatalf("direct simd=%d scalar=%d", simd, indexNScalar(p, set.set))
			}
		}
	})
}

func TestSIMDQuotingSemanticsUnchanged(t *testing.T) {
	// #14: quote opens only after '=', '#' inside quotes is not a comment.
	db, err := ParseOneString("size=6\" # inches\nname=\"foo#bar\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := db.SearchSlice(HasAttrValue("name", "foo#bar")); len(got) != 1 {
		t.Fatalf("quoted hash: %v", got)
	}
	if got := db.SearchSlice(HasAttrValue("size", `6"`)); len(got) != 1 {
		t.Fatalf("inch mark: %v", got)
	}
}

func TestSIMDLinearJoinUnchanged(t *testing.T) {
	// #17: a closed quote containing attr=" must not swallow a later real tuple.
	in := "k=\"x a=\"\n\ta=real\n"
	db, err := ParseOneString(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := db.SearchSlice(HasAttrValue("a", "real")); len(got) != 1 {
		t.Fatalf("false-join: %v", got)
	}
}

func TestSIMDInfo(t *testing.T) {
	fmt.Printf("simdIndexEnabled=%v\n", simdIndexEnabled())
}
