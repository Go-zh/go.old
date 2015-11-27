// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debug contains facilities for programs to debug themselves while
// they are running.

// debug 包含有程序在运行时调试其自身的功能.
package debug

import (
	"os"
	"runtime"
)

// PrintStack prints to standard error the stack trace returned by runtime.Stack.

// PrintStack 将 runtime.Stack 返回的栈跟踪信息打印到标准错误输出。
func PrintStack() {
	os.Stderr.Write(Stack())
}

// Stack returns a formatted stack trace of the goroutine that calls it.
// It calls runtime.Stack with a large enough buffer to capture the entire trace.

// Stack 返回格式化的Go程调用的栈跟踪信息。
// 它通过用一个足够大的缓冲调用 runtime.Stack 来捕获万种的跟踪。
func Stack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}
