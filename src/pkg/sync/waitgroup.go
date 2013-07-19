// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for.  Then each of the goroutines
// runs and calls Done when finished.  At the same time,
// Wait can be used to block until all goroutines have finished.

// WaitGroup 等待一组Go程的结束。
// 主Go程调用 Add 来设置等待的Go程数。然后该组中的每个Go程都会运行，并在结束时调用
// Done。同时，Wait 可被用于阻塞，直到所有Go程都结束。
type WaitGroup struct {
	m       Mutex
	counter int32
	waiters int32
	sema    *uint32
}

// WaitGroup creates a new semaphore each time the old semaphore
// is released. This is to avoid the following race:
//
// G1: Add(1)
// G1: go G2()
// G1: Wait() // Context switch after Unlock() and before Semacquire().
// G2: Done() // Release semaphore: sema == 1, waiters == 0. G1 doesn't run yet.
// G3: Wait() // Finds counter == 0, waiters == 0, doesn't block.
// G3: Add(1) // Makes counter == 1, waiters == 0.
// G3: go G4()
// G3: Wait() // G1 still hasn't run, G3 finds sema == 1, unblocked! Bug.

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
// Note that calls with positive delta must happen before the call to Wait,
// or else Wait may wait for too small a group. Typically this means the calls
// to Add should execute before the statement creating the goroutine or
// other event to be waited for. See the WaitGroup example.

// Add 添加 delta，对于 WaitGroup 的 counter 来说，它可能为负数。
// 若 counter 变为零，在 Wait() 被释放后所有Go程就会阻塞。
// 若 counter 变为负数，Add 就会引发Panic。
//
// 注意，用正整数的 delta 调用它必须发生在调用 Wait 之前，否则 Wait
// 等待一组的时间会太短。一般来说这意味着对 Add 的调用应当执行在该语句创建Go程，
// 或等待其它事件之前。具体见 WaitGroup 的示例。
func (wg *WaitGroup) Add(delta int) {
	if raceenabled {
		_ = wg.m.state // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			raceReleaseMerge(unsafe.Pointer(wg))
		}
		raceDisable()
		defer raceEnable()
	}
	v := atomic.AddInt32(&wg.counter, int32(delta))
	if raceenabled {
		if delta > 0 && v == int32(delta) {
			// The first increment must be synchronized with Wait.
			// Need to model this as a read, because there can be
			// several concurrent wg.counter transitions from 0.
			raceRead(unsafe.Pointer(&wg.sema))
		}
	}
	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}
	if v > 0 || atomic.LoadInt32(&wg.waiters) == 0 {
		return
	}
	wg.m.Lock()
	for i := int32(0); i < wg.waiters; i++ {
		runtime_Semrelease(wg.sema)
	}
	wg.waiters = 0
	wg.sema = nil
	wg.m.Unlock()
}

// Done decrements the WaitGroup counter.

// Done 递减 WaitGroup 的 counter。
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.

// Wait 阻塞 WaitGroup 直到其 counter 为零。
func (wg *WaitGroup) Wait() {
	if raceenabled {
		_ = wg.m.state // trigger nil deref early
		raceDisable()
	}
	if atomic.LoadInt32(&wg.counter) == 0 {
		if raceenabled {
			raceEnable()
			raceAcquire(unsafe.Pointer(wg))
		}
		return
	}
	wg.m.Lock()
	w := atomic.AddInt32(&wg.waiters, 1)
	// This code is racing with the unlocked path in Add above.
	// The code above modifies counter and then reads waiters.
	// We must modify waiters and then read counter (the opposite order)
	// 此代码与上面 Add 中的解锁路径竞争。上面的代码修改 counter 然后读取 waiters。
	// 我们必须修改 waiters 然后读取 counter（按相反顺序）来避免失去一次 Add。
	if atomic.LoadInt32(&wg.counter) == 0 {
		atomic.AddInt32(&wg.waiters, -1)
		if raceenabled {
			raceEnable()
			raceAcquire(unsafe.Pointer(wg))
			raceDisable()
		}
		wg.m.Unlock()
		if raceenabled {
			raceEnable()
		}
		return
	}
	if raceenabled && w == 1 {
		// Wait must be synchronized with the first Add.
		// Need to model this is as a write to race with the read in Add.
		// As a consequence, can do the write only for the first waiter,
		// otherwise concurrent Waits will race with each other.
		raceWrite(unsafe.Pointer(&wg.sema))
	}
	if wg.sema == nil {
		wg.sema = new(uint32)
	}
	s := wg.sema
	wg.m.Unlock()
	runtime_Semacquire(s)
	if raceenabled {
		raceEnable()
		raceAcquire(unsafe.Pointer(wg))
	}
}
