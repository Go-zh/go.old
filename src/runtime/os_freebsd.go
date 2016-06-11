// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/sys"
	"unsafe"
)

type mOS struct{}

//go:noescape
func thr_new(param *thrparam, size int32)

//go:noescape
func sigaltstack(new, old *stackt)

//go:noescape
func sigaction(sig int32, new, old *sigactiont)

//go:noescape
func sigprocmask(how int32, new, old *sigset)

//go:noescape
func setitimer(mode int32, new, old *itimerval)

//go:noescape
func sysctl(mib *uint32, miblen uint32, out *byte, size *uintptr, dst *byte, ndst uintptr) int32

//go:noescape
func getrlimit(kind int32, limit unsafe.Pointer) int32
func raise(sig int32)
func raiseproc(sig int32)

//go:noescape
func sys_umtx_op(addr *uint32, mode int32, val uint32, ptr2, ts *timespec) int32

func osyield()

// From FreeBSD's <sys/sysctl.h>
const (
	_CTL_HW  = 6
	_HW_NCPU = 3
)

var sigset_all = sigset{[4]uint32{^uint32(0), ^uint32(0), ^uint32(0), ^uint32(0)}}

func getncpu() int32 {
	mib := [2]uint32{_CTL_HW, _HW_NCPU}
	out := uint32(0)
	nout := unsafe.Sizeof(out)
	ret := sysctl(&mib[0], 2, (*byte)(unsafe.Pointer(&out)), &nout, nil, 0)
	if ret >= 0 {
		return int32(out)
	}
	return 1
}

// FreeBSD's umtx_op syscall is effectively the same as Linux's futex, and
// thus the code is largely similar. See Linux implementation
// and lock_futex.go for comments.

//go:nosplit
func futexsleep(addr *uint32, val uint32, ns int64) {
	systemstack(func() {
		futexsleep1(addr, val, ns)
	})
}

func futexsleep1(addr *uint32, val uint32, ns int64) {
	var tsp *timespec
	if ns >= 0 {
		var ts timespec
		ts.tv_nsec = 0
		ts.set_sec(int64(timediv(ns, 1000000000, (*int32)(unsafe.Pointer(&ts.tv_nsec)))))
		tsp = &ts
	}
	ret := sys_umtx_op(addr, _UMTX_OP_WAIT_UINT_PRIVATE, val, nil, tsp)
	if ret >= 0 || ret == -_EINTR {
		return
	}
	print("umtx_wait addr=", addr, " val=", val, " ret=", ret, "\n")
	*(*int32)(unsafe.Pointer(uintptr(0x1005))) = 0x1005
}

//go:nosplit
func futexwakeup(addr *uint32, cnt uint32) {
	ret := sys_umtx_op(addr, _UMTX_OP_WAKE_PRIVATE, cnt, nil, nil)
	if ret >= 0 {
		return
	}

	systemstack(func() {
		print("umtx_wake_addr=", addr, " ret=", ret, "\n")
	})
}

func thr_start()

// May run with m.p==nil, so write barriers are not allowed.
//go:nowritebarrier
func newosproc(mp *m, stk unsafe.Pointer) {
	if false {
		print("newosproc stk=", stk, " m=", mp, " g=", mp.g0, " thr_start=", funcPC(thr_start), " id=", mp.id, " ostk=", &mp, "\n")
	}

	// NOTE(rsc): This code is confused. stackbase is the top of the stack
	// and is equal to stk. However, it's working, so I'm not changing it.
	param := thrparam{
		start_func: funcPC(thr_start),
		arg:        unsafe.Pointer(mp),
		stack_base: mp.g0.stack.hi,
		stack_size: uintptr(stk) - mp.g0.stack.hi,
		child_tid:  unsafe.Pointer(&mp.procid),
		parent_tid: nil,
		tls_base:   unsafe.Pointer(&mp.tls[0]),
		tls_size:   unsafe.Sizeof(mp.tls),
	}

	var oset sigset
	sigprocmask(_SIG_SETMASK, &sigset_all, &oset)
	thr_new(&param, int32(unsafe.Sizeof(param)))
	sigprocmask(_SIG_SETMASK, &oset, nil)
}

func osinit() {
	ncpu = getncpu()
}

var urandom_dev = []byte("/dev/urandom\x00")

//go:nosplit
func getRandomData(r []byte) {
	fd := open(&urandom_dev[0], 0 /* O_RDONLY */, 0)
	n := read(fd, unsafe.Pointer(&r[0]), int32(len(r)))
	closefd(fd)
	extendRandom(r, int(n))
}

func goenvs() {
	goenvs_unix()
}

// Called to initialize a new m (including the bootstrap m).
// Called on the parent thread (main thread in case of bootstrap), can allocate memory.
func mpreinit(mp *m) {
	mp.gsignal = malg(32 * 1024)
	mp.gsignal.m = mp
}

//go:nosplit
func msigsave(mp *m) {
	sigprocmask(_SIG_SETMASK, nil, &mp.sigmask)
}

//go:nosplit
func msigrestore(sigmask sigset) {
	sigprocmask(_SIG_SETMASK, &sigmask, nil)
}

//go:nosplit
func sigblock() {
	sigprocmask(_SIG_SETMASK, &sigset_all, nil)
}

// Called to initialize a new m (including the bootstrap m).
// Called on the new thread, cannot allocate memory.
func minit() {
	_g_ := getg()

	// m.procid is a uint64, but thr_new writes a uint32 on 32-bit systems.
	// Fix it up. (Only matters on big-endian, but be clean anyway.)
	if sys.PtrSize == 4 {
		_g_.m.procid = uint64(*(*uint32)(unsafe.Pointer(&_g_.m.procid)))
	}

	// Initialize signal handling.
	var st stackt
	sigaltstack(nil, &st)
	if st.ss_flags&_SS_DISABLE != 0 {
		signalstack(&_g_.m.gsignal.stack)
		_g_.m.newSigstack = true
	} else {
		// Use existing signal stack.
		stsp := uintptr(unsafe.Pointer(st.ss_sp))
		_g_.m.gsignal.stack.lo = stsp
		_g_.m.gsignal.stack.hi = stsp + st.ss_size
		_g_.m.gsignal.stackguard0 = stsp + _StackGuard
		_g_.m.gsignal.stackguard1 = stsp + _StackGuard
		_g_.m.gsignal.stackAlloc = st.ss_size
		_g_.m.newSigstack = false
	}

	// restore signal mask from m.sigmask and unblock essential signals
	nmask := _g_.m.sigmask
	for i := range sigtable {
		if sigtable[i].flags&_SigUnblock != 0 {
			nmask.__bits[(i-1)/32] &^= 1 << ((uint32(i) - 1) & 31)
		}
	}
	sigprocmask(_SIG_SETMASK, &nmask, nil)
}

// Called from dropm to undo the effect of an minit.
//go:nosplit
func unminit() {
	if getg().m.newSigstack {
		signalstack(nil)
	}
}

func memlimit() uintptr {
	/*
		TODO: Convert to Go when something actually uses the result.
		Rlimit rl;
		extern byte runtime·text[], runtime·end[];
		uintptr used;

		if(runtime·getrlimit(RLIMIT_AS, &rl) != 0)
			return 0;
		if(rl.rlim_cur >= 0x7fffffff)
			return 0;

		// Estimate our VM footprint excluding the heap.
		// Not an exact science: use size of binary plus
		// some room for thread stacks.
		used = runtime·end - runtime·text + (64<<20);
		if(used >= rl.rlim_cur)
			return 0;

		// If there's not at least 16 MB left, we're probably
		// not going to be able to do much. Treat as no limit.
		rl.rlim_cur -= used;
		if(rl.rlim_cur < (16<<20))
			return 0;

		return rl.rlim_cur - used;
	*/

	return 0
}

func sigtramp()

type sigactiont struct {
	sa_handler uintptr
	sa_flags   int32
	sa_mask    sigset
}

//go:nosplit
//go:nowritebarrierrec
func setsig(i int32, fn uintptr, restart bool) {
	var sa sigactiont
	sa.sa_flags = _SA_SIGINFO | _SA_ONSTACK
	if restart {
		sa.sa_flags |= _SA_RESTART
	}
	sa.sa_mask = sigset_all
	if fn == funcPC(sighandler) {
		fn = funcPC(sigtramp)
	}
	sa.sa_handler = fn
	sigaction(i, &sa, nil)
}

//go:nosplit
//go:nowritebarrierrec
func setsigstack(i int32) {
	throw("setsigstack")
}

//go:nosplit
//go:nowritebarrierrec
func getsig(i int32) uintptr {
	var sa sigactiont
	sigaction(i, nil, &sa)
	if sa.sa_handler == funcPC(sigtramp) {
		return funcPC(sighandler)
	}
	return sa.sa_handler
}

//go:nosplit
func signalstack(s *stack) {
	var st stackt
	if s == nil {
		st.ss_flags = _SS_DISABLE
	} else {
		st.ss_sp = s.lo
		st.ss_size = s.hi - s.lo
		st.ss_flags = 0
	}
	sigaltstack(&st, nil)
}

//go:nosplit
//go:nowritebarrierrec
func updatesigmask(m [(_NSIG + 31) / 32]uint32) {
	var mask sigset
	copy(mask.__bits[:], m[:])
	sigprocmask(_SIG_SETMASK, &mask, nil)
}

func unblocksig(sig int32) {
	var mask sigset
	mask.__bits[(sig-1)/32] |= 1 << ((uint32(sig) - 1) & 31)
	sigprocmask(_SIG_UNBLOCK, &mask, nil)
}
