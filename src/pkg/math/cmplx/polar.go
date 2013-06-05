// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

// Polar returns the absolute value r and phase θ of x,
// such that x = r * e**θi.
// The phase is in the range [-Pi, Pi].

// Polar 返回 x 的绝对值 r 和相位 θ，使得 x = r * e**θi。
// 其相位在区间 [-Pi, Pi] 内。
func Polar(x complex128) (r, θ float64) {
	return Abs(x), Phase(x)
}
