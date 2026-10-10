//go:build goexperiment.simd && !amd64

package ndb

// arm64 NEON and wasm 128-bit SIMD are available in simd/archsimd, but
// Mask8x16 has no ToBits() on those arches in Go 1.27 (amd64 VPMOVMSKB
// only). A Store-and-scan fallback is slower than the scalar loop on the
// short, delimiter-dense slices ndb actually searches, so we keep the
// portable byte loops.

func simdIndexEnabled() bool { return false }

func indexByte4SIMD(p []byte, a, b, c, d byte) int {
	return indexByte4Scalar(p, a, b, c, d)
}

func indexByte5SIMD(p []byte, a, b, c, d, e byte) int {
	return indexByte5Scalar(p, a, b, c, d, e)
}

func indexByte6SIMD(p []byte, a, b, c, d, e, f byte) int {
	return indexByte6Scalar(p, a, b, c, d, e, f)
}
