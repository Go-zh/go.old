// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cmplx provides basic constants and mathematical functions for
// complex numbers.

// cmplx 包为复数提供了基本的常量和数学函数。
package cmplx

import "math"

// Abs returns the absolute value (also called the modulus) of x.

// Abs 返回 x 的绝对值（亦称为模）。
func Abs(x complex128) float64 { return math.Hypot(real(x), imag(x)) }
