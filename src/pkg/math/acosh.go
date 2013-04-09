// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code, the long comment, and the constants
// below are from FreeBSD's /usr/src/lib/msun/src/e_acosh.c
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
// __ieee754_acosh(x)
// Method :
//	Based on
//	        acosh(x) = log [ x + sqrt(x*x-1) ]
//	we have
//	        acosh(x) := log(x)+ln2,	if x is large; else
//	        acosh(x) := log(2x-1/(sqrt(x*x-1)+x)) if x>2; else
//	        acosh(x) := log1p(t+sqrt(2.0*t+t*t)); where t=x-1.
//
// Special cases:
//	acosh(x) is NaN with signal if x<1.
//	acosh(NaN) is NaN without signal.
//

// 原始C代码、详细注释、下面的常量以及此通知来自
// FreeBSD 的 /usr/src/lib/msun/src/e_acosh.c 文件。
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
// __ieee754_acosh(x)
// 方法：
//	基于
//	              acosh(x) = log [ x + sqrt(x*x-1) ]
//	我们有
//	若 x 很大，则 acosh(x) := log(x)+ln2；               否则
//	若 x>2，   则 acosh(x) := log(2x-1/(sqrt(x*x-1)+x))；否则
//	              acosh(x) := log1p(t+sqrt(2.0*t+t*t))； 其中 t=x-1。
//
// 特殊情况：
//	若 x<1，则 acosh(x)   为带符号 NaN。
//	           acosh(NaN) 为无符号 NaN。
//

// Acosh returns the inverse hyperbolic cosine of x.
//
// Special cases are:
//	Acosh(+Inf) = +Inf
//	Acosh(x) = NaN if x < 1
//	Acosh(NaN) = NaN

// Acosh 返回 x 的反双曲余弦值。
//
// 特殊情况为：
//	Acosh(+Inf) = +Inf
//	Acosh(x)    = NaN（若 x < 1）
//	Acosh(NaN)  = NaN
func Acosh(x float64) float64 {
	const (
		Ln2   = 6.93147180559945286227e-01 // 0x3FE62E42FEFA39EF
		Large = 1 << 28                    // 2**28
	)
	// first case is special case
	// 第一种情况为特殊情况
	switch {
	case x < 1 || IsNaN(x):
		return NaN()
	case x == 1:
		return 0
	case x >= Large:
		return Log(x) + Ln2 // x > 2**28
	case x > 2:
		return Log(2*x - 1/(x+Sqrt(x*x-1))) // 2**28 > x > 2
	}
	t := x - 1
	return Log1p(t + Sqrt(2*t+t*t)) // 2 >= x > 1
}
