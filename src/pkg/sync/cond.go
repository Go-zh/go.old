// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

// Cond implements a condition variable, a rendezvous point
// for goroutines waiting for or announcing the occurrence
// of an event.
//
// Each Cond has an associated Locker L (often a *Mutex or *RWMutex),
// which must be held when changing the condition and
// when calling the Wait method.

// Cond 实现了条件变量，即Go程等待的汇合点或宣布一个事件的发生。
//
// 每个 Cond 都有一个与其相关联的 Locker L（一般是 *Mutex 或 *RWMutex），
// 在改变该条件或调用 Wait 方法时，它必须保持不变。
type Cond struct {
	L Locker // held while observing or changing the condition // 在观测或更改条件时保持不变
	m Mutex  // held to avoid internal races // 为避免内部竞争而保持不变

	// We must be careful to make sure that when Signal
	// releases a semaphore, the corresponding acquire is
	// executed by a goroutine that was already waiting at
	// the time of the call to Signal, not one that arrived later.
	// To ensure this, we segment waiting goroutines into
	// generations punctuated by calls to Signal.  Each call to
	// Signal begins another generation if there are no goroutines
	// left in older generations for it to wake.  Because of this
	// optimization (only begin another generation if there
	// are no older goroutines left), we only need to keep track
	// of the two most recent generations, which we call old
	// and new.
	// 在 Signal 释放出一个信号时，我们必须小心确认Go程执行的相应捕获操作是否就绪，
	// 而不晚于调用 Signal 的时刻。为确保这一点，我们不时会通过调用 Signal
	// 来将等待的Go程分成几个阶段。若旧的阶段中没有等待被唤醒的Go程，每次调用
	// Signal 都会开始另一个阶段。由于这种优化（若没有留下旧的Go程，
	// 就会开始另一个阶段），我们只需跟踪最近的两个阶段，我们将这两个阶段称为
	// old 和 new。
	oldWaiters int     // number of waiters in old generation... // 旧阶段中的等待者数…
	oldSema    *uint32 // ... waiting on this semaphore // …等待这个信号

	newWaiters int     // number of waiters in new generation... // 新阶段中的等待者数…
	newSema    *uint32 // ... waiting on this semaphore // …等待这个信号
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
	if raceenabled {
		_ = c.m.state
		raceDisable()
	}
	c.m.Lock()
	if c.newSema == nil {
		c.newSema = new(uint32)
	}
	s := c.newSema
	c.newWaiters++
	c.m.Unlock()
	if raceenabled {
		raceEnable()
	}
	c.L.Unlock()
	runtime_Semacquire(s)
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
	if raceenabled {
		_ = c.m.state
		raceDisable()
	}
	c.m.Lock()
	if c.oldWaiters == 0 && c.newWaiters > 0 {
		// Retire old generation; rename new to old.
		// 放弃 old 阶段；将新的阶段重命名为 old。
		c.oldWaiters = c.newWaiters
		c.oldSema = c.newSema
		c.newWaiters = 0
		c.newSema = nil
	}
	if c.oldWaiters > 0 {
		c.oldWaiters--
		runtime_Semrelease(c.oldSema)
	}
	c.m.Unlock()
	if raceenabled {
		raceEnable()
	}
}

// Broadcast wakes all goroutines waiting on c.
//
// It is allowed but not required for the caller to hold c.L
// during the call.

// Broadcast 唤醒所有等待 c 的Go程。
//
// during the call.在调用其间可以保存 c.L，但并没有必要。
func (c *Cond) Broadcast() {
	if raceenabled {
		_ = c.m.state
		raceDisable()
	}
	c.m.Lock()
	// Wake both generations.
	// 唤醒两个阶段。
	if c.oldWaiters > 0 {
		for i := 0; i < c.oldWaiters; i++ {
			runtime_Semrelease(c.oldSema)
		}
		c.oldWaiters = 0
	}
	if c.newWaiters > 0 {
		for i := 0; i < c.newWaiters; i++ {
			runtime_Semrelease(c.newSema)
		}
		c.newWaiters = 0
		c.newSema = nil
	}
	c.m.Unlock()
	if raceenabled {
		raceEnable()
	}
}
