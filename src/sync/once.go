// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"sync/atomic"
)

// Once is an object that will perform exactly one action.

// Once 是只执行一个动作的对象。
type Once struct {
	m    Mutex
	done uint32
}

// Do calls the function f if and only if Do is being called for the
// first time for this instance of Once. In other words, given
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation. A new instance of
// Once is required for each function to execute.
//
// Do is intended for initialization that must be run exactly once. Since f
// is niladic, it may be necessary to use a function literal to capture the
// arguments to a function to be invoked by Do:
// 	config.once.Do(func() { config.init(filename) })
//
// Because no call to Do returns until the one call to f returns, if f causes
// Do to be called, it will deadlock.
//
// If f panics, Do considers it to have returned; future calls of Do return
// without calling f.

// Do 方法当且仅当连同此接收者第一次被调用是才执行函数 f。
// 	var once Once
// if once.Do(f) is called multiple times, only the first call will invoke f,
// even if f has a different value in each invocation.  A new instance of
// Once is required for each function to execute.
// 若 once.Do(f) 被调用多次，即使每一次请求的 f 值都不同，也只有第一次调用会请求 f。
// Once 的新实例需要为每一个函数所执行。
//
// Do 用于必须刚好运行一次的初始化。由于 f 是函数，它可能需要使用函数字面来为 Do
// 所请求的函数捕获实参：
// 	config.once.Do(func() { config.init(filename) })
//
// 由于 f 的调用返回之前没有 Do 的调用会返回，因此若 f 引起了 Do 的调用，它就会死锁。
//
// 若 f 发生派错（panic），Do 会考虑它是否已返回；将来对 Do 的调用会直接返回而不调用 f。
func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}
	// Slow-path.
	// 慢速通道。
	o.m.Lock()
	defer o.m.Unlock()
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
