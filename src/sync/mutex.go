// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sync provides basic synchronization primitives such as mutual
// exclusion locks.  Other than the Once and WaitGroup types, most are intended
// for use by low-level library routines.  Higher-level synchronization is
// better done via channels and communication.
//
// Values containing the types defined in this package should not be copied.

// sync 包提供了互斥锁这类的基本的同步原语.
// 除 Once 和 WaitGroup 之外的类型大多用于底层库的例程。
// 更高级的同步操作通过信道与通信进行。
//
// 在此包中定义的类型中包含的值不应当被复制。
package sync

import (
	"sync/atomic"
	"unsafe"
)

// A Mutex is a mutual exclusion lock.
// Mutexes can be created as part of other structures;
// the zero value for a Mutex is an unlocked mutex.

// Mutex 是一个互斥锁。
// Mutex 可作为其它结构的一部分来创建；Mutex 的零值即为已解锁的互斥体。
type Mutex struct {
	state int32
	sema  uint32
}

// A Locker represents an object that can be locked and unlocked.

// Locker 表示可被锁定并解锁的对象。
type Locker interface {
	Lock()
	Unlock()
}

const (
	mutexLocked = 1 << iota // mutex is locked // 互斥体已锁定。
	mutexWoken
	mutexWaiterShift = iota
)

// Lock locks m.
// If the lock is already in use, the calling goroutine
// blocks until the mutex is available.

// Lock 用于锁定 m。
// 若该锁正在使用，调用的Go程就会阻塞，直到该互斥体可用。
func (m *Mutex) Lock() {
	// Fast path: grab unlocked mutex.
	// 快速通道：抢占锁定的互斥体。
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		if raceenabled {
			raceAcquire(unsafe.Pointer(m))
		}
		return
	}

	awoke := false
	for {
		old := m.state
		new := old | mutexLocked
		if old&mutexLocked != 0 {
			new = old + 1<<mutexWaiterShift
		}
		if awoke {
			// The goroutine has been woken from sleep,
			// so we need to reset the flag in either case.
			// 此Go程已从睡眠状态被唤醒，因此无论在哪种状态下，
			// 我们都需要充值此标记。
			new &^= mutexWoken
		}
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			if old&mutexLocked == 0 {
				break
			}
			runtime_Semacquire(&m.sema)
			awoke = true
		}
	}

	if raceenabled {
		raceAcquire(unsafe.Pointer(m))
	}
}

// Unlock unlocks m.
// It is a run-time error if m is not locked on entry to Unlock.
//
// A locked Mutex is not associated with a particular goroutine.
// It is allowed for one goroutine to lock a Mutex and then
// arrange for another goroutine to unlock it.

// Unlock 用于解锁 m。
// 若 m 在进入 Unlock 前并未锁定，就会引发一个运行时错误。
//
// 已锁定的 Mutex 并不与特定的Go程相关联，这样便可让一个Go程锁定
// Mutex，然后安排其它Go程来解锁。
func (m *Mutex) Unlock() {
	if raceenabled {
		_ = m.state
		raceRelease(unsafe.Pointer(m))
	}

	// Fast path: drop lock bit.
	// 快速通道：锁定位。
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if (new+mutexLocked)&mutexLocked == 0 {
		panic("sync: unlock of unlocked mutex")
	}

	old := new
	for {
		// If there are no waiters or a goroutine has already
		// been woken or grabbed the lock, no need to wake anyone.
		// 若没有等待者，或一个Go程已被唤醒，或该Go程已经抢占了该锁时，
		// 就无需唤醒任何一个了。
		if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken) != 0 {
			return
		}
		// Grab the right to wake someone.
		// 抢占权利来唤醒某一个。
		new = (old - 1<<mutexWaiterShift) | mutexWoken
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			runtime_Semrelease(&m.sema)
			return
		}
		old = m.state
	}
}
