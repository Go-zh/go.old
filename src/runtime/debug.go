// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// Breakpoint executes a breakpoint trap.

// Breakpoint 执行一个断点陷阱。
func Breakpoint()

// LockOSThread wires the calling goroutine to its current operating system thread.
// Until the calling goroutine exits or calls UnlockOSThread, it will always
// execute in that thread, and no other goroutine can.

// LockOSThread 将调用的Go程连接到它当前操作系统的线程。
// 除非调用的Go程退出或调用 UnlockOSThread，否则它将总是在该线程中执行，而其它Go程则不能。
func LockOSThread()

// UnlockOSThread unwires the calling goroutine from its fixed operating system thread.
// If the calling goroutine has not called LockOSThread, UnlockOSThread is a no-op.

// UnlockOSThread 将调用的Go程从它固定的操作系统线程中断开。
// 若调用的Go程未调用 LockOSThread，UnlockOSThread 就是一个空操作。
func UnlockOSThread()

// GOMAXPROCS sets the maximum number of CPUs that can be executing
// simultaneously and returns the previous setting.  If n < 1, it does not
// change the current setting.
// The number of logical CPUs on the local machine can be queried with NumCPU.
// This call will go away when the scheduler improves.

// GOMAXPROCS 设置可同时使用执行的最大CPU数，并返回先前的设置。
// 若 n < 1，它就不会更改当前设置。本地机器的逻辑CPU数可通过 NumCPU 查询。
// 当调度器改进后，此调用将会消失。
func GOMAXPROCS(n int) int {
	if n > _MaxGomaxprocs {
		n = _MaxGomaxprocs
	}
	lock(&sched.lock)
	ret := int(gomaxprocs)
	unlock(&sched.lock)
	if n <= 0 || n == ret {
		return ret
	}

	semacquire(&worldsema, false)
	gp := getg()
	gp.m.gcing = 1
	onM(stoptheworld)

	// newprocs will be processed by starttheworld
	newprocs = int32(n)

	gp.m.gcing = 0
	semrelease(&worldsema)
	onM(starttheworld)
	return ret
}

// NumCPU returns the number of logical CPUs on the local machine.

// NumCPU 返回本地机器的逻辑CPU数。
func NumCPU() int {
	return int(ncpu)
}

// NumCgoCall returns the number of cgo calls made by the current process.

// NumCgoCall 返回由当前进程创建的cgo调用数。
func NumCgoCall() int64 {
	var n int64
	for mp := (*m)(atomicloadp(unsafe.Pointer(&allm))); mp != nil; mp = mp.alllink {
		n += int64(mp.ncgocall)
	}
	return n
}

// NumGoroutine returns the number of goroutines that currently exist.

// NumGoroutine 返回当前存在的Go程数。
func NumGoroutine() int {
	return int(gcount())
}

func gcount() int32
