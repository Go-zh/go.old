// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector: finalizers and block profiling.

package runtime

import "unsafe"

type finblock struct {
	alllink *finblock
	next    *finblock
	cnt     int32
	_       int32
	fin     [(_FinBlockSize - 2*ptrSize - 2*4) / unsafe.Sizeof(finalizer{})]finalizer
}

var finlock mutex  // protects the following variables
var fing *g        // goroutine that runs finalizers
var finq *finblock // list of finalizers that are to be executed
var finc *finblock // cache of free blocks
var finptrmask [_FinBlockSize / ptrSize / 8]byte
var fingwait bool
var fingwake bool
var allfin *finblock // list of all blocks

// NOTE: Layout known to queuefinalizer.
type finalizer struct {
	fn   *funcval       // function to call
	arg  unsafe.Pointer // ptr to object
	nret uintptr        // bytes of return values from fn
	fint *_type         // type of first argument of fn
	ot   *ptrtype       // type of ptr to object
}

var finalizer1 = [...]byte{
	// Each Finalizer is 5 words, ptr ptr INT ptr ptr (INT = uintptr here)
	// Each byte describes 8 words.
	// Need 8 Finalizers described by 5 bytes before pattern repeats:
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	//	ptr ptr INT ptr ptr
	// aka
	//
	//	ptr ptr INT ptr ptr ptr ptr INT
	//	ptr ptr ptr ptr INT ptr ptr ptr
	//	ptr INT ptr ptr ptr ptr INT ptr
	//	ptr ptr ptr INT ptr ptr ptr ptr
	//	INT ptr ptr ptr ptr INT ptr ptr
	//
	// Assumptions about Finalizer layout checked below.
	1<<0 | 1<<1 | 0<<2 | 1<<3 | 1<<4 | 1<<5 | 1<<6 | 0<<7,
	1<<0 | 1<<1 | 1<<2 | 1<<3 | 0<<4 | 1<<5 | 1<<6 | 1<<7,
	1<<0 | 0<<1 | 1<<2 | 1<<3 | 1<<4 | 1<<5 | 0<<6 | 1<<7,
	1<<0 | 1<<1 | 1<<2 | 0<<3 | 1<<4 | 1<<5 | 1<<6 | 1<<7,
	0<<0 | 1<<1 | 1<<2 | 1<<3 | 1<<4 | 0<<5 | 1<<6 | 1<<7,
}

func queuefinalizer(p unsafe.Pointer, fn *funcval, nret uintptr, fint *_type, ot *ptrtype) {
	lock(&finlock)
	if finq == nil || finq.cnt == int32(len(finq.fin)) {
		if finc == nil {
			// Note: write barrier here, assigning to finc, but should be okay.
			finc = (*finblock)(persistentalloc(_FinBlockSize, 0, &memstats.gc_sys))
			finc.alllink = allfin
			allfin = finc
			if finptrmask[0] == 0 {
				// Build pointer mask for Finalizer array in block.
				// Check assumptions made in finalizer1 array above.
				if (unsafe.Sizeof(finalizer{}) != 5*ptrSize ||
					unsafe.Offsetof(finalizer{}.fn) != 0 ||
					unsafe.Offsetof(finalizer{}.arg) != ptrSize ||
					unsafe.Offsetof(finalizer{}.nret) != 2*ptrSize ||
					unsafe.Offsetof(finalizer{}.fint) != 3*ptrSize ||
					unsafe.Offsetof(finalizer{}.ot) != 4*ptrSize) {
					throw("finalizer out of sync")
				}
				for i := range finptrmask {
					finptrmask[i] = finalizer1[i%len(finalizer1)]
				}
			}
		}
		block := finc
		finc = block.next
		block.next = finq
		finq = block
	}
	f := &finq.fin[finq.cnt]
	finq.cnt++
	f.fn = fn
	f.nret = nret
	f.fint = fint
	f.ot = ot
	f.arg = p
	fingwake = true
	unlock(&finlock)
}

//go:nowritebarrier
func iterate_finq(callback func(*funcval, unsafe.Pointer, uintptr, *_type, *ptrtype)) {
	for fb := allfin; fb != nil; fb = fb.alllink {
		for i := int32(0); i < fb.cnt; i++ {
			f := &fb.fin[i]
			callback(f.fn, f.arg, f.nret, f.fint, f.ot)
		}
	}
}

func wakefing() *g {
	var res *g
	lock(&finlock)
	if fingwait && fingwake {
		fingwait = false
		fingwake = false
		res = fing
	}
	unlock(&finlock)
	return res
}

var (
	fingCreate  uint32
	fingRunning bool
)

func createfing() {
	// start the finalizer goroutine exactly once
	if fingCreate == 0 && cas(&fingCreate, 0, 1) {
		go runfinq()
	}
}

// This is the goroutine that runs all of the finalizers
func runfinq() {
	var (
		frame    unsafe.Pointer
		framecap uintptr
	)

	for {
		lock(&finlock)
		fb := finq
		finq = nil
		if fb == nil {
			gp := getg()
			fing = gp
			fingwait = true
			goparkunlock(&finlock, "finalizer wait", traceEvGoBlock, 1)
			continue
		}
		unlock(&finlock)
		if raceenabled {
			racefingo()
		}
		for fb != nil {
			for i := fb.cnt; i > 0; i-- {
				f := (*finalizer)(add(unsafe.Pointer(&fb.fin), uintptr(i-1)*unsafe.Sizeof(finalizer{})))

				framesz := unsafe.Sizeof((interface{})(nil)) + uintptr(f.nret)
				if framecap < framesz {
					// The frame does not contain pointers interesting for GC,
					// all not yet finalized objects are stored in finq.
					// If we do not mark it as FlagNoScan,
					// the last finalized object is not collected.
					frame = mallocgc(framesz, nil, flagNoScan)
					framecap = framesz
				}

				if f.fint == nil {
					throw("missing type in runfinq")
				}
				switch f.fint.kind & kindMask {
				case kindPtr:
					// direct use of pointer
					*(*unsafe.Pointer)(frame) = f.arg
				case kindInterface:
					ityp := (*interfacetype)(unsafe.Pointer(f.fint))
					// set up with empty interface
					(*eface)(frame)._type = &f.ot.typ
					(*eface)(frame).data = f.arg
					if len(ityp.mhdr) != 0 {
						// convert to interface with methods
						// this conversion is guaranteed to succeed - we checked in SetFinalizer
						assertE2I(ityp, *(*interface{})(frame), (*fInterface)(frame))
					}
				default:
					throw("bad kind in runfinq")
				}
				fingRunning = true
				reflectcall(nil, unsafe.Pointer(f.fn), frame, uint32(framesz), uint32(framesz))
				fingRunning = false

				// drop finalizer queue references to finalized object
				f.fn = nil
				f.arg = nil
				f.ot = nil
				fb.cnt = i - 1
			}
			next := fb.next
			lock(&finlock)
			fb.next = finc
			finc = fb
			unlock(&finlock)
			fb = next
		}
	}
}

// SetFinalizer sets the finalizer associated with x to f.
// When the garbage collector finds an unreachable block
// with an associated finalizer, it clears the association and runs
// f(x) in a separate goroutine.  This makes x reachable again, but
// now without an associated finalizer.  Assuming that SetFinalizer
// is not called again, the next time the garbage collector sees
// that x is unreachable, it will free x.
//
// SetFinalizer(x, nil) clears any finalizer associated with x.
//
// The argument x must be a pointer to an object allocated by
// calling new or by taking the address of a composite literal.
// The argument f must be a function that takes a single argument
// to which x's type can be assigned, and can have arbitrary ignored return
// values. If either of these is not true, SetFinalizer aborts the
// program.
//
// Finalizers are run in dependency order: if A points at B, both have
// finalizers, and they are otherwise unreachable, only the finalizer
// for A runs; once A is freed, the finalizer for B can run.
// If a cyclic structure includes a block with a finalizer, that
// cycle is not guaranteed to be garbage collected and the finalizer
// is not guaranteed to run, because there is no ordering that
// respects the dependencies.
//
// The finalizer for x is scheduled to run at some arbitrary time after
// x becomes unreachable.
// There is no guarantee that finalizers will run before a program exits,
// so typically they are useful only for releasing non-memory resources
// associated with an object during a long-running program.
// For example, an os.File object could use a finalizer to close the
// associated operating system file descriptor when a program discards
// an os.File without calling Close, but it would be a mistake
// to depend on a finalizer to flush an in-memory I/O buffer such as a
// bufio.Writer, because the buffer would not be flushed at program exit.
//
// It is not guaranteed that a finalizer will run if the size of *x is
// zero bytes.
//
// It is not guaranteed that a finalizer will run for objects allocated
// in initializers for package-level variables. Such objects may be
// linker-allocated, not heap-allocated.
//
// A single goroutine runs all finalizers for a program, sequentially.
// If a finalizer must run for a long time, it should do so by starting
// a new goroutine.

// SetFinalizer 为 f 设置与 x 相关联的终结器。
// 当垃圾回收器找到一个无法访问的块及与其相关联的终结器时，就会清理该关联，
// 并在一个独立的Go程中运行f(x)。这会使 x 再次变得可访问，但现在没有了相关联的终结器。
// 假设 SetFinalizer 未被再次调用，当下一次垃圾回收器发现 x 无法访问时，就会释放 x。
//
// SetFinalizer(x, nil) 会清理任何与 x 相关联的终结器。
//
// 实参 x 必须是一个对象的指针，该对象通过调用新的或获取一个复合字面地址来分配。
// 实参 f 必须是一个函数，该函数获取一个 x 的类型的单一实参，并拥有可任意忽略的返回值。
// 只要这些条件有一个不满足，SetFinalizer 就会跳过该程序。
//
// 终结器按照依赖顺序运行：若 A 指向 B，则二者都有终结器，当只有 A 的终结器运行时，
// 它们才无法访问；一旦 A 被释放，则 B 的终结器便可运行。若循环依赖的结构包含块及其终结器，
// 则该循环并不能保证被垃圾回收，而其终结器并不能保证运行，这是因为其依赖没有顺序。
//
// x 的终结器预定为在 x 无法访问后的任意时刻运行。无法保证终结器会在程序退出前运行，
// 因此它们通常只在长时间运行的程序中释放一个关联至对象的非内存资源时使用。
// 例如，当程序丢弃 os.File 而没有调用 Close 时，该 os.File 对象便可使用一个终结器
// 来关闭与其相关联的操作系统文件描述符，但依赖终结器去刷新一个内存中的I/O缓存是错误的，
// 因为该缓存不会在程序退出时被刷新。
//
// 一个程序的单个Go程会按顺序运行所有的终结器。若某个终结器需要长时间运行，
// 它应当通过开始一个新的Go程来继续。
// TODO(osc): 仍需校对及语句优化
func SetFinalizer(obj interface{}, finalizer interface{}) {
	if debug.sbrk != 0 {
		// debug.sbrk never frees memory, so no finalizers run
		// (and we don't have the data structures to record them).
		return
	}
	e := (*eface)(unsafe.Pointer(&obj))
	etyp := e._type
	if etyp == nil {
		throw("runtime.SetFinalizer: first argument is nil")
	}
	if etyp.kind&kindMask != kindPtr {
		throw("runtime.SetFinalizer: first argument is " + *etyp._string + ", not pointer")
	}
	ot := (*ptrtype)(unsafe.Pointer(etyp))
	if ot.elem == nil {
		throw("nil elem type!")
	}

	// find the containing object
	_, base, _ := findObject(e.data)

	if base == nil {
		// 0-length objects are okay.
		if e.data == unsafe.Pointer(&zerobase) {
			return
		}

		// Global initializers might be linker-allocated.
		//	var Foo = &Object{}
		//	func main() {
		//		runtime.SetFinalizer(Foo, nil)
		//	}
		// The relevant segments are: noptrdata, data, bss, noptrbss.
		// We cannot assume they are in any order or even contiguous,
		// due to external linking.
		for datap := &firstmoduledata; datap != nil; datap = datap.next {
			if datap.noptrdata <= uintptr(e.data) && uintptr(e.data) < datap.enoptrdata ||
				datap.data <= uintptr(e.data) && uintptr(e.data) < datap.edata ||
				datap.bss <= uintptr(e.data) && uintptr(e.data) < datap.ebss ||
				datap.noptrbss <= uintptr(e.data) && uintptr(e.data) < datap.enoptrbss {
				return
			}
		}
		throw("runtime.SetFinalizer: pointer not in allocated block")
	}

	if e.data != base {
		// As an implementation detail we allow to set finalizers for an inner byte
		// of an object if it could come from tiny alloc (see mallocgc for details).
		if ot.elem == nil || ot.elem.kind&kindNoPointers == 0 || ot.elem.size >= maxTinySize {
			throw("runtime.SetFinalizer: pointer not at beginning of allocated block")
		}
	}

	f := (*eface)(unsafe.Pointer(&finalizer))
	ftyp := f._type
	if ftyp == nil {
		// switch to system stack and remove finalizer
		systemstack(func() {
			removefinalizer(e.data)
		})
		return
	}

	if ftyp.kind&kindMask != kindFunc {
		throw("runtime.SetFinalizer: second argument is " + *ftyp._string + ", not a function")
	}
	ft := (*functype)(unsafe.Pointer(ftyp))
	ins := *(*[]*_type)(unsafe.Pointer(&ft.in))
	if ft.dotdotdot || len(ins) != 1 {
		throw("runtime.SetFinalizer: cannot pass " + *etyp._string + " to finalizer " + *ftyp._string)
	}
	fint := ins[0]
	switch {
	case fint == etyp:
		// ok - same type
		goto okarg
	case fint.kind&kindMask == kindPtr:
		if (fint.x == nil || fint.x.name == nil || etyp.x == nil || etyp.x.name == nil) && (*ptrtype)(unsafe.Pointer(fint)).elem == ot.elem {
			// ok - not same type, but both pointers,
			// one or the other is unnamed, and same element type, so assignable.
			goto okarg
		}
	case fint.kind&kindMask == kindInterface:
		ityp := (*interfacetype)(unsafe.Pointer(fint))
		if len(ityp.mhdr) == 0 {
			// ok - satisfies empty interface
			goto okarg
		}
		if assertE2I2(ityp, obj, nil) {
			goto okarg
		}
	}
	throw("runtime.SetFinalizer: cannot pass " + *etyp._string + " to finalizer " + *ftyp._string)
okarg:
	// compute size needed for return parameters
	nret := uintptr(0)
	for _, t := range *(*[]*_type)(unsafe.Pointer(&ft.out)) {
		nret = round(nret, uintptr(t.align)) + uintptr(t.size)
	}
	nret = round(nret, ptrSize)

	// make sure we have a finalizer goroutine
	createfing()

	systemstack(func() {
		if !addfinalizer(e.data, (*funcval)(f.data), nret, fint, ot) {
			throw("runtime.SetFinalizer: finalizer already set")
		}
	})
}

// Look up pointer v in heap.  Return the span containing the object,
// the start of the object, and the size of the object.  If the object
// does not exist, return nil, nil, 0.
func findObject(v unsafe.Pointer) (s *mspan, x unsafe.Pointer, n uintptr) {
	c := gomcache()
	c.local_nlookup++
	if ptrSize == 4 && c.local_nlookup >= 1<<30 {
		// purge cache stats to prevent overflow
		lock(&mheap_.lock)
		purgecachedstats(c)
		unlock(&mheap_.lock)
	}

	// find span
	arena_start := uintptr(unsafe.Pointer(mheap_.arena_start))
	arena_used := uintptr(unsafe.Pointer(mheap_.arena_used))
	if uintptr(v) < arena_start || uintptr(v) >= arena_used {
		return
	}
	p := uintptr(v) >> pageShift
	q := p - arena_start>>pageShift
	s = *(**mspan)(add(unsafe.Pointer(mheap_.spans), q*ptrSize))
	if s == nil {
		return
	}
	x = unsafe.Pointer(uintptr(s.start) << pageShift)

	if uintptr(v) < uintptr(x) || uintptr(v) >= uintptr(unsafe.Pointer(s.limit)) || s.state != mSpanInUse {
		s = nil
		x = nil
		return
	}

	n = uintptr(s.elemsize)
	if s.sizeclass != 0 {
		x = add(x, (uintptr(v)-uintptr(x))/n*n)
	}
	return
}
