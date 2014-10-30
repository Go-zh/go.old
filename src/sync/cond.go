// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.
//
// A Cond can be created as part of other structures.
// A Cond must not be copied after first use.

// Cond 实现了条件变量，即Go程等待的汇合点或宣布一个事件的发生。
//
// 每个 Cond 都有一个与其相关联的 Locker L（一般是 *Mutex 或 *RWMutex），
// 在改变该条件或调用 Wait 方法时，它必须保持不变。
type Cond struct {
	// L is held while observing or changing the condition
	// L 在观测或更改条件时保持不变
	L Locker

	sema    syncSema
	waiters uint32 // number of waiters // 等待者的数量
	checker copyChecker
}

// NewCond returns a new Cond with Locker l.

// NewCond 用 Locker l 返回一个新的 Cond。
func NewCond(l Locker) *Cond {
	return &Cond{L: l}
}

// Wait atomically unlocks c.L and suspends execution
// of the calling goroutine.  After later resuming execution,
// Wait locks c.L before returning.  Unlike in other systems,
// Wait cannot return unless awoken by Broadcast or Signal.
//
// Because c.L is not locked when Wait first resumes, the caller
// typically cannot assume that the condition is true when
// Wait returns.  Instead, the caller should Wait in a loop:
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... make use of condition ...
//    c.L.Unlock()
//

// Wait 原子性地解锁 c.L 并挂起调用的Go程的执行。不像其它的系统那样，Wait
// 不会返回，除非它被 Broadcast 或 Signal 唤醒。
//
// 由于 Wait 第一次恢复时 c.L 并未锁定，因此调用者一般不能假定 Wait 返回时条件为真。
// 取而代之，调用者应当把 Wait 放入循环中：
//
//    c.L.Lock()
//    for !condition() {
//        c.Wait()
//    }
//    ... 使用 condition ...
//    c.L.Unlock()
//
func (c *Cond) Wait() {
	c.checker.check()
	if raceenabled {
		raceDisable()
	}
	atomic.AddUint32(&c.waiters, 1)
	if raceenabled {
		raceEnable()
	}
	c.L.Unlock()
	runtime_Syncsemacquire(&c.sema)
	c.L.Lock()
}

// Signal wakes one goroutine waiting on c, if there is any.
//
// It is allowed but not required for the caller to hold c.L
// during the call.

// Signal 用于唤醒等待 c 的Go程，如果有的话。
//
// during the call.在调用其间可以保存 c.L，但并没有必要。
func (c *Cond) Signal() {
	c.signalImpl(false)
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.

// Broadcast 唤醒所有等待 c 的Go程。
//
// during the call.在调用其间可以保存 c.L，但并没有必要。
func (c *Cond) Broadcast() {
	c.signalImpl(true)
}

func (c *Cond) signalImpl(all bool) {
	c.checker.check()
	if raceenabled {
		raceDisable()
	}
	for {
		old := atomic.LoadUint32(&c.waiters)
		if old == 0 {
			if raceenabled {
				raceEnable()
			}
			return
		}
		new := old - 1
		if all {
			new = 0
		}
		if atomic.CompareAndSwapUint32(&c.waiters, old, new) {
			if raceenabled {
				raceEnable()
			}
			runtime_Syncsemrelease(&c.sema, old-new)
			return
		}
	}
}

// copyChecker holds back pointer to itself to detect object copying.
type copyChecker uintptr

func (c *copyChecker) check() {
	if uintptr(*c) != uintptr(unsafe.Pointer(c)) &&
		!atomic.CompareAndSwapUintptr((*uintptr)(c), 0, uintptr(unsafe.Pointer(c))) &&
		uintptr(*c) != uintptr(unsafe.Pointer(c)) {
		panic("sync.Cond is copied")
	}
}
