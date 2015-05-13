// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// Per-thread (in Go, per-P) cache for small objects.
// No locking needed because it is per-thread (per-P).
type mcache struct {
	// The following members are accessed on every malloc,
	// so they are grouped here for better caching.
	next_sample      int32   // trigger heap sample after allocating this many bytes
	local_cachealloc uintptr // bytes allocated from cache since last lock of heap
	local_scan       uintptr // bytes of scannable heap allocated
	// Allocator cache for tiny objects w/o pointers.
	// See "Tiny allocator" comment in malloc.go.
	tiny             unsafe.Pointer
	tinyoffset       uintptr
	local_tinyallocs uintptr // number of tiny allocs not counted in other stats

	// The rest is not accessed on every malloc.
	alloc [_NumSizeClasses]*mspan // spans to allocate from

	stackcache [_NumStackOrders]stackfreelist

	// Local allocator stats, flushed during GC.
	local_nlookup    uintptr                  // number of pointer lookups
	local_largefree  uintptr                  // bytes freed for large objects (>maxsmallsize)
	local_nlargefree uintptr                  // number of frees for large objects (>maxsmallsize)
	local_nsmallfree [_NumSizeClasses]uintptr // number of frees for small objects (<=maxsmallsize)
}

// A gclink is a node in a linked list of blocks, like mlink,
// but it is opaque to the garbage collector.
// The GC does not trace the pointers during collection,
// and the compiler does not emit write barriers for assignments
// of gclinkptr values. Code should store references to gclinks
// as gclinkptr, not as *gclink.
type gclink struct {
	next gclinkptr
}

// A gclinkptr is a pointer to a gclink, but it is opaque
// to the garbage collector.
type gclinkptr uintptr

// ptr returns the *gclink form of p.
// The result should be used for accessing fields, not stored
// in other data structures.
func (p gclinkptr) ptr() *gclink {
	return (*gclink)(unsafe.Pointer(p))
}

type stackfreelist struct {
	list gclinkptr // linked list of free stacks
	size uintptr   // total size of stacks in list
}

// dummy MSpan that contains no free objects.
var emptymspan mspan

func allocmcache() *mcache {
	lock(&mheap_.lock)
	c := (*mcache)(fixAlloc_Alloc(&mheap_.cachealloc))
	unlock(&mheap_.lock)
	memclr(unsafe.Pointer(c), unsafe.Sizeof(*c))
	for i := 0; i < _NumSizeClasses; i++ {
		c.alloc[i] = &emptymspan
	}

	// Set first allocation sample size.
	rate := MemProfileRate
	if rate > 0x3fffffff { // make 2*rate not overflow
		rate = 0x3fffffff
	}
	if rate != 0 {
		c.next_sample = int32(int(fastrand1()) % (2 * rate))
	}

	return c
}

func freemcache(c *mcache) {
	systemstack(func() {
		mCache_ReleaseAll(c)
		stackcache_clear(c)

		// NOTE(rsc,rlh): If gcworkbuffree comes back, we need to coordinate
		// with the stealing of gcworkbufs during garbage collection to avoid
		// a race where the workbuf is double-freed.
		// gcworkbuffree(c.gcworkbuf)

		lock(&mheap_.lock)
		purgecachedstats(c)
		fixAlloc_Free(&mheap_.cachealloc, unsafe.Pointer(c))
		unlock(&mheap_.lock)
	})
}

// Gets a span that has a free object in it and assigns it
// to be the cached span for the given sizeclass.  Returns this span.
func mCache_Refill(c *mcache, sizeclass int32) *mspan {
	_g_ := getg()

	_g_.m.locks++
	// Return the current cached span to the central lists.
	s := c.alloc[sizeclass]
	if s.freelist.ptr() != nil {
		throw("refill on a nonempty span")
	}
	if s != &emptymspan {
		s.incache = false
	}

	// Get a new cached span from the central lists.
	s = mCentral_CacheSpan(&mheap_.central[sizeclass].mcentral)
	if s == nil {
		throw("out of memory")
	}
	if s.freelist.ptr() == nil {
		println(s.ref, (s.npages<<_PageShift)/s.elemsize)
		throw("empty span")
	}
	c.alloc[sizeclass] = s
	_g_.m.locks--
	return s
}

func mCache_ReleaseAll(c *mcache) {
	for i := 0; i < _NumSizeClasses; i++ {
		s := c.alloc[i]
		if s != &emptymspan {
			mCentral_UncacheSpan(&mheap_.central[i].mcentral, s)
			c.alloc[i] = &emptymspan
		}
	}
}
