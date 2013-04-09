// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_exp.c
// and came with this notice.  The go code is a simplified
// version of the original C.
//
// ====================================================
// Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved.
//
// Permission to use, copy, modify, and distribute this
// software is freely granted, provided that this notice
// is preserved.
// ====================================================
//
//
// exp(x)
// Returns the exponential of x.
//
// Method
//   1. Argument reduction:
//      Reduce x to an r so that |r| <= 0.5*ln2 ~ 0.34658.
//      Given x, find r and integer k such that
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2.
//
//      Here r will be represented as r = hi-lo for better
//      accuracy.
//
//   2. Approximation of exp(r) by a special rational function on
//      the interval [0,0.34658]:
//      Write
//          R(r**2) = r*(exp(r)+1)/(exp(r)-1) = 2 + r*r/6 - r**4/360 + ...
//      We use a special Remes algorithm on [0,0.34658] to generate
//      a polynomial of degree 5 to approximate R. The maximum error
//      of this polynomial approximation is bounded by 2**-59. In
//      other words,
//          R(z) ~ 2.0 + P1*z + P2*z**2 + P3*z**3 + P4*z**4 + P5*z**5
//      (where z=r*r, and the values of P1 to P5 are listed below)
//      and
//          |                  5          |     -59
//          | 2.0+P1*z+...+P5*z   -  R(z) | <= 2
//          |                             |
//      The computation of exp(r) thus becomes
//                             2*r
//              exp(r) = 1 + -------
//                            R - r
//                                 r*R1(r)
//                     = 1 + r + ----------- (for better accuracy)
//                                2 - R1(r)
//      where
//                               2       4             10
//              R1(r) = r - (P1*r  + P2*r  + ... + P5*r   ).
//
//   3. Scale back to obtain exp(x):
//      From step 1, we have
//         exp(x) = 2**k * exp(r)
//
// Special cases:
//      exp(INF) is INF, exp(NaN) is NaN;
//      exp(-INF) is 0, and
//      for finite argument, only exp(0)=1 is exact.
//
// Accuracy:
//      according to an error analysis, the error is always less than
//      1 ulp (unit in the last place).
//
// Misc. info.
//      For IEEE double
//          if x >  7.09782712893383973096e+02 then exp(x) overflow
//          if x < -7.45133219101941108420e+02 then exp(x) underflow
//
// Constants:
// The hexadecimal values are the intended ones for the following
// constants. The decimal values may be used, provided that the
// compiler will convert from decimal to binary accurately enough
// to produce the hexadecimal values shown.

// 原始C代码、详细注释、下面的常量以及此通知来自
// FreeBSD 的 /usr/src/lib/msun/src/e_exp.c 文件。
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
// exp(x)
// 返回 x 的指数。
//
// 方法：
//   1. 实参转换：
//      将 x 转换为 r，使得 |r| <= 0.5*ln2 ~ 0.34658。
//      给定 x，寻找 r 以及整数 k 使得
//
//               x = k*ln2 + r,  |r| <= 0.5*ln2。
//
//      为了更高的精度，这里的 r 将被表示为 r = hi-lo。
//
//   2. exp(r) 的近似值由特殊的有理函数在区间 [0,0.34658] 上计算：
//      写作
//          R(r**2) = r*(exp(r)+1)/(exp(r)-1) = 2 + r*r/6 - r**4/360 + ...
//      我们在 [0,0.34658] 上用特殊的Remes算法来生成5度的多项式，以逼近 R。
//      该多项式近似值的最大误差以 2**-59 为界。
//      换言之，即
//          R(z) ~ 2.0 + P1*z + P2*z**2 + P3*z**3 + P4*z**4 + P5*z**5
//              （其中 z=r*r，P1 至 P5 的值将在后面列出。）
//      且
//          |                  5          |     -59
//          | 2.0+P1*z+...+P5*z   -  R(z) | <= 2
//          |                             |
//      因此，对 exp(r) 的计算可转换为
//                             2*r
//              exp(r) = 1 + -------
//                            R - r
//                                 r*R1(r)
//                     = 1 + r + ----------- （用于更高的精度）
//                                2 - R1(r)
//      其中
//                               2       4             10
//              R1(r) = r - (P1*r  + P2*r  + ... + P5*r   )。
//
//   3. 按比例缩减以获得 exp(x)：
//      根据步骤1，我们有
//         exp(x) = 2**k * exp(r)
//
// 特殊情况：
//      exp(INF) 为 INF，exp(NaN) 为 NaN；
//      exp(-INF) 为 0，且
//      对于有限的实参，只有 exp(0)=1 是精确的。
//
// 精度：
//      取决于误差分析，其误差总是小于1 ulp（末位单元）。
//
// 其它信息：
//      对于IEEE双精度浮点数
//          若 x >  7.09782712893383973096e+02 则 exp(x) 向上溢出
//          若 x < -7.45133219101941108420e+02 则 exp(x) 向下溢出
//
//（后文信息只与C源码相关，故不作翻译。）

// Exp returns e**x, the base-e exponential of x.
//
// Special cases are:
//	Exp(+Inf) = +Inf
//	Exp(NaN) = NaN
// Very large values overflow to 0 or +Inf.
// Very small values underflow to 1.

// Exp 返回 e**x，即以 e 为底的 x 次幂。
//
// 特殊情况为：
//	Exp(+Inf) = +Inf
//	Exp(NaN)  = NaN
// 非常大的数会向上溢出为 0 或 +Inf。
// 非常小的数会向下溢出为 1。
func Exp(x float64) float64

func exp(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01
		Ln2Lo = 1.90821492927058770002e-10
		Log2e = 1.44269504088896338700e+00

		Overflow  = 7.09782712893383973096e+02
		Underflow = -7.45133219101941108420e+02
		NearZero  = 1.0 / (1 << 28) // 2**-28
	)

	// special cases
	// 特殊情况
	switch {
	case IsNaN(x) || IsInf(x, 1):
		return x
	case IsInf(x, -1):
		return 0
	case x > Overflow:
		return Inf(1)
	case x < Underflow:
		return 0
	case -NearZero < x && x < NearZero:
		return 1 + x
	}

	// reduce; computed as r = hi - lo for extra precision.
	// 分解；通过 r = hi - lo 计算来获取额外的精度。
	var k int
	switch {
	case x < 0:
		k = int(Log2e*x - 0.5)
	case x > 0:
		k = int(Log2e*x + 0.5)
	}
	hi := x - float64(k)*Ln2Hi
	lo := float64(k) * Ln2Lo

	// compute
	// 计算
	return expmulti(hi, lo, k)
}

// Exp2 returns 2**x, the base-2 exponential of x.
//
// Special cases are the same as Exp.

// Exp2 返回 2**x，即以 2 为底的 x 次指数。
//
// 特殊情况与 Exp 相同。
func Exp2(x float64) float64

func exp2(x float64) float64 {
	const (
		Ln2Hi = 6.93147180369123816490e-01
		Ln2Lo = 1.90821492927058770002e-10

		Overflow  = 1.0239999999999999e+03
		Underflow = -1.0740e+03
	)

	// special cases
	// 特殊情况
	switch {
	case IsNaN(x) || IsInf(x, 1):
		return x
	case IsInf(x, -1):
		return 0
	case x > Overflow:
		return Inf(1)
	case x < Underflow:
		return 0
	}

	// argument reduction; x = r×lg(e) + k with |r| ≤ ln(2)/2.
	// computed as r = hi - lo for extra precision.

	// 实参转换；x = r×lg(e) + k 其中 |r| ≤ ln(2)/2。
	// 通过 r = hi - lo 计算来获取额外的精度。
	var k int
	switch {
	case x > 0:
		k = int(x + 0.5)
	case x < 0:
		k = int(x - 0.5)
	}
	t := x - float64(k)
	hi := t * Ln2Hi
	lo := -t * Ln2Lo

	// compute
	// 计算
	return expmulti(hi, lo, k)
}

// exp1 returns e**r × 2**k where r = hi - lo and |r| ≤ ln(2)/2.

// exp1 返回 e**r × 2**k，其中 r = hi - lo 且 |r| ≤ ln(2)/2。
func expmulti(hi, lo float64, k int) float64 {
	const (
		P1 = 1.66666666666666019037e-01  /* 0x3FC55555; 0x5555553E */
		P2 = -2.77777777770155933842e-03 /* 0xBF66C16C; 0x16BEBD93 */
		P3 = 6.61375632143793436117e-05  /* 0x3F11566A; 0xAF25DE2C */
		P4 = -1.65339022054652515390e-06 /* 0xBEBBBD41; 0xC5D26BF1 */
		P5 = 4.13813679705723846039e-08  /* 0x3E663769; 0x72BEA4D0 */
	)

	r := hi - lo
	t := r * r
	c := r - t*(P1+t*(P2+t*(P3+t*(P4+t*P5))))
	y := 1 - ((lo - (r*c)/(2-c)) - hi)
	// TODO(rsc): make sure Ldexp can handle boundary k
	// TODO(rsc): 确认 Ldexp 可处理临界值 k
	return Ldexp(y, k)
}
