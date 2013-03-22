// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

/*
	Hypot -- sqrt(p*p + q*q), but overflows only if the result does.
*/
/*
	Hypot -- sqrt(p*p + q*q)，但只有在结果向上溢出时，该函数才会溢出。
*/

// Hypot returns Sqrt(p*p + q*q), taking care to avoid
// unnecessary overflow and underflow.
//
// Special cases are:
//	Hypot(p, q) = +Inf if p or q is infinite
//	Hypot(p, q) = NaN if p or q is NaN

// Hypot 返回 Sqrt(p*p + q*q)，小心避免不必要的向上溢出和向下溢出。
//
// 特殊情况为：
//	若 p 或 q 为 Inf，则 Hypot(p, q) = +Inf
//	若 p 或 q 为 NaN，则 Hypot(p, q) = NaN
func Hypot(p, q float64) float64

func hypot(p, q float64) float64 {
	// special cases
	// 特殊情况
	switch {
	case IsInf(p, 0) || IsInf(q, 0):
		return Inf(1)
	case IsNaN(p) || IsNaN(q):
		return NaN()
	}
	if p < 0 {
		p = -p
	}
	if q < 0 {
		q = -q
	}
	if p < q {
		p, q = q, p
	}
	if p == 0 {
		return 0
	}
	q = q / p
	return p * Sqrt(1+q*q)
}
