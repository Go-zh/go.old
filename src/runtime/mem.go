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
	Sys        uint64 // bytes obtained from system (sum of XxxSys below) // 从系统中获取的字节数（应当为下面 XxxSys 之和）
	Lookups    uint64 // number of pointer lookups          // 指针查找数
	Mallocs    uint64 // number of mallocs                  // malloc 数
	Frees      uint64 // number of frees                    // free 数

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
	StackInuse  uint64 // bytes used by stack allocator // 栈分配器使用的字节
	StackSys    uint64
	MSpanInuse  uint64 // mspan structures  // mspan（内存区间）结构数
	MSpanSys    uint64
	MCacheInuse uint64 // mcache structures // mcache（内存缓存）结构数
	MCacheSys   uint64
	BuckHashSys uint64 // profiling bucket hash table // 分析桶散列表
	GCSys       uint64 // GC metadata                 // GC 元数据
	OtherSys    uint64 // other system allocations    // 其它系统分配

	// Garbage collector statistics.
	// 垃圾收集器统计。
	NextGC       uint64 // next collection will happen when HeapAlloc ≥ this amount
	LastGC       uint64 // end time of last collection (nanoseconds since 1970)
	PauseTotalNs uint64
	// 最近GC暂停时间的循环缓存，最近一次应为 [(NumGC+255)%256]
	PauseNs  [256]uint64 // circular buffer of recent GC pause durations, most recent at [(NumGC+255)%256]
	PauseEnd [256]uint64 // circular buffer of recent GC pause end times
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

// Size of the trailing by_size array differs between Go and C,
// and all data after by_size is local to runtime, not exported.
// NumSizeClasses was changed, but we can not change Go struct because of backward compatibility.
// sizeof_C_MStats is what C thinks about size of Go struct.

// Go 和 C 的 by_size 数组结尾是不同的，且 by_size 之后的所有数据对运行时而言是局部的，未导出的。
// NumSizeClasses 已被修改，但考虑到向后兼容，我们不能修改 Go 的结构体。
// sizeof_C_MStats 对于 C 来说类似于 Go 的结构体大小。
var sizeof_C_MStats = unsafe.Offsetof(memstats.by_size) + 61*unsafe.Sizeof(memstats.by_size[0])

func init() {
	var memStats MemStats
	if sizeof_C_MStats != unsafe.Sizeof(memStats) {
		println(sizeof_C_MStats, unsafe.Sizeof(memStats))
		throw("MStats vs MemStatsType size mismatch")
	}
}

// ReadMemStats populates m with memory allocator statistics.

// ReadMemStats 将内存分配器的统计填充到 m 中。
func ReadMemStats(m *MemStats) {
	// Have to acquire worldsema to stop the world,
	// because stoptheworld can only be used by
	// one goroutine at a time, and there might be
	// a pending garbage collection already calling it.
	semacquire(&worldsema, false)
	gp := getg()
	gp.m.preemptoff = "read mem stats"
	systemstack(stoptheworld)

	systemstack(func() {
		readmemstats_m(m)
	})

	gp.m.preemptoff = ""
	gp.m.locks++
	semrelease(&worldsema)
	systemstack(starttheworld)
	gp.m.locks--
}

//go:linkname runtime_debug_WriteHeapDump runtime/debug.WriteHeapDump
func runtime_debug_WriteHeapDump(fd uintptr) {
	semacquire(&worldsema, false)
	gp := getg()
	gp.m.preemptoff = "write heap dump"
	systemstack(stoptheworld)

	systemstack(func() {
		writeheapdump_m(fd)
	})

	gp.m.preemptoff = ""
	gp.m.locks++
	semrelease(&worldsema)
	systemstack(starttheworld)
	gp.m.locks--
}
