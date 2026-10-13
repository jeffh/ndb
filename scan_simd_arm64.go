//go:build goexperiment.simd && arm64

package ndb

import (
	"math/bits"
	"simd/archsimd"
)

// NEON (Advanced SIMD) is part of the arm64 baseline, so unlike amd64 there
// is no runtime feature check.
//
// NEON has no movemask. nibbleMask packs a byte-compare result into 64 bits,
// four bits per byte, by viewing it as eight 16-bit lanes, shifting each
// right by 4 and narrowing back to bytes (USHR + XTN). Byte i of the input
// lands in bits 4i..4i+3, so the first set byte is TrailingZeros64(m)/4.
//
// shr4 must be BroadcastInt16x8(-4). Callers build it once outside their
// loops; ShiftAllRight(4) would rebuild that vector on every call.
func nibbleMask(m archsimd.Mask8x16, shr4 archsimd.Int16x8) uint64 {
	return m.ToInt8x16().ToBits().ReshapeToUint16s().Shift(shr4).TruncToUint8().ReshapeToUint64s().GetElem(0)
}

// tailNibbles keeps the nibbles of the first n bytes, 0 < n < 16.
func tailNibbles(m uint64, n int) uint64 {
	return m & (1<<(4*uint(n)) - 1)
}

// The indexByteN helpers scan tokens that are usually 2–30 bytes long. A
// short tail is still read as a full vector when the slice's capacity allows
// it (lines and tokens are subslices of the file buffer); bytes past len are
// masked off, never returned.

func indexByte4(p []byte, a, b, c, d byte) int {
	va := archsimd.BroadcastUint8x16(a)
	shr4 := archsimd.BroadcastInt16x8(-4)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		if m := nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)), shr4); m != 0 {
			return i + bits.TrailingZeros64(m)/4
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
	if m := tailNibbles(nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)), shr4), len(rest)); m != 0 {
		return i + bits.TrailingZeros64(m)/4
	}
	return -1
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	va := archsimd.BroadcastUint8x16(a)
	shr4 := archsimd.BroadcastInt16x8(-4)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	ve := archsimd.BroadcastUint8x16(e)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		if m := nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)), shr4); m != 0 {
			return i + bits.TrailingZeros64(m)/4
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
	if m := tailNibbles(nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)), shr4), len(rest)); m != 0 {
		return i + bits.TrailingZeros64(m)/4
	}
	return -1
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	va := archsimd.BroadcastUint8x16(a)
	shr4 := archsimd.BroadcastInt16x8(-4)
	vb := archsimd.BroadcastUint8x16(b)
	vc := archsimd.BroadcastUint8x16(c)
	vd := archsimd.BroadcastUint8x16(d)
	ve := archsimd.BroadcastUint8x16(e)
	vf := archsimd.BroadcastUint8x16(f)
	i := 0
	for ; i+16 <= len(p); i += 16 {
		v := archsimd.LoadUint8x16Array((*[16]byte)(p[i : i+16]))
		if m := nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).Or(v.Equal(vf)), shr4); m != 0 {
			return i + bits.TrailingZeros64(m)/4
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
	if m := tailNibbles(nibbleMask(v.Equal(va).Or(v.Equal(vb)).Or(v.Equal(vc)).Or(v.Equal(vd)).Or(v.Equal(ve)).Or(v.Equal(vf)), shr4), len(rest)); m != 0 {
		return i + bits.TrailingZeros64(m)/4
	}
	return -1
}

// indexCandidate is the NEON form of indexCandidateScalar, using the same
// first/last-byte filter as the AVX2 version over 16-byte blocks: compare 16
// positions against needle[0] and the 16 positions n-1 bytes later against
// needle[n-1], AND the masks, and verify only the surviving positions. The
// backslash stop folds into the same pass.
func indexCandidate(data, needle []byte, stopAtBackslash bool) int {
	n := len(needle)
	if n < 2 {
		return indexCandidateScalar(data, needle, stopAtBackslash)
	}
	vf := archsimd.BroadcastUint8x16(needle[0])
	vl := archsimd.BroadcastUint8x16(needle[n-1])
	vbs := archsimd.BroadcastUint8x16('\\')
	shr4 := archsimd.BroadcastInt16x8(-4)
	i := 0
	for ; i+n-1+16 <= len(data); i += 16 {
		first := archsimd.LoadUint8x16Array((*[16]byte)(data[i : i+16]))
		last := archsimd.LoadUint8x16Array((*[16]byte)(data[i+n-1 : i+n-1+16]))
		hit := first.Equal(vf).And(last.Equal(vl))
		if stopAtBackslash {
			hit = hit.Or(first.Equal(vbs))
		}
		m := nibbleMask(hit, shr4)
	candidates:
		for m != 0 {
			j := i + bits.TrailingZeros64(m)/4
			m &^= 0xf << (4 * uint(j-i))
			if stopAtBackslash && data[j] == '\\' {
				return j
			}
			for k := 1; k < n-1; k++ {
				if data[j+k] != needle[k] {
					continue candidates
				}
			}
			return j
		}
	}
	if j := indexCandidateScalar(data[i:], needle, stopAtBackslash); j >= 0 {
		return i + j
	}
	return -1
}
