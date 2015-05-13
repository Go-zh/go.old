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

#cgo darwin,!arm,!arm64 LDFLAGS: -lpthread
#cgo darwin,arm LDFLAGS: -framework CoreFoundation
#cgo darwin,arm64 LDFLAGS: -framework CoreFoundation
#cgo dragonfly LDFLAGS: -lpthread
#cgo freebsd LDFLAGS: -lpthread
#cgo android LDFLAGS: -llog
#cgo !android,linux LDFLAGS: -lpthread
#cgo netbsd LDFLAGS: -lpthread
#cgo openbsd LDFLAGS: -lpthread
#cgo windows LDFLAGS: -lm -mthreads

#cgo CFLAGS: -Wall -Werror

#cgo solaris CPPFLAGS: -D_POSIX_PTHREAD_SEMANTICS

*/
import "C"
