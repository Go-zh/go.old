// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pprof writes runtime profiling data in the format expected
// by the pprof visualization tool.
// For more information about pprof, see
// http://code.google.com/p/google-perftools/.

// pprof 包按照可视化工具 pprof 所要求的格式写出运行时分析数据.
// 更多有关 pprof 的信息见 http://code.google.com/p/google-perftools/。
package pprof

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

// BUG(rsc): A bug in the OS X Snow Leopard 64-bit kernel prevents
// CPU profiling from giving accurate results on that system.

// BUG(rsc): OS X Snow Leopard 64位内核有一个bug会妨碍CPU分析为该系统提供准确的结果。

// A Profile is a collection of stack traces showing the call sequences
// that led to instances of a particular event, such as allocation.
// Packages can create and maintain their own profiles; the most common
// use is for tracking resources that must be explicitly closed, such as files
// or network connections.
//
// A Profile's methods can be called from multiple goroutines simultaneously.
//
// Each Profile has a unique name.  A few profiles are predefined:
//
//	goroutine    - stack traces of all current goroutines
//	heap         - a sampling of all heap allocations
//	threadcreate - stack traces that led to the creation of new OS threads
//	block        - stack traces that led to blocking on synchronization primitives
//
// These predefined profiles maintain themselves and panic on an explicit
// Add or Remove method call.
//
// The CPU profile is not available as a Profile.  It has a special API,
// the StartCPUProfile and StopCPUProfile functions, because it streams
// output to a writer during profiling.
//

// Profile 是一个栈跟踪的集合，它显示了引导特定事件实例的调用序列，例如分配。
// 包可以创建并维护它们自己的分析，它一般用于跟踪必须被显式关闭的资源，例如文件或网络连接。
//
// 一个 Profile 的方法可被多个Go程同时调用。
//
// 每个 Profile 都有唯一的名称。有些 Profile 是预定义的：
//
//	goroutine    - 所有当前Go程的栈跟踪
//	heap         - 所有堆分配的采样
//	threadcreate - 引导新OS的线程创建的栈跟踪
//	block        - 引导同步原语中阻塞的栈跟踪
//
// 这些预声明分析并不能作为 Profile 使用。它有专门的API，即 StartCPUProfile 和
// StopCPUProfile 函数，因为它在分析时是以流的形式输出到写入器的。
//
type Profile struct {
	name  string
	mu    sync.Mutex
	m     map[interface{}][]uintptr
	count func() int
	write func(io.Writer, int) error
}

// profiles records all registered profiles.

// profiles 记录所有已注册的分析。
var profiles struct {
	mu sync.Mutex
	m  map[string]*Profile
}

var goroutineProfile = &Profile{
	name:  "goroutine",
	count: countGoroutine,
	write: writeGoroutine,
}

var threadcreateProfile = &Profile{
	name:  "threadcreate",
	count: countThreadCreate,
	write: writeThreadCreate,
}

var heapProfile = &Profile{
	name:  "heap",
	count: countHeap,
	write: writeHeap,
}

var blockProfile = &Profile{
	name:  "block",
	count: countBlock,
	write: writeBlock,
}

func lockProfiles() {
	profiles.mu.Lock()
	if profiles.m == nil {
		// Initial built-in profiles.
		profiles.m = map[string]*Profile{
			"goroutine":    goroutineProfile,
			"threadcreate": threadcreateProfile,
			"heap":         heapProfile,
			"block":        blockProfile,
		}
	}
}

func unlockProfiles() {
	profiles.mu.Unlock()
}

// NewProfile creates a new profile with the given name.
// If a profile with that name already exists, NewProfile panics.
// The convention is to use a 'import/path.' prefix to create
// separate name spaces for each package.

// NewProfile 以给定的名称创建一个新的分析。
// 若拥有该名称的分析已存在，NewProfile 就会引起恐慌。
// 约定使用一个 'import/path' 导入路径前缀来为每个包创建单独的命名空间。
func NewProfile(name string) *Profile {
	lockProfiles()
	defer unlockProfiles()
	if name == "" {
		panic("pprof: NewProfile with empty name")
	}
	if profiles.m[name] != nil {
		panic("pprof: NewProfile name already in use: " + name)
	}
	p := &Profile{
		name: name,
		m:    map[interface{}][]uintptr{},
	}
	profiles.m[name] = p
	return p
}

// Lookup returns the profile with the given name, or nil if no such profile exists.

// Lookup 返回给定名称的分析，若不存在该分析，则返回 nil。
func Lookup(name string) *Profile {
	lockProfiles()
	defer unlockProfiles()
	return profiles.m[name]
}

// Profiles returns a slice of all the known profiles, sorted by name.

// Profiles 返回所有已知分析的切片，按名称排序。
func Profiles() []*Profile {
	lockProfiles()
	defer unlockProfiles()

	var all []*Profile
	for _, p := range profiles.m {
		all = append(all, p)
	}

	sort.Sort(byName(all))
	return all
}

type byName []*Profile

func (x byName) Len() int           { return len(x) }
func (x byName) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byName) Less(i, j int) bool { return x[i].name < x[j].name }

// Name returns this profile's name, which can be passed to Lookup to reobtain the profile.

// Name 返回该分析的名称，它可被传入 Lookup 来重新获取该分析。
func (p *Profile) Name() string {
	return p.name
}

// Count returns the number of execution stacks currently in the profile.

// Count 返回该分析中当前执行栈的数量。
func (p *Profile) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.count != nil {
		return p.count()
	}
	return len(p.m)
}

// Add adds the current execution stack to the profile, associated with value.
// Add stores value in an internal map, so value must be suitable for use as
// a map key and will not be garbage collected until the corresponding
// call to Remove.  Add panics if the profile already contains a stack for value.
//
// The skip parameter has the same meaning as runtime.Caller's skip
// and controls where the stack trace begins.  Passing skip=0 begins the
// trace in the function calling Add.  For example, given this
// execution stack:
//
//	Add
//	called from rpc.NewClient
//	called from mypkg.Run
//	called from main.main
//
// Passing skip=0 begins the stack trace at the call to Add inside rpc.NewClient.
// Passing skip=1 begins the stack trace at the call to NewClient inside mypkg.Run.
//

// Add 将当前与值相关联的执行栈添加到该分析中。
// Add 在一个内部映射中存储值，因此值必须适于用作映射键，且在对应的 Remove
// 调用之前不会被垃圾收集。若分析已经包含了值的栈，Add 就会引发恐慌。
//
// skip 形参与 runtime.Caller 的 skip 意思相同，它用于控制栈跟踪从哪里开始。
// 传入 skip=0 会从函数调用 Add 处开始跟踪。例如，给定以下执行栈：
//
//	Add
//	调用自 rpc.NewClient
//	调用自 mypkg.Run
//	调用自 main.main
//
// 传入 skip=0 会从 rpc.NewClient 中的 Add 调用处开始栈跟踪。
// 传入 skip=1 会从 mypkg.Run 中的 NewClient 调用处开始栈跟踪。
//
func (p *Profile) Add(value interface{}, skip int) {
	if p.name == "" {
		panic("pprof: use of uninitialized Profile")
	}
	if p.write != nil {
		panic("pprof: Add called on built-in Profile " + p.name)
	}

	stk := make([]uintptr, 32)
	n := runtime.Callers(skip+1, stk[:])

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m[value] != nil {
		panic("pprof: Profile.Add of duplicate value")
	}
	p.m[value] = stk[:n]
}

// Remove removes the execution stack associated with value from the profile.
// It is a no-op if the value is not in the profile.

// Remove 从该分析中移除与值 value 相关联的执行栈。
// 若值 value 不在此分析中，则为空操作。
func (p *Profile) Remove(value interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.m, value)
}

// WriteTo writes a pprof-formatted snapshot of the profile to w.
// If a write to w returns an error, WriteTo returns that error.
// Otherwise, WriteTo returns nil.
//
// The debug parameter enables additional output.
// Passing debug=0 prints only the hexadecimal addresses that pprof needs.
// Passing debug=1 adds comments translating addresses to function names
// and line numbers, so that a programmer can read the profile without tools.
//
// The predefined profiles may assign meaning to other debug values;
// for example, when printing the "goroutine" profile, debug=2 means to
// print the goroutine stacks in the same form that a Go program uses
// when dying due to an unrecovered panic.

// WriteTo 将pprof格式的分析快照写入 w 中。
// 若一个向 w 的写入返回一个错误，WriteTo 就会返回该错误。
// 否则，WriteTo 就会返回 nil。
//
// debug 形参用于开启附加的输出。
// 传入 debug=0 只会打印pprof所需要的十六进制地址。
// 传入 debug=1 会将地址翻译为函数名和行号并添加注释，以便让程序员无需工具阅读分析报告。
//
// 预声明分析报告可为其它 debug 值赋予含义；例如，当打印“Go程”的分析报告时，
// debug=2 意为：由于不可恢复的恐慌而濒临崩溃时，使用与Go程序相同的形式打印Go程的栈信息。
func (p *Profile) WriteTo(w io.Writer, debug int) error {
	if p.name == "" {
		panic("pprof: use of zero Profile")
	}
	if p.write != nil {
		return p.write(w, debug)
	}

	// Obtain consistent snapshot under lock; then process without lock.
	// 在锁定状态下获得连续的快照；然后继续进行无锁处理。
	var all [][]uintptr
	p.mu.Lock()
	for _, stk := range p.m {
		all = append(all, stk)
	}
	p.mu.Unlock()

	// Map order is non-deterministic; make output deterministic.
	// 映射的顺序是不确定的；通过它来确定输出顺序。
	sort.Sort(stackProfile(all))

	return printCountProfile(w, debug, p.name, stackProfile(all))
}

type stackProfile [][]uintptr

func (x stackProfile) Len() int              { return len(x) }
func (x stackProfile) Stack(i int) []uintptr { return x[i] }
func (x stackProfile) Swap(i, j int)         { x[i], x[j] = x[j], x[i] }
func (x stackProfile) Less(i, j int) bool {
	t, u := x[i], x[j]
	for k := 0; k < len(t) && k < len(u); k++ {
		if t[k] != u[k] {
			return t[k] < u[k]
		}
	}
	return len(t) < len(u)
}

// A countProfile is a set of stack traces to be printed as counts
// grouped by stack trace.  There are multiple implementations:
// all that matters is that we can find out how many traces there are
// and obtain each trace in turn.

// countProfile 是一组栈跟踪，它作为栈跟踪分组的计数被打印出来。
// 它有多种实现：最重要的是，我们可以找出这里有多少跟踪信息，并轮流获取每一个跟踪。
type countProfile interface {
	Len() int
	Stack(i int) []uintptr
}

// printCountProfile prints a countProfile at the specified debug level.

// printCountProfile 按指定的 debug 级别打印 countProfile。
func printCountProfile(w io.Writer, debug int, name string, p countProfile) error {
	b := bufio.NewWriter(w)
	var tw *tabwriter.Writer
	w = b
	if debug > 0 {
		tw = tabwriter.NewWriter(w, 1, 8, 1, '\t', 0)
		w = tw
	}

	fmt.Fprintf(w, "%s profile: total %d\n", name, p.Len())

	// Build count of each stack.
	// 建立每一个栈的计数。
	var buf bytes.Buffer
	key := func(stk []uintptr) string {
		buf.Reset()
		fmt.Fprintf(&buf, "@")
		for _, pc := range stk {
			fmt.Fprintf(&buf, " %#x", pc)
		}
		return buf.String()
	}
	m := map[string]int{}
	n := p.Len()
	for i := 0; i < n; i++ {
		m[key(p.Stack(i))]++
	}

	// Print stacks, listing count on first occurrence of a unique stack.
	// 打印栈信息，列出一个唯一栈第一次出现的计数。
	for i := 0; i < n; i++ {
		stk := p.Stack(i)
		s := key(stk)
		if count := m[s]; count != 0 {
			fmt.Fprintf(w, "%d %s\n", count, s)
			if debug > 0 {
				printStackRecord(w, stk, false)
			}
			delete(m, s)
		}
	}

	if tw != nil {
		tw.Flush()
	}
	return b.Flush()
}

// printStackRecord prints the function + source line information
// for a single stack trace.

// printStackRecord 为单个栈跟踪打印出函数+源行的信息。
func printStackRecord(w io.Writer, stk []uintptr, allFrames bool) {
	show := allFrames
	for _, pc := range stk {
		f := runtime.FuncForPC(pc)
		if f == nil {
			show = true
			fmt.Fprintf(w, "#\t%#x\n", pc)
		} else {
			file, line := f.FileLine(pc)
			name := f.Name()
			// Hide runtime.goexit and any runtime functions at the beginning.
			// This is useful mainly for allocation traces.
			// 隐藏 runtime.goexit 以及任何在起始处的运行时函数。
			// 这主要用于分配跟踪。
			if name == "runtime.goexit" || !show && strings.HasPrefix(name, "runtime.") {
				continue
			}
			show = true
			fmt.Fprintf(w, "#\t%#x\t%s+%#x\t%s:%d\n", pc, f.Name(), pc-f.Entry(), file, line)
		}
	}
	if !show {
		// We didn't print anything; do it again,
		// and this time include runtime functions.
		// 我们无需打印任何东西；再做一次，而这次包括运行时函数。
		printStackRecord(w, stk, true)
		return
	}
	fmt.Fprintf(w, "\n")
}

// Interface to system profiles.

// 系统分析的接口。

type byInUseBytes []runtime.MemProfileRecord

func (x byInUseBytes) Len() int           { return len(x) }
func (x byInUseBytes) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byInUseBytes) Less(i, j int) bool { return x[i].InUseBytes() > x[j].InUseBytes() }

// WriteHeapProfile is shorthand for Lookup("heap").WriteTo(w, 0).
// It is preserved for backwards compatibility.

// WriteHeapProfile 是 Lookup("heap").WriteTo(w, 0) 的简写。
// 它是为了保持向后兼容性而存在的。
func WriteHeapProfile(w io.Writer) error {
	return writeHeap(w, 0)
}

// countHeap returns the number of records in the heap profile.

// countHeap 返回堆分析中的记录数。
func countHeap() int {
	n, _ := runtime.MemProfile(nil, true)
	return n
}

// writeHeap writes the current runtime heap profile to w.

// writeHeap 将当前运行时堆的分析报告写入到 w 中。
func writeHeap(w io.Writer, debug int) error {
	// Find out how many records there are (MemProfile(nil, true)),
	// allocate that many records, and get the data.
	// There's a race—more records might be added between
	// the two calls—so allocate a few extra records for safety
	// and also try again if we're very unlucky.
	// The loop should only execute one iteration in the common case.
	// 找出这里有多少记录（MemProfile(nil, true)），为它们分配一些记录，并获取数据。
	// 这里有个竞争——在两次调用之间可能会添加更多记录——因此为安全起见，
	// 我们分配了额外的记录，如果不走运的话可以再试一次。
	// 此循环在一般情况下应当只执行一次迭代。
	var p []runtime.MemProfileRecord
	n, ok := runtime.MemProfile(nil, true)
	for {
		// Allocate room for a slightly bigger profile,
		// in case a few more entries have been added
		// since the call to MemProfile.
		// 为稍大一点的分析报告分配空间，以防调用 MemProfile 时增加更多条目。
		p = make([]runtime.MemProfileRecord, n+50)
		n, ok = runtime.MemProfile(p, true)
		if ok {
			p = p[0:n]
			break
		}
		// Profile grew; try again.
		// 分析报告增加，然后重试。
	}

	sort.Sort(byInUseBytes(p))

	b := bufio.NewWriter(w)
	var tw *tabwriter.Writer
	w = b
	if debug > 0 {
		tw = tabwriter.NewWriter(w, 1, 8, 1, '\t', 0)
		w = tw
	}

	var total runtime.MemProfileRecord
	for i := range p {
		r := &p[i]
		total.AllocBytes += r.AllocBytes
		total.AllocObjects += r.AllocObjects
		total.FreeBytes += r.FreeBytes
		total.FreeObjects += r.FreeObjects
	}

	// Technically the rate is MemProfileRate not 2*MemProfileRate,
	// but early versions of the C++ heap profiler reported 2*MemProfileRate,
	// so that's what pprof has come to expect.
	// 技术上速率应为 MemProfileRate 而非 2*MemProfileRate，但早期版本的 C++
	// 堆分析器会报告2*MemProfileRate，所以这就是pprof必须这样预期的原因。
	fmt.Fprintf(w, "heap profile: %d: %d [%d: %d] @ heap/%d\n",
		total.InUseObjects(), total.InUseBytes(),
		total.AllocObjects, total.AllocBytes,
		2*runtime.MemProfileRate)

	for i := range p {
		r := &p[i]
		fmt.Fprintf(w, "%d: %d [%d: %d] @",
			r.InUseObjects(), r.InUseBytes(),
			r.AllocObjects, r.AllocBytes)
		for _, pc := range r.Stack() {
			fmt.Fprintf(w, " %#x", pc)
		}
		fmt.Fprintf(w, "\n")
		if debug > 0 {
			printStackRecord(w, r.Stack(), false)
		}
	}

	// Print memstats information too.
	// Pprof will ignore, but useful for people
	// 打印 memstats 信息。pprof 会忽略它，但这对人们却很有用。
	if debug > 0 {
		s := new(runtime.MemStats)
		runtime.ReadMemStats(s)
		fmt.Fprintf(w, "\n# runtime.MemStats\n")
		fmt.Fprintf(w, "# Alloc = %d\n", s.Alloc)
		fmt.Fprintf(w, "# TotalAlloc = %d\n", s.TotalAlloc)
		fmt.Fprintf(w, "# Sys = %d\n", s.Sys)
		fmt.Fprintf(w, "# Lookups = %d\n", s.Lookups)
		fmt.Fprintf(w, "# Mallocs = %d\n", s.Mallocs)
		fmt.Fprintf(w, "# Frees = %d\n", s.Frees)

		fmt.Fprintf(w, "# HeapAlloc = %d\n", s.HeapAlloc)
		fmt.Fprintf(w, "# HeapSys = %d\n", s.HeapSys)
		fmt.Fprintf(w, "# HeapIdle = %d\n", s.HeapIdle)
		fmt.Fprintf(w, "# HeapInuse = %d\n", s.HeapInuse)
		fmt.Fprintf(w, "# HeapReleased = %d\n", s.HeapReleased)
		fmt.Fprintf(w, "# HeapObjects = %d\n", s.HeapObjects)

		fmt.Fprintf(w, "# Stack = %d / %d\n", s.StackInuse, s.StackSys)
		fmt.Fprintf(w, "# MSpan = %d / %d\n", s.MSpanInuse, s.MSpanSys)
		fmt.Fprintf(w, "# MCache = %d / %d\n", s.MCacheInuse, s.MCacheSys)
		fmt.Fprintf(w, "# BuckHashSys = %d\n", s.BuckHashSys)

		fmt.Fprintf(w, "# NextGC = %d\n", s.NextGC)
		fmt.Fprintf(w, "# PauseNs = %d\n", s.PauseNs)
		fmt.Fprintf(w, "# NumGC = %d\n", s.NumGC)
		fmt.Fprintf(w, "# EnableGC = %v\n", s.EnableGC)
		fmt.Fprintf(w, "# DebugGC = %v\n", s.DebugGC)
	}

	if tw != nil {
		tw.Flush()
	}
	return b.Flush()
}

// countThreadCreate returns the size of the current ThreadCreateProfile.

// countThreadCreate 返回当前 ThreadCreateProfile 的大小。
func countThreadCreate() int {
	n, _ := runtime.ThreadCreateProfile(nil)
	return n
}

// writeThreadCreate writes the current runtime ThreadCreateProfile to w.

// writeThreadCreate 将当前运行时 ThreadCreateProfile 写入 w 中。
func writeThreadCreate(w io.Writer, debug int) error {
	return writeRuntimeProfile(w, debug, "threadcreate", runtime.ThreadCreateProfile)
}

// countGoroutine returns the number of goroutines.

// countGoroutine 返回Go程数。
func countGoroutine() int {
	return runtime.NumGoroutine()
}

// writeGoroutine writes the current runtime GoroutineProfile to w.

// writeGoroutine 将当前运行时 GoroutineProfilewrites 写入 w 中。
func writeGoroutine(w io.Writer, debug int) error {
	if debug >= 2 {
		return writeGoroutineStacks(w)
	}
	return writeRuntimeProfile(w, debug, "goroutine", runtime.GoroutineProfile)
}

func writeGoroutineStacks(w io.Writer) error {
	// We don't know how big the buffer needs to be to collect
	// all the goroutines.  Start with 1 MB and try a few times, doubling each time.
	// Give up and use a truncated trace if 64 MB is not enough.
	// 我们不知道收集所有Go程需要多大的缓存。从1MB开始然后重试几次，每次加倍。
	// 如果到了64MB还不够的话就放弃，转而使用截断跟踪。
	buf := make([]byte, 1<<20)
	for i := 0; ; i++ {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		if len(buf) >= 64<<20 {
			// Filled 64 MB - stop there.
			// 填满64MB就停止。
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	_, err := w.Write(buf)
	return err
}

func writeRuntimeProfile(w io.Writer, debug int, name string, fetch func([]runtime.StackRecord) (int, bool)) error {
	// Find out how many records there are (fetch(nil)),
	// allocate that many records, and get the data.
	// There's a race—more records might be added between
	// the two calls—so allocate a few extra records for safety
	// and also try again if we're very unlucky.
	// The loop should only execute one iteration in the common case.
	// 找出这里有多少记录（fetch(nil)），为它们分配一些记录，并获取数据。
	// 这里有个竞争——在两次调用之间可能会添加更多记录——因此为安全起见，
	// 我们分配了额外的记录，如果不走运的话可以再试一次。
	// 此循环在一般情况下应当只执行一次迭代。
	var p []runtime.StackRecord
	n, ok := fetch(nil)
	for {
		// Allocate room for a slightly bigger profile,
		// in case a few more entries have been added
		// since the call to ThreadProfile.
		// 为稍大一点的分析报告分配空间，以防调用 ThreadProfile 时增加更多条目。
		p = make([]runtime.StackRecord, n+10)
		n, ok = fetch(p)
		if ok {
			p = p[0:n]
			break
		}
		// Profile grew; try again.
		// 分析报告增加，然后重试。
	}

	return printCountProfile(w, debug, name, runtimeProfile(p))
}

type runtimeProfile []runtime.StackRecord

func (p runtimeProfile) Len() int              { return len(p) }
func (p runtimeProfile) Stack(i int) []uintptr { return p[i].Stack() }

var cpu struct {
	sync.Mutex
	profiling bool
	done      chan bool
}

// StartCPUProfile enables CPU profiling for the current process.
// While profiling, the profile will be buffered and written to w.
// StartCPUProfile returns an error if profiling is already enabled.

// StartCPUProfile 为当前进程开启CPU分析。
// 在分析时，分析报告会缓存并写入到 w 中。若分析已经开启，StartCPUProfile
// 就会返回错误。
func StartCPUProfile(w io.Writer) error {
	// The runtime routines allow a variable profiling rate,
	// but in practice operating systems cannot trigger signals
	// at more than about 500 Hz, and our processing of the
	// signal is not cheap (mostly getting the stack trace).
	// 100 Hz is a reasonable choice: it is frequent enough to
	// produce useful data, rare enough not to bog down the
	// system, and a nice round number to make it easy to
	// convert sample counts to seconds.  Instead of requiring
	// each client to specify the frequency, we hard code it.
	// 运行时例程允许可变的分析速率，但在实践中，操作系统并不能以超过500Hz
	// 的频率触发信号，而我们对信号的处理并不廉价（主要是获得栈跟踪信息）。
	// 100Hz是个不错的选择：此频率足以产生有用的信息，又不至于让系统瘫痪，
	// 而且这个不错的整数也能让采样计数转换成秒变得很容易。
	// 因此我们不会让每一个客户端都指定频率，而是将这一点作为硬性规定。
	const hz = 100

	// Avoid queueing behind StopCPUProfile.
	// Could use TryLock instead if we had it.
	// 避免在 StopCPUProfile 后面排队。
	// 如果我们已经开始分析，也可以使用 TryLock 代替。
	if cpu.profiling {
		return fmt.Errorf("cpu profiling already in use")
	}

	cpu.Lock()
	defer cpu.Unlock()
	if cpu.done == nil {
		cpu.done = make(chan bool)
	}
	// Double-check.
	// 再次检查。
	if cpu.profiling {
		return fmt.Errorf("cpu profiling already in use")
	}
	cpu.profiling = true
	runtime.SetCPUProfileRate(hz)
	go profileWriter(w)
	return nil
}

func profileWriter(w io.Writer) {
	for {
		data := runtime.CPUProfile()
		if data == nil {
			break
		}
		w.Write(data)
	}
	cpu.done <- true
}

// StopCPUProfile stops the current CPU profile, if any.
// StopCPUProfile only returns after all the writes for the
// profile have completed.

// StopCPUProfile 会停止当前的CPU分析，如果有的话。
// StopCPUProfile 只会在所有的分析报告写入完毕后才会返回。
func StopCPUProfile() {
	cpu.Lock()
	defer cpu.Unlock()

	if !cpu.profiling {
		return
	}
	cpu.profiling = false
	runtime.SetCPUProfileRate(0)
	<-cpu.done
}

type byCycles []runtime.BlockProfileRecord

func (x byCycles) Len() int           { return len(x) }
func (x byCycles) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
func (x byCycles) Less(i, j int) bool { return x[i].Cycles > x[j].Cycles }

// countBlock returns the number of records in the blocking profile.

// countBlock 返回阻塞分析报告职工的记录数。
func countBlock() int {
	n, _ := runtime.BlockProfile(nil)
	return n
}

// writeBlock 将当前阻塞分析写入 w 中。
func writeBlock(w io.Writer, debug int) error {
	var p []runtime.BlockProfileRecord
	n, ok := runtime.BlockProfile(nil)
	for {
		p = make([]runtime.BlockProfileRecord, n+50)
		n, ok = runtime.BlockProfile(p)
		if ok {
			p = p[:n]
			break
		}
	}

	sort.Sort(byCycles(p))

	b := bufio.NewWriter(w)
	var tw *tabwriter.Writer
	w = b
	if debug > 0 {
		tw = tabwriter.NewWriter(w, 1, 8, 1, '\t', 0)
		w = tw
	}

	fmt.Fprintf(w, "--- contention:\n")
	fmt.Fprintf(w, "cycles/second=%v\n", runtime_cyclesPerSecond())
	for i := range p {
		r := &p[i]
		fmt.Fprintf(w, "%v %v @", r.Cycles, r.Count)
		for _, pc := range r.Stack() {
			fmt.Fprintf(w, " %#x", pc)
		}
		fmt.Fprint(w, "\n")
		if debug > 0 {
			printStackRecord(w, r.Stack(), false)
		}
	}

	if tw != nil {
		tw.Flush()
	}
	return b.Flush()
}

func runtime_cyclesPerSecond() int64
