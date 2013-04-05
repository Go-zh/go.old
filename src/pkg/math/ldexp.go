// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Ldexp is the inverse of Frexp.
// It returns frac × 2**exp.
//
// Special cases are:
//	Ldexp(±0, exp) = ±0
//	Ldexp(±Inf, exp) = ±Inf
//	Ldexp(NaN, exp) = NaN

// Ldexp 为 Frexp 的反函数。
// 它返回 frac × 2**exp。
//
// 特殊情况为：
//	Ldexp(±0, exp)   = ±0
//	Ldexp(±Inf, exp) = ±Inf
//	Ldexp(NaN, exp)  = NaN
func Ldexp(frac float64, exp int) float64

func ldexp(frac float64, exp int) float64 {
	// special cases
	// 特殊情况
	switch {
	case frac == 0:
		// 正确返回 -0
		return frac // correctly return -0
	case IsInf(frac, 0) || IsNaN(frac):
		return frac
	}
	frac, e := normalize(frac)
	exp += e
	x := Float64bits(frac)
	exp += int(x>>shift)&mask - bias
	if exp < -1074 {
		// 向下溢出
		return Copysign(0, frac) // underflow
	}
	// 向上溢出
	if exp > 1023 { // overflow
		if frac < 0 {
			return Inf(-1)
		}
		return Inf(1)
	}
	var m float64 = 1
	// 非正常表示情况
	if exp < -1022 { // denormal
		exp += 52
		m = 1.0 / (1 << 52) // 2**-52
	}
	x &^= mask << shift
	x |= uint64(exp+bias) << shift
	return m * Float64frombits(x)
}
