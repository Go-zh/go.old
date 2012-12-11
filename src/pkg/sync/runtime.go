// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

// defined in package runtime
// 已在 runtime 包中定义。

// Semacquire waits until *s > 0 and then atomically decrements it.
// It is intended as a simple sleep primitive for use by the synchronization
// library and should not be used directly.

// Semacquire 一直等到 *s > 0 时，然后原子性地对它进行减量。
// 其目的是用作同步库的简单睡眠原语，你不应直接使用它。
func runtime_Semacquire(s *uint32)

// Semrelease atomically increments *s and notifies a waiting goroutine
// if one is blocked in Semacquire.
// It is intended as a simple wakeup primitive for use by the synchronization
// library and should not be used directly.

// Semrelease 原子性地对 *s 进行减量，若有等待的Go程在 Semacquire 中被阻塞就通知它。
// 其目的是用作同步库的简单唤醒原语，你不应直接使用它。
func runtime_Semrelease(s *uint32)
