// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point tangent.
*/
/*
	浮点数正切。
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
//      tan.c
//
//      Circular tangent
//
// SYNOPSIS:
//
// double x, y, tan();
// y = tan( x );
//
// DESCRIPTION:
//
// Returns the circular tangent of the radian argument x.
//
// Range reduction is modulo pi/4.  A rational function
//       x + x**3 P(x**2)/Q(x**2)
// is employed in the basic interval [0, pi/4].
//
// ACCURACY:
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC      +-1.07e9      44000      4.1e-17     1.0e-17
//    IEEE     +-1.07e9      30000      2.9e-16     8.1e-17
//
// Partial loss of accuracy begins to occur at x = 2**30 = 1.074e9.  The loss
// is not gradual, but jumps suddenly to about 1 part in 10e7.  Results may
// be meaningless for x > 2**49 = 5.6e14.
// [Accuracy loss statement from sin.go comments.]
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

// 原始C代码、详细注释、下面的常量以及此通知来自
// http://netlib.sandia.gov/cephes/cmath/sin.c 文件，可从
// http://www.netlib.org/cephes/cmath.tgz 处获取。
// 此Go代码为原始C代码的简化版本。
//
//      tan.c
//
//      圆形正切
//
// 概览：
//
// double x, y, tan();
// y = tan( x );
//
// 描述：
//
// 返回弧度参数 x 的圆形正切。
//
// 范围换算以 pi/4 取模。有理函数
//       x + x**3 P(x**2)/Q(x**2)
// 在基本区间 [0, pi/4] 中被采用。
//
// 精度:
//                        相对误差:
//    算法     范围      # 测试次数  峰值     均方根
//    DEC      +-1.07e9    44000     4.1e-17  1.0e-17
//    IEEE     +-1.07e9    30000     2.9e-16  8.1e-17
//
// 精度的部分损失源于 x = 2**30 = 1.074e9。该损失不是渐进的，而是突然跳到
// 10e7 中的一部分。对于 x > 2**49 = 5.6e14 来说，其结果可能没有意义。
// [精度损失在 sin.go 的注释中声明。]
//
//（版权声明见上。）

// tan coefficients
// 正切系数
var _tanP = [...]float64{
	-1.30936939181383777646E4, // 0xc0c992d8d24f3f38
	1.15351664838587416140E6,  // 0x413199eca5fc9ddd
	-1.79565251976484877988E7, // 0xc1711fead3299176
}
var _tanQ = [...]float64{
	1.00000000000000000000E0,
	1.36812963470692954678E4,  //0x40cab8a5eeb36572
	-1.32089234440210967447E6, //0xc13427bc582abc96
	2.50083801823357915839E7,  //0x4177d98fc2ead8ef
	-5.38695755929454629881E7, //0xc189afe03cbe5a31
}

// Tan returns the tangent of x.
//
// Special cases are:
//	Tan(±0) = ±0
//	Tan(±Inf) = NaN
//	Tan(NaN) = NaN

// Tan 返回 x 的正切值。
//
// 特殊情况为：
//	Tan(±0)   = ±0
//	Tan(±Inf) = NaN
//	Tan(NaN)  = NaN
func Tan(x float64) float64

func tan(x float64) float64 {
	const (
		// 将 Pi/4 分为三部分
		PI4A = 7.85398125648498535156E-1                             // 0x3fe921fb40000000, Pi/4 split into three parts
		PI4B = 3.77489470793079817668E-8                             // 0x3e64442d00000000,
		PI4C = 2.69515142907905952645E-15                            // 0x3ce8469898cc5170,
		M4PI = 1.273239544735162542821171882678754627704620361328125 // 4/pi
	)
	// special cases
	// 特殊情况
	switch {
	case x == 0 || IsNaN(x):
		return x // return ±0 || NaN()
	case IsInf(x, 0):
		return NaN()
	}

	// make argument positive but save the sign
	// 使实参变为整数，但保留符号
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}

	// x/(Pi/4) 的整数部分，作为整数以用于相位角的测试
	j := int64(x * M4PI) // integer part of x/(Pi/4), as integer for tests on the phase angle
	y := float64(j)      // integer part of x/(Pi/4), as float // x/(Pi/4) 的整数部分，作为浮点数

	/* map zeros and singularities to origin */
	// 将零和奇点映射为原点
	if j&1 == 1 {
		j += 1
		y += 1
	}

	z := ((x - y*PI4A) - y*PI4B) - y*PI4C
	zz := z * z

	if zz > 1e-14 {
		y = z + z*(zz*(((_tanP[0]*zz)+_tanP[1])*zz+_tanP[2])/((((zz+_tanQ[1])*zz+_tanQ[2])*zz+_tanQ[3])*zz+_tanQ[4]))
	} else {
		y = z
	}
	if j&2 == 2 {
		y = -1 / y
	}
	if sign {
		y = -y
	}
	return y
}
