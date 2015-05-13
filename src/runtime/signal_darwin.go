// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

type sigTabT struct {
	flags int32
	name  string
}

var sigtable = [...]sigTabT{
	/* 0 */ {0, "SIGNONE: no trap"},
	/* 1 */ {_SigNotify + _SigKill, "SIGHUP: terminal line hangup"},
	/* 2 */ {_SigNotify + _SigKill, "SIGINT: interrupt"},
	/* 3 */ {_SigNotify + _SigThrow, "SIGQUIT: quit"},
	/* 4 */ {_SigThrow, "SIGILL: illegal instruction"},
	/* 5 */ {_SigThrow, "SIGTRAP: trace trap"},
	/* 6 */ {_SigNotify + _SigThrow, "SIGABRT: abort"},
	/* 7 */ {_SigThrow, "SIGEMT: emulate instruction executed"},
	/* 8 */ {_SigPanic, "SIGFPE: floating-point exception"},
	/* 9 */ {0, "SIGKILL: kill"},
	/* 10 */ {_SigPanic, "SIGBUS: bus error"},
	/* 11 */ {_SigPanic, "SIGSEGV: segmentation violation"},
	/* 12 */ {_SigThrow, "SIGSYS: bad system call"},
	/* 13 */ {_SigNotify, "SIGPIPE: write to broken pipe"},
	/* 14 */ {_SigNotify, "SIGALRM: alarm clock"},
	/* 15 */ {_SigNotify + _SigKill, "SIGTERM: termination"},
	/* 16 */ {_SigNotify, "SIGURG: urgent condition on socket"},
	/* 17 */ {0, "SIGSTOP: stop"},
	/* 18 */ {_SigNotify + _SigDefault, "SIGTSTP: keyboard stop"},
	/* 19 */ {0, "SIGCONT: continue after stop"},
	/* 20 */ {_SigNotify, "SIGCHLD: child status has changed"},
	/* 21 */ {_SigNotify + _SigDefault, "SIGTTIN: background read from tty"},
	/* 22 */ {_SigNotify + _SigDefault, "SIGTTOU: background write to tty"},
	/* 23 */ {_SigNotify, "SIGIO: i/o now possible"},
	/* 24 */ {_SigNotify, "SIGXCPU: cpu limit exceeded"},
	/* 25 */ {_SigNotify, "SIGXFSZ: file size limit exceeded"},
	/* 26 */ {_SigNotify, "SIGVTALRM: virtual alarm clock"},
	/* 27 */ {_SigNotify, "SIGPROF: profiling alarm clock"},
	/* 28 */ {_SigNotify, "SIGWINCH: window size change"},
	/* 29 */ {_SigNotify, "SIGINFO: status request from keyboard"},
	/* 30 */ {_SigNotify, "SIGUSR1: user-defined signal 1"},
	/* 31 */ {_SigNotify, "SIGUSR2: user-defined signal 2"},
}

//go:noescape
func sigfwd(fn uintptr, sig uint32, info *siginfo, ctx unsafe.Pointer)

//go:noescape
func sigreturn(ctx unsafe.Pointer, infostyle uint32)

//go:nosplit
func sigtrampgo(fn uintptr, infostyle, sig uint32, info *siginfo, ctx unsafe.Pointer) {
	if sigfwdgo(sig, info, ctx) {
		sigreturn(ctx, infostyle)
		return
	}
	g := getg()
	if g == nil {
		badsignal(uintptr(sig))
		sigreturn(ctx, infostyle)
		return
	}
	setg(g.m.gsignal)
	sighandler(sig, info, ctx, g)
	setg(g)
	sigreturn(ctx, infostyle)
}
