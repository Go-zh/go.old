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

// Complex circular arc sine
//
// DESCRIPTION:
//
// Inverse complex sine:
//                               2
// w = -i clog( iz + csqrt( 1 - z ) ).
//
// casin(z) = -i casinh(iz)
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10     10100       2.1e-15     3.4e-16
//    IEEE      -10,+10     30000       2.2e-14     2.7e-15
// Larger relative error can be observed for z near zero.
// Also tested by csin(casin(z)) = z.

// 复数的反正弦
//
// 描述：
//
// 反转的复数正弦：
//                               2
// w = -i clog( iz + csqrt( 1 - z ) ).
//
// casin(z) = -i casinh(iz)
//
// 精度：
//
//                         相对误差:
//    算法      范围       # 测试次数     峰值         均方根
//    DEC       -10,+10     10100       2.1e-15     3.4e-16
//    IEEE      -10,+10     30000       2.2e-14     2.7e-15
// 当 z 接近零时可观察到较大的相对误差。
// 此函数也通过了 csin(casin(z)) = z 的测试。

// Asin returns the inverse sine of x.

// Asin 返回 x 的反正弦值。
func Asin(x complex128) complex128 {
	if imag(x) == 0 {
		if math.Abs(real(x)) > 1 {
			return complex(math.Pi/2, 0) // DOMAIN error // 范围误差
		}
		return complex(math.Asin(real(x)), 0)
	}
	ct := complex(-imag(x), real(x)) // i * x
	xx := x * x
	x1 := complex(1-real(xx), -imag(xx)) // 1 - x*x
	x2 := Sqrt(x1)                       // x2 = sqrt(1 - x*x)
	w := Log(ct + x2)
	return complex(imag(w), -real(w)) // -i * w
}

// Asinh returns the inverse hyperbolic sine of x.

// Asinh 返回 x 的反双曲正弦值。
func Asinh(x complex128) complex128 {
	// TODO check range
	if imag(x) == 0 {
		if math.Abs(real(x)) > 1 {
			return complex(math.Pi/2, 0) // DOMAIN error // 范围误差
		}
		return complex(math.Asinh(real(x)), 0)
	}
	xx := x * x
	x1 := complex(1+real(xx), imag(xx)) // 1 + x*x
	return Log(x + Sqrt(x1))            // log(x + sqrt(1 + x*x))
}

// Complex circular arc cosine
//
// DESCRIPTION:
//
// w = arccos z  =  PI/2 - arcsin z.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      5200      1.6e-15      2.8e-16
//    IEEE      -10,+10     30000      1.8e-14      2.2e-15

// 复数的反余弦
//
// 描述：
//
// w = arccos z  =  PI/2 - arcsin z.
//
// 精度：
//
//                         相对误差:
//    算法      范围       # 测试次数     峰值         均方根
//    DEC       -10,+10      5200      1.6e-15      2.8e-16
//    IEEE      -10,+10     30000      1.8e-14      2.2e-15

// Acos returns the inverse cosine of x.

// Acos 返回 x 的反余弦值
func Acos(x complex128) complex128 {
	w := Asin(x)
	return complex(math.Pi/2-real(w), -imag(w))
}

// Acosh returns the inverse hyperbolic cosine of x.

// Acosh 返回 x 的反双曲余弦值。
func Acosh(x complex128) complex128 {
	w := Acos(x)
	if imag(w) <= 0 {
		return complex(-imag(w), real(w)) // i * w
	}
	return complex(imag(w), -real(w)) // -i * w
}

// Complex circular arc tangent
//
// DESCRIPTION:
//
// If
//     z = x + iy,
//
// then
//          1       (    2x     )
// Re w  =  - arctan(-----------)  +  k PI
//          2       (     2    2)
//                  (1 - x  - y )
//
//               ( 2         2)
//          1    (x  +  (y+1) )
// Im w  =  - log(------------)
//          4    ( 2         2)
//               (x  +  (y-1) )
//
// Where k is an arbitrary integer.
//
// catan(z) = -i catanh(iz).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      5900       1.3e-16     7.8e-18
//    IEEE      -10,+10     30000       2.3e-15     8.5e-17
// The check catan( ctan(z) )  =  z, with |x| and |y| < PI/2,
// had peak relative error 1.5e-16, rms relative error
// 2.9e-17.  See also clog().

// 复数的反正切
//
// 描述：
//
// 若
//     z = x + iy,
//
// 则
//          1       (    2x     )
// Re w  =  - arctan(-----------)  +  k PI
//          2       (     2    2)
//                  (1 - x  - y )
//
//               ( 2         2)
//          1    (x  +  (y+1) )
// Im w  =  - log(------------)
//          4    ( 2         2)
//               (x  +  (y-1) )
//
// 其中 k 为任意整数。
//
// catan(z) = -i catanh(iz).
//
// 精度：
//
//                         相对误差:
//    算法      范围       # 测试次数     峰值         均方根
//    DEC       -10,+10      5900       1.3e-16     7.8e-18
//    IEEE      -10,+10     30000       2.3e-15     8.5e-17
// 以 |x| 和 |y| < PI/2 对 catan( ctan(z) )  =  z 进行检测，
// 有峰值相对误差 1.5e-16，均方根相对误差 2.9e-17。参见 clog()。

// Atan returns the inverse tangent of x.

// Atan 返回 x 的反正切。
func Atan(x complex128) complex128 {
	if real(x) == 0 && imag(x) > 1 {
		return NaN()
	}

	x2 := real(x) * real(x)
	a := 1 - x2 - imag(x)*imag(x)
	if a == 0 {
		return NaN()
	}
	t := 0.5 * math.Atan2(2*real(x), a)
	w := reducePi(t)

	t = imag(x) - 1
	b := x2 + t*t
	if b == 0 {
		return NaN()
	}
	t = imag(x) + 1
	c := (x2 + t*t) / b
	return complex(w, 0.25*math.Log(c))
}

// Atanh returns the inverse hyperbolic tangent of x.

// Atanh 返回 x 的双曲反正切。
func Atanh(x complex128) complex128 {
	z := complex(-imag(x), real(x)) // z = i * x
	z = Atan(z)
	return complex(imag(z), -real(z)) // z = -i * z
}
