// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// Should be a built-in for unsafe.Pointer?
//go:nosplit
func add(p unsafe.Pointer, x uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

// getg returns the pointer to the current g.
// The compiler rewrites calls to this function into instructions
// that fetch the g directly (from TLS or from the dedicated register).
func getg() *g

// mcall switches from the g to the g0 stack and invokes fn(g),
// where g is the goroutine that made the call.
// mcall saves g's current PC/SP in g->sched so that it can be restored later.
// It is up to fn to arrange for that later execution, typically by recording
// g in a data structure, causing something to call ready(g) later.
// mcall returns to the original goroutine g later, when g has been rescheduled.
// fn must not return at all; typically it ends by calling schedule, to let the m
// run other goroutines.
//
// mcall can only be called from g stacks (not g0, not gsignal).
//
// This must NOT be go:noescape: if fn is a stack-allocated closure,
// fn puts g on a run queue, and g executes before fn returns, the
// closure will be invalidated while it is still executing.
func mcall(fn func(*g))

// systemstack runs fn on a system stack.
// If systemstack is called from the per-OS-thread (g0) stack, or
// if systemstack is called from the signal handling (gsignal) stack,
// systemstack calls fn directly and returns.
// Otherwise, systemstack is being called from the limited stack
// of an ordinary goroutine. In this case, systemstack switches
// to the per-OS-thread stack, calls fn, and switches back.
// It is common to use a func literal as the argument, in order
// to share inputs and outputs with the code around the call
// to system stack:
//
//	... set up y ...
//	systemstack(func() {
//		x = bigcall(y)
//	})
//	... use x ...
//
//go:noescape
func systemstack(fn func())

func badsystemstack() {
	throw("systemstack called from unexpected goroutine")
}

// memclr clears n bytes starting at ptr.
// in memclr_*.s
//go:noescape
func memclr(ptr unsafe.Pointer, n uintptr)

//go:linkname reflect_memclr reflect.memclr
func reflect_memclr(ptr unsafe.Pointer, n uintptr) {
	memclr(ptr, n)
}

// memmove copies n bytes from "from" to "to".
// in memmove_*.s
//go:noescape
func memmove(to, from unsafe.Pointer, n uintptr)

//go:linkname reflect_memmove reflect.memmove
func reflect_memmove(to, from unsafe.Pointer, n uintptr) {
	memmove(to, from, n)
}

// exported value for testing
var hashLoad = loadFactor

// in asm_*.s
func fastrand1() uint32

// in asm_*.s
//go:noescape
func memequal(a, b unsafe.Pointer, size uintptr) bool

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to a single xor instruction.
// USE CAREFULLY!
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

func cgocallback(fn, frame unsafe.Pointer, framesize uintptr)
func gogo(buf *gobuf)
func gosave(buf *gobuf)
func mincore(addr unsafe.Pointer, n uintptr, dst *byte) int32

//go:noescape
func jmpdefer(fv *funcval, argp uintptr)
func exit1(code int32)
func asminit()
func setg(gg *g)
func breakpoint()

// reflectcall calls fn with a copy of the n argument bytes pointed at by arg.
// After fn returns, reflectcall copies n-retoffset result bytes
// back into arg+retoffset before returning. If copying result bytes back,
// the caller should pass the argument frame type as argtype, so that
// call can execute appropriate write barriers during the copy.
// Package reflect passes a frame type. In package runtime, there is only
// one call that copies results back, in cgocallbackg1, and it does NOT pass a
// frame type, meaning there are no write barriers invoked. See that call
// site for justification.
func reflectcall(argtype *_type, fn, arg unsafe.Pointer, argsize uint32, retoffset uint32)

func procyield(cycles uint32)

type neverCallThisFunction struct{}

// goexit is the return stub at the top of every goroutine call stack.
// Each goroutine stack is constructed as if goexit called the
// goroutine's entry point function, so that when the entry point
// function returns, it will return to goexit, which will call goexit1
// to perform the actual exit.
//
// This function must never be called directly. Call goexit1 instead.
// gentraceback assumes that goexit terminates the stack. A direct
// call on the stack will cause gentraceback to stop walking the stack
// prematurely and if there are leftover stack barriers it may panic.
func goexit(neverCallThisFunction)

// Not all cgocallback_gofunc frames are actually cgocallback_gofunc,
// so not all have these arguments. Mark them uintptr so that the GC
// does not misinterpret memory when the arguments are not present.
// cgocallback_gofunc is not called from go, only from cgocallback,
// so the arguments will be found via cgocallback's pointer-declared arguments.
// See the assembly implementations for more details.
func cgocallback_gofunc(fv uintptr, frame uintptr, framesize uintptr)

// publicationBarrier performs a store/store barrier (a "publication"
// or "export" barrier). Some form of synchronization is required
// between initializing an object and making that object accessible to
// another processor. Without synchronization, the initialization
// writes and the "publication" write may be reordered, allowing the
// other processor to follow the pointer and observe an uninitialized
// object. In general, higher-level synchronization should be used,
// such as locking or an atomic pointer write. publicationBarrier is
// for when those aren't an option, such as in the implementation of
// the memory manager.
//
// There's no corresponding barrier for the read side because the read
// side naturally has a data dependency order. All architectures that
// Go supports or seems likely to ever support automatically enforce
// data dependency ordering.
func publicationBarrier()

//go:noescape
func setcallerpc(argp unsafe.Pointer, pc uintptr)

// getcallerpc returns the program counter (PC) of its caller's caller.
// getcallersp returns the stack pointer (SP) of its caller's caller.
// For both, the argp must be a pointer to the caller's first function argument.
// The implementation may or may not use argp, depending on
// the architecture.
//
// For example:
//
//	func f(arg1, arg2, arg3 int) {
//		pc := getcallerpc(unsafe.Pointer(&arg1))
//		sp := getcallersp(unsafe.Pointer(&arg1))
//	}
//
// These two lines find the PC and SP immediately following
// the call to f (where f will return).
//
// The call to getcallerpc and getcallersp must be done in the
// frame being asked about. It would not be correct for f to pass &arg1
// to another function g and let g call getcallerpc/getcallersp.
// The call inside g might return information about g's caller or
// information about f's caller or complete garbage.
//
// The result of getcallersp is correct at the time of the return,
// but it may be invalidated by any subsequent call to a function
// that might relocate the stack in order to grow or shrink it.
// A general rule is that the result of getcallersp should be used
// immediately and can only be passed to nosplit functions.

//go:noescape
func getcallerpc(argp unsafe.Pointer) uintptr

//go:noescape
func getcallersp(argp unsafe.Pointer) uintptr

//go:noescape
func asmcgocall(fn, arg unsafe.Pointer) int32

// argp used in Defer structs when there is no argp.
const _NoArgs = ^uintptr(0)

func morestack()
func rt0_go()

// stackBarrier records that the stack has been unwound past a certain
// point. It is installed over a return PC on the stack. It must
// retrieve the original return PC from g.stkbuf, increment
// g.stkbufPos to record that the barrier was hit, and jump to the
// original return PC.
func stackBarrier()

// return0 is a stub used to return 0 from deferproc.
// It is called at the very end of deferproc to signal
// the calling Go function that it should not jump
// to deferreturn.
// in asm_*.s
func return0()

//go:linkname time_now time.now
func time_now() (sec int64, nsec int32)

// in asm_*.s
// not called directly; definitions here supply type information for traceback.
func call32(fn, arg unsafe.Pointer, n, retoffset uint32)
func call64(fn, arg unsafe.Pointer, n, retoffset uint32)
func call128(fn, arg unsafe.Pointer, n, retoffset uint32)
func call256(fn, arg unsafe.Pointer, n, retoffset uint32)
func call512(fn, arg unsafe.Pointer, n, retoffset uint32)
func call1024(fn, arg unsafe.Pointer, n, retoffset uint32)
func call2048(fn, arg unsafe.Pointer, n, retoffset uint32)
func call4096(fn, arg unsafe.Pointer, n, retoffset uint32)
func call8192(fn, arg unsafe.Pointer, n, retoffset uint32)
func call16384(fn, arg unsafe.Pointer, n, retoffset uint32)
func call32768(fn, arg unsafe.Pointer, n, retoffset uint32)
func call65536(fn, arg unsafe.Pointer, n, retoffset uint32)
func call131072(fn, arg unsafe.Pointer, n, retoffset uint32)
func call262144(fn, arg unsafe.Pointer, n, retoffset uint32)
func call524288(fn, arg unsafe.Pointer, n, retoffset uint32)
func call1048576(fn, arg unsafe.Pointer, n, retoffset uint32)
func call2097152(fn, arg unsafe.Pointer, n, retoffset uint32)
func call4194304(fn, arg unsafe.Pointer, n, retoffset uint32)
func call8388608(fn, arg unsafe.Pointer, n, retoffset uint32)
func call16777216(fn, arg unsafe.Pointer, n, retoffset uint32)
func call33554432(fn, arg unsafe.Pointer, n, retoffset uint32)
func call67108864(fn, arg unsafe.Pointer, n, retoffset uint32)
func call134217728(fn, arg unsafe.Pointer, n, retoffset uint32)
func call268435456(fn, arg unsafe.Pointer, n, retoffset uint32)
func call536870912(fn, arg unsafe.Pointer, n, retoffset uint32)
func call1073741824(fn, arg unsafe.Pointer, n, retoffset uint32)

func systemstack_switch()

func prefetcht0(addr uintptr)
func prefetcht1(addr uintptr)
func prefetcht2(addr uintptr)
func prefetchnta(addr uintptr)

func unixnanotime() int64 {
	sec, nsec := time_now()
	return sec*1e9 + int64(nsec)
}

// round n up to a multiple of a.  a must be a power of 2.
func round(n, a uintptr) uintptr {
	return (n + a - 1) &^ (a - 1)
}

// checkASM returns whether assembly runtime checks have passed.
func checkASM() bool
