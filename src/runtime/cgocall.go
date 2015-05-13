// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Cgo call and callback support.
//
// To call into the C function f from Go, the cgo-generated code calls
// runtime.cgocall(_cgo_Cfunc_f, frame), where _cgo_Cfunc_f is a
// gcc-compiled function written by cgo.
//
// runtime.cgocall (below) locks g to m, calls entersyscall
// so as not to block other goroutines or the garbage collector,
// and then calls runtime.asmcgocall(_cgo_Cfunc_f, frame).
//
// runtime.asmcgocall (in asm_$GOARCH.s) switches to the m->g0 stack
// (assumed to be an operating system-allocated stack, so safe to run
// gcc-compiled code on) and calls _cgo_Cfunc_f(frame).
//
// _cgo_Cfunc_f invokes the actual C function f with arguments
// taken from the frame structure, records the results in the frame,
// and returns to runtime.asmcgocall.
//
// After it regains control, runtime.asmcgocall switches back to the
// original g (m->curg)'s stack and returns to runtime.cgocall.
//
// After it regains control, runtime.cgocall calls exitsyscall, which blocks
// until this m can run Go code without violating the $GOMAXPROCS limit,
// and then unlocks g from m.
//
// The above description skipped over the possibility of the gcc-compiled
// function f calling back into Go.  If that happens, we continue down
// the rabbit hole during the execution of f.
//
// To make it possible for gcc-compiled C code to call a Go function p.GoF,
// cgo writes a gcc-compiled function named GoF (not p.GoF, since gcc doesn't
// know about packages).  The gcc-compiled C function f calls GoF.
//
// GoF calls crosscall2(_cgoexp_GoF, frame, framesize).  Crosscall2
// (in cgo/gcc_$GOARCH.S, a gcc-compiled assembly file) is a two-argument
// adapter from the gcc function call ABI to the 6c function call ABI.
// It is called from gcc to call 6c functions.  In this case it calls
// _cgoexp_GoF(frame, framesize), still running on m->g0's stack
// and outside the $GOMAXPROCS limit.  Thus, this code cannot yet
// call arbitrary Go code directly and must be careful not to allocate
// memory or use up m->g0's stack.
//
// _cgoexp_GoF calls runtime.cgocallback(p.GoF, frame, framesize).
// (The reason for having _cgoexp_GoF instead of writing a crosscall3
// to make this call directly is that _cgoexp_GoF, because it is compiled
// with 6c instead of gcc, can refer to dotted names like
// runtime.cgocallback and p.GoF.)
//
// runtime.cgocallback (in asm_$GOARCH.s) switches from m->g0's
// stack to the original g (m->curg)'s stack, on which it calls
// runtime.cgocallbackg(p.GoF, frame, framesize).
// As part of the stack switch, runtime.cgocallback saves the current
// SP as m->g0->sched.sp, so that any use of m->g0's stack during the
// execution of the callback will be done below the existing stack frames.
// Before overwriting m->g0->sched.sp, it pushes the old value on the
// m->g0 stack, so that it can be restored later.
//
// runtime.cgocallbackg (below) is now running on a real goroutine
// stack (not an m->g0 stack).  First it calls runtime.exitsyscall, which will
// block until the $GOMAXPROCS limit allows running this goroutine.
// Once exitsyscall has returned, it is safe to do things like call the memory
// allocator or invoke the Go callback function p.GoF.  runtime.cgocallbackg
// first defers a function to unwind m->g0.sched.sp, so that if p.GoF
// panics, m->g0.sched.sp will be restored to its old value: the m->g0 stack
// and the m->curg stack will be unwound in lock step.
// Then it calls p.GoF.  Finally it pops but does not execute the deferred
// function, calls runtime.entersyscall, and returns to runtime.cgocallback.
//
// After it regains control, runtime.cgocallback switches back to
// m->g0's stack (the pointer is still in m->g0.sched.sp), restores the old
// m->g0.sched.sp value from the stack, and returns to _cgoexp_GoF.
//
// _cgoexp_GoF immediately returns to crosscall2, which restores the
// callee-save registers for gcc and returns to GoF, which returns to f.

package runtime

import "unsafe"

// Call from Go to C.
//go:nosplit
func cgocall(fn, arg unsafe.Pointer) {
	cgocall_errno(fn, arg)
}

//go:nosplit
func cgocall_errno(fn, arg unsafe.Pointer) int32 {
	if !iscgo && GOOS != "solaris" && GOOS != "windows" {
		throw("cgocall unavailable")
	}

	if fn == nil {
		throw("cgocall nil")
	}

	if raceenabled {
		racereleasemerge(unsafe.Pointer(&racecgosync))
	}

	/*
	 * Lock g to m to ensure we stay on the same stack if we do a
	 * cgo callback. Add entry to defer stack in case of panic.
	 */
	lockOSThread()
	mp := getg().m
	mp.ncgocall++
	mp.ncgo++
	defer endcgo(mp)

	/*
	 * Announce we are entering a system call
	 * so that the scheduler knows to create another
	 * M to run goroutines while we are in the
	 * foreign code.
	 *
	 * The call to asmcgocall is guaranteed not to
	 * split the stack and does not allocate memory,
	 * so it is safe to call while "in a system call", outside
	 * the $GOMAXPROCS accounting.
	 */
	entersyscall(0)
	errno := asmcgocall_errno(fn, arg)
	exitsyscall(0)

	return errno
}

//go:nosplit
func endcgo(mp *m) {
	mp.ncgo--

	if raceenabled {
		raceacquire(unsafe.Pointer(&racecgosync))
	}

	unlockOSThread() // invalidates mp
}

// Helper functions for cgo code.

func cmalloc(n uintptr) unsafe.Pointer {
	var args struct {
		n   uint64
		ret unsafe.Pointer
	}
	args.n = uint64(n)
	cgocall(_cgo_malloc, unsafe.Pointer(&args))
	if args.ret == nil {
		throw("C malloc failed")
	}
	return args.ret
}

func cfree(p unsafe.Pointer) {
	cgocall(_cgo_free, p)
}

// Call from C back to Go.
//go:nosplit
func cgocallbackg() {
	gp := getg()
	if gp != gp.m.curg {
		println("runtime: bad g in cgocallback")
		exit(2)
	}

	// entersyscall saves the caller's SP to allow the GC to trace the Go
	// stack. However, since we're returning to an earlier stack frame and
	// need to pair with the entersyscall() call made by cgocall, we must
	// save syscall* and let reentersyscall restore them.
	savedsp := unsafe.Pointer(gp.syscallsp)
	savedpc := gp.syscallpc
	exitsyscall(0) // coming out of cgo call
	cgocallbackg1()
	// going back to cgo call
	reentersyscall(savedpc, uintptr(savedsp))
}

func cgocallbackg1() {
	gp := getg()
	if gp.m.needextram {
		gp.m.needextram = false
		systemstack(newextram)
	}

	if gp.m.ncgo == 0 {
		// The C call to Go came from a thread not currently running
		// any Go. In the case of -buildmode=c-archive or c-shared,
		// this call may be coming in before package initialization
		// is complete. Wait until it is.
		<-main_init_done
	}

	// Add entry to defer stack in case of panic.
	restore := true
	defer unwindm(&restore)

	if raceenabled {
		raceacquire(unsafe.Pointer(&racecgosync))
	}

	type args struct {
		fn      *funcval
		arg     unsafe.Pointer
		argsize uintptr
	}
	var cb *args

	// Location of callback arguments depends on stack frame layout
	// and size of stack frame of cgocallback_gofunc.
	sp := gp.m.g0.sched.sp
	switch GOARCH {
	default:
		throw("cgocallbackg is unimplemented on arch")
	case "arm":
		// On arm, stack frame is two words and there's a saved LR between
		// SP and the stack frame and between the stack frame and the arguments.
		cb = (*args)(unsafe.Pointer(sp + 4*ptrSize))
	case "arm64":
		// On arm64, stack frame is four words and there's a saved LR between
		// SP and the stack frame and between the stack frame and the arguments.
		cb = (*args)(unsafe.Pointer(sp + 5*ptrSize))
	case "amd64":
		// On amd64, stack frame is one word, plus caller PC.
		if framepointer_enabled {
			// In this case, there's also saved BP.
			cb = (*args)(unsafe.Pointer(sp + 3*ptrSize))
			break
		}
		cb = (*args)(unsafe.Pointer(sp + 2*ptrSize))
	case "386":
		// On 386, stack frame is three words, plus caller PC.
		cb = (*args)(unsafe.Pointer(sp + 4*ptrSize))
	case "ppc64", "ppc64le":
		// On ppc64, stack frame is two words and there's a
		// saved LR between SP and the stack frame and between
		// the stack frame and the arguments.
		cb = (*args)(unsafe.Pointer(sp + 4*ptrSize))
	}

	// Invoke callback.
	// NOTE(rsc): passing nil for argtype means that the copying of the
	// results back into cb.arg happens without any corresponding write barriers.
	// For cgo, cb.arg points into a C stack frame and therefore doesn't
	// hold any pointers that the GC can find anyway - the write barrier
	// would be a no-op.
	reflectcall(nil, unsafe.Pointer(cb.fn), unsafe.Pointer(cb.arg), uint32(cb.argsize), 0)

	if raceenabled {
		racereleasemerge(unsafe.Pointer(&racecgosync))
	}

	// Do not unwind m->g0->sched.sp.
	// Our caller, cgocallback, will do that.
	restore = false
}

func unwindm(restore *bool) {
	if !*restore {
		return
	}
	// Restore sp saved by cgocallback during
	// unwind of g's stack (see comment at top of file).
	mp := acquirem()
	sched := &mp.g0.sched
	switch GOARCH {
	default:
		throw("unwindm not implemented")
	case "386", "amd64":
		sched.sp = *(*uintptr)(unsafe.Pointer(sched.sp))
	case "arm":
		sched.sp = *(*uintptr)(unsafe.Pointer(sched.sp + 4))
	case "arm64":
		sched.sp = *(*uintptr)(unsafe.Pointer(sched.sp + 16))
	case "ppc64", "ppc64le":
		sched.sp = *(*uintptr)(unsafe.Pointer(sched.sp + 8))
	}
	releasem(mp)
}

// called from assembly
func badcgocallback() {
	throw("misaligned stack in cgocallback")
}

// called from (incomplete) assembly
func cgounimpl() {
	throw("cgo not implemented")
}

var racecgosync uint64 // represents possible synchronization in C code
