// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/s_log1p.c
// and came with this notice.  The go code is a simplified
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
//
// double log1p(double x)
//
// Method :
//   1. Argument Reduction: find k and f such that
//                      1+x = 2**k * (1+f),
//         where  sqrt(2)/2 < 1+f < sqrt(2) .
//
//      Note. If k=0, then f=x is exact. However, if k!=0, then f
//      may not be representable exactly. In that case, a correction
//      term is need. Let u=1+x rounded. Let c = (1+x)-u, then
//      log(1+x) - log(u) ~ c/u. Thus, we proceed to compute log(u),
//      and add back the correction term c/u.
//      (Note: when x > 2**53, one can simply return log(x))
//
//   2. Approximation of log1p(f).
//      Let s = f/(2+f) ; based on log(1+f) = log(1+s) - log(1-s)
//               = 2s + 2/3 s**3 + 2/5 s**5 + .....,
//               = 2s + s*R
//      We use a special Reme algorithm on [0,0.1716] to generate
//      a polynomial of degree 14 to approximate R The maximum error
//      of this polynomial approximation is bounded by 2**-58.45. In
//      other words,
//                      2      4      6      8      10      12      14
//          R(z) ~ Lp1*s +Lp2*s +Lp3*s +Lp4*s +Lp5*s  +Lp6*s  +Lp7*s
//      (the values of Lp1 to Lp7 are listed in the program)
//      and
//          |      2          14          |     -58.45
//          | Lp1*s +...+Lp7*s    -  R(z) | <= 2
//          |                             |
//      Note that 2s = f - s*f = f - hfsq + s*hfsq, where hfsq = f*f/2.
//      In order to guarantee error in log below 1ulp, we compute log
//      by
//              log1p(f) = f - (hfsq - s*(hfsq+R)).
//
//   3. Finally, log1p(x) = k*ln2 + log1p(f).
//                        = k*ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*ln2_lo)))
//      Here ln2 is split into two floating point number:
//                   ln2_hi + ln2_lo,
//      where n*ln2_hi is always exact for |n| < 2000.
//
// Special cases:
//      log1p(x) is NaN with signal if x < -1 (including -INF) ;
//      log1p(+INF) is +INF; log1p(-1) is -INF with signal;
//      log1p(NaN) is that NaN with no signal.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.
//
// Note: Assuming log() return accurate answer, the following
//       algorithm can be used to compute log1p(x) to within a few ULP:
//
//              u = 1+x;
//              if(u==1.0) return x ; else
//                         return log(u)*(x/(u-1.0));
//
//       See HP-15C Advanced Functions Handbook, p.193.

// 原始C代码、详细注释、下面的常量以及此通知来自
// FreeBSD 的 /usr/src/lib/msun/src/s_log1p.c 文件。
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
// double log1p(double x)
//
// 方法：
//   1. 实参转换：
//      寻找 k 和 f 使得
//          1+x = 2**k * (1+f)，
//      其中 sqrt(2)/2 < 1+f < sqrt(2)。
//
//      注：若 k=0，则 f=x 是精确的。然而，若 k!=0，那么 f 可能无法精确地表示。
//      这种情况下，需要一个修正项。设 u=1+x 的近似值，c = (1+x)-u，那么
//      log(1+x) - log(u) ~ c/u。我们据此可计算 log(u)，并加上修正项 c/u。
//      （注：当 x > 2**53 时，该函数可简单地返回 log(x)）
//
//   2. log1p(f) 的逼近式：
//      设 s = f/(2+f)，基于 log(1+f) = log(1+s) - log(1-s)，我们有
//           = 2s + 2/3 s**3 + 2/5 s**5 + .....，
//           = 2s + s*R
//      我们在 [0,0.1716] 上使用特殊的雷默算法来生成14阶的多项式，以此来逼近 R。
//      此多项逼近式的最大误差以 2**-58.45 为界。换句话说，
//                      2      4      6      8      10      12      14
//          R(z) ~ Lp1*s +Lp2*s +Lp3*s +Lp4*s +Lp5*s  +Lp6*s  +Lp7*s
//         （Lp1 至 Lp7 的值已在程序中列出）
//      且
//          |      2          14       |     -58.45
//          | Lp1*s +...+Lp7*s  - R(z) | <= 2
//          |                          |
//      注意 2s = f - s*f = f - hfsq + s*hfsq，其中 hfsq = f*f/2。
//      为确保 log 中的误差小于 1ulp，我们通过下式来计算 log：
//              log1p(f) = f - (hfsq - s*(hfsq+R))
//
//   3. 最后，
//           log1p(x) = k*ln2 + log1p(f)
//                    = k*ln2_hi+(f-(hfsq-(s*(hfsq+R)+k*ln2_lo)))
//      此处 ln2 被分为两个浮点数：
//           ln2_hi + ln2_lo
//      其中对于 |n| < 2000，n*ln2_hi 总是精确的。
//
// 特殊情况：
//      若 x < -1（包括 -INF），则
//         log1p(x)    为带符号 NaN；
//         log1p(+INF) 为 +INF；
//         log1p(-1)   为 -INF；
//         log1p(NaN)  为无符号 NaN。
//
// 精度：
//      取决于误差分析，误差总是小于 1 ulp（末位单元）。
//
//（后文信息只与C源码相关，故不作翻译。）

// Log1p returns the natural logarithm of 1 plus its argument x.
// It is more accurate than Log(1 + x) when x is near zero.
//
// Special cases are:
//	Log1p(+Inf) = +Inf
//	Log1p(±0) = ±0
//	Log1p(-1) = -Inf
//	Log1p(x < -1) = NaN
//	Log1p(NaN) = NaN

// Log1p 返回 1 加其实参 x 的自然对数。
// 当 x 接近 0 时，该函数比 Log(1 + x) 精确。
//
// 特殊情况为：
//	Log1p(+Inf)   = +Inf
//	Log1p(±0)     = ±0
//	Log1p(-1)     = -Inf
//	Log1p(x < -1) = NaN
//	Log1p(NaN)    = NaN
func Log1p(x float64) float64

func log1p(x float64) float64 {
	const (
		Sqrt2M1     = 4.142135623730950488017e-01  // Sqrt(2)-1 = 0x3fda827999fcef34
		Sqrt2HalfM1 = -2.928932188134524755992e-01 // Sqrt(2)/2-1 = 0xbfd2bec333018866
		Small       = 1.0 / (1 << 29)              // 2**-29 = 0x3e20000000000000
		Tiny        = 1.0 / (1 << 54)              // 2**-54
		Two53       = 1 << 53                      // 2**53
		Ln2Hi       = 6.93147180369123816490e-01   // 3fe62e42fee00000
		Ln2Lo       = 1.90821492927058770002e-10   // 3dea39ef35793c76
		Lp1         = 6.666666666666735130e-01     // 3FE5555555555593
		Lp2         = 3.999999999940941908e-01     // 3FD999999997FA04
		Lp3         = 2.857142874366239149e-01     // 3FD2492494229359
		Lp4         = 2.222219843214978396e-01     // 3FCC71C51D8E78AF
		Lp5         = 1.818357216161805012e-01     // 3FC7466496CB03DE
		Lp6         = 1.531383769920937332e-01     // 3FC39A09D078C69F
		Lp7         = 1.479819860511658591e-01     // 3FC2F112DF3E5244
	)

	// special cases
	// 特殊情况
	switch {
	// 包括 -Inf
	case x < -1 || IsNaN(x): // includes -Inf
		return NaN()
	case x == -1:
		return Inf(-1)
	case IsInf(x, 1):
		return Inf(1)
	}

	absx := x
	if absx < 0 {
		absx = -absx
	}

	var f float64
	var iu uint64
	k := 1
	if absx < Sqrt2M1 { //  |x| < Sqrt(2)-1
		if absx < Small { // |x| < 2**-29
			if absx < Tiny { // |x| < 2**-54
				return x
			}
			return x - x*x*0.5
		}
		if x > Sqrt2HalfM1 { // Sqrt(2)/2-1 < x
			// (Sqrt(2)/2-1) < x < (Sqrt(2)-1)
			k = 0
			f = x
			iu = 1
		}
	}
	var c float64
	if k != 0 {
		var u float64
		if absx < Two53 { // 1<<53
			u = 1.0 + x
			iu = Float64bits(u)
			k = int((iu >> 52) - 1023)
			if k > 0 {
				c = 1.0 - (u - x)
			} else {
				// 修正项
				c = x - (u - 1.0) // correction term
				c /= u
			}
		} else {
			u = x
			iu = Float64bits(u)
			k = int((iu >> 52) - 1023)
			c = 0
		}
		iu &= 0x000fffffffffffff
		// Sqrt(2) 尾数
		if iu < 0x0006a09e667f3bcd { // mantissa of Sqrt(2)
			u = Float64frombits(iu | 0x3ff0000000000000) // normalize u // 规范化 u
		} else {
			k += 1
			u = Float64frombits(iu | 0x3fe0000000000000) // normalize u/2 // 规范化 u/2
			iu = (0x0010000000000000 - iu) >> 2
		}
		f = u - 1.0 // Sqrt(2)/2 < u < Sqrt(2)
	}
	hfsq := 0.5 * f * f
	var s, R, z float64
	if iu == 0 { // |f| < 2**-20
		if f == 0 {
			if k == 0 {
				return 0
			} else {
				c += float64(k) * Ln2Lo
				return float64(k)*Ln2Hi + c
			}
		}
		// 避免除法
		R = hfsq * (1.0 - 0.66666666666666666*f) // avoid division
		if k == 0 {
			return f - R
		}
		return float64(k)*Ln2Hi - ((R - (float64(k)*Ln2Lo + c)) - f)
	}
	s = f / (2.0 + f)
	z = s * s
	R = z * (Lp1 + z*(Lp2+z*(Lp3+z*(Lp4+z*(Lp5+z*(Lp6+z*Lp7))))))
	if k == 0 {
		return f - (hfsq - s*(hfsq+R))
	}
	return float64(k)*Ln2Hi - ((hfsq - (s*(hfsq+R) + (float64(k)*Ln2Lo + c))) - f)
}
