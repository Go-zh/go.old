// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Floating-point arcsine and arccosine.

	They are implemented by computing the arctangent
	after appropriate range reduction.
*/
/*
	浮点数反正弦和反余弦。

	在适当减少范围后，它们通过计算反正切来实现。
*/

// Asin returns the arcsine of x.
//
// Special cases are:
//	Asin(±0) = ±0
//	Asin(x) = NaN if x < -1 or x > 1

// Asin 返回 x 的反正弦值。
//
// 特殊情况为：
//	Asin(±0) = ±0
//	Asin(x)  = NaN（若 x < -1 或 x > 1）
func Asin(x float64) float64

func asin(x float64) float64 {
	if x == 0 {
		return x // special case // 特殊情况
	}
	sign := false
	if x < 0 {
		x = -x
		sign = true
	}
	if x > 1 {
		return NaN() // special case // 特殊情况
	}

	temp := Sqrt(1 - x*x)
	if x > 0.7 {
		temp = Pi/2 - satan(temp/x)
	} else {
		temp = satan(x / temp)
	}

	if sign {
		temp = -temp
	}
	return temp
}

// Acos returns the arccosine of x.
//
// Special case is:
//	Acos(x) = NaN if x < -1 or x > 1

// Acos 返回 x 的反余弦值。
//
// 特殊情况为：
//	Acos(x) = NaN（若 x < -1 或 x > 1）
func Acos(x float64) float64

func acos(x float64) float64 {
	return Pi/2 - Asin(x)
}
