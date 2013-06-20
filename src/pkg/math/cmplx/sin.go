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

// Complex circular sine
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//     w = sin x  cosh y  +  i cos x sinh y.
//
// csin(z) = -i csinh(iz).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      8400       5.3e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16
// Also tested by csin(casin(z)) = z.

// 复数的正弦
//
// 描述：
//
// 若
//     z = x + iy,
//
// 则
//
//     w = sin x  cosh y  +  i cos x sinh y.
//
// csin(z) = -i csinh(iz).
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    DEC       -10,+10      8400       5.3e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16
// 此函数也通过了 csin(casin(z)) = z 的测试。

// Sin returns the sine of x.

// Sin 返回 x 的正弦值。
func Sin(x complex128) complex128 {
	s, c := math.Sincos(real(x))
	sh, ch := sinhcosh(imag(x))
	return complex(s*ch, c*sh)
}

// Complex hyperbolic sine
//
// DESCRIPTION:
//
// csinh z = (cexp(z) - cexp(-z))/2
//         = sinh x * cos y  +  i cosh x * sin y .
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       3.1e-16     8.2e-17

// 复数的双曲正弦
//
// 描述：
//
// csinh z = (cexp(z) - cexp(-z))/2
//         = sinh x * cos y  +  i cosh x * sin y .
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    IEEE      -10,+10     30000       3.1e-16     8.2e-17

// Sinh returns the hyperbolic sine of x.

// Sinh 返回 x 的双曲正弦值。
func Sinh(x complex128) complex128 {
	s, c := math.Sincos(imag(x))
	sh, ch := sinhcosh(real(x))
	return complex(c*sh, s*ch)
}

// Complex circular cosine
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//
//     w = cos x  cosh y  -  i sin x sinh y.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      8400       4.5e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16

// 复数的余弦
//
// 描述
//
// 若
//     z = x + iy,
//
// 则
//
//     w = cos x  cosh y  -  i sin x sinh y.
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    DEC       -10,+10      8400       4.5e-17     1.3e-17
//    IEEE      -10,+10     30000       3.8e-16     1.0e-16

// Cos returns the cosine of x.

// Cos 返回 x 的余弦值。
func Cos(x complex128) complex128 {
	s, c := math.Sincos(real(x))
	sh, ch := sinhcosh(imag(x))
	return complex(c*ch, -s*sh)
}

// Complex hyperbolic cosine
//
// DESCRIPTION:
//
// ccosh(z) = cosh x  cos y + i sinh x sin y .
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    IEEE      -10,+10     30000       2.9e-16     8.1e-17

// 负数的双曲余弦
//
// 描述
//
// ccosh(z) = cosh x  cos y + i sinh x sin y .
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    IEEE      -10,+10     30000       2.9e-16     8.1e-17

// Cosh returns the hyperbolic cosine of x.

// Cosh 返回 x 的双曲余弦值。
func Cosh(x complex128) complex128 {
	s, c := math.Sincos(imag(x))
	sh, ch := sinhcosh(real(x))
	return complex(c*ch, s*sh)
}

// calculate sinh and cosh

// 计算 sinh 和 cosh
func sinhcosh(x float64) (sh, ch float64) {
	if math.Abs(x) <= 0.5 {
		return math.Sinh(x), math.Cosh(x)
	}
	e := math.Exp(x)
	ei := 0.5 / e
	e *= 0.5
	return e - ei, e + ei
}
