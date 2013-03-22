// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

const (
	uvnan    = 0x7FF8000000000001
	uvinf    = 0x7FF0000000000000
	uvneginf = 0xFFF0000000000000
	mask     = 0x7FF
	shift    = 64 - 11 - 1
	bias     = 1023
)

// Inf returns positive infinity if sign >= 0, negative infinity if sign < 0.

// Inf 返回无穷大值。若 sign >= 0，则返回正无穷大；若 sign < 0，则返回负无穷大。
func Inf(sign int) float64 {
	var v uint64
	if sign >= 0 {
		v = uvinf
	} else {
		v = uvneginf
	}
	return Float64frombits(v)
}

// NaN returns an IEEE 754 ``not-a-number'' value.

// NaN 返回IEEE 754定义的“非数值”。
func NaN() float64 { return Float64frombits(uvnan) }

// IsNaN returns whether f is an IEEE 754 ``not-a-number'' value.

// IsNaN 判断 f 是否为IEEE 754定义的“非数值”。
func IsNaN(f float64) (is bool) {
	// IEEE 754 says that only NaNs satisfy f != f.
	// To avoid the floating-point hardware, could use:
	//	x := Float64bits(f);
	//	return uint32(x>>shift)&mask == mask && x != uvinf && x != uvneginf
	//
	// IEEE 754定义了只有 NaN 满足 f != f。
	// 为避免浮点数硬件，应使用：
	//	x := Float64bits(f);
	//	return uint32(x>>shift)&mask == mask && x != uvinf && x != uvneginf
	return f != f
}

// IsInf returns whether f is an infinity, according to sign.
// If sign > 0, IsInf returns whether f is positive infinity.
// If sign < 0, IsInf returns whether f is negative infinity.
// If sign == 0, IsInf returns whether f is either infinity.

// IsInf 判断 f 是否为无穷大值，视 sign 而定。
// 若 sign > 0，IsInf 就判断 f 是否为正无穷大。
// 若 sign < 0，IsInf 就判断 f 是否为负无穷大。
// 若 sign == 0，IsInf 就判断 f 是否为无穷大。
func IsInf(f float64, sign int) bool {
	// Test for infinity by comparing against maximum float.
	// To avoid the floating-point hardware, could use:
	//	x := Float64bits(f);
	//	return sign >= 0 && x == uvinf || sign <= 0 && x == uvneginf;
	//
	// 通过与最大的浮点数值进行比较来测试是否为无穷大。
	// 为避免浮点数硬件，应使用：
	//	x := Float64bits(f);
	//	return sign >= 0 && x == uvinf || sign <= 0 && x == uvneginf;
	return sign >= 0 && f > MaxFloat64 || sign <= 0 && f < -MaxFloat64
}

// normalize returns a normal number y and exponent exp
// satisfying x == y × 2**exp. It assumes x is finite and non-zero.

// TDOD(osc): normalize =?
// normalize 返回一个普通数 y 和一个指数 exp，使得它们满足 x == y × 2**exp。
// 该函数假定 x 为有限的非零数值。
func normalize(x float64) (y float64, exp int) {
	const SmallestNormal = 2.2250738585072014e-308 // 2**-1022
	if Abs(x) < SmallestNormal {
		return x * (1 << 52), -52
	}
	return x, 0
}
