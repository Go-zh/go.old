// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package debug

import (
	"strings"
	"testing"
)

type T int

func (t *T) ptrmethod() []byte {
	return Stack()
}
func (t T) method() []byte {
	return t.ptrmethod()
}

/*
	The traceback should look something like this, modulo line numbers and hex constants.
	Don't worry much about the base levels, but check the ones in our own package.

		goroutine 10 [running]:
		runtime/debug.Stack(0x0, 0x0, 0x0)
			/Users/r/go/src/runtime/debug/stack.go:28 +0x80
		runtime/debug.(*T).ptrmethod(0xc82005ee70, 0x0, 0x0, 0x0)
			/Users/r/go/src/runtime/debug/stack_test.go:15 +0x29
		runtime/debug.T.method(0x0, 0x0, 0x0, 0x0)
			/Users/r/go/src/runtime/debug/stack_test.go:18 +0x32
		runtime/debug.TestStack(0xc8201ce000)
			/Users/r/go/src/runtime/debug/stack_test.go:37 +0x38
		testing.tRunner(0xc8201ce000, 0x664b58)
			/Users/r/go/src/testing/testing.go:456 +0x98
		created by testing.RunTests
			/Users/r/go/src/testing/testing.go:561 +0x86d
*/

/*
	回溯信息看起来应该是这样的，模数行号以及二进制内容。
	不要担心有太多基础层面的信息，只要检查我们自己包中的那个就行了。

		/Users/r/go/src/pkg/runtime/debug/stack_test.go:15 (0x13878)
			(*T).ptrmethod: return Stack()
		/Users/r/go/src/pkg/runtime/debug/stack_test.go:18 (0x138dd)
			T.method: return t.ptrmethod()
		/Users/r/go/src/pkg/runtime/debug/stack_test.go:23 (0x13920)
			TestStack: b := T(0).method()
		/Users/r/go/src/pkg/testing/testing.go:132 (0x14a7a)
			tRunner: test.F(t)
		/Users/r/go/src/pkg/runtime/proc.c:145 (0xc970)
			???: runtime·unlock(&runtime·sched);
*/
func TestStack(t *testing.T) {
	b := T(0).method()
	lines := strings.Split(string(b), "\n")
	if len(lines) < 6 {
		t.Fatal("too few lines")
	}
	n := 0
	frame := func(line, code string) {
		check(t, lines[n], code)
		n++
		check(t, lines[n], line)
		n++
	}
	n++
	frame("src/runtime/debug/stack.go", "runtime/debug.Stack")
	frame("src/runtime/debug/stack_test.go", "runtime/debug.(*T).ptrmethod")
	frame("src/runtime/debug/stack_test.go", "runtime/debug.T.method")
	frame("src/runtime/debug/stack_test.go", "runtime/debug.TestStack")
	frame("src/testing/testing.go", "")
}

func check(t *testing.T, line, has string) {
	if strings.Index(line, has) < 0 {
		t.Errorf("expected %q in %q", has, line)
	}
}
