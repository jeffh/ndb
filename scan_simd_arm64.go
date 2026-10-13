//go:build goexperiment.simd && arm64

package ndb

import (
	"math/bits"
	"simd/archsimd"
	"unsafe"
)

// NEON (Advanced SIMD) is part of the arm64 baseline, so unlike amd64 there
// is no runtime feature check and no upper-register state to clear.
//
// NEON has no movemask. nibbleMask packs a byte-compare result into 64 bits,
// four bits per byte, by viewing it as eight 16-bit lanes, shifting each
// right by 4 and narrowing back to bytes (USHL by -4, then XTN). Byte i of the input
// lands in bits 4i..4i+3, so the first set byte is TrailingZeros64(m)/4.
//
// shr4 must be BroadcastInt16x8(-4). Callers build it once outside their
// loops; ShiftAllRight(4) would rebuild that vector on every call.
func nibbleMask(m archsimd.Mask8x16, shr4 archsimd.Int16x8) uint64 {
	return m.ToInt8x16().ToBits().ReshapeToUint16s().Shift(shr4).TruncToUint8().ReshapeToUint64s().GetElem(0)
}

// The parser's indexByte4/5/6 stay scalar on arm64. Its tokens are usually
// under ten bytes, and on GitHub's arm64 runners a NEON version lost 12-17% on
// parse-heavy benchmarks: building five or six broadcast vectors and a
// nibble mask per call costs more than the short scalar loop it replaces,
// which also inlines into its callers.

func indexByte4(p []byte, a, b, c, d byte) int {
	return indexByte4Scalar(p, a, b, c, d)
}

func indexByte5(p []byte, a, b, c, d, e byte) int {
	return indexByte5Scalar(p, a, b, c, d, e)
}

func indexByte6(p []byte, a, b, c, d, e, f byte) int {
	return indexByte6Scalar(p, a, b, c, d, e, f)
}

// indexCandidate is the NEON form of indexCandidateScalar, using the same
// first/last-byte filter as the AVX2 version: compare positions against
// needle[0] and the positions n-1 bytes later against needle[n-1], AND the
// masks, and verify only the surviving positions. The backslash stop folds
// into the same pass. Each iteration covers 32 bytes as two 16-byte halves
// and tests their OR once, so a block without candidates costs a single
// mask extraction.
func indexCandidate(data, needle []byte, stopAtBackslash bool) int {
	n := len(needle)
	if n < 2 {
		return indexCandidateScalar(data, needle, stopAtBackslash)
	}
	vf := archsimd.BroadcastUint8x16(needle[0])
	vl := archsimd.BroadcastUint8x16(needle[n-1])
	vbs := archsimd.BroadcastUint8x16('\\')
	shr4 := archsimd.BroadcastInt16x8(-4)
	base := unsafe.Pointer(unsafe.SliceData(data))
	i := 0
	for ; i+n-1+32 <= len(data); i += 32 {
		// The loop condition keeps both 32-byte windows inside data, so
		// address them directly instead of paying slice bounds checks.
		f := (*[32]byte)(unsafe.Add(base, i))
		l := (*[32]byte)(unsafe.Add(base, i+n-1))
		f0 := archsimd.LoadUint8x16Array((*[16]byte)(f[:16]))
		f1 := archsimd.LoadUint8x16Array((*[16]byte)(f[16:]))
		h0 := f0.Equal(vf).And(archsimd.LoadUint8x16Array((*[16]byte)(l[:16])).Equal(vl))
		h1 := f1.Equal(vf).And(archsimd.LoadUint8x16Array((*[16]byte)(l[16:])).Equal(vl))
		if stopAtBackslash {
			h0 = h0.Or(f0.Equal(vbs))
			h1 = h1.Or(f1.Equal(vbs))
		}
		if nibbleMask(h0.Or(h1), shr4) == 0 {
			continue
		}
		for half, h := range [2]archsimd.Mask8x16{h0, h1} {
			off := i + 16*half
			m := nibbleMask(h, shr4)
		candidates:
			for m != 0 {
				j := off + bits.TrailingZeros64(m)/4
				m &^= 0xf << (4 * uint(j-off))
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
	}
	if j := indexCandidateScalar(data[i:], needle, stopAtBackslash); j >= 0 {
		return i + j
	}
	return -1
}
