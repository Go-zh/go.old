// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package unsafe contains operations that step around the type safety of Go programs.
*/

/*
	unsafe 包含有关于Go程序类型安全的所有操作.
*/
package unsafe

// ArbitraryType is here for the purposes of documentation only and is not actually
// part of the unsafe package.  It represents the type of an arbitrary Go expression.

// ArbitraryType 在此处只用作文档目的，它实际上并不是 unsafe 包的一部分。
// 它代表任意一个Go表达式的类型。
type ArbitraryType int

// Pointer represents a pointer to an arbitrary type.  There are four special operations
// available for type Pointer that are not available for other types.
//	1) A pointer value of any type can be converted to a Pointer.
//	2) A Pointer can be converted to a pointer value of any type.
//	3) A uintptr can be converted to a Pointer.
//	4) A Pointer can be converted to a uintptr.
// Pointer therefore allows a program to defeat the type system and read and write
// arbitrary memory. It should be used with extreme care.

// Pointer 代表一个指向任意类型的指针。
// 有三种特殊的操作可用于类型指针而不能用于其它类型。
//	1) 任意类型的指针值均可转换为 Pointer。
//	2) Pointer 均可转换为任意类型的指针值。
//	3) uintptr 均可转换为 Pointer。
//	4) Pointer 均可转换为 uintptr。
// 因此 Pointer 允许程序击溃类型系统并读写任意内存。它应当被用得非常小心。
type Pointer *ArbitraryType

// Sizeof returns the size in bytes occupied by the value v.  The size is that of the
// "top level" of the value only.  For instance, if v is a slice, it returns the size of
// the slice descriptor, not the size of the memory referenced by the slice.

// Sizeof 返回被值 v 所占用的字节大小。
// 该大小只是最“顶级”的值。例如，若 v 是一个切片，它会返回该切片描述符的大小，
// 而非该切片引用的内存大小。
func Sizeof(v ArbitraryType) uintptr

// Offsetof returns the offset within the struct of the field represented by v,
// which must be of the form structValue.field.  In other words, it returns the
// number of bytes between the start of the struct and the start of the field.

// Offsetof 返回由 v 所代表的结构中字段的偏移，它必须为 structValue.field 的形式。
// 换句话说，它返回该结构起始处与该字段起始处之间的字节数。
func Offsetof(v ArbitraryType) uintptr

// Alignof returns the alignment of the value v.  It is the maximum value m such
// that the address of a variable with the type of v will always be zero mod m.
// If v is of the form structValue.field, it returns the alignment of field f within struct object obj.

// Alignof 返回 v 值的对齐方式。
// 其返回值 m 满足变量 v 的类型地址与 m 取模为 0 的最大值。若 v 是 structValue.field
// 的形式，它会返回字段 f 在其相应结构对象 obj 中的对齐方式。
func Alignof(v ArbitraryType) uintptr
