//go:build !goexperiment.simd

package ndb

func simdIndexEnabled() bool { return false }

// These loops stay in this file so the default (no GOEXPERIMENT) build can
// inline them into parseTuple/stripComment the same way #17 did.

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
