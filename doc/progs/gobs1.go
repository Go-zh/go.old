// compile

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gobs1

type T struct{ X, Y, Z int } // 只有已导出的字段才会被编码或解码。
var t = T{X: 7, Y: 0, Z: 8}

// STOP OMIT

type U struct{ X, Y *int8 } // 注意：指向 int8 的指针
var u U

// STOP OMIT

type Node struct {
	Value       int
	Left, Right *Node
}

// STOP OMIT
