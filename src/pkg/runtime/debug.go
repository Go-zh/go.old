// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

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
func GOMAXPROCS(n int) int

// NumCPU returns the number of logical CPUs on the local machine.

// NumCPU 返回本地机器的逻辑CPU数。
func NumCPU() int

// NumCgoCall returns the number of cgo calls made by the current process.

// NumCgoCall 返回由当前进程创建的cgo调用数。
func NumCgoCall() int64

// NumGoroutine returns the number of goroutines that currently exist.

// NumGoroutine 返回当前存在的Go程数。
func NumGoroutine() int

// MemProfileRate controls the fraction of memory allocations
// that are recorded and reported in the memory profile.
// The profiler aims to sample an average of
// one allocation per MemProfileRate bytes allocated.
//
// To include every allocated block in the profile, set MemProfileRate to 1.
// To turn off profiling entirely, set MemProfileRate to 0.
//
// The tools that process the memory profiles assume that the
// profile rate is constant across the lifetime of the program
// and equal to the current value.  Programs that change the
// memory profiling rate should do so just once, as early as
// possible in the execution of the program (for example,
// at the beginning of main).

// MemProfileRate 用于控制内存分析中记录并报告的内存分配。
// 对于一个分配，平均每分配 MemProfileRate 个字节，该分析器就收集一份样本。
//
// 要在分析中包括每一次分配阻塞，需将 MemProfileRate 置为1；
// 要完全关闭分析，需将 MemProfileRate 置为0。
//
// 此内存分析工具假定在程序的整个生命周期中，分析速率为常量且等于当前值。
// 该程序在执行过程中，应当尽早修改内存分析速率，且只修改一次（例如，在 main 的开始处）。
var MemProfileRate int = 512 * 1024

// A MemProfileRecord describes the live objects allocated
// by a particular call sequence (stack trace).

// MemProfileRecord 用于描述由具体调用序列所分配的活动对象（栈跟踪信息）。
type MemProfileRecord struct {
	AllocBytes, FreeBytes     int64       // number of bytes allocated, freed    // 已分配的和已释放的字节数
	AllocObjects, FreeObjects int64       // number of objects allocated, freed  // 已分配的和已释放的对象数
	Stack0                    [32]uintptr // stack trace for this record; ends at first 0 entry // 此记录的栈跟踪信息；在第一个值为0的项处结束
}

// InUseBytes returns the number of bytes in use (AllocBytes - FreeBytes).

// InUseBytes 返回正在使用的字节数（AllocBytes - FreeBytes）。
func (r *MemProfileRecord) InUseBytes() int64 { return r.AllocBytes - r.FreeBytes }

// InUseObjects returns the number of objects in use (AllocObjects - FreeObjects).

// // InUseBytes 返回正在使用的对象数（AllocObjects - FreeObjects）。
func (r *MemProfileRecord) InUseObjects() int64 {
	return r.AllocObjects - r.FreeObjects
}

// Stack returns the stack trace associated with the record,
// a prefix of r.Stack0.

// Stack 返回关联至此记录的栈跟踪信息，即 r.Stack0 的前缀。
func (r *MemProfileRecord) Stack() []uintptr {
	for i, v := range r.Stack0 {
		if v == 0 {
			return r.Stack0[0:i]
		}
	}
	return r.Stack0[0:]
}

// MemProfile returns n, the number of records in the current memory profile.
// If len(p) >= n, MemProfile copies the profile into p and returns n, true.
// If len(p) < n, MemProfile does not change p and returns n, false.
//
// If inuseZero is true, the profile includes allocation records
// where r.AllocBytes > 0 but r.AllocBytes == r.FreeBytes.
// These are sites where memory was allocated, but it has all
// been released back to the runtime.
//
// Most clients should use the runtime/pprof package or
// the testing package's -test.memprofile flag instead
// of calling MemProfile directly.

// MemProfile 返回当前内存分析中的记录数 n。
// 若 len(p) >= n，MemProfile 就会将此分析赋值到 p 中并返回 n, true。
// 若 len(p) < n，MemProfile 则不会更改 p，而只返回 n, false。
//
// 若 inuseZero 为 true，该分析就会包含分配记录，其中 r.AllocBytes > 0，
// 而 r.AllocBytes == r.FreeBytes。这些位置的内存已经分配，但它们都会释放回运行时。
//
// 大多数客户端应当使用 runtime/pprof 包或 testing 包的 -test.memprofile 标记，
// 而非直接调用 MemProfile。
func MemProfile(p []MemProfileRecord, inuseZero bool) (n int, ok bool)

// A StackRecord describes a single execution stack.

// StackRecord 记录一个单一执行的栈。
type StackRecord struct {
	// 此记录的栈跟踪信息；在第一个值为0的项处结束
	Stack0 [32]uintptr // stack trace for this record; ends at first 0 entry
}

// Stack returns the stack trace associated with the record,
// a prefix of r.Stack0.

// Stack 返回关联至此记录的栈跟踪信息，即 r.Stack0 的前缀。
func (r *StackRecord) Stack() []uintptr {
	for i, v := range r.Stack0 {
		if v == 0 {
			return r.Stack0[0:i]
		}
	}
	return r.Stack0[0:]
}

// ThreadCreateProfile returns n, the number of records in the thread creation profile.
// If len(p) >= n, ThreadCreateProfile copies the profile into p and returns n, true.
// If len(p) < n, ThreadCreateProfile does not change p and returns n, false.
//
// Most clients should use the runtime/pprof package instead
// of calling ThreadCreateProfile directly.

// ThreadCreateProfile 返回线程创建分析中的记录数 n。
// 若 len(p) >= n，ThreadCreateProfile 就会将此分析赋值到 p 中并返回 n, true。
// 若 len(p) < n，ThreadCreateProfile 则不会更改 p，而只返回 n, false。
//
// 大多数客户端应当使用 runtime/pprof 包，而非直接调用 ThreadCreateProfile。
func ThreadCreateProfile(p []StackRecord) (n int, ok bool)

// GoroutineProfile returns n, the number of records in the active goroutine stack profile.
// If len(p) >= n, GoroutineProfile copies the profile into p and returns n, true.
// If len(p) < n, GoroutineProfile does not change p and returns n, false.
//
// Most clients should use the runtime/pprof package instead
// of calling GoroutineProfile directly.

// GoroutineProfile 返回活动Go程栈分析中的记录数 n。
// 若 len(p) >= n，GoroutineProfile 就会将此分析赋值到 p 中并返回 n, true。
// 若 len(p) < n，GoroutineProfile 则不会更改 p，而只返回 n, false。
//
// 大多数客户端应当使用 runtime/pprof 包，而非直接调用 GoroutineProfile。
func GoroutineProfile(p []StackRecord) (n int, ok bool)

// CPUProfile returns the next chunk of binary CPU profiling stack trace data,
// blocking until data is available.  If profiling is turned off and all the profile
// data accumulated while it was on has been returned, CPUProfile returns nil.
// The caller must save the returned data before calling CPUProfile again.
//
// Most clients should use the runtime/pprof package or
// the testing package's -test.cpuprofile flag instead of calling
// CPUProfile directly.

// CPUProfile 返回下一个CPU栈跟踪数据的二进制字节片，它会阻塞直到数据可用。
// 若分析关闭且所有积累的分析数据而它已被返回，CPUProfile 就会返回 nil。
// 调用者必须在再次调用 CPUProfile 前保存返回的数据。
//
// 大多数客户端应当使用 runtime/pprof 包或 testing 包的 -test.memprofile 标记，
// 而非直接调用 CPUProfile。
func CPUProfile() []byte

// SetCPUProfileRate sets the CPU profiling rate to hz samples per second.
// If hz <= 0, SetCPUProfileRate turns off profiling.
// If the profiler is on, the rate cannot be changed without first turning it off.
//
// Most clients should use the runtime/pprof package or
// the testing package's -test.cpuprofile flag instead of calling
// SetCPUProfileRate directly.

// SetCPUProfileRate 将 hz 置为CPU分析频率每秒的抽样。
// 若 hz <= 0，SetCPUProfileRate 就会关闭分析。
// 若分析器为打开状态，其频率在它第一次关闭之前就无法更改。
//
// 大多数客户端应当使用 runtime/pprof 包或 testing 包的 -test.memprofile 标记，
// 而非直接调用 SetCPUProfileRate。
func SetCPUProfileRate(hz int)

// SetBlockProfileRate controls the fraction of goroutine blocking events
// that are reported in the blocking profile.  The profiler aims to sample
// an average of one blocking event per rate nanoseconds spent blocked.
//
// To include every blocking event in the profile, pass rate = 1.
// To turn off profiling entirely, pass rate <= 0.

// SetBlockProfileRate 用于控制在阻塞分析中报告的Go程阻塞事件。
// 对于一个阻塞事件，平均每阻塞 rate 纳秒，该分析器就采集一份样本。
//
// 要在分析中包括每一个阻塞事件，需传入 rate = 1；要完全关闭分析，需传入 rate <= 0。
func SetBlockProfileRate(rate int)

// BlockProfileRecord describes blocking events originated
// at a particular call sequence (stack trace).

// BlockProfileRecord 用于描述由具体调用序列所产生的阻塞事件阻塞事件（栈跟踪信息）。
type BlockProfileRecord struct {
	Count  int64
	Cycles int64
	StackRecord
}

// BlockProfile returns n, the number of records in the current blocking profile.
// If len(p) >= n, BlockProfile copies the profile into p and returns n, true.
// If len(p) < n, BlockProfile does not change p and returns n, false.
//
// Most clients should use the runtime/pprof package or
// the testing package's -test.blockprofile flag instead
// of calling BlockProfile directly.

// BlockProfile 返回
// GoroutineProfile 返回当前阻塞分析中的记录数 n。
// 若 len(p) >= n，BlockProfile 就会将此分析赋值到 p 中并返回 n, true。
// 若 len(p) < n，BlockProfile 则不会更改 p，而只返回 n, false。
//
// 大多数客户端应当使用 runtime/pprof 包或 testing 包的 -test.memprofile 标记，
// 而非直接调用 BlockProfile。
func BlockProfile(p []BlockProfileRecord) (n int, ok bool)

// Stack formats a stack trace of the calling goroutine into buf
// and returns the number of bytes written to buf.
// If all is true, Stack formats stack traces of all other goroutines
// into buf after the trace for the current goroutine.

// Stack 将调用Go程测栈跟踪信息格式化写入到 buf 中并返回写入 buf 的字节数。
// 若 all 为 true，Stack 会在当前Go程的跟踪信息后，
// 将其它所有Go程的栈跟踪信息都格式化写入到 buf 中。
func Stack(buf []byte, all bool) int
