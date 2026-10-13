//go:build goexperiment.simd && amd64

package ndb

import (
	"math/bits"
	"simd/archsimd"
)

// hasAVX2 gates every archsimd call below. 128-bit ops only need AVX, but
// AVX2 implies AVX and the candidate scan uses 256-bit compares.
var hasAVX2 = archsimd.X86.AVX2()

// The indexByteN helpers scan tokens that are usually 2–30 bytes long, so
// they use 16-byte vectors. A short tail is still read as a full vector when
// the slice's capacity allows it (lines and tokens are subslices of the file
// buffer); bytes past len are masked off, never returned.

func indexByte4(p []byte, a, b, c, d byte) int {
	if !hasAVX2 {
		return indexByte4Scalar(p, a, b, c, d)
	}
	va := archsimd.BroadcastUint8x16(a)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		m := v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros16(m)
		}
	}
	rest := p[i:]
	if len(rest) == 0 {
		return -1
	}
	if cap(rest) < 16 {
		if j := indexByte4Scalar(rest, a, b, c, d); j >= 0 {
			return i + j
		}
		return -1
	}
	v := archsimd.LoadUint8x16Array((*[16]byte)(rest[:16:16]))
	m := uint32(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).ToBits())
	if m &= 1<<uint(len(rest)) - 1; m != 0 {
		return i + bits.TrailingZeros32(m)
	}
	return -1
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	if !hasAVX2 {
		return indexByte5Scalar(p, a, b, c, d, e)
	}
	va := archsimd.BroadcastUint8x16(a)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	ve := archsimd.BroadcastUint8x16(e)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		m := v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros16(m)
		}
	}
	rest := p[i:]
	if len(rest) == 0 {
		return -1
	}
	if cap(rest) < 16 {
		if j := indexByte5Scalar(rest, a, b, c, d, e); j >= 0 {
			return i + j
		}
		return -1
	}
	v := archsimd.LoadUint8x16Array((*[16]byte)(rest[:16:16]))
	m := uint32(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).ToBits())
	if m &= 1<<uint(len(rest)) - 1; m != 0 {
		return i + bits.TrailingZeros32(m)
	}
	return -1
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	if !hasAVX2 {
		return indexByte6Scalar(p, a, b, c, d, e, f)
	}
	va := archsimd.BroadcastUint8x16(a)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	ve := archsimd.BroadcastUint8x16(e)
	vf := archsimd.BroadcastUint8x16(f)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		m := v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).Or(v.Equal(vf)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros16(m)
		}
	}
	rest := p[i:]
	if len(rest) == 0 {
		return -1
	}
	if cap(rest) < 16 {
		if j := indexByte6Scalar(rest, a, b, c, d, e, f); j >= 0 {
			return i + j
		}
		return -1
	}
	v := archsimd.LoadUint8x16Array((*[16]byte)(rest[:16:16]))
	m := uint32(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).Or(v.Equal(vf)).ToBits())
	if m &= 1<<uint(len(rest)) - 1; m != 0 {
		return i + bits.TrailingZeros32(m)
	}
	return -1
}

// indexCandidate is the SIMD form of indexCandidateScalar. It uses the
// first/last-byte filter: compare 32 positions against needle[0] and the 32
// positions n-1 bytes later against needle[n-1], AND the masks, and verify
// only the surviving positions. Unlike bytes.Index it does not degrade when
// needle[0] is common (bytes.Index pivots on the first byte alone). The
// backslash stop folds into the same pass.
func indexCandidate(data, needle []byte, stopAtBackslash bool) int {
	if !hasAVX2 || len(needle) < 2 {
		return indexCandidateScalar(data, needle, stopAtBackslash)
	}
	j, i := indexCandidateAVX2(data, needle, stopAtBackslash)
	// The compiler does not yet emit VZEROUPPER after 256-bit code. Without
	// it, SSE code that runs next (most of the parser and runtime) picks up
	// false dependencies on the dirty upper halves and slows down sharply.
	archsimd.ClearAVXUpperBits()
	if j >= 0 {
		return j
	}
	if j := indexCandidateScalar(data[i:], needle, stopAtBackslash); j >= 0 {
		return i + j
	}
	return -1
}

// indexCandidateAVX2 scans whole 32-byte blocks. It returns the hit, or -1
// and the offset where the unscanned tail begins. It makes no calls so the
// YMM state stays local until indexCandidate clears it.
func indexCandidateAVX2(data, needle []byte, stopAtBackslash bool) (hit, tail int) {
	n := len(needle)
	vf := archsimd.BroadcastUint8x32(needle[0])
	vl := archsimd.BroadcastUint8x32(needle[n-1])
	vbs := archsimd.BroadcastUint8x32('\\')
	i := 0
	for ; i+n-1+32 <= len(data); i += 32 {
		first := archsimd.LoadUint8x32Array((*[32]byte)(data[i : i+32]))
		last := archsimd.LoadUint8x32Array((*[32]byte)(data[i+n-1 : i+n-1+32]))
		m := first.Equal(vf).And(last.Equal(vl)).ToBits()
		if stopAtBackslash {
			m |= first.Equal(vbs).ToBits()
		}
	candidates:
		for m != 0 {
			j := i + bits.TrailingZeros32(m)
			m &= m - 1
			if stopAtBackslash && data[j] == '\\' {
				return j, 0
			}
			for k := 1; k < n-1; k++ {
				if data[j+k] != needle[k] {
					continue candidates
				}
			}
			return j, 0
		}
	}
	return -1, i
}
