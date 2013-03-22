// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package math

// Signbit returns true if x is negative or negative zero.

// Signbit 判断 x 是否为负值或负零。
func Signbit(x float64) bool {
	return Float64bits(x)&(1<<63) != 0
}
