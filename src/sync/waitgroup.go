// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"internal/race"
	"sync/atomic"
	"unsafe"
)

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
//
// A WaitGroup must not be copied after first use.

// WaitGroup 等待一组Go程的结束。
// 主Go程调用 Add 来设置等待的Go程数。然后该组中的每个Go程都会运行，并在结束时调用
// Done。同时，Wait 可被用于阻塞，直到所有Go程都结束。
//
// WaitGroup 在第一次使用后必须不能被复制。
type WaitGroup struct {
	noCopy noCopy

	// 64-bit value: high 32 bits are counter, low 32 bits are waiter count.
	// 64-bit atomic operations require 64-bit alignment, but 32-bit
	// compilers do not ensure it. So we allocate 12 bytes and then use
	// the aligned 8 bytes in them as state.
	state1 [12]byte
	sema   uint32
}

func (wg *WaitGroup) state() *uint64 {
	if uintptr(unsafe.Pointer(&wg.state1))%8 == 0 {
		return (*uint64)(unsafe.Pointer(&wg.state1))
	} else {
		return (*uint64)(unsafe.Pointer(&wg.state1[4]))
	}
}

// WaitGroup 每当旧的信号被释放时，就会创建一个新的信号。这是为了避免以下竞争：
//
// G1: Add(1)
// G1: go G2()
// G1: Wait() // 在 Unlock() 之后 Semacquire() 之前进行上下文切换。
// G2: Done() // 释放信号：sema == 1，waiters == 0。G1 还不会被运行。
// G3: Wait() // 发现 counter == 0，waiters == 0，不会阻塞。
// G3: Add(1) // 使 counter == 1，waiters == 0。
// G3: go G4()
// G3: Wait() // G1 仍然没有运行，但 G3 发现 sema == 1，就解阻了！Bug。

// Add adds delta, which may be negative, to the WaitGroup counter.
// If the counter becomes zero, all goroutines blocked on Wait are released.
// If the counter goes negative, Add panics.
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.

// Add 添加 delta，对于 WaitGroup 的 counter 来说，它可能为负数。
// 若 counter 变为零，在 Wait() 被释放后所有Go程就会阻塞。
// 若 counter 变为负数，Add 就会引发Panic。
//
// 注意，当 counter 为零时，用正整数的 delta 调用它必须发生在调用 Wait 之前。
// 用负整数的 delta 调用它，或在 counter 大于零时开始用正整数的 delta 调用它，
// 那么它可以在任何时候发生。
// 一般来说，这意味着对 Add 的调用应当在该语句创建Go程，或等待其它事件之前执行。
// 具体见 WaitGroup 的示例。
func (wg *WaitGroup) Add(delta int) {
	statep := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	state := atomic.AddUint64(statep, uint64(delta)<<32)
	v := int32(state >> 32)
	w := uint32(state)
	if race.Enabled {
		if delta > 0 && v == int32(delta) {
			// The first increment must be synchronized with Wait.
			// Need to model this as a read, because there can be
			// several concurrent wg.counter transitions from 0.
			race.Read(unsafe.Pointer(&wg.sema))
		}
	}
	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}
	if w != 0 && delta > 0 && v == int32(delta) {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	if v > 0 || w == 0 {
		return
	}
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	if *statep != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	*statep = 0
	for ; w != 0; w-- {
		runtime_Semrelease(&wg.sema)
	}
}

// Done decrements the WaitGroup counter.

// Done 递减 WaitGroup 的 counter。
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.

// Wait 阻塞 WaitGroup 直到其 counter 为零。
func (wg *WaitGroup) Wait() {
	statep := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		race.Disable()
	}
	for {
		state := atomic.LoadUint64(statep)
		v := int32(state >> 32)
		w := uint32(state)
		if v == 0 {
			// Counter is 0, no need to wait.
			// 计数器为 0，无需等待。
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
		// Increment waiters count.
		// 递增等待者计数。
		if atomic.CompareAndSwapUint64(statep, state, state+1) {
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(&wg.sema))
			}
			runtime_Semacquire(&wg.sema)
			if *statep != 0 {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
	}
}
