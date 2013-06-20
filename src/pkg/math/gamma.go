// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from http://netlib.sandia.gov/cephes/cprob/gamma.c.
// The go code is a simplified version of the original C.
//
//      tgamma.c
//
//      Gamma function
//
// SYNOPSIS:
//
// double x, y, tgamma();
// extern int signgam;
//
// y = tgamma( x );
//
// DESCRIPTION:
//
// Returns gamma function of the argument.  The result is
// correctly signed, and the sign (+1 or -1) is also
// returned in a global (extern) variable named signgam.
// This variable is also filled in by the logarithmic gamma
// function lgamma().
//
// Arguments |x| <= 34 are reduced by recurrence and the function
// approximated by a rational function of degree 6/7 in the
// interval (2,3).  Large arguments are handled by Stirling's
// formula. Large negative arguments are made positive using
// a reflection formula.
//
// ACCURACY:
//
//                      Relative error:
// arithmetic   domain     # trials      peak         rms
//    DEC      -34, 34      10000       1.3e-16     2.5e-17
//    IEEE    -170,-33      20000       2.3e-15     3.3e-16
//    IEEE     -33,  33     20000       9.4e-16     2.2e-16
//    IEEE      33, 171.6   20000       2.3e-15     3.2e-16
//
// Error for arguments outside the test range will be larger
// owing to error amplification by the exponential function.
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
// http://netlib.sandia.gov/cephes/cprob/gamma.c
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
//      tgamma.c
//
//      伽马函数
//
// 概览：
//
// double x, y, tgamma();
// extern int signgam;
//
// y = tgamma( x );
//
// 描述：
//
// 返回实参的伽马函数。其结果能正确地处理正负，且其符号（+1 或 -1）也能以名为
// signgam 的全局（外部）变量返回。该变量亦可由对数伽马函数 lgamma() 填充。
//
// 实参 |x| <= 34 可通过循环转换，此函数也可通过一个 6/7 阶有理函数在区间 (2,3)
// 内逼近。大数实参可通过斯特灵公式处理。大负数实参可通过反射公式变为正数。
//
// 精度：
//
//                      相对误差：
//    算法     范围         测试次数    峰值       均方根
//    DEC     -34, 34      10000      1.3e-16   2.5e-17
//    IEEE   -170,-33      20000      2.3e-15   3.3e-16
//    IEEE    -33, 33      20000      9.4e-16   2.2e-16
//    IEEE     33, 171.6   20000      2.3e-15   3.2e-16
//
// 测试范围之外的实参误差将会大于被指数函数放大的误差。

var _gamP = [...]float64{
	1.60119522476751861407e-04,
	1.19135147006586384913e-03,
	1.04213797561761569935e-02,
	4.76367800457137231464e-02,
	2.07448227648435975150e-01,
	4.94214826801497100753e-01,
	9.99999999999999996796e-01,
}
var _gamQ = [...]float64{
	-2.31581873324120129819e-05,
	5.39605580493303397842e-04,
	-4.45641913851797240494e-03,
	1.18139785222060435552e-02,
	3.58236398605498653373e-02,
	-2.34591795718243348568e-01,
	7.14304917030273074085e-02,
	1.00000000000000000320e+00,
}
var _gamS = [...]float64{
	7.87311395793093628397e-04,
	-2.29549961613378126380e-04,
	-2.68132617805781232825e-03,
	3.47222221605458667310e-03,
	8.33333333333482257126e-02,
}

// Gamma function computed by Stirling's formula.
// The polynomial is valid for 33 <= x <= 172.

// 伽马函数通过斯特灵公式计算。
// 此多项式只对 33 <= x <= 172 有效。
func stirling(x float64) float64 {
	const (
		SqrtTwoPi   = 2.506628274631000502417
		MaxStirling = 143.01608
	)
	w := 1 / x
	w = 1 + w*((((_gamS[0]*w+_gamS[1])*w+_gamS[2])*w+_gamS[3])*w+_gamS[4])
	y := Exp(x)
	// 避免 Pow() 溢出
	if x > MaxStirling { // avoid Pow() overflow
		v := Pow(x, 0.5*x-0.25)
		y = v * (v / y)
	} else {
		y = Pow(x, x-0.5) / y
	}
	y = SqrtTwoPi * y * w
	return y
}

// Gamma returns the Gamma function of x.
//
// Special cases are:
//	Gamma(+Inf) = +Inf
//	Gamma(+0) = +Inf
//	Gamma(-0) = -Inf
//	Gamma(x) = NaN for integer x < 0
//	Gamma(-Inf) = NaN
//	Gamma(NaN) = NaN

// Gamma 返回 x 的伽马函数。
//
// 特殊情况为：
//	Gamma(+Inf) = +Inf
//	Gamma(+0)   = +Inf
//	Gamma(-0)   = -Inf
//	Gamma(x)    = NaN（对于整数 x < 0）
//	Gamma(-Inf) = NaN
//	Gamma(NaN)  = NaN
func Gamma(x float64) float64 {
	const Euler = 0.57721566490153286060651209008240243104215933593992 // A001620
	// special cases
	// 特殊情况
	switch {
	case isNegInt(x) || IsInf(x, -1) || IsNaN(x):
		return NaN()
	case x == 0:
		if Signbit(x) {
			return Inf(-1)
		}
		return Inf(1)
	case x < -170.5674972726612 || x > 171.61447887182298:
		return Inf(1)
	}
	q := Abs(x)
	p := Floor(q)
	if q > 33 {
		if x >= 0 {
			return stirling(x)
		}
		signgam := 1
		if ip := int(p); ip&1 == 0 {
			signgam = -1
		}
		z := q - p
		if z > 0.5 {
			p = p + 1
			z = q - p
		}
		z = q * Sin(Pi*z)
		if z == 0 {
			return Inf(signgam)
		}
		z = Pi / (Abs(z) * stirling(q))
		return float64(signgam) * z
	}

	// Reduce argument
	// 转换实参
	z := 1.0
	for x >= 3 {
		x = x - 1
		z = z * x
	}
	for x < 0 {
		if x > -1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}
	for x < 2 {
		if x < 1e-09 {
			goto small
		}
		z = z / x
		x = x + 1
	}

	if x == 2 {
		return z
	}

	x = x - 2
	p = (((((x*_gamP[0]+_gamP[1])*x+_gamP[2])*x+_gamP[3])*x+_gamP[4])*x+_gamP[5])*x + _gamP[6]
	q = ((((((x*_gamQ[0]+_gamQ[1])*x+_gamQ[2])*x+_gamQ[3])*x+_gamQ[4])*x+_gamQ[5])*x+_gamQ[6])*x + _gamQ[7]
	return z * p / q

small:
	if x == 0 {
		return Inf(1)
	}
	return z / ((1 + Euler*x) * x)
}

func isNegInt(x float64) bool {
	if x < 0 {
		_, xf := Modf(x)
		return xf == 0
	}
	return false
}
