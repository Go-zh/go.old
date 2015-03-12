// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package unsafe contains operations that step around the type safety of Go programs.

	Packages that import unsafe may be non-portable and are not protected by the
	Go 1 compatibility guidelines.
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

// Sizeof takes an expression x of any type and returns the size in bytes
// of a hypothetical variable v as if v was declared via var v = x.
// The size does not include any memory possibly referenced by x.
// For instance, if x is a slice,  Sizeof returns the size of the slice
// descriptor, not the size of the memory referenced by the slice.

// Sizeof 接受一个任意类型的表达式 x 并返回假定的变量 v 的字节大小，这里的 v 可看做通过
// var v = x 声明的变量。该大小并不包括 x 可能引用的任何内存。例如，若 x 是一个切片，
// Sizeof 会返回该切片描述符所示的大小，而非该切片引用的内存大小。
func Sizeof(x ArbitraryType) uintptr

// Offsetof returns the offset within the struct of the field represented by x,
// which must be of the form structValue.field.  In other words, it returns the
// number of bytes between the start of the struct and the start of the field.

// Offsetof 返回 x 所代表的结构体中字段的偏移量，它必须为 structValue.field 的形式。
// 换言之，它返回该结构体起始处与该字段起始处之间的字节数。
func Offsetof(x ArbitraryType) uintptr

// Alignof takes an expression x of any type and returns the alignment
// of a hypothetical variable v as if v was declared via var v = x.
// It is the largest value m such that the address of v is zero mod m.

// Alignof 接受一个任意类型的表达式 x 并返回假定的变量 v 的对齐，这里的 v 可看做通过
// var v = x 声明的变量。它是最大值 m 使其满足 v 的地址取模 m 为零。
// TODO(osc): 需优化语句
func Alignof(x ArbitraryType) uintptr
