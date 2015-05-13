// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector: marking and scanning

package runtime

import "unsafe"

// Scan all of the stacks, greying (or graying if in America) the referents
// but not blackening them since the mark write barrier isn't installed.
//go:nowritebarrier
func gcscan_m() {
	_g_ := getg()

	// Grab the g that called us and potentially allow rescheduling.
	// This allows it to be scanned like other goroutines.
	mastergp := _g_.m.curg
	casgstatus(mastergp, _Grunning, _Gwaiting)
	mastergp.waitreason = "garbage collection scan"

	// Span sweeping has been done by finishsweep_m.
	// Long term we will want to make this goroutine runnable
	// by placing it onto a scanenqueue state and then calling
	// runtime·restartg(mastergp) to make it Grunnable.
	// At the bottom we will want to return this p back to the scheduler.

	// Prepare flag indicating that the scan has not been completed.
	local_allglen := gcResetGState()

	work.nwait = 0
	work.ndone = 0
	work.nproc = 1 // For now do not do this in parallel.
	//	ackgcphase is not needed since we are not scanning running goroutines.
	parforsetup(work.markfor, work.nproc, uint32(_RootCount+local_allglen), false, markroot)
	parfordo(work.markfor)

	lock(&allglock)
	// Check that gc work is done.
	for i := 0; i < local_allglen; i++ {
		gp := allgs[i]
		if !gp.gcworkdone {
			throw("scan missed a g")
		}
	}
	unlock(&allglock)

	casgstatus(mastergp, _Gwaiting, _Grunning)
	// Let the g that called us continue to run.
}

// ptrmask for an allocation containing a single pointer.
var oneptrmask = [...]uint8{1}

//go:nowritebarrier
func markroot(desc *parfor, i uint32) {
	// TODO: Consider using getg().m.p.ptr().gcw.
	var gcw gcWork

	// Note: if you add a case here, please also update heapdump.go:dumproots.
	switch i {
	case _RootData:
		for datap := &firstmoduledata; datap != nil; datap = datap.next {
			scanblock(datap.data, datap.edata-datap.data, datap.gcdatamask.bytedata, &gcw)
		}

	case _RootBss:
		for datap := &firstmoduledata; datap != nil; datap = datap.next {
			scanblock(datap.bss, datap.ebss-datap.bss, datap.gcbssmask.bytedata, &gcw)
		}

	case _RootFinalizers:
		for fb := allfin; fb != nil; fb = fb.alllink {
			scanblock(uintptr(unsafe.Pointer(&fb.fin[0])), uintptr(fb.cnt)*unsafe.Sizeof(fb.fin[0]), &finptrmask[0], &gcw)
		}

	case _RootSpans:
		// mark MSpan.specials
		sg := mheap_.sweepgen
		for spanidx := uint32(0); spanidx < uint32(len(work.spans)); spanidx++ {
			s := work.spans[spanidx]
			if s.state != mSpanInUse {
				continue
			}
			if !useCheckmark && s.sweepgen != sg {
				// sweepgen was updated (+2) during non-checkmark GC pass
				print("sweep ", s.sweepgen, " ", sg, "\n")
				throw("gc: unswept span")
			}
			for sp := s.specials; sp != nil; sp = sp.next {
				if sp.kind != _KindSpecialFinalizer {
					continue
				}
				// don't mark finalized object, but scan it so we
				// retain everything it points to.
				spf := (*specialfinalizer)(unsafe.Pointer(sp))
				// A finalizer can be set for an inner byte of an object, find object beginning.
				p := uintptr(s.start<<_PageShift) + uintptr(spf.special.offset)/s.elemsize*s.elemsize
				if gcphase != _GCscan {
					scanobject(p, &gcw) // scanned during mark termination
				}
				scanblock(uintptr(unsafe.Pointer(&spf.fn)), ptrSize, &oneptrmask[0], &gcw)
			}
		}

	case _RootFlushCaches:
		if gcphase != _GCscan { // Do not flush mcaches during GCscan phase.
			flushallmcaches()
		}

	default:
		// the rest is scanning goroutine stacks
		if uintptr(i-_RootCount) >= allglen {
			throw("markroot: bad index")
		}
		gp := allgs[i-_RootCount]

		// remember when we've first observed the G blocked
		// needed only to output in traceback
		status := readgstatus(gp) // We are not in a scan state
		if (status == _Gwaiting || status == _Gsyscall) && gp.waitsince == 0 {
			gp.waitsince = work.tstart
		}

		// Shrink a stack if not much of it is being used but not in the scan phase.
		if gcphase == _GCmarktermination {
			// Shrink during STW GCmarktermination phase thus avoiding
			// complications introduced by shrinking during
			// non-STW phases.
			shrinkstack(gp)
		}
		if readgstatus(gp) == _Gdead {
			gp.gcworkdone = true
		} else {
			gp.gcworkdone = false
		}
		restart := stopg(gp)

		// goroutine will scan its own stack when it stops running.
		// Wait until it has.
		for readgstatus(gp) == _Grunning && !gp.gcworkdone {
		}

		// scanstack(gp) is done as part of gcphasework
		// But to make sure we finished we need to make sure that
		// the stack traps have all responded so drop into
		// this while loop until they respond.
		for !gp.gcworkdone {
			status = readgstatus(gp)
			if status == _Gdead {
				gp.gcworkdone = true // scan is a noop
				break
			}
			if status == _Gwaiting || status == _Grunnable {
				restart = stopg(gp)
			}
		}
		if restart {
			restartg(gp)
		}
	}

	gcw.dispose()
}

// gcAssistAlloc records and allocation of size bytes and, if
// allowAssist is true, may assist GC scanning in proportion to the
// allocations performed by this mutator since the last assist.
//
// It should only be called if gcAssistAlloc != 0.
//
// This must be called with preemption disabled.
//go:nowritebarrier
func gcAssistAlloc(size uintptr, allowAssist bool) {
	// Find the G responsible for this assist.
	gp := getg()
	if gp.m.curg != nil {
		gp = gp.m.curg
	}

	// Record allocation.
	gp.gcalloc += size

	if !allowAssist {
		return
	}

	// Compute the amount of assist scan work we need to do.
	scanWork := int64(gcController.assistRatio*float64(gp.gcalloc)) - gp.gcscanwork
	// scanWork can be negative if the last assist scanned a large
	// object and we're still ahead of our assist goal.
	if scanWork <= 0 {
		return
	}

	// Steal as much credit as we can from the background GC's
	// scan credit. This is racy and may drop the background
	// credit below 0 if two mutators steal at the same time. This
	// will just cause steals to fail until credit is accumulated
	// again, so in the long run it doesn't really matter, but we
	// do have to handle the negative credit case.
	bgScanCredit := atomicloadint64(&gcController.bgScanCredit)
	stolen := int64(0)
	if bgScanCredit > 0 {
		if bgScanCredit < scanWork {
			stolen = bgScanCredit
		} else {
			stolen = scanWork
		}
		xaddint64(&gcController.bgScanCredit, -scanWork)

		scanWork -= stolen
		gp.gcscanwork += stolen

		if scanWork == 0 {
			return
		}
	}

	// Perform assist work
	systemstack(func() {
		// Track time spent in this assist. Since we're on the
		// system stack, this is non-preemptible, so we can
		// just measure start and end time.
		startTime := nanotime()

		xadd(&work.nwait, -1)

		// drain own cached work first in the hopes that it
		// will be more cache friendly.
		gcw := &getg().m.p.ptr().gcw
		startScanWork := gcw.scanWork
		gcDrainN(gcw, scanWork)
		// Record that we did this much scan work.
		gp.gcscanwork += gcw.scanWork - startScanWork
		// No need to dispose since we're not in mark termination.

		// If this is the last worker and we ran out of work,
		// signal a completion point.
		if xadd(&work.nwait, +1) == work.nproc && work.full == 0 && work.partial == 0 {
			// This has reached a background completion
			// point.
			gcBgMarkDone()
		}

		duration := nanotime() - startTime
		_p_ := gp.m.p.ptr()
		_p_.gcAssistTime += duration
		if _p_.gcAssistTime > gcAssistTimeSlack {
			xaddint64(&gcController.assistTime, _p_.gcAssistTime)
			_p_.gcAssistTime = 0
		}
	})
}

// The gp has been moved to a GC safepoint. GC phase specific
// work is done here.
//go:nowritebarrier
func gcphasework(gp *g) {
	switch gcphase {
	default:
		throw("gcphasework in bad gcphase")
	case _GCoff, _GCquiesce, _GCstw, _GCsweep:
		// No work.
	case _GCscan:
		// scan the stack, mark the objects, put pointers in work buffers
		// hanging off the P where this is being run.
		// Indicate that the scan is valid until the goroutine runs again
		scanstack(gp)
	case _GCmark:
		// No work.
	case _GCmarktermination:
		scanstack(gp)
		// All available mark work will be emptied before returning.
	}
	gp.gcworkdone = true
}

//go:nowritebarrier
func scanstack(gp *g) {
	if gp.gcscanvalid {
		return
	}

	if readgstatus(gp)&_Gscan == 0 {
		print("runtime:scanstack: gp=", gp, ", goid=", gp.goid, ", gp->atomicstatus=", hex(readgstatus(gp)), "\n")
		throw("scanstack - bad status")
	}

	switch readgstatus(gp) &^ _Gscan {
	default:
		print("runtime: gp=", gp, ", goid=", gp.goid, ", gp->atomicstatus=", readgstatus(gp), "\n")
		throw("mark - bad status")
	case _Gdead:
		return
	case _Grunning:
		print("runtime: gp=", gp, ", goid=", gp.goid, ", gp->atomicstatus=", readgstatus(gp), "\n")
		throw("scanstack: goroutine not stopped")
	case _Grunnable, _Gsyscall, _Gwaiting:
		// ok
	}

	if gp == getg() {
		throw("can't scan our own stack")
	}
	mp := gp.m
	if mp != nil && mp.helpgc != 0 {
		throw("can't scan gchelper stack")
	}

	gcw := &getg().m.p.ptr().gcw
	scanframe := func(frame *stkframe, unused unsafe.Pointer) bool {
		// Pick up gcw as free variable so gentraceback and friends can
		// keep the same signature.
		scanframeworker(frame, unused, gcw)
		return true
	}
	gentraceback(^uintptr(0), ^uintptr(0), 0, gp, 0, nil, 0x7fffffff, scanframe, nil, 0)
	tracebackdefers(gp, scanframe, nil)
	if gcphase == _GCmarktermination {
		gcw.dispose()
	}
	gp.gcscanvalid = true
}

// Scan a stack frame: local variables and function arguments/results.
//go:nowritebarrier
func scanframeworker(frame *stkframe, unused unsafe.Pointer, gcw *gcWork) {

	f := frame.fn
	targetpc := frame.continpc
	if targetpc == 0 {
		// Frame is dead.
		return
	}
	if _DebugGC > 1 {
		print("scanframe ", funcname(f), "\n")
	}
	if targetpc != f.entry {
		targetpc--
	}
	pcdata := pcdatavalue(f, _PCDATA_StackMapIndex, targetpc)
	if pcdata == -1 {
		// We do not have a valid pcdata value but there might be a
		// stackmap for this function.  It is likely that we are looking
		// at the function prologue, assume so and hope for the best.
		pcdata = 0
	}

	// Scan local variables if stack frame has been allocated.
	size := frame.varp - frame.sp
	var minsize uintptr
	switch thechar {
	case '6', '8':
		minsize = 0
	case '7':
		minsize = spAlign
	default:
		minsize = ptrSize
	}
	if size > minsize {
		stkmap := (*stackmap)(funcdata(f, _FUNCDATA_LocalsPointerMaps))
		if stkmap == nil || stkmap.n <= 0 {
			print("runtime: frame ", funcname(f), " untyped locals ", hex(frame.varp-size), "+", hex(size), "\n")
			throw("missing stackmap")
		}

		// Locals bitmap information, scan just the pointers in locals.
		if pcdata < 0 || pcdata >= stkmap.n {
			// don't know where we are
			print("runtime: pcdata is ", pcdata, " and ", stkmap.n, " locals stack map entries for ", funcname(f), " (targetpc=", targetpc, ")\n")
			throw("scanframe: bad symbol table")
		}
		bv := stackmapdata(stkmap, pcdata)
		size = uintptr(bv.n) * ptrSize
		scanblock(frame.varp-size, size, bv.bytedata, gcw)
	}

	// Scan arguments.
	if frame.arglen > 0 {
		var bv bitvector
		if frame.argmap != nil {
			bv = *frame.argmap
		} else {
			stkmap := (*stackmap)(funcdata(f, _FUNCDATA_ArgsPointerMaps))
			if stkmap == nil || stkmap.n <= 0 {
				print("runtime: frame ", funcname(f), " untyped args ", hex(frame.argp), "+", hex(frame.arglen), "\n")
				throw("missing stackmap")
			}
			if pcdata < 0 || pcdata >= stkmap.n {
				// don't know where we are
				print("runtime: pcdata is ", pcdata, " and ", stkmap.n, " args stack map entries for ", funcname(f), " (targetpc=", targetpc, ")\n")
				throw("scanframe: bad symbol table")
			}
			bv = stackmapdata(stkmap, pcdata)
		}
		scanblock(frame.argp, uintptr(bv.n)*ptrSize, bv.bytedata, gcw)
	}
}

// TODO(austin): Can we consolidate the gcDrain* functions?

// gcDrain scans objects in work buffers, blackening grey
// objects until all work buffers have been drained.
// If flushScanCredit != -1, gcDrain flushes accumulated scan work
// credit to gcController.bgScanCredit whenever gcw's local scan work
// credit exceeds flushScanCredit.
//go:nowritebarrier
func gcDrain(gcw *gcWork, flushScanCredit int64) {
	if gcphase != _GCmark && gcphase != _GCmarktermination {
		throw("scanblock phase incorrect")
	}

	var lastScanFlush, nextScanFlush int64
	if flushScanCredit != -1 {
		lastScanFlush = gcw.scanWork
		nextScanFlush = lastScanFlush + flushScanCredit
	} else {
		nextScanFlush = int64(^uint64(0) >> 1)
	}

	for {
		// If another proc wants a pointer, give it some.
		if work.nwait > 0 && work.full == 0 {
			gcw.balance()
		}

		b := gcw.get()
		if b == 0 {
			// work barrier reached
			break
		}
		// If the current wbuf is filled by the scan a new wbuf might be
		// returned that could possibly hold only a single object. This
		// could result in each iteration draining only a single object
		// out of the wbuf passed in + a single object placed
		// into an empty wbuf in scanobject so there could be
		// a performance hit as we keep fetching fresh wbufs.
		scanobject(b, gcw)

		// Flush background scan work credit to the global
		// account if we've accumulated enough locally so
		// mutator assists can draw on it.
		if gcw.scanWork >= nextScanFlush {
			credit := gcw.scanWork - lastScanFlush
			xaddint64(&gcController.bgScanCredit, credit)
			lastScanFlush = gcw.scanWork
			nextScanFlush = lastScanFlush + flushScanCredit
		}
	}
	if flushScanCredit != -1 {
		credit := gcw.scanWork - lastScanFlush
		xaddint64(&gcController.bgScanCredit, credit)
	}
}

// gcDrainUntilPreempt blackens grey objects until g.preempt is set.
// This is best-effort, so it will return as soon as it is unable to
// get work, even though there may be more work in the system.
//go:nowritebarrier
func gcDrainUntilPreempt(gcw *gcWork, flushScanCredit int64) {
	if gcphase != _GCmark {
		println("gcphase =", gcphase)
		throw("gcDrainUntilPreempt phase incorrect")
	}

	var lastScanFlush, nextScanFlush int64
	if flushScanCredit != -1 {
		lastScanFlush = gcw.scanWork
		nextScanFlush = lastScanFlush + flushScanCredit
	} else {
		nextScanFlush = int64(^uint64(0) >> 1)
	}

	gp := getg()
	for !gp.preempt {
		// If the work queue is empty, balance. During
		// concurrent mark we don't really know if anyone else
		// can make use of this work, but even if we're the
		// only worker, the total cost of this per cycle is
		// only O(_WorkbufSize) pointer copies.
		if work.full == 0 && work.partial == 0 {
			gcw.balance()
		}

		b := gcw.tryGet()
		if b == 0 {
			// No more work
			break
		}
		scanobject(b, gcw)

		// Flush background scan work credit to the global
		// account if we've accumulated enough locally so
		// mutator assists can draw on it.
		if gcw.scanWork >= nextScanFlush {
			credit := gcw.scanWork - lastScanFlush
			xaddint64(&gcController.bgScanCredit, credit)
			lastScanFlush = gcw.scanWork
			nextScanFlush = lastScanFlush + flushScanCredit
		}
	}
	if flushScanCredit != -1 {
		credit := gcw.scanWork - lastScanFlush
		xaddint64(&gcController.bgScanCredit, credit)
	}
}

// gcDrainN blackens grey objects until it has performed roughly
// scanWork units of scan work. This is best-effort, so it may perform
// less work if it fails to get a work buffer. Otherwise, it will
// perform at least n units of work, but may perform more because
// scanning is always done in whole object increments.
//go:nowritebarrier
func gcDrainN(gcw *gcWork, scanWork int64) {
	targetScanWork := gcw.scanWork + scanWork
	for gcw.scanWork < targetScanWork {
		// This might be a good place to add prefetch code...
		// if(wbuf.nobj > 4) {
		//         PREFETCH(wbuf->obj[wbuf.nobj - 3];
		//  }
		b := gcw.tryGet()
		if b == 0 {
			return
		}
		scanobject(b, gcw)
	}
}

// scanblock scans b as scanobject would, but using an explicit
// pointer bitmap instead of the heap bitmap.
//
// This is used to scan non-heap roots, so it does not update
// gcw.bytesMarked or gcw.scanWork.
//
//go:nowritebarrier
func scanblock(b0, n0 uintptr, ptrmask *uint8, gcw *gcWork) {
	// Use local copies of original parameters, so that a stack trace
	// due to one of the throws below shows the original block
	// base and extent.
	b := b0
	n := n0

	arena_start := mheap_.arena_start
	arena_used := mheap_.arena_used

	for i := uintptr(0); i < n; {
		// Find bits for the next word.
		bits := uint32(*addb(ptrmask, i/(ptrSize*8)))
		if bits == 0 {
			i += ptrSize * 8
			continue
		}
		for j := 0; j < 8 && i < n; j++ {
			if bits&1 != 0 {
				// Same work as in scanobject; see comments there.
				obj := *(*uintptr)(unsafe.Pointer(b + i))
				if obj != 0 && arena_start <= obj && obj < arena_used {
					if obj, hbits, span := heapBitsForObject(obj); obj != 0 {
						greyobject(obj, b, i, hbits, span, gcw)
					}
				}
			}
			bits >>= 1
			i += ptrSize
		}
	}
}

// scanobject scans the object starting at b, adding pointers to gcw.
// b must point to the beginning of a heap object; scanobject consults
// the GC bitmap for the pointer mask and the spans for the size of the
// object (it ignores n).
//go:nowritebarrier
func scanobject(b uintptr, gcw *gcWork) {
	arena_start := mheap_.arena_start
	arena_used := mheap_.arena_used

	// Find bits of the beginning of the object.
	// b must point to the beginning of a heap object, so
	// we can get its bits and span directly.
	hbits := heapBitsForAddr(b)
	s := spanOfUnchecked(b)
	n := s.elemsize
	if n == 0 {
		throw("scanobject n == 0")
	}

	var i uintptr
	for i = 0; i < n; i += ptrSize {
		// Find bits for this word.
		if i != 0 {
			// Avoid needless hbits.next() on last iteration.
			hbits = hbits.next()
		}
		// During checkmarking, 1-word objects store the checkmark
		// in the type bit for the one word. The only one-word objects
		// are pointers, or else they'd be merged with other non-pointer
		// data into larger allocations.
		bits := hbits.bits()
		if i >= 2*ptrSize && bits&bitMarked == 0 {
			break // no more pointers in this object
		}
		if bits&bitPointer == 0 {
			continue // not a pointer
		}

		// Work here is duplicated in scanblock and above.
		// If you make changes here, make changes there too.
		obj := *(*uintptr)(unsafe.Pointer(b + i))

		// At this point we have extracted the next potential pointer.
		// Check if it points into heap.
		if obj != 0 && arena_start <= obj && obj < arena_used {
			// Mark the object.
			if obj, hbits, span := heapBitsForObject(obj); obj != 0 {
				greyobject(obj, b, i, hbits, span, gcw)
			}
		}
	}
	gcw.bytesMarked += uint64(n)
	gcw.scanWork += int64(i)
}

// Shade the object if it isn't already.
// The object is not nil and known to be in the heap.
// Preemption must be disabled.
//go:nowritebarrier
func shade(b uintptr) {
	if obj, hbits, span := heapBitsForObject(b); obj != 0 {
		gcw := &getg().m.p.ptr().gcw
		greyobject(obj, 0, 0, hbits, span, gcw)
		if gcphase == _GCmarktermination {
			// Ps aren't allowed to cache work during mark
			// termination.
			gcw.dispose()
		}
	}
}

// obj is the start of an object with mark mbits.
// If it isn't already marked, mark it and enqueue into gcw.
// base and off are for debugging only and could be removed.
//go:nowritebarrier
func greyobject(obj, base, off uintptr, hbits heapBits, span *mspan, gcw *gcWork) {
	// obj should be start of allocation, and so must be at least pointer-aligned.
	if obj&(ptrSize-1) != 0 {
		throw("greyobject: obj not pointer-aligned")
	}

	if useCheckmark {
		if !hbits.isMarked() {
			printlock()
			print("runtime:greyobject: checkmarks finds unexpected unmarked object obj=", hex(obj), "\n")
			print("runtime: found obj at *(", hex(base), "+", hex(off), ")\n")

			// Dump the source (base) object
			gcDumpObject("base", base, off)

			// Dump the object
			gcDumpObject("obj", obj, ^uintptr(0))

			throw("checkmark found unmarked object")
		}
		if hbits.isCheckmarked(span.elemsize) {
			return
		}
		hbits.setCheckmarked(span.elemsize)
		if !hbits.isCheckmarked(span.elemsize) {
			throw("setCheckmarked and isCheckmarked disagree")
		}
	} else {
		// If marked we have nothing to do.
		if hbits.isMarked() {
			return
		}
		hbits.setMarked()

		// If this is a noscan object, fast-track it to black
		// instead of greying it.
		if !hbits.hasPointers(span.elemsize) {
			gcw.bytesMarked += uint64(span.elemsize)
			return
		}
	}

	// Queue the obj for scanning. The PREFETCH(obj) logic has been removed but
	// seems like a nice optimization that can be added back in.
	// There needs to be time between the PREFETCH and the use.
	// Previously we put the obj in an 8 element buffer that is drained at a rate
	// to give the PREFETCH time to do its work.
	// Use of PREFETCHNTA might be more appropriate than PREFETCH

	gcw.put(obj)
}

// gcDumpObject dumps the contents of obj for debugging and marks the
// field at byte offset off in obj.
func gcDumpObject(label string, obj, off uintptr) {
	if obj < mheap_.arena_start || obj >= mheap_.arena_used {
		print(label, "=", hex(obj), " is not a heap object\n")
		return
	}
	k := obj >> _PageShift
	x := k
	x -= mheap_.arena_start >> _PageShift
	s := h_spans[x]
	print(label, "=", hex(obj), " k=", hex(k))
	if s == nil {
		print(" s=nil\n")
		return
	}
	print(" s.start*_PageSize=", hex(s.start*_PageSize), " s.limit=", hex(s.limit), " s.sizeclass=", s.sizeclass, " s.elemsize=", s.elemsize, "\n")
	for i := uintptr(0); i < s.elemsize; i += ptrSize {
		print(" *(", label, "+", i, ") = ", hex(*(*uintptr)(unsafe.Pointer(obj + uintptr(i)))))
		if i == off {
			print(" <==")
		}
		print("\n")
	}
}

// When in GCmarkterminate phase we allocate black.
//go:nowritebarrier
func gcmarknewobject_m(obj, size uintptr) {
	if gcphase != _GCmarktermination {
		throw("marking new object while not in mark termination phase")
	}
	if useCheckmark { // The world should be stopped so this should not happen.
		throw("gcmarknewobject called while doing checkmark")
	}

	heapBitsForAddr(obj).setMarked()
	xadd64(&work.bytesMarked, int64(size))
}

// Checkmarking

// To help debug the concurrent GC we remark with the world
// stopped ensuring that any object encountered has their normal
// mark bit set. To do this we use an orthogonal bit
// pattern to indicate the object is marked. The following pattern
// uses the upper two bits in the object's bounday nibble.
// 01: scalar  not marked
// 10: pointer not marked
// 11: pointer     marked
// 00: scalar      marked
// Xoring with 01 will flip the pattern from marked to unmarked and vica versa.
// The higher bit is 1 for pointers and 0 for scalars, whether the object
// is marked or not.
// The first nibble no longer holds the typeDead pattern indicating that the
// there are no more pointers in the object. This information is held
// in the second nibble.

// If useCheckmark is true, marking of an object uses the
// checkmark bits (encoding above) instead of the standard
// mark bits.
var useCheckmark = false

//go:nowritebarrier
func initCheckmarks() {
	useCheckmark = true
	for _, s := range work.spans {
		if s.state == _MSpanInUse {
			heapBitsForSpan(s.base()).initCheckmarkSpan(s.layout())
		}
	}
}

func clearCheckmarks() {
	useCheckmark = false
	for _, s := range work.spans {
		if s.state == _MSpanInUse {
			heapBitsForSpan(s.base()).clearCheckmarkSpan(s.layout())
		}
	}
}
