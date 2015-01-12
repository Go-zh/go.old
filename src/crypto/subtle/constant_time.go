// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//	Package subtle implements functions that are often useful in cryptographic
//	code but require careful thought to use correctly.

// subtle 包实现了一些在加密算法代码中经常用到的函数，但是这些函数需要仔细地考虑
// 才能知道如何正确地使用。
package subtle

//	ConstantTimeCompare returns 1 iff the two slices, x
//	and y, have equal contents. The time taken is a function of the length of
//	the slices and is independent of the contents.

// ConstantTimeCompare 返回1如果 x 和 y 这两个切片内容完全相同。所消耗的CPU时间与
// 切片的长度线性相关而与其具体的内容无关。
func ConstantTimeCompare(x, y []byte) int {
	if len(x) != len(y) {
		return 0
	}

	var v byte

	for i := 0; i < len(x); i++ {
		v |= x[i] ^ y[i]
	}

	return ConstantTimeByteEq(v, 0)
}

//	ConstantTimeSelect returns x if v is 1 and y if v is 0.
//	Its behavior is undefined if v takes any other value.

// ConstantTimeSelect 返回 x 如果 v 为1；返回  y 如果 v 为0。
// 如果 v 为其他值，该函数的行为未定义。
func ConstantTimeSelect(v, x, y int) int { return ^(v-1)&x | (v-1)&y }

//	ConstantTimeByteEq returns 1 if x == y and 0 otherwise.

// ConstantTimeByteEq 返回1，如果 x == y；相反则返回0。
func ConstantTimeByteEq(x, y uint8) int {
	z := ^(x ^ y)
	z &= z >> 4
	z &= z >> 2
	z &= z >> 1

	return int(z)
}

//	ConstantTimeEq returns 1 if x == y and 0 otherwise.

// ConstantTimeEq 返回1，如果 x == y；相反则返回0。
func ConstantTimeEq(x, y int32) int {
	z := ^(x ^ y)
	z &= z >> 16
	z &= z >> 8
	z &= z >> 4
	z &= z >> 2
	z &= z >> 1

	return int(z & 1)
}

//	ConstantTimeCopy copies the contents of y into x (a slice of equal length)
//	if v == 1. If v == 0, x is left unchanged. Its behavior is undefined if v
//	takes any other value.

// ConstantTimeCopy 将 y 拷贝到 x 中(这两个切片的长度相等)如果 v == 1；如果 v == 0，
// x 将保持不变。如果 v 为其他值，其行为为定义。
func ConstantTimeCopy(v int, x, y []byte) {
	if len(x) != len(y) {
		panic("subtle: slices have different lengths")
	}

	xmask := byte(v - 1)
	ymask := byte(^(v - 1))
	for i := 0; i < len(x); i++ {
		x[i] = x[i]&xmask | y[i]&ymask
	}
}

//	ConstantTimeLessOrEq returns 1 if x <= y and 0 otherwise.
//	Its behavior is undefined if x or y are negative or > 2**31 - 1.

// ConstantTimeLessOrEq 返回1，如果 x <= y；否则返回0。如果 x 或 y 为负数，
// 或者大于 2**31 - 1 ，其行为未定义。
func ConstantTimeLessOrEq(x, y int) int {
	x32 := int32(x)
	y32 := int32(y)
	return int(((x32 - y32 - 1) >> 31) & 1)
}
