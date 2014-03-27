// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/c9x-complex/clog.c.
// The go code is a simplified version of the original C.
//
// Cephes Math Library Release 2.8:  June, 2000
// Copyright 1984, 1987, 1989, 1992, 2000 by Stephen L. Moshier
//
// The readme file at http://netlib.sandia.gov/cephes/ says:
//    Some software in this archive may be from the book _Methods and
// Programs for Mathematical Functions_ (Prentice-Hall or Simon & Schuster
// International, 1989) or from the Cephes Mathematical Library, a
// commercial product. In either event, it is copyrighted by the author.
// What you see here may be used freely but it comes with no support or
// guarantee.
//
//   The two known misprints in the book are repaired here in the
// source listings for the gamma function and the incomplete beta
// integral.
//
//   Stephen L. Moshier
//   moshier@na-net.ornl.gov

// Complex power function
//
// DESCRIPTION:
//
// Raises complex A to the complex Zth power.
// Definition is per AMS55 # 4.2.8,
// analytically equivalent to cpow(a,z) = cexp(z clog(a)).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       9.4e-15     1.5e-15

// 复数的幂函数
//
// 描述：
//
// 求出复数 A 的复数 Z 次幂。
// 定义遵循 AMS55 # 4.2.8，
// 解析式等价于 cpow(a,z) = cexp(z clog(a))。
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    IEEE      -10,+10     30000       9.4e-15     1.5e-15

// Pow returns x**y, the base-x exponential of y.
// For generalized compatiblity with math.Pow:
// Pow(0, ±0) returns 1+0i
// Pow(0, c) for real(c)<0 returns Inf+0i if imag(c) is zero, otherwise Inf+Inf i.

// Pow 返回 x**y，即以 x 为底的 y 次幂。
// 对于 math.Pow 的通用化兼容：
// Pow(0, ±0) 返回 1+0i
// 若 imag(c) 为零，则 Pow(0, c) 在 real(c)<0 时返回 Inf+0i, 否则返回 Inf+Inf i。
func Pow(x, y complex128) complex128 {
	if x == 0 { // Guaranteed also true for x == -0.
		r, i := real(y), imag(y)
		switch {
		case r == 0:
			return 1
		case r < 0:
			if i == 0 {
				return complex(math.Inf(1), 0)
			}
			return Inf()
		case r > 0:
			return 0
		}
		panic("not reached")
	}
	modulus := Abs(x)
	if modulus == 0 {
		return complex(0, 0)
	}
	r := math.Pow(modulus, real(y))
	arg := Phase(x)
	theta := real(y) * arg
	if imag(y) != 0 {
		r *= math.Exp(-imag(y) * arg)
		theta += imag(y) * math.Log(modulus)
	}
	s, c := math.Sincos(theta)
	return complex(r*c, r*s)
}
