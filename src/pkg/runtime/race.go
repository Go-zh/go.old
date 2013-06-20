// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build race

// Public race detection API, present iff build with -race.

// 公共的竞争检测API，当且仅当使用 -race 构建时才会出现。
package runtime

import (
	"unsafe"
)

// RaceDisable disables handling of race events in the current goroutine.

// RaceDisable 关闭当前Go程中竞争事件的处理。
func RaceDisable()

// RaceEnable re-enables handling of race events in the current goroutine.

// RaceEnable 重新开启当前Go程中竞争事件的处理。
func RaceEnable()

func RaceAcquire(addr unsafe.Pointer)
func RaceRelease(addr unsafe.Pointer)
func RaceReleaseMerge(addr unsafe.Pointer)

func RaceRead(addr unsafe.Pointer)
func RaceWrite(addr unsafe.Pointer)
func RaceReadRange(addr unsafe.Pointer, len int)
func RaceWriteRange(addr unsafe.Pointer, len int)

func RaceSemacquire(s *uint32)
func RaceSemrelease(s *uint32)
