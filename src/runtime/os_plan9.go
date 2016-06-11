// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

type mOS struct {
	waitsemacount uint32
	notesig       *int8
	errstr        *byte
}

func closefd(fd int32) int32

//go:noescape
func open(name *byte, mode, perm int32) int32

//go:noescape
func pread(fd int32, buf unsafe.Pointer, nbytes int32, offset int64) int32

//go:noescape
func pwrite(fd int32, buf unsafe.Pointer, nbytes int32, offset int64) int32

func seek(fd int32, offset int64, whence int32) int64

//go:noescape
func exits(msg *byte)

//go:noescape
func brk_(addr unsafe.Pointer) int32

func sleep(ms int32) int32

func rfork(flags int32) int32

//go:noescape
func plan9_semacquire(addr *uint32, block int32) int32

//go:noescape
func plan9_tsemacquire(addr *uint32, ms int32) int32

//go:noescape
func plan9_semrelease(addr *uint32, count int32) int32

//go:noescape
func notify(fn unsafe.Pointer) int32

func noted(mode int32) int32

//go:noescape
func nsec(*int64) int64

//go:noescape
func sigtramp(ureg, msg unsafe.Pointer)

func setfpmasks()

//go:noescape
func tstart_plan9(newm *m)

func errstr() string

type _Plink uintptr

//go:linkname os_sigpipe os.sigpipe
func os_sigpipe() {
	throw("too many writes on closed pipe")
}

func sigpanic() {
	g := getg()
	if !canpanic(g) {
		throw("unexpected signal during runtime execution")
	}

	note := gostringnocopy((*byte)(unsafe.Pointer(g.m.notesig)))
	switch g.sig {
	case _SIGRFAULT, _SIGWFAULT:
		i := index(note, "addr=")
		if i >= 0 {
			i += 5
		} else if i = index(note, "va="); i >= 0 {
			i += 3
		} else {
			panicmem()
		}
		addr := note[i:]
		g.sigcode1 = uintptr(atolwhex(addr))
		if g.sigcode1 < 0x1000 || g.paniconfault {
			panicmem()
		}
		print("unexpected fault address ", hex(g.sigcode1), "\n")
		throw("fault")
	case _SIGTRAP:
		if g.paniconfault {
			panicmem()
		}
		throw(note)
	case _SIGINTDIV:
		panicdivide()
	case _SIGFLOAT:
		panicfloat()
	default:
		panic(errorString(note))
	}
}

func atolwhex(p string) int64 {
	for hasprefix(p, " ") || hasprefix(p, "\t") {
		p = p[1:]
	}
	neg := false
	if hasprefix(p, "-") || hasprefix(p, "+") {
		neg = p[0] == '-'
		p = p[1:]
		for hasprefix(p, " ") || hasprefix(p, "\t") {
			p = p[1:]
		}
	}
	var n int64
	switch {
	case hasprefix(p, "0x"), hasprefix(p, "0X"):
		p = p[2:]
		for ; len(p) > 0; p = p[1:] {
			if '0' <= p[0] && p[0] <= '9' {
				n = n*16 + int64(p[0]-'0')
			} else if 'a' <= p[0] && p[0] <= 'f' {
				n = n*16 + int64(p[0]-'a'+10)
			} else if 'A' <= p[0] && p[0] <= 'F' {
				n = n*16 + int64(p[0]-'A'+10)
			} else {
				break
			}
		}
	case hasprefix(p, "0"):
		for ; len(p) > 0 && '0' <= p[0] && p[0] <= '7'; p = p[1:] {
			n = n*8 + int64(p[0]-'0')
		}
	default:
		for ; len(p) > 0 && '0' <= p[0] && p[0] <= '9'; p = p[1:] {
			n = n*10 + int64(p[0]-'0')
		}
	}
	if neg {
		n = -n
	}
	return n
}

type sigset struct{}

// Called to initialize a new m (including the bootstrap m).
// Called on the parent thread (main thread in case of bootstrap), can allocate memory.
func mpreinit(mp *m) {
	// Initialize stack and goroutine for note handling.
	mp.gsignal = malg(32 * 1024)
	mp.gsignal.m = mp
	mp.notesig = (*int8)(mallocgc(_ERRMAX, nil, true))
	// Initialize stack for handling strings from the
	// errstr system call, as used in package syscall.
	mp.errstr = (*byte)(mallocgc(_ERRMAX, nil, true))
}

func msigsave(mp *m) {
}

func msigrestore(sigmask sigset) {
}

func sigblock() {
}

// Called to initialize a new m (including the bootstrap m).
// Called on the new thread, cannot allocate memory.
func minit() {
	if atomic.Load(&exiting) != 0 {
		exits(&emptystatus[0])
	}
	// Mask all SSE floating-point exceptions
	// when running on the 64-bit kernel.
	setfpmasks()
}

// Called from dropm to undo the effect of an minit.
func unminit() {
}

var sysstat = []byte("/dev/sysstat\x00")

func getproccount() int32 {
	var buf [2048]byte
	fd := open(&sysstat[0], _OREAD, 0)
	if fd < 0 {
		return 1
	}
	ncpu := int32(0)
	for {
		n := read(fd, unsafe.Pointer(&buf), int32(len(buf)))
		if n <= 0 {
			break
		}
		for i := int32(0); i < n; i++ {
			if buf[i] == '\n' {
				ncpu++
			}
		}
	}
	closefd(fd)
	if ncpu == 0 {
		ncpu = 1
	}
	return ncpu
}

var pid = []byte("#c/pid\x00")

func getpid() uint64 {
	var b [20]byte
	fd := open(&pid[0], 0, 0)
	if fd >= 0 {
		read(fd, unsafe.Pointer(&b), int32(len(b)))
		closefd(fd)
	}
	c := b[:]
	for c[0] == ' ' || c[0] == '\t' {
		c = c[1:]
	}
	return uint64(_atoi(c))
}

func osinit() {
	initBloc()
	ncpu = getproccount()
	getg().m.procid = getpid()
	notify(unsafe.Pointer(funcPC(sigtramp)))
}

func crash() {
	notify(nil)
	*(*int)(nil) = 0
}

//go:nosplit
func getRandomData(r []byte) {
	extendRandom(r, 0)
}

func goenvs() {
}

func initsig(preinit bool) {
}

//go:nosplit
func osyield() {
	sleep(0)
}

//go:nosplit
func usleep(µs uint32) {
	ms := int32(µs / 1000)
	if ms == 0 {
		ms = 1
	}
	sleep(ms)
}

//go:nosplit
func nanotime() int64 {
	var scratch int64
	ns := nsec(&scratch)
	// TODO(aram): remove hack after I fix _nsec in the pc64 kernel.
	if ns == 0 {
		return scratch
	}
	return ns
}

//go:nosplit
func itoa(buf []byte, val uint64) []byte {
	i := len(buf) - 1
	for val >= 10 {
		buf[i] = byte(val%10 + '0')
		i--
		val /= 10
	}
	buf[i] = byte(val + '0')
	return buf[i:]
}

var goexits = []byte("go: exit ")
var emptystatus = []byte("\x00")
var exiting uint32

func goexitsall(status *byte) {
	var buf [_ERRMAX]byte
	if !atomic.Cas(&exiting, 0, 1) {
		return
	}
	getg().m.locks++
	n := copy(buf[:], goexits)
	n = copy(buf[n:], gostringnocopy(status))
	pid := getpid()
	for mp := (*m)(atomic.Loadp(unsafe.Pointer(&allm))); mp != nil; mp = mp.alllink {
		if mp.procid != 0 && mp.procid != pid {
			postnote(mp.procid, buf[:])
		}
	}
	getg().m.locks--
}

var procdir = []byte("/proc/")
var notefile = []byte("/note\x00")

func postnote(pid uint64, msg []byte) int {
	var buf [128]byte
	var tmp [32]byte
	n := copy(buf[:], procdir)
	n += copy(buf[n:], itoa(tmp[:], pid))
	copy(buf[n:], notefile)
	fd := open(&buf[0], _OWRITE, 0)
	if fd < 0 {
		return -1
	}
	len := findnull(&msg[0])
	if write(uintptr(fd), unsafe.Pointer(&msg[0]), int32(len)) != int64(len) {
		closefd(fd)
		return -1
	}
	closefd(fd)
	return 0
}

//go:nosplit
func exit(e int) {
	var status []byte
	if e == 0 {
		status = emptystatus
	} else {
		// build error string
		var tmp [32]byte
		status = append(itoa(tmp[:len(tmp)-1], uint64(e)), 0)
	}
	goexitsall(&status[0])
	exits(&status[0])
}

// May run with m.p==nil, so write barriers are not allowed.
//go:nowritebarrier
func newosproc(mp *m, stk unsafe.Pointer) {
	if false {
		print("newosproc mp=", mp, " ostk=", &mp, "\n")
	}
	pid := rfork(_RFPROC | _RFMEM | _RFNOWAIT)
	if pid < 0 {
		throw("newosproc: rfork failed")
	}
	if pid == 0 {
		tstart_plan9(mp)
	}
}

//go:nosplit
func semacreate(mp *m) {
}

//go:nosplit
func semasleep(ns int64) int {
	_g_ := getg()
	if ns >= 0 {
		ms := timediv(ns, 1000000, nil)
		if ms == 0 {
			ms = 1
		}
		ret := plan9_tsemacquire(&_g_.m.waitsemacount, ms)
		if ret == 1 {
			return 0 // success
		}
		return -1 // timeout or interrupted
	}
	for plan9_semacquire(&_g_.m.waitsemacount, 1) < 0 {
		// interrupted; try again (c.f. lock_sema.go)
	}
	return 0 // success
}

//go:nosplit
func semawakeup(mp *m) {
	plan9_semrelease(&mp.waitsemacount, 1)
}

//go:nosplit
func read(fd int32, buf unsafe.Pointer, n int32) int32 {
	return pread(fd, buf, n, -1)
}

//go:nosplit
func write(fd uintptr, buf unsafe.Pointer, n int32) int64 {
	return int64(pwrite(int32(fd), buf, n, -1))
}

func memlimit() uint64 {
	return 0
}

var _badsignal = []byte("runtime: signal received on thread not created by Go.\n")

// This runs on a foreign stack, without an m or a g. No stack split.
//go:nosplit
func badsignal2() {
	pwrite(2, unsafe.Pointer(&_badsignal[0]), int32(len(_badsignal)), -1)
	exits(&_badsignal[0])
}

func raisebadsignal(sig int32) {
	badsignal2()
}

func _atoi(b []byte) int {
	n := 0
	for len(b) > 0 && '0' <= b[0] && b[0] <= '9' {
		n = n*10 + int(b[0]) - '0'
		b = b[1:]
	}
	return n
}

func signame(sig uint32) string {
	if sig >= uint32(len(sigtable)) {
		return ""
	}
	return sigtable[sig].name
}
