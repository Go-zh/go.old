// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// Compiler is the name of the compiler toolchain that built the
// running binary.  Known toolchains are:
//
//	gc      The 5g/6g/8g compiler suite at code.google.com/p/go.
//	gccgo   The gccgo front end, part of the GCC compiler suite.
//

// Compiler 为构建了可运行二进制文件的编译工具链。已知的工具链为：
//	go       code.google.com/p/go 上的 5g/6g/8g 编译器套件。
//	gccgo    gccgo前端，GCC编译器条件的一部分。
const Compiler = "gc"
