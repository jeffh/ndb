//go:build !goexperiment.simd || !amd64

package ndb

func indexByte4(p []byte, a, b, c, d byte) int {
	return indexByte4Scalar(p, a, b, c, d)
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	return indexByte5Scalar(p, a, b, c, d, e)
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	return indexByte6Scalar(p, a, b, c, d, e, f)
}

func indexCandidate(data, needle []byte, stopAtBackslash bool) int {
	return indexCandidateScalar(data, needle, stopAtBackslash)
}
