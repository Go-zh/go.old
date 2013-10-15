// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !race

// Package atomic provides low-level atomic memory primitives
// useful for implementing synchronization algorithms.
//
// These functions require great care to be used correctly.
// Except for special, low-level applications, synchronization is better
// done with channels or the facilities of the sync package.
// Share memory by communicating;
// don't communicate by sharing memory.
//
// The swap operation, implemented by the SwapT functions, is the atomic
// equivalent of:
//
//	old = *addr
//	*addr = new
//	return old
//
// The compare-and-swap operation, implemented by the CompareAndSwapT
// functions, is the atomic equivalent of:
//
//	if *addr == old {
//		*addr = new
//		return true
//	}
//	return false
//
// The add operation, implemented by the AddT functions, is the atomic
// equivalent of:
//
//	*addr += delta
//	return *addr
//
// The load and store operations, implemented by the LoadT and StoreT
// functions, are the atomic equivalents of "return *addr" and
// "*addr = val".
//

// atomic 包提供了底层的原子性内存原语，这对于同步算法的实现很有用.
//
// 这些函数一定要非常小心地，正确地使用。特别是对于底层应用来说，最好使用信道或
// sync 包中提供的功能来完成。
//
// 不要通过共享内存来通信，应该通过通信来共享内存。
//
// “比较并交换”操作由 CompareAndSwapT 函数实现，它在原子性上等价于：
//
//	if *addr == old {
//		*addr = new
//		return true
//	}
//	return false
//
// “加上”操作由 AddT 函数实现，它在原子性上等价于：
//
//	*addr += delta
//	return *addr
//
// “载入并存储”操作由 LoadT 函数和 StoreT 函数实现，它们在原子性上分别等价于：
//
//	"return *addr"
// 和
//	"*addr = val".
//
package atomic

import (
	"unsafe"
)

// BUG(rsc): On x86-32, the 64-bit functions use instructions unavailable before the Pentium MMX.
//
// On non-Linux ARM, the 64-bit functions use instructions unavailable before the ARMv6k core.
//
// On both ARM and x86-32, it is the caller's responsibility to arrange for 64-bit
// alignment of 64-bit words accessed atomically. The first word in a global
// variable or in an allocated struct or slice can be relied upon to be
// 64-bit aligned.

// BUG(rsc): 在ARM上，64位函数使用的指令在ARM 11之前不可用。
//
// 在x86-32上，64位函数使用的指令在Pentium MMX之前不可用。

// SwapInt32 atomically stores new into *addr and returns the previous *addr value.

// SwapInt32 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapInt32(addr *int32, new int32) (old int32)

// SwapInt64 atomically stores new into *addr and returns the previous *addr value.

// SwapInt64 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapInt64(addr *int64, new int64) (old int64)

// SwapUint32 atomically stores new into *addr and returns the previous *addr value.

// SwapUint32 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapUint32(addr *uint32, new uint32) (old uint32)

// SwapUint64 atomically stores new into *addr and returns the previous *addr value.

// SwapUint64 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapUint64(addr *uint64, new uint64) (old uint64)

// SwapUintptr atomically stores new into *addr and returns the previous *addr value.

// SwapUintptr 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapUintptr(addr *uintptr, new uintptr) (old uintptr)

// SwapPointer atomically stores new into *addr and returns the previous *addr value.

// SwapPointer 自动将 new 存储到 *addr 中并返回上一个 *addr 值。
func SwapPointer(addr *unsafe.Pointer, new unsafe.Pointer) (old unsafe.Pointer)

// CompareAndSwapInt32 executes the compare-and-swap operation for an int32 value.

// CompareAndSwapInt32 为一个 int32 类型的值执行“比较并交换”操作。
func CompareAndSwapInt32(addr *int32, old, new int32) (swapped bool)

// CompareAndSwapInt64 executes the compare-and-swap operation for an int64 value.

// CompareAndSwapInt64 为一个 int64 类型的值执行“比较并交换”操作。
func CompareAndSwapInt64(addr *int64, old, new int64) (swapped bool)

// CompareAndSwapUint32 executes the compare-and-swap operation for a uint32 value.

// CompareAndSwapUint32 为一个 uint32 类型的值执行“比较并交换”操作。
func CompareAndSwapUint32(addr *uint32, old, new uint32) (swapped bool)

// CompareAndSwapUint64 executes the compare-and-swap operation for a uint64 value.

// CompareAndSwapUint64 为一个 uint64 类型的值执行“比较并交换”操作。
func CompareAndSwapUint64(addr *uint64, old, new uint64) (swapped bool)

// CompareAndSwapUintptr executes the compare-and-swap operation for a uintptr value.

// CompareAndSwapUintptr 为一个 uintptr 类型的值执行“比较并交换”操作。
func CompareAndSwapUintptr(addr *uintptr, old, new uintptr) (swapped bool)

// CompareAndSwapPointer executes the compare-and-swap operation for a unsafe.Pointer value.

// CompareAndSwapPointer 为一个 unsafe.Pointer 类型的值执行“比较并交换”操作。
func CompareAndSwapPointer(addr *unsafe.Pointer, old, new unsafe.Pointer) (swapped bool)

// AddInt32 atomically adds delta to *addr and returns the new value.

// AddInt32 自动将 delta 加上 *addr 并返回新值。
func AddInt32(addr *int32, delta int32) (new int32)

// AddUint32 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint32(&x, ^uint32(c-1)).
// In particular, to decrement x, do AddUint32(&x, ^uint32(0)).

// AddUint32 自动将 delta 加上 *addr 并返回新值。
// 要从 x 中减去一个带符号正整数常量 c，需执行 AddUint32(&x, ^uint32(c-1))。
// 特别地，要减量 x，需执行 AddUint32(&x, ^uint32(0))。
func AddUint32(addr *uint32, delta uint32) (new uint32)

// AddInt64 atomically adds delta to *addr and returns the new value.

// AddInt64 自动将 delta 加上 *addr 并返回新值。
func AddInt64(addr *int64, delta int64) (new int64)

// AddUint64 atomically adds delta to *addr and returns the new value.
// To subtract a signed positive constant value c from x, do AddUint64(&x, ^uint64(c-1)).
// In particular, to decrement x, do AddUint64(&x, ^uint64(0)).

// AddUint64 自动将 delta 加上 *addr 并返回新值。
// 要从 x 中减去一个带符号正整数常量 c，需执行 AddUint64(&x, ^uint64(c-1))。
// 特别地，要减量 x，需执行 AddUint64(&x, ^uint64(0))。
func AddUint64(addr *uint64, delta uint64) (new uint64)

// AddUintptr atomically adds delta to *addr and returns the new value.

// AddUintptr 自动将 delta 加上 *addr 并返回新值。
func AddUintptr(addr *uintptr, delta uintptr) (new uintptr)

// LoadInt32 atomically loads *addr.

// LoadInt32 自动载入 *addr。
func LoadInt32(addr *int32) (val int32)

// LoadInt64 atomically loads *addr.

// LoadInt64 自动载入 *addr。
func LoadInt64(addr *int64) (val int64)

// LoadUint32 atomically loads *addr.

// LoadUint32 自动载入 *addr。
func LoadUint32(addr *uint32) (val uint32)

// LoadUint64 atomically loads *addr.

// LoadUint64 自动载入 *addr。
func LoadUint64(addr *uint64) (val uint64)

// LoadUintptr atomically loads *addr.

// LoadUintptr 自动载入 *addr。
func LoadUintptr(addr *uintptr) (val uintptr)

// LoadPointer atomically loads *addr.

// LoadPointer 自动载入 *addr。
func LoadPointer(addr *unsafe.Pointer) (val unsafe.Pointer)

// StoreInt32 atomically stores val into *addr.

// StoreInt32 自动将 val 存储到 *addr 中。
func StoreInt32(addr *int32, val int32)

// StoreInt64 atomically stores val into *addr.

// StoreInt64 自动将 val 存储到 *addr 中。
func StoreInt64(addr *int64, val int64)

// StoreUint32 atomically stores val into *addr.

// StoreUint32 自动将 val 存储到 *addr 中。
func StoreUint32(addr *uint32, val uint32)

// StoreUint64 atomically stores val into *addr.

// StoreUint64 自动将 val 存储到 *addr 中。
func StoreUint64(addr *uint64, val uint64)

// StoreUint64 atomically stores val into *addr.

// StoreUint64 自动将 val 存储到 *addr 中。
func StoreUintptr(addr *uintptr, val uintptr)

// StorePointer atomically stores val into *addr.

// StorePointer 自动将 val 存储到 *addr 中。
func StorePointer(addr *unsafe.Pointer, val unsafe.Pointer)

// Helper for ARM.  Linker will discard on other systems

// ARM助手。连接器在其它系统上会丢弃它
func panic64() {
	panic("sync/atomic: broken 64-bit atomic operations (buggy QEMU)")
}
