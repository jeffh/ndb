package ndb

import "bytes"

// Portable scanners. scan_simd_amd64.go replaces the exported-to-package
// entry points (indexByte4/5/6, indexCandidate) with archsimd versions when
// built with GOEXPERIMENT=simd on amd64; scan_generic.go forwards to these
// otherwise. The SIMD versions also fall back here on CPUs without AVX2.

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

// indexCandidateScalar returns the first index in data where needle starts,
// or, when stopAtBackslash is set, where a '\\' occurs, whichever is first.
// It returns -1 if neither occurs. needle must be non-empty.
func indexCandidateScalar(data, needle []byte, stopAtBackslash bool) int {
	i := bytes.Index(data, needle)
	if stopAtBackslash {
		head := data
		if i >= 0 {
			head = data[:i]
		}
		if j := bytes.IndexByte(head, '\\'); j >= 0 {
			return j
		}
	}
	return i
}
