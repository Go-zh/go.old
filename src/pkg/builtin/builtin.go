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
	builtin 包为Go的预声明标识符提供了文档.
	此处列出的条目其实并不在 buildin 包中，对它们的描述只是为了让 godoc
	给该语言的特殊标识符提供文档。
*/
package builtin

// bool is the set of boolean values, true and false.

// bool 是布尔值的集合，即 true 和 false。
type bool bool

// true and false are the two untyped boolean values.

// true 和 false 是两个无类型布尔值。
const (
	// 无类型布尔。
	true  = 0 == 0 // Untyped bool.
	false = 0 != 0 // Untyped bool.
)

// uint8 is the set of all unsigned 8-bit integers.
// Range: 0 through 255.

// uint8 是所有无符号8位整数的集合。
// 范围：0 至 255。
type uint8 uint8

// uint16 is the set of all unsigned 16-bit integers.
// Range: 0 through 65535.

// uint16 是所有无符号16位整数的集合。
// 范围：0 至 65535。
type uint16 uint16

// uint32 is the set of all unsigned 32-bit integers.
// Range: 0 through 4294967295.

// uint32 是所有无符号32位整数的集合。
// 范围：0 至 4294967295。
type uint32 uint32

// uint64 is the set of all unsigned 64-bit integers.
// Range: 0 through 18446744073709551615.

// uint64 是所有无符号64位整数的集合。
// 范围：0 至 18446744073709551615。
type uint64 uint64

// int8 is the set of all signed 8-bit integers.
// Range: -128 through 127.

// int8 是所有带符号8位整数的集合。
// 范围：-128 至 127。
type int8 int8

// int16 is the set of all signed 16-bit integers.
// Range: -32768 through 32767.

// int16 是所有带符号16位整数的集合。
// 范围：-32768 至 32767。
type int16 int16

// int32 is the set of all signed 32-bit integers.
// Range: -2147483648 through 2147483647.

// int32 是所有带符号32位整数的集合。
// 范围：-2147483648 至 2147483647。
type int32 int32

// int64 is the set of all signed 64-bit integers.
// Range: -9223372036854775808 through 9223372036854775807.

// int64 是所有带符号64位整数的集合。
// 范围：-9223372036854775808 至 9223372036854775807。
type int64 int64

// float32 is the set of all IEEE-754 32-bit floating-point numbers.

// float32 是所有IEEE-754 32位浮点数的集合。
type float32 float32

// float64 is the set of all IEEE-754 64-bit floating-point numbers.

// float64 是所有IEEE-754 64位浮点数的集合。
type float64 float64

// complex64 is the set of all complex numbers with float32 real and
// imaginary parts.

// complex64 是所有实部和虚部为 float32 的复数集合。
type complex64 complex64

// complex128 is the set of all complex numbers with float64 real and
// imaginary parts.

// complex128 是所有实部和虚部为 float64 的复数集合。
type complex128 complex128

// string is the set of all strings of 8-bit bytes, conventionally but not
// necessarily representing UTF-8-encoded text. A string may be empty, but
// not nil. Values of string type are immutable.

// string 是所有8位字节的字符串集合，习惯上用于代表以UTF-8编码的文本，但并不必须如此。
// string 可为空，但不为 nil。string 类型的值是不变的。
type string string

// int is a signed integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, int32.

// int 是带符号整数类型，其大小至少为32位。
// 它是一种确切的类型，而不是 int32 的别名。
type int int

// uint is an unsigned integer type that is at least 32 bits in size. It is a
// distinct type, however, and not an alias for, say, uint32.

// uint 是无符号整数类型，其大小至少为32位。
// 它是一种确切的类型，而不是 uint32 的别名。
type uint uint

// uintptr is an integer type that is large enough to hold the bit pattern of
// any pointer.

// uintptr 为整数类型，其大小足以容纳任何指针的位模式。
type uintptr uintptr

// byte is an alias for uint8 and is equivalent to uint8 in all ways. It is
// used, by convention, to distinguish byte values from 8-bit unsigned
// integer values.

// byte 为 uint8 的别名，它完全等价于 uint8。
// 习惯上用它来区别字节值和8位无符号整数值。
type byte byte

// rune is an alias for int32 and is equivalent int32 in all ways. It is
// used, by convention, to distinguish character values from integer values.

// rune 为 int32 的别名，它完全等价于 int32。
// 习惯上用它来区别字符值和整数值。
type rune rune

// iota is a predeclared identifier representing the untyped integer ordinal
// number of the current const specification in a (usually parenthesized)
// const declaration. It is zero-indexed.

// iota 为预声明的标识符，它表示常量声明中（一般在括号中），
// 当前常量规范的无类型化整数序数。它从0开始索引。
const iota = 0 // Untyped int. // 无类型化 int。

// nil is a predeclared identifier representing the zero value for a
// pointer, channel, func, interface, map, or slice type.

// nil 为预声明的标示符，它表示指针、信道、函数、接口、映射或切片类型的零值。
var nil Type // Type must be a pointer, channel, func, interface, map, or slice type
// Type 必须为指针、信道、函数、接口、映射或切片类型。

// Type is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.

// Type 在此只用作文档目的。
// 它代表所有Go的类型，但对于任何给定的函数请求来说，它都代表与其相同的类型。
type Type int

// Type1 is here for the purposes of documentation only. It is a stand-in
// for any Go type, but represents the same type for any given function
// invocation.

// Type1 在此只用作文档目的。
// 它代表所有Go的类型，但对于任何给定的函数请求来说，它都代表与其相同的类型。
type Type1 int

// IntegerType is here for the purposes of documentation only. It is a stand-in
// for any integer type: int, uint, int8 etc.

// IntegerType 在此只用作文档目的。
// 它代表所有的整数类型：如 int、uint、int8 等。
type IntegerType int

// FloatType is here for the purposes of documentation only. It is a stand-in
// for either float type: float32 or float64.

// FloatType 在此只用作文档目的。
// 它代表所有的浮点数类型：即 float32 或 float64。
type FloatType float32

// ComplexType is here for the purposes of documentation only. It is a
// stand-in for either complex type: complex64 or complex128.

// ComplexType 在此只用作文档目的。
// 它代表所有的复数类型：即 complex64 或 complex128。
type ComplexType complex64

// The append built-in function appends elements to the end of a slice. If
// it has sufficient capacity, the destination is resliced to accommodate the
// new elements. If it does not, a new underlying array will be allocated.
// Append returns the updated slice. It is therefore necessary to store the
// result of append, often in the variable holding the slice itself:
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
// As a special case, it is legal to append a string to a byte slice, like this:
//	slice = append([]byte("hello "), "world"...)

// append 内建函数将元素追加到切片的末尾。
// 若它有足够的容量，其目标就会重新切片以容纳新的元素。否则，就会分配一个新的基本数组。
// append 返回更新后的切片。因此必须存储追加后的结果，通常为包含该切片自身的变量：
//	slice = append(slice, elem1, elem2)
//	slice = append(slice, anotherSlice...)
// 作为一种特殊的情况，将字符追加到字节数组之后是合法的，就像这样：
//	slice = append([]byte("hello "), "world"...)
func append(slice []Type, elems ...Type) []Type

// The copy built-in function copies elements from a source slice into a
// destination slice. (As a special case, it also will copy bytes from a
// string to a slice of bytes.) The source and destination may overlap. Copy
// returns the number of elements copied, which will be the minimum of
// len(src) and len(dst).

// copy 内建函数将元素从来源切片复制到目标切片中。
// （特殊情况是，它也能将字节从字符串复制到字节切片中）。来源和目标可以重叠。
// copy 返回被复制的元素数量，它会是 len(src) 和 len(dst) 中较小的那个。
func copy(dst, src []Type) int

// The delete built-in function deletes the element with the specified key
// (m[key]) from the map. If m is nil or there is no such element, delete
// is a no-op.

// delete 内建函数按照指定的键将元素从映射中删除。
// 若 m 为 nil 或无此元素，delete 即为空操作。
func delete(m map[Type]Type1, key Type)

// The len built-in function returns the length of v, according to its type:
//	Array: the number of elements in v.
//	Pointer to array: the number of elements in *v (even if v is nil).
//	Slice, or map: the number of elements in v; if v is nil, len(v) is zero.
//	String: the number of bytes in v.
//	Channel: the number of elements queued (unread) in the channel buffer;
//	if v is nil, len(v) is zero.

// len 内建函数返回 v 的长度，这取决于具体类型：
//	数组：v 中元素的数量。
//	数组指针：*v 中元素的数量（即使 v 为 nil）。
//	切片或映射：v 中元素的数量；若 v 为 nil，len(v) 即为零。
//	字符串：v 中字节的数量。
//	信道：信道缓存中队列（未读取）元素的数量；若 v 为 nil，len(v) 即为零。
func len(v Type) int

// The cap built-in function returns the capacity of v, according to its type:
//	Array: the number of elements in v (same as len(v)).
//	Pointer to array: the number of elements in *v (same as len(v)).
//	Slice: the maximum length the slice can reach when resliced;
//	if v is nil, cap(v) is zero.
//	Channel: the channel buffer capacity, in units of elements;
//	if v is nil, cap(v) is zero.

// cap 内建函数返回 v 的容量，这取决于具体类型：
//	数组：v 中元素的数量（与 len(v) 相同）。
//	数组指针：*v 中元素的数量（与 len(v) 相同）。
//	切片：在重新切片时，切片能够达到的最大长度；若 v 为 nil，len(v) 即为零。
//	信道：按照元素的单元，相应信道缓存的容量；若 v 为 nil，len(v) 即为零。
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

// make 内建函数分配并初始化一个类型为切片、映射、或（仅仅为）信道的对象。
// 与 new 相同的是，其第一个实参为类型，而非值。不同的是，make 的返回类型
// 与其参数相同，而非指向它的指针。其具体结果取决于具体的类型：
//	切片：size 指定了其长度。该切片的容量等于其长度。第二个整数实参可用来指定
//		不同的容量；它必须不小于其长度，因此 make([]int, 0, 10) 会分配一个长度为0，
//		容量为10的切片。
//	映射：初始分配的创建取决于 size，但产生的映射长度为0。size 可以省略，这种情况下
//		就会分配一个小的起始大小。
//	信道：信道的缓存根据指定的缓存容量初始化。若 size 为零或被省略，该信道即为无缓存的。
func make(Type, size IntegerType) Type

// The new built-in function allocates memory. The first argument is a type,
// not a value, and the value returned is a pointer to a newly
// allocated zero value of that type.

// new 内建函数分配内存。
// 其第一个实参为类型，而非值，其返回值为指向该类型的新分配的零值的指针。
func new(Type) *Type

// The complex built-in function constructs a complex value from two
// floating-point values. The real and imaginary parts must be of the same
// size, either float32 or float64 (or assignable to them), and the return
// value will be the corresponding complex type (complex64 for float32,
// complex128 for float64).

// complex 内建函数将两个浮点数值构造成一个复数值。
// 其实部和虚部的大小必须相同，即 float32 或 float64（或可赋予它们的），其返回值
// 即为对应的复数类型（complex64 对应 float32，complex128 对应 float64）。
func complex(r, i FloatType) ComplexType

// The real built-in function returns the real part of the complex number c.
// The return value will be floating point type corresponding to the type of c.

// real 内建函数返回复数 c 的实部。
// 其返回值为对应于 c 类型的浮点数。
func real(c ComplexType) FloatType

// The imag built-in function returns the imaginary part of the complex
// number c. The return value will be floating point type corresponding to
// the type of c.

// imag 内建函数返回复数 c 的虚部。
// 其返回值为对应于 c 类型的浮点数。
func imag(c ComplexType) FloatType

// The close built-in function closes a channel, which must be either
// bidirectional or send-only. It should be executed only by the sender,
// never the receiver, and has the effect of shutting down the channel after
// the last sent value is received. After the last value has been received
// from a closed channel c, any receive from c will succeed without
// blocking, returning the zero value for the channel element. The form
//	x, ok := <-c
// will also set ok to false for a closed channel.

// close 内建函数关闭信道，该信道必须为双向的或只发送的。
// 它应当只由发送者执行，而不应由接收者执行，其效果是在最后发送的值被接收后停止该信道。
// 在最后一个值从已关闭的信道 c 中被接收后，任何从 c 的接收操作都会无阻塞成功，
// 它会返回该信道元素类型的零值。对于已关闭的信道，形式
//	x, ok := <-c
// 还会将 ok 置为 false。
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

// panic 内建函数停止当前Go程的正常执行。
// 当函数 F 调用 panic 时，F 的正常执行就会立刻停止。任何由 F 推迟的函数执行都会
// 按照一般的方式运行，接着 F 返回给其调用者。对于其调用者 G，F 的请求行为如同
// 对 panic 的调用，即终止 G 的执行并运行任何被推迟的函数。这会持续到该Go程
// 中所有函数都按相反的顺序停止执行之后。此时，该程序会被终止，而错误情况会被报告，
// 包括引发该恐慌的实参值。此终止序列称为恐慌过程，并可通过内建函数 recover 控制。
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

// recover 内建函数允许程序管理恐慌过程中的Go程。
// 在已推迟函数（而不是任何被它调用的函数）中，执行 recover 调用会通过恢复正常的执行
// 并取回传至 panic 调用的错误值来停止该恐慌过程序列。若 recover 在已推迟函数之外被调用，
// 它将不会停止恐慌过程序列。在此情况下，或当该Go程不在恐慌过程中时，或提供给 panic
// 的实参为 nil 时，recover 就会返回 nil。因此 recover 的返回值就报告了该Go程是否
// 在恐慌过程中。
func recover() interface{}

// The error built-in interface type is the conventional interface for
// representing an error condition, with the nil value representing no error.

// error 内建接口类型是表示错误情况的约定接口，nil 值即表示没有错误。
type error interface {
	Error() string
}
