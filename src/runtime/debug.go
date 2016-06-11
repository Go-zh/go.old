// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// GOMAXPROCS sets the maximum number of CPUs that can be executing
// simultaneously and returns the previous setting. If n < 1, it does not
// change the current setting.
// The number of logical CPUs on the local machine can be queried with NumCPU.
// This call will go away when the scheduler improves.

// GOMAXPROCS 设置可同时执行的最大CPU数并返回先前的设置。
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

	stopTheWorld("GOMAXPROCS")

	// newprocs will be processed by startTheWorld
	newprocs = int32(n)

	startTheWorld()
	return ret
}

// NumCPU returns the number of logical CPUs usable by the current process.
//
// The set of available CPUs is checked by querying the operating system
// at process startup. Changes to operating system CPU allocation after
// process startup are not reflected.

// NumCPU 返回当前进程可用的逻辑CPU数。
//
// 可用CPU的设置会在进程启动时通过查询操作系统获得。进程启动后更改操作系统的CPU
// 分配并不会反映出来。
func NumCPU() int {
	return int(ncpu)
}

// NumCgoCall returns the number of cgo calls made by the current process.

// NumCgoCall 返回由当前进程创建的cgo调用数。
func NumCgoCall() int64 {
	var n int64
	for mp := (*m)(atomic.Loadp(unsafe.Pointer(&allm))); mp != nil; mp = mp.alllink {
		n += int64(mp.ncgocall)
	}
	return n
}

// NumGoroutine returns the number of goroutines that currently exist.

// NumGoroutine 返回当前存在的Go程数。
func NumGoroutine() int {
	return int(gcount())
}
