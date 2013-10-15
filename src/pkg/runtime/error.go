// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

// The Error interface identifies a run time error.

// Error 接口用于标识运行时错误。
type Error interface {
	error

	// RuntimeError is a no-op function but
	// serves to distinguish types that are runtime
	// errors from ordinary errors: a type is a
	// runtime error if it has a RuntimeError method.
	//
	// RuntimeError 是一个无操作函数，它只用于区分是运行时错误还是一般错误：
	// 若一个类型拥有 RuntimeError 方法，它就是运行时错误。
	RuntimeError()
}

// A TypeAssertionError explains a failed type assertion.

// TypeAssertionError 用于阐明失败的类型断言。
type TypeAssertionError struct {
	interfaceString string
	concreteString  string
	assertedString  string
	missingMethod   string // one method needed by Interface, missing from Concrete
	// Interface 所需要的一个方法，在 Concrete 中没有
}

func (*TypeAssertionError) RuntimeError() {}

func (e *TypeAssertionError) Error() string {
	inter := e.interfaceString
	if inter == "" {
		inter = "interface"
	}
	if e.concreteString == "" {
		return "interface conversion: " + inter + " is nil, not " + e.assertedString
	}
	if e.missingMethod == "" {
		return "interface conversion: " + inter + " is " + e.concreteString +
			", not " + e.assertedString
	}
	return "interface conversion: " + e.concreteString + " is not " + e.assertedString +
		": missing method " + e.missingMethod
}

// For calling from C.

// 用于C的调用。
func newTypeAssertionError(ps1, ps2, ps3 *string, pmeth *string, ret *interface{}) {
	var s1, s2, s3, meth string

	if ps1 != nil {
		s1 = *ps1
	}
	if ps2 != nil {
		s2 = *ps2
	}
	if ps3 != nil {
		s3 = *ps3
	}
	if pmeth != nil {
		meth = *pmeth
	}
	*ret = &TypeAssertionError{s1, s2, s3, meth}
}

// An errorString represents a runtime error described by a single string.

// errorString 表示由单一字符串描述的运行时错误。
type errorString string

func (e errorString) RuntimeError() {}

func (e errorString) Error() string {
	return "runtime error: " + string(e)
}

// For calling from C.

// 用于C的调用。
func newErrorString(s string, ret *interface{}) {
	*ret = errorString(s)
}

// An errorCString represents a runtime error described by a single C string.
type errorCString uintptr

func (e errorCString) RuntimeError() {}

func cstringToGo(uintptr) string

func (e errorCString) Error() string {
	return "runtime error: " + cstringToGo(uintptr(e))
}

// For calling from C.
func newErrorCString(s uintptr, ret *interface{}) {
	*ret = errorCString(s)
}

type stringer interface {
	String() string
}

func typestring(interface{}) string

// For calling from C.
// Prints an argument passed to panic.
// There's room for arbitrary complexity here, but we keep it
// simple and handle just a few important cases: int, string, and Stringer.

// 用于C的调用。
// 打印传至 panic 的实参。这里可容纳任意的复杂度，但我们为了保持它的简单，
// 只处理一些重要的情况：int、string 和 Stringer。
func printany(i interface{}) {
	switch v := i.(type) {
	case nil:
		print("nil")
	case stringer:
		print(v.String())
	case error:
		print(v.Error())
	case int:
		print(v)
	case string:
		print(v)
	default:
		print("(", typestring(i), ") ", i)
	}
}

// called from generated code

// 由生成的代码调用。
func panicwrap(pkg, typ, meth string) {
	panic("value method " + pkg + "." + typ + "." + meth + " called using nil *" + typ + " pointer")
}
