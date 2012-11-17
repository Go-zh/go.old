// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package debug contains facilities for programs to debug themselves while
// they are running.

// debug 包含有程序在运行时调试其自身的功能.
package debug

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
)

var (
	dunno     = []byte("???")
	centerDot = []byte("·")
	dot       = []byte(".")
)

// PrintStack prints to standard error the stack trace returned by Stack.

// PrintStack 将 Stack 返回的栈跟踪信息打印到标准错误输出。
func PrintStack() {
	os.Stderr.Write(stack())
}

// Stack returns a formatted stack trace of the goroutine that calls it.
// For each routine, it includes the source line information and PC value,
// then attempts to discover, for Go functions, the calling function or
// method and the text of the line containing the invocation.

// Stack 返回格式化的Go程调用的栈跟踪信息。
// 对于每一个例程，它包括来源行的信息和 PC 值，然后尝试获取，对于Go函数，
// 则是调用的函数或方法及其包含请求的行的文本。
func Stack() []byte {
	return stack()
}

// stack implements Stack, skipping 2 frames

// stack 实现了 Stack，跳过2帧
func stack() []byte {
	buf := new(bytes.Buffer) // the returned data // 返回的数据
	// As we loop, we open files and read them. These variables record the currently
	// loaded file.
	// 我们的循环打开文件并读取它们。这些变量记录了当前已加载的文件。
	var lines [][]byte
	var lastFile string
	// 我们关心 Caller 的调用者，因此跳过2帧
	for i := 2; ; i++ { // Caller we care about is the user, 2 frames up
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		// Print this much at least.  If we can't find the source, it won't show.
		// 至少要打印这么多信息。如果我们找不到来源，它就不会被显示。
		fmt.Fprintf(buf, "%s:%d (0x%x)\n", file, line, pc)
		if file != lastFile {
			data, err := ioutil.ReadFile(file)
			if err != nil {
				continue
			}
			lines = bytes.Split(data, []byte{'\n'})
			lastFile = file
		}
		// 在栈跟踪中，行号从1开始，单我们的数组下标却是从0开始
		line-- // in stack trace, lines are 1-indexed but our array is 0-indexed
		fmt.Fprintf(buf, "\t%s: %s\n", function(pc), source(lines, line))
	}
	return buf.Bytes()
}

// source returns a space-trimmed slice of the n'th line.

// source 以整洁的形式返回第 n 行的切片。
func source(lines [][]byte, n int) []byte {
	if n < 0 || n >= len(lines) {
		return dunno
	}
	return bytes.Trim(lines[n], " \t")
}

// function returns, if possible, the name of the function containing the PC.

// function 在可能的情况下会返回包含在 PC 中的函数名。
func function(pc uintptr) []byte {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := []byte(fn.Name())
	// The name includes the path name to the package, which is unnecessary
	// since the file name is already included.  Plus, it has center dots.
	// That is, we see
	//	runtime/debug.*T·ptrmethod
	// and want
	//	*T.ptrmethod
	//
	// name 包括该包的路径名，若已经包含了文件名，它就不是必要的了。
	// 另外，它有中间点，也就是说，我们看到
	//	runtime/debug.*T·ptrmethod
	// 而想要
	//	*T.ptrmethod
	if period := bytes.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	name = bytes.Replace(name, centerDot, dot, -1)
	return name
}
