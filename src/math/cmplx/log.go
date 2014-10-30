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

// Complex natural logarithm
//
// DESCRIPTION:
//
// Returns complex logarithm to the base e (2.718...) of
// the complex argument z.
//
// If
//       z = x + iy, r = sqrt( x**2 + y**2 ),
// then
//       w = log(r) + i arctan(y/x).
//
// The arctangent ranges from -PI to +PI.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC       -10,+10      7000       8.5e-17     1.9e-17
//    IEEE      -10,+10     30000       5.0e-15     1.1e-16
//
// Larger relative error can be observed for z near 1 +i0.
// In IEEE arithmetic the peak absolute error is 5.2e-16, rms
// absolute error 1.0e-16.

// 复数的自然对数
//
// 描述：
//
// 返回以 e（2.718...）为底复数实参 z 的复数对数。
//
// 若
//       z = x + iy, r = sqrt( x**2 + y**2 ),
// 则
//       w = log(r) + i arctan(y/x).
//
// 其反正切范围从 -Pi 至 +Pi。
//
// 精度：
//
//                         相对误差:
//    算法      范围         测试次数     峰值         均方根
//    DEC       -10,+10      7000       8.5e-17     1.9e-17
//    IEEE      -10,+10     30000       5.0e-15     1.1e-16
//
// 当 z 接近 1+0i 时可观察到较大的相对误差。
// IEEE算法的峰值绝对误差为 5.2e-16，均方根绝对误差为 1.0e-16。

// Log returns the natural logarithm of x.

// Log 返回 x 的自然对数。
func Log(x complex128) complex128 {
	return complex(math.Log(Abs(x)), Phase(x))
}

// Log10 returns the decimal logarithm of x.

// Log10 返回 x 的十进制对数。
func Log10(x complex128) complex128 {
	return math.Log10E * Log(x)
}
