// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// The original C code and the long comment below are
// from FreeBSD's /usr/src/lib/msun/src/e_sqrt.c and
// came with this notice.  The go code is a simplified
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
// __ieee754_sqrt(x)
// Return correctly rounded sqrt.
//           -----------------------------------------
//           | Use the hardware sqrt if you have one |
//           -----------------------------------------
// Method:
//   Bit by bit method using integer arithmetic. (Slow, but portable)
//   1. Normalization
//      Scale x to y in [1,4) with even powers of 2:
//      find an integer k such that  1 <= (y=x*2**(2k)) < 4, then
//              sqrt(x) = 2**k * sqrt(y)
//   2. Bit by bit computation
//      Let q  = sqrt(y) truncated to i bit after binary point (q = 1),
//           i                                                   0
//                                     i+1         2
//          s  = 2*q , and      y  =  2   * ( y - q  ).          (1)
//           i      i            i                 i
//
//      To compute q    from q , one checks whether
//                  i+1       i
//
//                            -(i+1) 2
//                      (q + 2      )  <= y.                     (2)
//                        i
//                                                            -(i+1)
//      If (2) is false, then q   = q ; otherwise q   = q  + 2      .
//                             i+1   i             i+1   i
//
//      With some algebraic manipulation, it is not difficult to see
//      that (2) is equivalent to
//                             -(i+1)
//                      s  +  2       <= y                       (3)
//                       i                i
//
//      The advantage of (3) is that s  and y  can be computed by
//                                    i      i
//      the following recurrence formula:
//          if (3) is false
//
//          s     =  s  ,       y    = y   ;                     (4)
//           i+1      i          i+1    i
//
//      otherwise,
//                         -i                      -(i+1)
//          s     =  s  + 2  ,  y    = y  -  s  - 2              (5)
//           i+1      i          i+1    i     i
//
//      One may easily use induction to prove (4) and (5).
//      Note. Since the left hand side of (3) contain only i+2 bits,
//            it does not necessary to do a full (53-bit) comparison
//            in (3).
//   3. Final rounding
//      After generating the 53 bits result, we compute one more bit.
//      Together with the remainder, we can decide whether the
//      result is exact, bigger than 1/2ulp, or less than 1/2ulp
//      (it will never equal to 1/2ulp).
//      The rounding mode can be detected by checking whether
//      huge + tiny is equal to huge, and whether huge - tiny is
//      equal to huge for some floating point number "huge" and "tiny".
//
//
// Notes:  Rounding mode detection omitted.  The constants "mask", "shift",
// and "bias" are found in src/pkg/math/bits.go

// 原始C代码、详细注释、下面的常量以及此通知来自
// FreeBSD 的 /usr/src/lib/msun/src/e_sqrt.c 文件。
// 此Go代码为原始C代码的简化版本。
//
//（版权声明见上。）
//
// __ieee754_sqrt(x)
// 返回正确舍入的平方根。
//
//                 ---------------------------------
//                 | 若你有硬件sqrt指令，请使用它。|
//                 ---------------------------------
//
// （译注：此处用^号表示上标，_号表示下标，括号层次表示分组，原始形式见上。）
//
// 方法：
//   通过整数运算逐位进行计算。（较慢，但可移植。）
//   1. 规范化
//      将 x 按照2的偶数次幂的比例缩放为 y ，使得 y 在 [1,4) 内：
//      寻找一个整数 k，使其满足 1 <= (y=x*2**(2k)) < 4，则
//              sqrt(x) = 2**k * sqrt(y)
//
//   2. 逐位计算
//      从二进制小数点后截断成 i 位，设 q_i = sqrt(y)，（q_0 = 1），则有
//            s_i = 2 * q_i，且 y_i = 2^(i+1) * (y - q_i^2)。        (1)
//
//      要根据 q_i 计算 q_(i+1)，需判断
//                  [q_i + 2^-(i+1)]^2 <= y 是否为假，               (2)
//
//      若(2)式为假，则 q_(i+1) = q_i；
//                 否则 q_(i+1) = q_i + 2^-(i+1)。
//
//      根据一些代数运算，不难看出，(2)式等价于
//                      s_i + 2^-(i+1) <= y_i                        (3)
//
//      (3)式的优势在于 s_i 和 y_i 可通过以下递归式计算：
//
//          若(3)式为假，则
//                  s_(i+1) = s_i，y_(i+1) = y_i；                   (4)
//
//          否则
//          s_(i+1) = s_i + 2^-1，y_(i+1) = y_i - s_i - 2^-(i+i)     (5)
//
//      (4)式和(5)式可以很容易地使用归纳法来证明。
//      注意：(3)式的左边仅包含 i+2 位，无需在(3)式中进行所有的（53位）比较。
//
//   3. 最终舍入
//      在生成53位的结果后，我们还要计算更多的位。连同前面的一起，
//      我们可以确定其结果是否精确，大于 1/2ulp，还是小于 1/2ulp
//      （它绝不会等于 1/2ulp）。
//      对于一些浮点数的“huge”和“tiny”，这种舍入方式可通过检测 huge + tiny
//      等于 huge，还是 huge - tiny 等于 huge 来发现。
//
//
// 注：舍入方式检测在此省略。常量“mask”、“shift”和"bias"可在
// src/pkg/math/bits.go 中找到。

// Sqrt returns the square root of x.
//
// Special cases are:
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = ±0
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN

// Sqrt 返回 x 的平方根。
//
// 特殊情况为：
//	Sqrt(+Inf)  = +Inf
//	Sqrt(±0)    = ±0
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN)   = NaN
func Sqrt(x float64) float64

func sqrt(x float64) float64 {
	// special cases
	// 特殊情况
	switch {
	case x == 0 || IsNaN(x) || IsInf(x, 1):
		return x
	case x < 0:
		return NaN()
	}
	ix := Float64bits(x)
	// normalize x
	// 规范化 x
	exp := int((ix >> shift) & mask)
	if exp == 0 { // 次规范化 x
		for ix&1<<shift == 0 {
			ix <<= 1
			exp--
		}
		exp++
	}
	exp -= bias // unbias exponent // 反偏移指数
	ix &^= mask << shift
	ix |= 1 << shift
	// 若 exp 为奇数，则乘二使其成为偶数
	if exp&1 == 1 { // odd exp, double x to make it even
		ix <<= 1
	}
	// exp = exp/2，平方根的指数
	exp >>= 1 // exp = exp/2, exponent of square root
	// generate sqrt(x) bit by bit
	// 逐位生成 sqrt(x)
	ix <<= 1
	var q, s uint64               // q = sqrt(x)
	// r = 将位从最高有效位移至最低有效位
	r := uint64(1 << (shift + 1)) // r = moving bit from MSB to LSB
	for r != 0 {
		t := s + r
		if t <= ix {
			s = t + r
			ix -= t
			q += r
		}
		ix <<= 1
		r >>= 1
	}
	// final rounding
	// 最终舍入
	if ix != 0 { // remainder, result not exact    // 若剩余的结果不精确，
		q += q & 1 // round according to extra bit // 就根据多余的位舍入。
	}
	// 有效数字 + 偏移指数。
	ix = q>>1 + uint64(exp-1+bias)<<shift // significand + biased exponent
	return Float64frombits(ix)
}

func sqrtC(f float64, r *float64) {
	*r = sqrt(f)
}
