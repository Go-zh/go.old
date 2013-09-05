// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package cgo contains runtime support for code generated
by the cgo tool.  See the documentation for the cgo command
for details on using cgo.
*/

/*
cgo 包含有 cgo 工具生成的代码的运行时支持.
使用 cgo 的详情见 cgo 命令的文档。
*/
package cgo

/*

#cgo darwin LDFLAGS: -lpthread
#cgo dragonfly LDFLAGS: -lpthread
#cgo freebsd LDFLAGS: -lpthread
#cgo linux LDFLAGS: -lpthread
#cgo netbsd LDFLAGS: -lpthread
#cgo openbsd LDFLAGS: -lpthread
#cgo windows LDFLAGS: -lm -mthreads

#cgo CFLAGS: -Wall -Werror

*/
import "C"

// Supports _cgo_panic by converting a string constant to an empty
// interface.
// 通过将字符串常量转换为空接口来支持 _cgo_panic。

func cgoStringToEface(s string, ret *interface{}) {
	*ret = s
}
