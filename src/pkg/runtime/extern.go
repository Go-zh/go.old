// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package runtime contains operations that interact with Go's runtime system,
such as functions to control goroutines. It also includes the low-level type information
used by the reflect package; see reflect's documentation for the programmable
interface to the run-time type system.

Environment Variables

The following environment variables ($name or %name%, depending on the host
operating system) control the run-time behavior of Go programs. The meanings
and use may change from release to release.

The GOGC variable sets the initial garbage collection target percentage.
A collection is triggered when the ratio of freshly allocated data to live data
remaining after the previous collection reaches this percentage. The default
is GOGC=100. Setting GOGC=off disables the garbage collector entirely.
The runtime/debug package's SetGCPercent function allows changing this
percentage at run time. See http://golang.org/pkg/runtime/debug/#SetGCPercent.

The GODEBUG variable controls debug output from the runtime. GODEBUG value is
a comma-separated list of name=val pairs. Supported names are:

	allocfreetrace: setting allocfreetrace=1 causes every allocation to be
	profiled and a stack trace printed on each object's allocation and free.

	efence: setting efence=1 causes the allocator to run in a mode
	where each object is allocated on a unique page and addresses are
	never recycled.

	gctrace: setting gctrace=1 causes the garbage collector to emit a single line to standard
	error at each collection, summarizing the amount of memory collected and the
	length of the pause. Setting gctrace=2 emits the same summary but also
	repeats each collection.

	gcdead: setting gcdead=1 causes the garbage collector to clobber all stack slots
	that it thinks are dead.

	scheddetail: setting schedtrace=X and scheddetail=1 causes the scheduler to emit
	detailed multiline info every X milliseconds, describing state of the scheduler,
	processors, threads and goroutines.

	schedtrace: setting schedtrace=X causes the scheduler to emit a single line to standard
	error every X milliseconds, summarizing the scheduler state.

The GOMAXPROCS variable limits the number of operating system threads that
can execute user-level Go code simultaneously. There is no limit to the number of threads
that can be blocked in system calls on behalf of Go code; those do not count against
the GOMAXPROCS limit. This package's GOMAXPROCS function queries and changes
the limit.

The GOTRACEBACK variable controls the amount of output generated when a Go
program fails due to an unrecovered panic or an unexpected runtime condition.
By default, a failure prints a stack trace for every extant goroutine, eliding functions
internal to the run-time system, and then exits with exit code 2.
If GOTRACEBACK=0, the per-goroutine stack traces are omitted entirely.
If GOTRACEBACK=1, the default behavior is used.
If GOTRACEBACK=2, the per-goroutine stack traces include run-time functions.
If GOTRACEBACK=crash, the per-goroutine stack traces include run-time functions,
and if possible the program crashes in an operating-specific manner instead of
exiting. For example, on Unix systems, the program raises SIGABRT to trigger a
core dump.

The GOARCH, GOOS, GOPATH, and GOROOT environment variables complete
the set of Go environment variables. They influence the building of Go programs
(see http://golang.org/cmd/go and http://golang.org/pkg/go/build).
GOARCH, GOOS, and GOROOT are recorded at compile time and made available by
constants or functions in this package, but they do not influence the execution
of the run-time system.
*/

/*
TODO(osc): 需更新
runtime 包含与Go的运行时系统进行交互的操作，例如用于控制Go程的函数.
它也包括用于 reflect 包的底层类型信息；运行时类型系统的可编程接口见 reflect 文档。

环境变量

以下环境变量（$name 或 %name%, 取决于宿主操作系统）控制了Go程序的运行时行为。
其意义与使用方法在发行版之间可能有所不同。

GOGC 变量用于设置初始垃圾回收的目标百分比。从上次回收后开始，当新分配数据的比例占到剩余实时数据的此百分比时，
就会再次触发回收。默认为 GOGC=100。要完全关闭垃圾回收器，需设置 GOGC=off。runtime/debug
包的 SetGCPercent 函数允许在运行时更改此百分比。
详见 http://zh.golanger.com/pkg/runtime/debug/#SetGCPercent。

GOGCTRACE 变量用于控制来自垃圾回收器的调试输出。设置 GOGCTRACE=1 会使垃圾回收器发出
每一次回收所产生的单行标准错误输出、概述回收的内存量以及暂停的时长。设置 GOGCTRACE=2
不仅会发出同样的概述，还会重复每一次回收。

GOMAXPROCS 变量用于限制可同时执行的用户级Go代码所产生的操作系统线程数。对于Go代码所代表的系统调用而言，
可被阻塞的线程则没有限制；它们不计入 GOMAXPROCS 的限制。本包中的 GOMAXPROCS 函数可查询并更改此限制。

GOTRACEBACK 用于控制因未恢复的恐慌或意外的运行时状况导致Go程序运行失败时所产生的输出量。
默认情况下，失败会为每个现有的Go程打印出栈跟踪，省略运行时系统的内部函数，并以退出码 2 退出。
若 GOTRACEBACK=0，则每个Go程的栈跟踪都会完全省略。
若 GOTRACEBACK=1，则采用默认的行为。
若 GOTRACEBACK=2，则每个Go程的栈跟踪，包括运行时函数都会输出。
若 GOTRACEBACK=crash，则每个Go程的栈跟踪，包括运行时函数，都会输出，
此外程序可能以操作系统特定的方式崩溃而非退出。例如，在Unix系统上，程序会发出 SIGABRT
信号，从而触发内核转储。

GOARCH、GOOS、GOPATH 和 GOROOT 环境变量均为Go的环境变量。它们影响了Go程序的构建
（详见 http://golang.org/cmd/go 和 http://golang.org/pkg/go/build）。
GOARCH、GOOS 和 GOROOT 会在编译时被记录，并使该包中的常量或函数变得可用，
但它们并不影响运行时系统的执行。
*/
package runtime

// Gosched yields the processor, allowing other goroutines to run.  It does not
// suspend the current goroutine, so execution resumes automatically.

// Gosched 使当前Go程放弃处理器以让其它Go程运行。
// 它不会挂起当前Go程，因而它会自动继续执行。
func Gosched()

// Goexit terminates the goroutine that calls it.  No other goroutine is affected.
// Goexit runs all deferred calls before terminating the goroutine.
//
// Calling Goexit from the main goroutine terminates that goroutine
// without func main returning. Since func main has not returned,
// the program continues execution of other goroutines.
// If all other goroutines exit, the program crashes.

// Goexit 终止调用它的Go程。
// 其它Go程则不受影响。Goexit 会在终止该Go程前调用所有已推迟的调用。
//
// main Go程在调用 Goexit 后会直接终止而不等待 func main 返回。由于 func main 并未返回，
// 该程序会继续执行其它Go程。若所有其它的Go程都已退出，该程序就会崩溃。
func Goexit()

// Caller reports file and line number information about function invocations on
// the calling goroutine's stack.  The argument skip is the number of stack frames
// to ascend, with 0 identifying the caller of Caller.  (For historical reasons the
// meaning of skip differs between Caller and Callers.) The return values report the
// program counter, file name, and line number within the file of the corresponding
// call.  The boolean ok is false if it was not possible to recover the information.

// Caller 报告关于调用Go程的栈上的函数调用的文件和行号信息。
// 实参 skip 为占用的栈帧数，若为0则表示 Caller 的调用者。（由于历史原因，skip
// 的意思在 Caller 和 Callers 中并不相同。）返回值报告程序计数器，
// 文件名及对应调用的文件中的行号。若无法获得信息，布尔值 ok 即为 false。
func Caller(skip int) (pc uintptr, file string, line int, ok bool)

// Callers fills the slice pc with the program counters of function invocations
// on the calling goroutine's stack.  The argument skip is the number of stack frames
// to skip before recording in pc, with 0 identifying the frame for Callers itself and
// 1 identifying the caller of Callers.
// It returns the number of entries written to pc.

// Callers 把调用它的函数Go程栈上的程序计数器填入切片 pc 中。
// 实参 skip 为开始在 pc 中记录之前所要跳过的栈帧数，若为0则表示 Callers 自身的栈帧，
// 若为1则表示 Callers 的调用者。它返回写入到 pc 中的项数。
func Callers(skip int, pc []uintptr) int

type Func struct {
	opaque struct{} // unexported field to disallow conversions // 用未导出字段来进制转换
}

// FuncForPC returns a *Func describing the function that contains the
// given program counter address, or else nil.

// FuncForPC 返回一个 *Func，它描述了包含给定程序计数器地址的函数，否则返回 nil。
func FuncForPC(pc uintptr) *Func

// Name returns the name of the function.

// Name 返回该函数的名称
func (f *Func) Name() string {
	return funcname_go(f)
}

// Entry returns the entry address of the function.

// Entry 返回该项函数的地址。
func (f *Func) Entry() uintptr {
	return funcentry_go(f)
}

// FileLine returns the file name and line number of the
// source code corresponding to the program counter pc.
// The result will not be accurate if pc is not a program
// counter within f.

// FileLine 返回与程序计数器 pc 对应的源码文件名和行号。
// 若 pc 不是 f 中的程序计数器，其结果将是不确定的。
func (f *Func) FileLine(pc uintptr) (file string, line int) {
	return funcline_go(f, pc)
}

// implemented in symtab.c

// 在 symtab.c 中实现
func funcline_go(*Func, uintptr) (string, int)
func funcname_go(*Func) string
func funcentry_go(*Func) uintptr

func getgoroot() string

// GOROOT returns the root of the Go tree.
// It uses the GOROOT environment variable, if set,
// or else the root used during the Go build.

// GOROOT 返回Go目录树的根目录。
// 若设置了GOROOT环境变量，就会使用它，否则就会将Go的构建目录作为根目录
func GOROOT() string {
	s := getgoroot()
	if s != "" {
		return s
	}
	return defaultGoroot
}

// Version returns the Go tree's version string.
// It is either the commit hash and date at the time of the build or,
// when possible, a release tag like "go1.3".

// Version 返回Go目录树的版本字符串。
// 它一般是一个提交散列值及其构建时间，也可能是一个类似于 "go1.3" 的发行标注。
func Version() string {
	return theVersion
}

// GOOS is the running program's operating system target:
// one of darwin, freebsd, linux, and so on.

// GOOS 为所运行程序的目标操作系统：
// darwin、freebsd或linux等等。
const GOOS string = theGoos

// GOARCH is the running program's architecture target:
// 386, amd64, or arm.

// GOARCH 为所运行程序的目标架构：
// 386、amd64 或 arm。
const GOARCH string = theGoarch
