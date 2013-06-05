// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point sine and cosine.
*/
/*
	浮点数正弦和余弦。
*/

// The original C code, the long comment, and the constants
// below were from http://netlib.sandia.gov/cephes/cmath/sin.c,
// available from http://www.netlib.org/cephes/cmath.tgz.
// The go code is a simplified version of the original C.
//
//      sin.c
//
//      Circular sine
//
// SYNOPSIS:
//
// double x, y, sin();
// y = sin( x );
//
// DESCRIPTION:
//
// Range reduction is into intervals of pi/4.  The reduction error is nearly
// eliminated by contriving an extended precision modular arithmetic.
//
// Two polynomial approximating functions are employed.
// Between 0 and pi/4 the sine is approximated by
//      x  +  x**3 P(x**2).
// Between pi/4 and pi/2 the cosine is represented as
//      1  -  x**2 Q(x**2).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain      # trials      peak         rms
//    DEC       0, 10       150000       3.0e-17     7.8e-18
//    IEEE -1.07e9,+1.07e9  130000       2.1e-16     5.4e-17
//
// Partial loss of accuracy begins to occur at x = 2**30 = 1.074e9.  The loss
// is not gradual, but jumps suddenly to about 1 part in 10e7.  Results may
// be meaningless for x > 2**49 = 5.6e14.
//
//      cos.c
//
//      Circular cosine
//
// SYNOPSIS:
//
// double x, y, cos();
// y = cos( x );
//
// DESCRIPTION:
//
// Range reduction is into intervals of pi/4.  The reduction error is nearly
// eliminated by contriving an extended precision modular arithmetic.
//
// Two polynomial approximating functions are employed.
// Between 0 and pi/4 the cosine is approximated by
//      1  -  x**2 Q(x**2).
// Between pi/4 and pi/2 the sine is represented as
//      x  +  x**3 P(x**2).
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain      # trials      peak         rms
//    IEEE -1.07e9,+1.07e9  130000       2.1e-16     5.4e-17
//    DEC        0,+1.07e9   17000       3.0e-17     7.2e-18
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
//      sin.c
//
//      正弦
//
// 概览：
//
// double x, y, sin();
// y = sin( x );
//
// 描述：
//
// 将范围转换为 pi/4 的区间。转换误差通过设计高精度模块化运算几乎被消除了。
//
// 这里采用了两个多项式逼近函数。
// 在 0 和 pi/4 之间，其正弦逼近于
//      x + x**3 P(x**2)。
// 在 pi/4 和 pi/2 之间，其余弦表示为
//      1 - x**2 Q(x**2)。
//
// 精度:
//                        相对误差:
//    算法      范围      # 测试次数  峰值     均方根
//    DEC       0, 10       150000    3.0e-17  7.8e-18
//    IEEE -1.07e9,+1.07e9  130000    2.1e-16  5.4e-17
//
// 精度的部分损失源于 x = 2**30 = 1.074e9。该损失不是渐进的，而是突然跳到
// 10e7 中的一部分。对于 x > 2**49 = 5.6e14 来说，其结果可能没有意义。
//
//      cos.c
//
//      余弦
//
// 概览：
//
// double x, y, cos();
// y = cos( x );
//
// 描述：
//
// 将范围转换为 pi/4 的区间。转换误差通过设计高精度模块化运算几乎被消除了。
//
// 这里采用了两个多项式逼近函数。
// 在 0 和 pi/4 之间，其余弦逼近于
//      1  -  x**2 Q(x**2)。
// 在 pi/4 和 pi/2 之间，其正弦表示为
//      x  +  x**3 P(x**2)。
//
// 精度：
//
//                         相对误差:
//    算法      范围       # 测试次数  峰值     均方根
//    IEEE -1.07e9,+1.07e9   130000    2.1e-16  5.4e-17
//    DEC        0,+1.07e9   17000     3.0e-17  7.2e-18
//
//（版权声明见上。）

// sin coefficients
// 正弦系数
var _sin = [...]float64{
	1.58962301576546568060E-10, // 0x3de5d8fd1fd19ccd
	-2.50507477628578072866E-8, // 0xbe5ae5e5a9291f5d
	2.75573136213857245213E-6,  // 0x3ec71de3567d48a1
	-1.98412698295895385996E-4, // 0xbf2a01a019bfdf03
	8.33333333332211858878E-3,  // 0x3f8111111110f7d0
	-1.66666666666666307295E-1, // 0xbfc5555555555548
}

// cos coefficients
// 余弦系数
var _cos = [...]float64{
	-1.13585365213876817300E-11, // 0xbda8fa49a0861a9b
	2.08757008419747316778E-9,   // 0x3e21ee9d7b4e3f05
	-2.75573141792967388112E-7,  // 0xbe927e4f7eac4bc6
	2.48015872888517045348E-5,   // 0x3efa01a019c844f5
	-1.38888888888730564116E-3,  // 0xbf56c16c16c14f91
	4.16666666666665929218E-2,   // 0x3fa555555555554b
}

// Cos returns the cosine of x.
//
// Special cases are:
//	Cos(±Inf) = NaN
//	Cos(NaN) = NaN

// Cos 返回 x 的余弦值。
//
// 特殊情况为：
//	Cos(±Inf) = NaN
//	Cos(NaN)  = NaN
func Cos(x float64) float64

func cos(x float64) float64 {
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
	case IsNaN(x) || IsInf(x, 0):
		return NaN()
	}

	// make argument positive
	// 使实参变为整数
	sign := false
	if x < 0 {
		x = -x
	}
	// x/(Pi/4) 的整数部分，作为整数以用于相位角的测试
	j := int64(x * M4PI) // integer part of x/(Pi/4), as integer for tests on the phase angle
	y := float64(j)      // integer part of x/(Pi/4), as float // x/(Pi/4) 的整数部分，作为浮点数

	// map zeros to origin
	// 将零映射为原点
	if j&1 == 1 {
		j += 1
		y += 1
	}
	// 卦限以2π弧度取模（360度）
	j &= 7 // octant modulo 2Pi radians (360 degrees)
	if j > 3 {
		j -= 4
		sign = !sign
	}
	if j > 1 {
		sign = !sign
	}
	// 高精度模数运算
	z := ((x - y*PI4A) - y*PI4B) - y*PI4C // Extended precision modular arithmetic
	zz := z * z
	if j == 1 || j == 2 {
		y = z + z*zz*((((((_sin[0]*zz)+_sin[1])*zz+_sin[2])*zz+_sin[3])*zz+_sin[4])*zz+_sin[5])
	} else {
		y = 1.0 - 0.5*zz + zz*zz*((((((_cos[0]*zz)+_cos[1])*zz+_cos[2])*zz+_cos[3])*zz+_cos[4])*zz+_cos[5])
	}
	if sign {
		y = -y
	}
	return y
}

// Sin returns the sine of x.
//
// Special cases are:
//	Sin(±0) = ±0
//	Sin(±Inf) = NaN
//	Sin(NaN) = NaN

// Sin 返回 x 的正弦值。
//
// 特殊情况为：
//	Sin(±0)   = ±0
//	Sin(±Inf) = NaN
//	Sin(NaN)  = NaN
func Sin(x float64) float64

func sin(x float64) float64 {
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
		return x // return ±0 || NaN() // 返回 ±0 || NaN()
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

	// map zeros to origin
	// 将零映射为原点
	if j&1 == 1 {
		j += 1
		y += 1
	}
	// 卦限以2π弧度取模（360度）
	j &= 7 // octant modulo 2Pi radians (360 degrees)
	// reflect in x axis // 反映在 x 轴
	if j > 3 {
		sign = !sign
		j -= 4
	}

	// 高精度模数运算
	z := ((x - y*PI4A) - y*PI4B) - y*PI4C // Extended precision modular arithmetic
	zz := z * z
	if j == 1 || j == 2 {
		y = 1.0 - 0.5*zz + zz*zz*((((((_cos[0]*zz)+_cos[1])*zz+_cos[2])*zz+_cos[3])*zz+_cos[4])*zz+_cos[5])
	} else {
		y = z + z*zz*((((((_sin[0]*zz)+_sin[1])*zz+_sin[2])*zz+_sin[3])*zz+_sin[4])*zz+_sin[5])
	}
	if sign {
		y = -y
	}
	return y
}
