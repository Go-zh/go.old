// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package builtin provides documentation for Go's predeclared identifiers.
	The items documented here are not actually in package builtin
	but their descriptions here allow godoc to present documentation
	for the language's special identifiers.
*/

/*
	builtin 包为Go的预声明标识符提供了文档。
	此处记录的条目实际上并不在 buildin 包中，但此处对它们的描述允许 godoc
	为该语言的特殊标识符提供文档。
*/
package builtin

// bool is the set of boolean values, true and false.

// bool 为布尔值的集合，即 true 与 false。
type bool bool

// uint8 is the set of all unsigned 8-bit integers.
// Range: 0 through 255.

// uint8 为所有无符号8位整数的集合。
// 范围：0 至 255。
type uint8 uint8

// uint16 is the set of all unsigned 16-bit integers.
// Range: 0 through 65535.

// uint16 为所有无符号16位整数的集合。
// 范围：0 至 65535。
type uint16 uint16

// uint32 is the set of all unsigned 32-bit integers.
// Range: 0 through 4294967295.

// uint32 为所有无符号32位整数的集合。
// 范围：0 至 4294967295。
type uint32 uint32

// uint64 is the set of all unsigned 64-bit integers.
// Range: 0 through 18446744073709551615.

// uint64 为所有无符号64位整数的集合。
// 范围：0 至 18446744073709551615。
type uint64 uint64

// int8 is the set of all signed 8-bit integers.
// Range: -128 through 127.

// int8 为所有带符号8位整数的集合。
// 范围：-128 至 127。
type int8 int8

// int16 is the set of all signed 16-bit integers.
// Range: -32768 through 32767.

// int16 为所有带符号16位整数的集合。
// 范围：-32768 至 32767。
type int16 int16

// int32 is the set of all signed 32-bit integers.
// Range: -2147483648 through 2147483647.

// int32 为所有带符号32位整数的集合。
// 范围：-32768 至 32767。
type int32 int32

// int64 is the set of all signed 64-bit integers.
// Range: -9223372036854775808 through 9223372036854775807.

// int64 为所有带符号64位整数的集合。
// 范围：-9223372036854775808 至 9223372036854775807。
type int64 int64

// float32 is the set of all IEEE-754 32-bit floating-point numbers.

// float32 为所有IEEE-754 32位浮点数的集合。
type float32 float32

// float64 is the set of all IEEE-754 64-bit floating-point numbers.

// float64 为所有IEEE-754 64位浮点数的集合。
type float64 float64

// complex64 is the set of all complex numbers with float32 real and
// imaginary parts.

// complex64 为所有带 float32 类型实部和虚部的复数集合。
type complex64 complex64

// complex128 is the set of all complex numbers with float64 real and
// imaginary parts.

// complex128 为所有带 float64 类型实部和虚部的复数集合。
type complex128 complex128

// string is the set of all strings of 8-bit bytes, conventionally but not
// necessarily representing UTF-8-encoded text. A string may be empty, but
// not nil. Values of string type are immutable.

// string 为所有8位字节的字符串集合，习惯于但并不必须代表以UTF-8编码的文本。
// string 可能为空，但不为 nil。string 类型的值是不变的。
type string string

// int is a signed integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, int32.

// int 为带符号整数类型，其大小至少为32位。
// 它是一种不同的类型，而不是所谓的 int32 的别名。
type int int

// uint is an unsigned integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, uint32.

// uint 为无符号整数类型，其大小至少为32位。
// 它是一种不同的类型，而不是所谓的 uint32 的别名。
type uint uint

// uintptr is an integer type that is large enough to hold the bit pattern of
// any pointer.

// uintptr 为整数类型，其大小足以容纳任何指针的位模式。
type uintptr uintptr

// byte is an alias for uint8 and is equivalent to uint8 in all ways. It is
// used, by convention, to distinguish byte values from 8-bit unsigned
// integer values.

// byte 为 uint8 的别名，它在各方面上都等价于 uint8。
// 它习惯用于区别字节值与8位无符号整数值。
type byte byte

// rune is an alias for int and is equivalent to int in all ways. It is
// used, by convention, to distinguish character values from integer values.
// In a future version of Go, it will change to an alias of int32.

// rune 为 int 的别名，它在各方面上都等价于 int。
// 它习惯用于区别字符值与整数值。在未来的Go版本中，它将会更改为 int32 的别名。
type rune rune

// Type is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.

// Type 在此只用作文档目的。
// 它是任何Go类型的替身，但对于任何给定的函数调用来说，它都代表与其相同的类型。
type Type int

// Type1 is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.

// Type1 在此只用作文档目的。
// 它是任何Go类型的替身，但对于任何给定的函数调用来说，它都代表与其相同的类型。
type Type1 int

// IntegerType is here for the purposes of documentation only. It is a stand-in
// for any integer type: int, uint, int8 etc.

// IntegerType 在此只用作文档目的。
// 它是任何整数类型的替身：如 int、uint、int8 等。
type IntegerType int

// FloatType is here for the purposes of documentation only. It is a stand-in
// for either float type: float32 or float64.

// FloatType 在此只用作文档目的。
// 它是任何浮点数类型的替身：即 float32 或 float64。
type FloatType float32

// ComplexType is here for the purposes of documentation only. It is a
// stand-in for either complex type: complex64 or complex128.

// ComplexType 在此只用作文档目的。
// 它是任何复数类型的替身：即 complex64 或 complex128。
type ComplexType complex64

// The append built-in function appends elements to the end of a slice. If
// it has sufficient capacity, the destination is resliced to accommodate the
// new elements. If it does not, a new underlying array will be allocated.
// Append returns the updated slice. It is therefore necessary to store the
// result of append, often in the variable holding the slice itself:
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)

// append 内建函数用于将元素追加到切片的末尾之后。若它有足够的容量，
// 其目标即为重新切片以适应新的元素。若否，则会分配一个新的基本数组。
// append 返回更新后的切片。因此必须存储追加后的结果，通常为包含该切片自身的变量：
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
func append(slice []Type, elems ...Type) []Type

// The copy built-in function copies elements from a source slice into a
// destination slice. (As a special case, it also will copy bytes from a
// string to a slice of bytes.) The source and destination may overlap. Copy
// returns the number of elements copied, which will be the minimum of
// len(src) and len(dst).
func copy(dst, src []Type) int

// The delete built-in function deletes the element with the specified key
// (m[key]) from the map. If there is no such element, delete is a no-op.
// If m is nil, delete panics.
func delete(m map[Type]Type1, key Type)

// The len built-in function returns the length of v, according to its type:
//	Array: the number of elements in v.
//	Pointer to array: the number of elements in *v (even if v is nil).
//	Slice, or map: the number of elements in v; if v is nil, len(v) is zero.
//	String: the number of bytes in v.
//	Channel: the number of elements queued (unread) in the channel buffer;
//	if v is nil, len(v) is zero.
func len(v Type) int

// The cap built-in function returns the capacity of v, according to its type:
//	Array: the number of elements in v (same as len(v)).
//	Pointer to array: the number of elements in *v (same as len(v)).
//	Slice: the maximum length the slice can reach when resliced;
//	if v is nil, cap(v) is zero.
//	Channel: the channel buffer capacity, in units of elements;
//	if v is nil, cap(v) is zero.
func cap(v Type) int

// The make built-in function allocates and initializes an object of type
// slice, map, or chan (only). Like new, the first argument is a type, not a
// value. Unlike new, make's return type is the same as the type of its
// argument, not a pointer to it. The specification of the result depends on
// the type:
//	Slice: The size specifies the length. The capacity of the slice is
//	equal to its length. A second integer argument may be provided to
//	specify a different capacity; it must be no smaller than the
//	length, so make([]int, 0, 10) allocates a slice of length 0 and
//	capacity 10.
//	Map: An initial allocation is made according to the size but the
//	resulting map has length 0. The size may be omitted, in which case
//	a small starting size is allocated.
//	Channel: The channel's buffer is initialized with the specified
//	buffer capacity. If zero, or the size is omitted, the channel is
//	unbuffered.
func make(Type, size IntegerType) Type

// The new built-in function allocates memory. The first argument is a type,
// not a value, and the value returned is a pointer to a newly
// allocated zero value of that type.
func new(Type) *Type

// The complex built-in function constructs a complex value from two
// floating-point values. The real and imaginary parts must be of the same
// size, either float32 or float64 (or assignable to them), and the return
// value will be the corresponding complex type (complex64 for float32,
// complex128 for float64).
func complex(r, i FloatType) ComplexType

// The real built-in function returns the real part of the complex number c.
// The return value will be floating point type corresponding to the type of c.
func real(c ComplexType) FloatType

// The imag built-in function returns the imaginary part of the complex
// number c. The return value will be floating point type corresponding to
// the type of c.
func imag(c ComplexType) FloatType

// The close built-in function closes a channel, which must be either
// bidirectional or send-only. It should be executed only by the sender,
// never the receiver, and has the effect of shutting down the channel after
// the last sent value is received. After the last value has been received
// from a closed channel c, any receive from c will succeed without
// blocking, returning the zero value for the channel element. The form
//	x, ok := <-c
// will also set ok to false for a closed channel.
func close(c chan<- Type)

// The panic built-in function stops normal execution of the current
// goroutine. When a function F calls panic, normal execution of F stops
// immediately. Any functions whose execution was deferred by F are run in
// the usual way, and then F returns to its caller. To the caller G, the
// invocation of F then behaves like a call to panic, terminating G's
// execution and running any deferred functions. This continues until all
// functions in the executing goroutine have stopped, in reverse order. At
// that point, the program is terminated and the error condition is reported,
// including the value of the argument to panic. This termination sequence
// is called panicking and can be controlled by the built-in function
// recover.
func panic(v interface{})

// The recover built-in function allows a program to manage behavior of a
// panicking goroutine. Executing a call to recover inside a deferred
// function (but not any function called by it) stops the panicking sequence
// by restoring normal execution and retrieves the error value passed to the
// call of panic. If recover is called outside the deferred function it will
// not stop a panicking sequence. In this case, or when the goroutine is not
// panicking, or if the argument supplied to panic was nil, recover returns
// nil. Thus the return value from recover reports whether the goroutine is
// panicking.
func recover() interface{}

// The error built-in interface type is the conventional interface for
// representing an error condition, with the nil value representing no error.
type error interface {
	Error() string
}
