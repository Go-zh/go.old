// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx

import "math"

// Phase returns the phase (also called the argument) of x.
// The returned value is in the range [-Pi, Pi].

// Phase 返回 x 的 相位（亦称为辐角）。
// 其返回值在区间 [-Pi, Pi] 内。
func Phase(x complex128) float64 { return math.Atan2(imag(x), real(x)) }
