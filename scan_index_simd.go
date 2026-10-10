//go:build goexperiment.simd

package ndb

// simdMin is the smallest haystack that is worth a vector load. Below this
// the scalar loop wins because ndb delimiters (=, space) usually appear in
// the first few bytes of a tuple.
const simdMin = 32

func indexByte4(p []byte, a, b, c, d byte) int {
	if simdIndexEnabled() && len(p) >= simdMin {
		return indexByte4SIMD(p, a, b, c, d)
	}
	return indexByte4Scalar(p, a, b, c, d)
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	if simdIndexEnabled() && len(p) >= simdMin {
		return indexByte5SIMD(p, a, b, c, d, e)
	}
	return indexByte5Scalar(p, a, b, c, d, e)
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	if simdIndexEnabled() && len(p) >= simdMin {
		return indexByte6SIMD(p, a, b, c, d, e, f)
	}
	return indexByte6Scalar(p, a, b, c, d, e, f)
}
