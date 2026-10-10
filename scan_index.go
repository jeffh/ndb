package ndb

// Scalar first-of-N byte searches. These are the portable fallback and the
// reference used by SIMD parity tests. GOEXPERIMENT=simd may replace the
// public indexByte4/5/6 wrappers with simd/archsimd on supported CPUs.
//
// bytes.IndexByte already has AVX2 assembly in internal/bytealg; these helpers
// exist because the stdlib has no first-of-4/5/6 equivalent that is faster
// than a byte loop on short, delimiter-dense ndb slices.

func indexByte4Scalar(p []byte, a, b, c, d byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d {
			return i
		}
	}
	return -1
}

func indexByte5Scalar(p []byte, a, b, c, d, e byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d || x == e {
			return i
		}
	}
	return -1
}

func indexByte6Scalar(p []byte, a, b, c, d, e, f byte) int {
	for i, x := range p {
		if x == a || x == b || x == c || x == d || x == e || x == f {
			return i
		}
	}
	return -1
}
