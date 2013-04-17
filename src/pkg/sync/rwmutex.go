// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
	"unsafe"
)

// An RWMutex is a reader/writer mutual exclusion lock.
// The lock can be held by an arbitrary number of readers
// or a single writer.
// RWMutexes can be created as part of other
// structures; the zero value for a RWMutex is
// an unlocked mutex.

// RWMutex 是一个读写互斥锁。
// 该说可被任意多个读取器或单个写入器所持有。RWMutex 可作为其它结构的一部分来创建；
// RWMutex 的零值即为已解锁的互斥体。
type RWMutex struct {
	w           Mutex  // held if there are pending writers // 若还有正在等待的写入器就保持不变
	writerSem   uint32 // semaphore for writers to wait for completing readers // 等待读取器完成的写入器的信号
	readerSem   uint32 // semaphore for readers to wait for completing writers // 等待写入器完成的读取器的信号
	readerCount int32  // number of pending readers   // 等待的读取器
	readerWait  int32  // number of departing readers // 离开的读取器
}

const rwmutexMaxReaders = 1 << 30

// RLock locks rw for reading.

// RLock 为 rw 的读取将其锁定。
func (rw *RWMutex) RLock() {
	if raceenabled {
		_ = rw.w.state
		raceDisable()
	}
	if atomic.AddInt32(&rw.readerCount, 1) < 0 {
		// A writer is pending, wait for it.
		// 读取器正在等待它
		runtime_Semacquire(&rw.readerSem)
	}
	if raceenabled {
		raceEnable()
		raceAcquire(unsafe.Pointer(&rw.readerSem))
	}
}

// RUnlock undoes a single RLock call;
// it does not affect other simultaneous readers.
// It is a run-time error if rw is not locked for reading
// on entry to RUnlock.

// RUnlock 撤销单次 RLock 调用，它对于其它同时存在的读取器则没有效果。
// 若 rw 并没有为读取而锁定，调用 RUnlock 就会引发一个运行时错误。
func (rw *RWMutex) RUnlock() {
	if raceenabled {
		_ = rw.w.state
		raceReleaseMerge(unsafe.Pointer(&rw.writerSem))
		raceDisable()
	}
	if atomic.AddInt32(&rw.readerCount, -1) < 0 {
		// A writer is pending.
		// 写入器正在等待。
		if atomic.AddInt32(&rw.readerWait, -1) == 0 {
			// The last reader unblocks the writer.
			// 上一个读取器为该写入器消除阻塞。
			runtime_Semrelease(&rw.writerSem)
		}
	}
	if raceenabled {
		raceEnable()
	}
}

// Lock locks rw for writing.
// If the lock is already locked for reading or writing,
// Lock blocks until the lock is available.
// To ensure that the lock eventually becomes available,
// a blocked Lock call excludes new readers from acquiring
// the lock.

// Lock 为 rw 的写入将其锁定。
// 若该锁已经为读取或写入而锁定，Lock 就会阻塞直到该锁可用。
// 为确保该锁最终可用，已阻塞的 Lock 调用会从获得的锁中排除新的读取器。
func (rw *RWMutex) Lock() {
	if raceenabled {
		_ = rw.w.state
		raceDisable()
	}
	// First, resolve competition with other writers.
	// 首先，解决与其它写入器的竞争。
	rw.w.Lock()
	// Announce to readers there is a pending writer.
	// 通知读取器现在有一个等待的读取器。
	r := atomic.AddInt32(&rw.readerCount, -rwmutexMaxReaders) + rwmutexMaxReaders
	// Wait for active readers.
	// 等待活动的读取器。
	if r != 0 && atomic.AddInt32(&rw.readerWait, r) != 0 {
		runtime_Semacquire(&rw.writerSem)
	}
	if raceenabled {
		raceEnable()
		raceAcquire(unsafe.Pointer(&rw.readerSem))
		raceAcquire(unsafe.Pointer(&rw.writerSem))
	}
}

// Unlock unlocks rw for writing.  It is a run-time error if rw is
// not locked for writing on entry to Unlock.
//
// As with Mutexes, a locked RWMutex is not associated with a particular
// goroutine.  One goroutine may RLock (Lock) an RWMutex and then
// arrange for another goroutine to RUnlock (Unlock) it.

// Unlock 为 rw 的写入将其解锁。
// 若 rw 并没有为写入而锁定，调用 Unlock 就会引发一个运行时错误。
//
// As with Mutexes, a locked RWMutex is not associated with a particular
// goroutine.  One goroutine may RLock (Lock) an RWMutex and then
// arrange for another goroutine to RUnlock (Unlock) it.
// 正如 Mutex 一样，已锁定的 RWMutex 并不与特定的Go程相关联。一个Go程可
// RLock（Lock）一个 RWMutex，然后安排其它Go程来 RUnlock（Unlock）它。
func (rw *RWMutex) Unlock() {
	if raceenabled {
		_ = rw.w.state
		raceRelease(unsafe.Pointer(&rw.readerSem))
		raceRelease(unsafe.Pointer(&rw.writerSem))
		raceDisable()
	}

	// Announce to readers there is no active writer.
	// 通知读取器现在没有活动的写入器。
	r := atomic.AddInt32(&rw.readerCount, rwmutexMaxReaders)
	// Unblock blocked readers, if any.
	// 若有，就 Unblock 已锁定的读取器。
	for i := 0; i < int(r); i++ {
		runtime_Semrelease(&rw.readerSem)
	}
	// Allow other writers to proceed.
	// 允许其它写入器继续。
	rw.w.Unlock()
	if raceenabled {
		raceEnable()
	}
}

// RLocker returns a Locker interface that implements
// the Lock and Unlock methods by calling rw.RLock and rw.RUnlock.

// RLocker 返回一个 Locker 接口，该接口通过调用  rw.RLock 和 rw.RUnlock 实现了
// Lock 和 Unlock 方法。
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }
