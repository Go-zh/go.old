// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point logarithm.
*/

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_log.c
// and came with this notice.  The go code is a simpler
// version of the original C.
//
// ====================================================
// Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.
//
// Developed at SunPro, a Sun Microsystems, Inc. business.
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
// __ieee754_log(x)
// Return the logarithm of x
//
// Method :
//   1. Argument Reduction: find k and f such that
//          x = 2**k * (1+f),
//     where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//   2. Approximation of log(1+f).
//  Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//       = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//           = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//  a polynomial of degree 14 to approximate R.  The maximum error
//  of this polynomial approximation is bounded by 2**-58.45. In
//  other words,
//              2      4      6      8      10      12      14
//      R(z) ~ L1*s +L2*s +L3*s +L4*s +L5*s  +L6*s  +L7*s
//  (the values of L1 to L7 are listed in the program) and
//      |      2          14          |     -58.45
//      | L1*s +...+L7*s    -  R(z) | <= 2
//      |                             |
//  Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//  In order to guarantee error in log below 1ulp, we compute log by
//      log(1+f) = f - s*(f - R)        (if f is not too large)
//      log(1+f) = f - (hfsq - s*(hfsq+R)). (better accuracy)
//
//  3. Finally,  log(x) = k*Ln2 + log(1+f).
//              = k*Ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*Ln2_lo)))
//     Here Ln2 is split into two floating point number:
//          Ln2_hi + Ln2_lo,
//     where n*Ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//  log(x) is NaN with signal if x < 0 (including -INF) ;
//  log(+INF) is +INF; log(0) is -INF with signal;
//  log(NaN) is that NaN with no signal.
//
// Accuracy:
//  according to an error analysis, the error is always less than
//  1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.

// 原始C代码、详细注释、下面的常量以及此通知来自
// FreeBSD 的 /usr/src/lib/msun/src/e_log.c 文件。
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
// __ieee754_log(x)
// 返回 x 的对数
//
// 方法：
//   1. 实参转换：
//      寻找 k 和 f 使得
//          x = 2**k * (1+f)，
//      其中 sqrt(2)/2 < 1+f < sqrt(2)。
//
//   2. log(1+f) 的逼近式。
//      设 s = f/(2+f)，基于 log(1+f) = log(1+s) - log(1-s)，我们有
//         s = 2s + 2/3 s**3 + 2/5 s**5 + ...
//           = 2s + s*R
//      我们在 [0, 0.1716] 中使用了特殊的雷默算法，生成14阶多项式来逼近 R。
//      此多项式近似值的最大误差临界于 2**-58.45。换句话说，
//
//                      2     4     6     8     10     12     14
//           R(z) ~ L1*s +L2*s +L3*s +L4*s +L5*s  +L6*s  +L7*s
//                     （L1 至 L7 的值已在程序中列出）
//      且
//           |     2         14       |     -58.45
//           | L1*s +...+L7*s  - R(z) | <= 2
//           |                        |
//
//      注意 2s = f - s*f = f - hfsq + s*hfsq，其中 hfsq = f*f/2。
//      为确保 log 的误差小于 1ulp，我们通过下式来计算 log：
//      若 f 不算太大，就采用
//           log(1+f) = f - s*(f - R)，
//      若需更高的精度，则采用
//           log(1+f) = f - (hfsq - s*(hfsq+R))。
//
//   3. 最后，
//           log(x) = k*Ln2 + log(1+f)
//                  = k*Ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*Ln2_lo)))
//      此处 Ln2 被分为两个浮点数：
//           Ln2_hi + Ln2_lo，
//      其中对于 |n| < 2000，n*Ln2_hi 总是精确的。
//
// 特殊情况：
//      若 x < 0（包括 -INF），则
//         log(x)    为带信号  NaN；
//         log(+INF) 为      +INF；
//         log(0)    为带信号 -INF；
//         log(NaN)  为无信号  NaN。
//
// 精度：
//      取决于误差分析，误差总是小于 1 ulp（末位单元）。
//
//（后文信息只与C源码相关，故不作翻译。）

// Log returns the natural logarithm of x.
//
// Special cases are:
//  Log(+Inf) = +Inf
//  Log(0) = -Inf
//  Log(x < 0) = NaN
//  Log(NaN) = NaN

// Log 返回 x 的自然对数。
//
// 特殊情况为
//	Log(+Inf)  = +Inf
//	Log(0)     = -Inf
//	Log(x < 0) = NaN
//	Log(NaN)   = NaN
func Log(x float64) float64

func log(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01 /* 3fe62e42 fee00000 */
		Ln2Lo = 1.90821492927058770002e-10 /* 3dea39ef 35793c76 */
		L1    = 6.666666666666735130e-01   /* 3FE55555 55555593 */
		L2    = 3.999999999940941908e-01   /* 3FD99999 9997FA04 */
		L3    = 2.857142874366239149e-01   /* 3FD24924 94229359 */
		L4    = 2.222219843214978396e-01   /* 3FCC71C5 1D8E78AF */
		L5    = 1.818357216161805012e-01   /* 3FC74664 96CB03DE */
		L6    = 1.531383769920937332e-01   /* 3FC39A09 D078C69F */
		L7    = 1.479819860511658591e-01   /* 3FC2F112 DF3E5244 */
	)

	// special cases
	// 特殊情况
	switch {
	case IsNaN(x) || IsInf(x, 1):
		return x
	case x < 0:
		return NaN()
	case x == 0:
		return Inf(-1)
	}

	// reduce
	// 转换
	f1, ki := Frexp(x)
	if f1 < Sqrt2/2 {
		f1 *= 2
		ki--
	}
	f := f1 - 1
	k := float64(ki)

	// compute
	// 计算
	s := f / (2 + f)
	s2 := s * s
	s4 := s2 * s2
	t1 := s2 * (L1 + s4*(L3+s4*(L5+s4*L7)))
	t2 := s4 * (L2 + s4*(L4+s4*L6))
	R := t1 + t2
	hfsq := 0.5 * f * f
	return k*Ln2Hi - ((hfsq - (s*(hfsq+R) + k*Ln2Lo)) - f)
}
