// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

func isOddInt(x float64) bool {
	xi, xf := Modf(x)
	return xf == 0 && int64(xi)&1 == 1
}

// Special cases taken from FreeBSD's /usr/src/lib/msun/src/e_pow.c
// updated by IEEE Std. 754-2008 "Section 9.2.1 Special values".

// 特殊情况来自于 FreeBSD 的 /usr/src/lib/msun/src/e_pow.c 文件，
// 根据IEEE 754-2008标准《章节 9.2.1 特殊值》更新。

// Pow returns x**y, the base-x exponential of y.
//
// Special cases are (in order):
//	Pow(x, ±0) = 1 for any x
//	Pow(1, y) = 1 for any y
//	Pow(x, 1) = x for any x
//	Pow(NaN, y) = NaN
//	Pow(x, NaN) = NaN
//	Pow(±0, y) = ±Inf for y an odd integer < 0
//	Pow(±0, -Inf) = +Inf
//	Pow(±0, +Inf) = +0
//	Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
//	Pow(±0, y) = ±0 for y an odd integer > 0
//	Pow(±0, y) = +0 for finite y > 0 and not an odd integer
//	Pow(-1, ±Inf) = 1
//	Pow(x, +Inf) = +Inf for |x| > 1
//	Pow(x, -Inf) = +0 for |x| > 1
//	Pow(x, +Inf) = +0 for |x| < 1
//	Pow(x, -Inf) = +Inf for |x| < 1
//	Pow(+Inf, y) = +Inf for y > 0
//	Pow(+Inf, y) = +0 for y < 0
//	Pow(-Inf, y) = Pow(-0, -y)
//	Pow(x, y) = NaN for finite x < 0 and finite non-integer y

// Pow 返回 x**y，即以 x 为底的 y 次幂。
//
// 特殊情况为（按顺序）：
//	Pow(x, ±0)    = 1   （对于任何 x）
//	Pow(1, y)     = 1   （对于任何 y）
//	Pow(x, 1)     = x   （对于任何 x）
//	Pow(NaN, y)   = NaN
//	Pow(x, NaN)   = NaN
//	Pow(±0, y)    = ±Inf（对于奇整数 y < 0）
//	Pow(±0, -Inf) = +Inf
//	Pow(±0, +Inf) = +0
//	Pow(±0, y)    = +Inf（对于有限非奇整数 y < 0）
//	Pow(±0, y)    = ±0  （对于奇整数 y > 0）
//	Pow(±0, y)    = +0  （对于有限非奇整数 y >）
//	Pow(-1, ±Inf) = 1
//	Pow(x, +Inf)  = +Inf（对于 |x| > 1）
//	Pow(x, -Inf)  = +0  （对于 |x| > 1）
//	Pow(x, +Inf)  = +0  （对于 |x| < 1）
//	Pow(x, -Inf)  = +Inf（对于 |x| < 1）
//	Pow(+Inf, y)  = +Inf（对于 y > 0）
//	Pow(+Inf, y)  = +0  （对于 y < 0）
//	Pow(-Inf, y)  = Pow(-0, -y)
//	Pow(x, y)     = NaN （对于有限数 x < 0 和有限非整数 y）
func Pow(x, y float64) float64 {
	switch {
	case y == 0 || x == 1:
		return 1
	case y == 1:
		return x
	case y == 0.5:
		return Sqrt(x)
	case y == -0.5:
		return 1 / Sqrt(x)
	case IsNaN(x) || IsNaN(y):
		return NaN()
	case x == 0:
		switch {
		case y < 0:
			if isOddInt(y) {
				return Copysign(Inf(1), x)
			}
			return Inf(1)
		case y > 0:
			if isOddInt(y) {
				return x
			}
			return 0
		}
	case IsInf(y, 0):
		switch {
		case x == -1:
			return 1
		case (Abs(x) < 1) == IsInf(y, 1):
			return 0
		default:
			return Inf(1)
		}
	case IsInf(x, 0):
		if IsInf(x, -1) {
			return Pow(1/x, -y) // Pow(-0, -y)
		}
		switch {
		case y < 0:
			return 0
		case y > 0:
			return Inf(1)
		}
	}

	absy := y
	flip := false
	if absy < 0 {
		absy = -absy
		flip = true
	}
	yi, yf := Modf(absy)
	if yf != 0 && x < 0 {
		return NaN()
	}
	if yi >= 1<<63 {
		return Exp(y * Log(x))
	}

	// ans = a1 * 2**ae (= 1 for now).
	// ans = a1 * 2**ae（现在等于 1）。
	a1 := 1.0
	ae := 0

	// ans *= x**yf
	if yf != 0 {
		if yf > 0.5 {
			yf--
			yi++
		}
		a1 = Exp(yf * Log(x))
	}

	// ans *= x**yi
	// by multiplying in successive squarings
	// of x according to bits of yi.
	// accumulate powers of two into exp.
	//
	// ans *= x**yi
	// 根据 yi 的位数，将 x 的连续平方相乘，
	// 以此将 2 的幂累加成指数。
	x1, xe := Frexp(x)
	for i := int64(yi); i != 0; i >>= 1 {
		if i&1 == 1 {
			a1 *= x1
			ae += xe
		}
		x1 *= x1
		xe <<= 1
		if x1 < .5 {
			x1 += x1
			xe--
		}
	}

	// ans = a1*2**ae
	// if flip { ans = 1 / ans }
	// but in the opposite order
	//
	// ans = a1*2**ae
	// 若 flip 为真就执行 { ans = 1 / ans }
	// 否则就以相反的顺序。
	if flip {
		a1 = 1 / a1
		ae = -ae
	}
	return Ldexp(a1, ae)
}
