//go:build goexperiment.simd && amd64

package ndb

import (
	"math/bits"
	"simd/archsimd"
)

var (
	useAVX2   bool
	useAVX512 bool
)

func init() {
	// archsimd.X86.AVX512 is AVX512F+CD+BW+DQ+VL, which is the bundle
	// required for Uint8x64 / Mask8x64.ToBits (KMOVB).
	useAVX512 = archsimd.X86.AVX512()
	useAVX2 = archsimd.X86.AVX2()
}

func simdIndexEnabled() bool { return useAVX2 || useAVX512 }

func indexByte4SIMD(p []byte, a, b, c, d byte) int {
	if useAVX512 && len(p) >= 64 {
		return indexByte4AVX512(p, a, b, c, d)
	}
	return indexByte4AVX2(p, a, b, c, d)
}

func indexByte5SIMD(p []byte, a, b, c, d, e byte) int {
	if useAVX512 && len(p) >= 64 {
		return indexByte5AVX512(p, a, b, c, d, e)
	}
	return indexByte5AVX2(p, a, b, c, d, e)
}

func indexByte6SIMD(p []byte, a, b, c, d, e, f byte) int {
	if useAVX512 && len(p) >= 64 {
		return indexByte6AVX512(p, a, b, c, d, e, f)
	}
	return indexByte6AVX2(p, a, b, c, d, e, f)
}

func indexByte4AVX2(p []byte, a, b, c, d byte) int {
	ba := archsimd.BroadcastUint8x32(a)
	bb := archsimd.BroadcastUint8x32(b)
	bc := archsimd.BroadcastUint8x32(c)
	bd := archsimd.BroadcastUint8x32(d)
	n := len(p)
	i := 0
	for i+32 <= n {
		v := archsimd.LoadUint8x32(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros32(m)
		}
		i += 32
	}
	if i < n {
		if j := indexByte4Scalar(p[i:], a, b, c, d); j >= 0 {
			return i + j
		}
	}
	return -1
}

func indexByte5AVX2(p []byte, a, b, c, d, e byte) int {
	ba := archsimd.BroadcastUint8x32(a)
	bb := archsimd.BroadcastUint8x32(b)
	bc := archsimd.BroadcastUint8x32(c)
	bd := archsimd.BroadcastUint8x32(d)
	be := archsimd.BroadcastUint8x32(e)
	n := len(p)
	i := 0
	for i+32 <= n {
		v := archsimd.LoadUint8x32(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).Or(v.Equal(be)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros32(m)
		}
		i += 32
	}
	if i < n {
		if j := indexByte5Scalar(p[i:], a, b, c, d, e); j >= 0 {
			return i + j
		}
	}
	return -1
}

func indexByte6AVX2(p []byte, a, b, c, d, e, f byte) int {
	ba := archsimd.BroadcastUint8x32(a)
	bb := archsimd.BroadcastUint8x32(b)
	bc := archsimd.BroadcastUint8x32(c)
	bd := archsimd.BroadcastUint8x32(d)
	be := archsimd.BroadcastUint8x32(e)
	bf := archsimd.BroadcastUint8x32(f)
	n := len(p)
	i := 0
	for i+32 <= n {
		v := archsimd.LoadUint8x32(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).Or(v.Equal(be)).Or(v.Equal(bf)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros32(m)
		}
		i += 32
	}
	if i < n {
		if j := indexByte6Scalar(p[i:], a, b, c, d, e, f); j >= 0 {
			return i + j
		}
	}
	return -1
}

func indexByte4AVX512(p []byte, a, b, c, d byte) int {
	ba := archsimd.BroadcastUint8x64(a)
	bb := archsimd.BroadcastUint8x64(b)
	bc := archsimd.BroadcastUint8x64(c)
	bd := archsimd.BroadcastUint8x64(d)
	n := len(p)
	i := 0
	for i+64 <= n {
		v := archsimd.LoadUint8x64(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros64(m)
		}
		i += 64
	}
	if i < n {
		if n-i >= 32 && useAVX2 {
			if j := indexByte4AVX2(p[i:], a, b, c, d); j >= 0 {
				return i + j
			}
			return -1
		}
		if j := indexByte4Scalar(p[i:], a, b, c, d); j >= 0 {
			return i + j
		}
	}
	return -1
}

func indexByte5AVX512(p []byte, a, b, c, d, e byte) int {
	ba := archsimd.BroadcastUint8x64(a)
	bb := archsimd.BroadcastUint8x64(b)
	bc := archsimd.BroadcastUint8x64(c)
	bd := archsimd.BroadcastUint8x64(d)
	be := archsimd.BroadcastUint8x64(e)
	n := len(p)
	i := 0
	for i+64 <= n {
		v := archsimd.LoadUint8x64(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).Or(v.Equal(be)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros64(m)
		}
		i += 64
	}
	if i < n {
		if n-i >= 32 && useAVX2 {
			if j := indexByte5AVX2(p[i:], a, b, c, d, e); j >= 0 {
				return i + j
			}
			return -1
		}
		if j := indexByte5Scalar(p[i:], a, b, c, d, e); j >= 0 {
			return i + j
		}
	}
	return -1
}

func indexByte6AVX512(p []byte, a, b, c, d, e, f byte) int {
	ba := archsimd.BroadcastUint8x64(a)
	bb := archsimd.BroadcastUint8x64(b)
	bc := archsimd.BroadcastUint8x64(c)
	bd := archsimd.BroadcastUint8x64(d)
	be := archsimd.BroadcastUint8x64(e)
	bf := archsimd.BroadcastUint8x64(f)
	n := len(p)
	i := 0
	for i+64 <= n {
		v := archsimd.LoadUint8x64(p[i:])
		m := v.Equal(ba).Or(v.Equal(bb)).Or(v.Equal(bc)).Or(v.Equal(bd)).Or(v.Equal(be)).Or(v.Equal(bf)).ToBits()
		if m != 0 {
			return i + bits.TrailingZeros64(m)
		}
		i += 64
	}
	if i < n {
		if n-i >= 32 && useAVX2 {
			if j := indexByte6AVX2(p[i:], a, b, c, d, e, f); j >= 0 {
				return i + j
			}
			return -1
		}
		if j := indexByte6Scalar(p[i:], a, b, c, d, e, f); j >= 0 {
			return i + j
		}
	}
	return -1
}
