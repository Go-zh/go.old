// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import "unsafe"

// Note: the MemStats struct should be kept in sync with
// struct MStats in malloc.h

// 注意：MemStats 结构体应当与 malloc.h 中的结构体 MStats 保持同步。

// A MemStats records statistics about the memory allocator.

// MemStats 用于记录内存分配器的统计量。
type MemStats struct {
	// General statistics.
	// 一般统计。
	Alloc      uint64 // bytes allocated and still in use   // 已分配且仍在使用的字节数
	TotalAlloc uint64 // bytes allocated (even if freed)    // 已分配（包括已释放的）字节数
	// 从系统中获取的字节数（应当为下面 XxxSys 之和）
	Sys     uint64 // bytes obtained from system (should be sum of XxxSys below)
	Lookups uint64 // number of pointer lookups  // 指针查找数
	Mallocs uint64 // number of mallocs          // malloc 数
	Frees   uint64 // number of frees            // free 数

	// Main allocation heap statistics.
	// 主分配堆统计。
	HeapAlloc    uint64 // bytes allocated and still in use // 已分配且仍在使用的字节数
	HeapSys      uint64 // bytes obtained from system       // 从系统中获取的字节数
	HeapIdle     uint64 // bytes in idle spans              // 空闲区间的字节数
	HeapInuse    uint64 // bytes in non-idle span           // 非空闲区间的字节数
	HeapReleased uint64 // bytes released to the OS         // 释放给OS的字节数
	HeapObjects  uint64 // total number of allocated objects// 已分配对象的总数

	// Low-level fixed-size structure allocator statistics.
	//	Inuse is bytes used now.
	//	Sys is bytes obtained from system.
	//
	// 底层固定大小的结构分配器统计。
	//	Inuse 为正在使用的字节数。
	//	Sys   为从系统获取的字节数。
	StackInuse  uint64 // bootstrap stacks  // 引导栈
	StackSys    uint64
	MSpanInuse  uint64 // mspan structures  // mspan（内存区间）结构数
	MSpanSys    uint64
	MCacheInuse uint64 // mcache structures // mcache（内存缓存）结构数
	MCacheSys   uint64
	BuckHashSys uint64 // profiling bucket hash table // 分析桶散列表

	// Garbage collector statistics.
	// 垃圾收集器统计。
	NextGC       uint64 // next run in HeapAlloc time (bytes) // 下次运行的 HeapAlloc 时间（字节）
	LastGC       uint64 // last run in absolute time (ns)     // 上次运行的绝对时间（纳秒 ns）
	PauseTotalNs uint64
	// 最近GC暂停时间的循环缓存，最近一次应为 [(NumGC+255)%256]
	PauseNs  [256]uint64 // circular buffer of recent GC pause times, most recent at [(NumGC+255)%256]
	NumGC    uint32
	EnableGC bool
	DebugGC  bool

	// Per-size allocation statistics.
	// 61 is NumSizeClasses in the C code.
	// 每个分配的大小统计。
	// 61 是C代码中的 NumSizeClasses
	BySize [61]struct {
		Size    uint32
		Mallocs uint64
		Frees   uint64
	}
}

var sizeof_C_MStats uintptr // filled in by malloc.goc  // 由 malloc.goc 填入

var memStats MemStats

func init() {
	if sizeof_C_MStats != unsafe.Sizeof(memStats) {
		println(sizeof_C_MStats, unsafe.Sizeof(memStats))
		panic("MStats vs MemStatsType size mismatch")
	}
}

// ReadMemStats populates m with memory allocator statistics.

// ReadMemStats 将内存分配器的统计填充到 m 中。
func ReadMemStats(m *MemStats)

// GC runs a garbage collection.

// GC 运行一次垃圾回收。
func GC()
