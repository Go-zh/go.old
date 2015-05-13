// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Garbage collector: write barriers.
//
// For the concurrent garbage collector, the Go compiler implements
// updates to pointer-valued fields that may be in heap objects by
// emitting calls to write barriers. This file contains the actual write barrier
// implementation, markwb, and the various wrappers called by the
// compiler to implement pointer assignment, slice assignment,
// typed memmove, and so on.

package runtime

import "unsafe"

// markwb is the mark-phase write barrier, the only barrier we have.
// The rest of this file exists only to make calls to this function.
//
// This is the Dijkstra barrier coarsened to always shade the ptr (dst) object.
// The original Dijkstra barrier only shaded ptrs being placed in black slots.
//
// Shade indicates that it has seen a white pointer by adding the referent
// to wbuf as well as marking it.
//
// slot is the destination (dst) in go code
// ptr is the value that goes into the slot (src) in the go code
//
// Dijkstra pointed out that maintaining the no black to white
// pointers means that white to white pointers not need
// to be noted by the write barrier. Furthermore if either
// white object dies before it is reached by the
// GC then the object can be collected during this GC cycle
// instead of waiting for the next cycle. Unfortunately the cost of
// ensure that the object holding the slot doesn't concurrently
// change to black without the mutator noticing seems prohibitive.
//
// Consider the following example where the mutator writes into
// a slot and then loads the slot's mark bit while the GC thread
// writes to the slot's mark bit and then as part of scanning reads
// the slot.
//
// Initially both [slot] and [slotmark] are 0 (nil)
// Mutator thread          GC thread
// st [slot], ptr          st [slotmark], 1
//
// ld r1, [slotmark]       ld r2, [slot]
//
// Without an expensive memory barrier between the st and the ld, the final
// result on most HW (including 386/amd64) can be r1==r2==0. This is a classic
// example of what can happen when loads are allowed to be reordered with older
// stores (avoiding such reorderings lies at the heart of the classic
// Peterson/Dekker algorithms for mutual exclusion). Rather than require memory
// barriers, which will slow down both the mutator and the GC, we always grey
// the ptr object regardless of the slot's color.
//go:nowritebarrier
func gcmarkwb_m(slot *uintptr, ptr uintptr) {
	switch gcphase {
	default:
		throw("gcphasework in bad gcphase")

	case _GCoff, _GCquiesce, _GCstw, _GCsweep, _GCscan:
		// ok

	case _GCmark, _GCmarktermination:
		if ptr != 0 && inheap(ptr) {
			shade(ptr)
		}
	}
}

// Write barrier calls must not happen during critical GC and scheduler
// related operations. In particular there are times when the GC assumes
// that the world is stopped but scheduler related code is still being
// executed, dealing with syscalls, dealing with putting gs on runnable
// queues and so forth. This code can not execute write barriers because
// the GC might drop them on the floor. Stopping the world involves removing
// the p associated with an m. We use the fact that m.p == nil to indicate
// that we are in one these critical section and throw if the write is of
// a pointer to a heap object.
//go:nosplit
func writebarrierptr_nostore1(dst *uintptr, src uintptr) {
	mp := acquirem()
	if mp.inwb || mp.dying > 0 {
		releasem(mp)
		return
	}
	systemstack(func() {
		if mp.p == 0 && memstats.enablegc && !mp.inwb && inheap(src) {
			throw("writebarrierptr_nostore1 called with mp.p == nil")
		}
		mp.inwb = true
		gcmarkwb_m(dst, src)
	})
	mp.inwb = false
	releasem(mp)
}

// NOTE: Really dst *unsafe.Pointer, src unsafe.Pointer,
// but if we do that, Go inserts a write barrier on *dst = src.
//go:nosplit
func writebarrierptr(dst *uintptr, src uintptr) {
	*dst = src
	if !writeBarrierEnabled {
		return
	}
	if src != 0 && (src < _PhysPageSize || src == poisonStack) {
		systemstack(func() {
			print("runtime: writebarrierptr *", dst, " = ", hex(src), "\n")
			throw("bad pointer in write barrier")
		})
	}
	writebarrierptr_nostore1(dst, src)
}

// Like writebarrierptr, but the store has already been applied.
// Do not reapply.
//go:nosplit
func writebarrierptr_nostore(dst *uintptr, src uintptr) {
	if !writeBarrierEnabled {
		return
	}
	if src != 0 && (src < _PhysPageSize || src == poisonStack) {
		systemstack(func() { throw("bad pointer in write barrier") })
	}
	writebarrierptr_nostore1(dst, src)
}

//go:nosplit
func writebarrierstring(dst *[2]uintptr, src [2]uintptr) {
	writebarrierptr(&dst[0], src[0])
	dst[1] = src[1]
}

//go:nosplit
func writebarrierslice(dst *[3]uintptr, src [3]uintptr) {
	writebarrierptr(&dst[0], src[0])
	dst[1] = src[1]
	dst[2] = src[2]
}

//go:nosplit
func writebarrieriface(dst *[2]uintptr, src [2]uintptr) {
	writebarrierptr(&dst[0], src[0])
	writebarrierptr(&dst[1], src[1])
}

//go:generate go run wbfat_gen.go -- wbfat.go
//
// The above line generates multiword write barriers for
// all the combinations of ptr+scalar up to four words.
// The implementations are written to wbfat.go.

// typedmemmove copies a value of type t to dst from src.
//go:nosplit
func typedmemmove(typ *_type, dst, src unsafe.Pointer) {
	memmove(dst, src, typ.size)
	if typ.kind&kindNoPointers != 0 {
		return
	}
	heapBitsBulkBarrier(uintptr(dst), typ.size)
}

//go:linkname reflect_typedmemmove reflect.typedmemmove
func reflect_typedmemmove(typ *_type, dst, src unsafe.Pointer) {
	typedmemmove(typ, dst, src)
}

// typedmemmovepartial is like typedmemmove but assumes that
// dst and src point off bytes into the value and only copies size bytes.
//go:linkname reflect_typedmemmovepartial reflect.typedmemmovepartial
func reflect_typedmemmovepartial(typ *_type, dst, src unsafe.Pointer, off, size uintptr) {
	memmove(dst, src, size)
	if !writeBarrierEnabled || typ.kind&kindNoPointers != 0 || size < ptrSize || !inheap(uintptr(dst)) {
		return
	}

	if frag := -off & (ptrSize - 1); frag != 0 {
		dst = add(dst, frag)
		size -= frag
	}
	heapBitsBulkBarrier(uintptr(dst), size&^(ptrSize-1))
}

// callwritebarrier is invoked at the end of reflectcall, to execute
// write barrier operations to record the fact that a call's return
// values have just been copied to frame, starting at retoffset
// and continuing to framesize. The entire frame (not just the return
// values) is described by typ. Because the copy has already
// happened, we call writebarrierptr_nostore, and we must be careful
// not to be preempted before the write barriers have been run.
//go:nosplit
func callwritebarrier(typ *_type, frame unsafe.Pointer, framesize, retoffset uintptr) {
	if !writeBarrierEnabled || typ == nil || typ.kind&kindNoPointers != 0 || framesize-retoffset < ptrSize || !inheap(uintptr(frame)) {
		return
	}
	heapBitsBulkBarrier(uintptr(add(frame, retoffset)), framesize)
}

//go:nosplit
func typedslicecopy(typ *_type, dst, src slice) int {
	n := dst.len
	if n > src.len {
		n = src.len
	}
	if n == 0 {
		return 0
	}
	dstp := unsafe.Pointer(dst.array)
	srcp := unsafe.Pointer(src.array)

	if raceenabled {
		callerpc := getcallerpc(unsafe.Pointer(&typ))
		pc := funcPC(slicecopy)
		racewriterangepc(dstp, uintptr(n)*typ.size, callerpc, pc)
		racereadrangepc(srcp, uintptr(n)*typ.size, callerpc, pc)
	}

	if !writeBarrierEnabled {
		memmove(dstp, srcp, uintptr(n)*typ.size)
		return n
	}

	systemstack(func() {
		if uintptr(srcp) < uintptr(dstp) && uintptr(srcp)+uintptr(n)*typ.size > uintptr(dstp) {
			// Overlap with src before dst.
			// Copy backward, being careful not to move dstp/srcp
			// out of the array they point into.
			dstp = add(dstp, uintptr(n-1)*typ.size)
			srcp = add(srcp, uintptr(n-1)*typ.size)
			i := 0
			for {
				typedmemmove(typ, dstp, srcp)
				if i++; i >= n {
					break
				}
				dstp = add(dstp, -typ.size)
				srcp = add(srcp, -typ.size)
			}
		} else {
			// Copy forward, being careful not to move dstp/srcp
			// out of the array they point into.
			i := 0
			for {
				typedmemmove(typ, dstp, srcp)
				if i++; i >= n {
					break
				}
				dstp = add(dstp, typ.size)
				srcp = add(srcp, typ.size)
			}
		}
	})
	return int(n)
}

//go:linkname reflect_typedslicecopy reflect.typedslicecopy
func reflect_typedslicecopy(elemType *_type, dst, src slice) int {
	return typedslicecopy(elemType, dst, src)
}
