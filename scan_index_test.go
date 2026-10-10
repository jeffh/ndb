package ndb

import (
	"bytes"
	"testing"
)

var indexSets = []struct {
	name string
	set  []byte
}{
	{"4", []byte{' ', '\t', '\r', '\n'}},
	{"5", []byte{'=', ' ', '\t', '\r', '\n'}},
	{"6", []byte{'=', '#', ' ', '\t', '\r', '\n'}},
}

func indexRef(p []byte, set []byte) int {
	for i, x := range p {
		if bytes.IndexByte(set, x) >= 0 {
			return i
		}
	}
	return -1
}

func indexN(p []byte, set []byte) int {
	switch len(set) {
	case 4:
		return indexByte4(p, set[0], set[1], set[2], set[3])
	case 5:
		return indexByte5(p, set[0], set[1], set[2], set[3], set[4])
	case 6:
		return indexByte6(p, set[0], set[1], set[2], set[3], set[4], set[5])
	default:
		panic("bad set")
	}
}

func indexNScalar(p []byte, set []byte) int {
	switch len(set) {
	case 4:
		return indexByte4Scalar(p, set[0], set[1], set[2], set[3])
	case 5:
		return indexByte5Scalar(p, set[0], set[1], set[2], set[3], set[4])
	case 6:
		return indexByte6Scalar(p, set[0], set[1], set[2], set[3], set[4], set[5])
	default:
		panic("bad set")
	}
}

func TestIndexByteMatchesRef(t *testing.T) {
	haystacks := [][]byte{
		nil,
		{},
		[]byte("a"),
		[]byte("="),
		[]byte("sys=host"),
		[]byte("ip=10.0.0.1 sys=host00000000 dom=host00000000.example.net"),
		[]byte("\towner=alice loc=rack-3 extra=0 extra=1 # trailing comment"),
		[]byte("note=\"a b # c\""),
		bytes.Repeat([]byte("abcdefgh"), 8),
		append(bytes.Repeat([]byte("abcdefgh"), 16), '=', 'x'),
		append(bytes.Repeat([]byte{'x'}, 63), '#'),
		append(bytes.Repeat([]byte{'x'}, 64), ' '),
		append(bytes.Repeat([]byte{'x'}, 127), '\t'),
		bytes.Repeat([]byte{'='}, 3),
	}
	for _, set := range indexSets {
		for i, h := range haystacks {
			want := indexRef(h, set.set)
			if got := indexN(h, set.set); got != want {
				t.Errorf("indexByte%s(%q) = %d, want %d", set.name, h, got, want)
			}
			if got := indexNScalar(h, set.set); got != want {
				t.Errorf("indexByte%sScalar(%q) = %d, want %d", set.name, h, got, want)
			}
			_ = i
		}
	}
}

func TestIndexByteEveryOffset(t *testing.T) {
	for _, n := range []int{0, 1, 7, 15, 16, 31, 32, 33, 63, 64, 65, 80, 127, 128, 200} {
		for _, set := range indexSets {
			for off := 0; off <= n; off++ {
				h := bytes.Repeat([]byte{'x'}, n)
				want := -1
				if off < n {
					h[off] = set.set[off%len(set.set)]
					want = off
				}
				if got := indexN(h, set.set); got != want {
					t.Fatalf("n=%d off=%d set=%s: got %d want %d", n, off, set.name, got, want)
				}
			}
		}
	}
}

func TestIndexByteFirstMatchWins(t *testing.T) {
	h := []byte("a=b c#d")
	if got, want := indexByte5(h, '=', ' ', '\t', '\r', '\n'), 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got, want := indexByte6(h, '=', '#', ' ', '\t', '\r', '\n'), 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	if got, want := indexByte4(h, ' ', '\t', '\r', '\n'), 3; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
}

func TestSIMDDefaultBuildUsesFallback(t *testing.T) {
	if simdIndexEnabled() {
		t.Log("SIMD index path enabled (GOEXPERIMENT=simd and supported CPU)")
		return
	}
	// Default go test must keep using the scalar loops.
	p := []byte("ip=10.0.0.1 sys=host")
	if indexByte5(p, '=', ' ', '\t', '\r', '\n') != indexByte5Scalar(p, '=', ' ', '\t', '\r', '\n') {
		t.Fatal("fallback path diverged from scalar")
	}
}

func FuzzIndexByteParity(f *testing.F) {
	f.Add([]byte("ip=10.0.0.1 sys=host"), uint8(5))
	f.Add([]byte(""), uint8(4))
	f.Add([]byte("="), uint8(5))
	f.Add([]byte("   #"), uint8(6))
	f.Add(bytes.Repeat([]byte("abc=def "), 20), uint8(5))
	f.Fuzz(func(t *testing.T, p []byte, which uint8) {
		set := indexSets[int(which)%len(indexSets)]
		want := indexRef(p, set.set)
		if got := indexN(p, set.set); got != want {
			t.Fatalf("indexByte%s(%q) = %d, want %d", set.name, p, got, want)
		}
		if got := indexNScalar(p, set.set); got != want {
			t.Fatalf("scalar %s(%q) = %d, want %d", set.name, p, got, want)
		}
	})
}

func BenchmarkIndexByteRealistic(b *testing.B) {
	line := []byte("ip=10.255.255.255 sys=host00000000 dom=host00000000.example.net")
	value := []byte("host00000000.example.net")
	b.Run("tuple5-early", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if indexByte5(line, '=', ' ', '\t', '\r', '\n') < 0 {
				b.Fatal("miss")
			}
		}
	})
	b.Run("tuple5-early-scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if indexByte5Scalar(line, '=', ' ', '\t', '\r', '\n') < 0 {
				b.Fatal("miss")
			}
		}
	})
	b.Run("value4-end", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = indexByte4(value, ' ', '\t', '\r', '\n')
		}
	})
	b.Run("value4-end-scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = indexByte4Scalar(value, ' ', '\t', '\r', '\n')
		}
	})
	long := bytes.Repeat([]byte("abcdefghij"), 20) // 200 bytes, miss
	b.Run("miss5-200", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if indexByte5(long, '=', ' ', '\t', '\r', '\n') != -1 {
				b.Fatal("hit")
			}
		}
	})
	b.Run("miss5-200-scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if indexByte5Scalar(long, '=', ' ', '\t', '\r', '\n') != -1 {
				b.Fatal("hit")
			}
		}
	})
}

