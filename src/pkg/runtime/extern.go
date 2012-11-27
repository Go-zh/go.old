// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package runtime contains operations that interact with Go's runtime system,
	such as functions to control goroutines. It also includes the low-level type information
	used by the reflect package; see reflect's documentation for the programmable
	interface to the run-time type system.
*/

/*
	runtime 包含有和Go的运行时系统进行交互的操作，例如用于控制Go程的函数.
	它也包括用于 reflect 包的底层类型信息；运行时类型系统的可编程接口见 reflect 文档。
*/
package runtime

// Gosched yields the processor, allowing other goroutines to run.  It does not
// suspend the current goroutine, so execution resumes automatically.

// Gosched 使当前Go程放弃处理器以让其它Go程运行。
// 它不会挂起当前Go程，因而它会自动继续执行。
func Gosched()

// Goexit terminates the goroutine that calls it.  No other goroutine is affected.
// Goexit runs all deferred calls before terminating the goroutine.

// Goexit 终止调用它的Go程。
// 其它Go程则不受影响。Goexit 会在终止该Go程前调用所有已推迟的调用。
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

type Func struct { // Keep in sync with runtime.h:struct Func // 与 runtime.h:struct Func 保持同步
	name   string
	typ    string  // go type string            // go 的类型字符串
	src    string  // src file name             // src 文件名
	pcln   []byte  // pc/ln tab for this func   // 此函数的 pc/ln 表
	entry  uintptr // entry pc                  // pc 的条目 entry
	pc0    uintptr // starting pc, ln for table // 起始于 pc，ln 为表
	ln0    int32
	frame  int32 // stack frame size            // 栈帧 frame 的大小
	args   int32 // number of 32-bit in/out args// 32位 in/out 实参 args 的数量
	locals int32 // number of 32-bit locals     // 32位 局部变量 locals 的数量
}

// FuncForPC returns a *Func describing the function that contains the
// given program counter address, or else nil.

// FuncForPC 返回一个 *Func，它描述了包含给定程序计数器地址的函数，否则返回 nil。
func FuncForPC(pc uintptr) *Func

// Name returns the name of the function.

// Name 返回该函数的名称
func (f *Func) Name() string { return f.name }

// Entry returns the entry address of the function.

// Entry 返回该项函数的地址。
func (f *Func) Entry() uintptr { return f.entry }

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

// mid returns the current os thread (m) id.

// mid 返回当前OS线程的(m)id。
func mid() uint32

// SetFinalizer sets the finalizer associated with x to f.
// When the garbage collector finds an unreachable block
// with an associated finalizer, it clears the association and runs
// f(x) in a separate goroutine.  This makes x reachable again, but
// now without an associated finalizer.  Assuming that SetFinalizer
// is not called again, the next time the garbage collector sees
// that x is unreachable, it will free x.
//
// SetFinalizer(x, nil) clears any finalizer associated with x.
//
// The argument x must be a pointer to an object allocated by
// calling new or by taking the address of a composite literal.
// The argument f must be a function that takes a single argument
// of x's type and can have arbitrary ignored return values.
// If either of these is not true, SetFinalizer aborts the program.
//
// Finalizers are run in dependency order: if A points at B, both have
// finalizers, and they are otherwise unreachable, only the finalizer
// for A runs; once A is freed, the finalizer for B can run.
// If a cyclic structure includes a block with a finalizer, that
// cycle is not guaranteed to be garbage collected and the finalizer
// is not guaranteed to run, because there is no ordering that
// respects the dependencies.
//
// The finalizer for x is scheduled to run at some arbitrary time after
// x becomes unreachable.
// There is no guarantee that finalizers will run before a program exits,
// so typically they are useful only for releasing non-memory resources
// associated with an object during a long-running program.
// For example, an os.File object could use a finalizer to close the
// associated operating system file descriptor when a program discards
// an os.File without calling Close, but it would be a mistake
// to depend on a finalizer to flush an in-memory I/O buffer such as a
// bufio.Writer, because the buffer would not be flushed at program exit.
//
// A single goroutine runs all finalizers for a program, sequentially.
// If a finalizer must run for a long time, it should do so by starting
// a new goroutine.

// SetFinalizer 为 f 设置与 x 相关联的终结器。
// 当垃圾收集器找到一个无法访问的块及与其相关联的终结器时，就会清理该关联，
// 并在一个独立的Go程中运行f(x)。这会使 x 再次变得可访问，但现在没有了相关联的终结器。
// 假设 SetFinalizer 未被再次调用，当下一次垃圾收集器发现 x 无法访问时，就会释放 x。
//
// SetFinalizer(x, nil) 会清理任何与 x 相关联的终结器。
//
// 实参 x 必须是一个对象的指针，该对象通过调用新的或获取一个复合字面地址来分配。
// 实参 f 必须是一个函数，该函数获取一个 x 的类型的单一实参，并拥有可任意忽略的返回值。
// 只要这些条件有一个不满足，SetFinalizer 就会跳过该程序。
//
// 终结器按照依赖顺序运行：若 A 指向 B，则二者都有终结器，当只有 A 的终结器运行时，
// 它们才无法访问；一旦 A 被释放，则 B 的终结器便可运行。若循环依赖的结构包含块及其终结器，
// 则该循环并不能保证被垃圾收集，而其终结器并不能保证运行，这是因为其依赖没有顺序。
//
// x 的终结器预定为在 x 无法访问后的任意时刻运行。无法保证终结器会在程序退出前运行，
// 因此它们通常只在长时间运行的程序中释放一个关联至对象的非内存资源时使用。
// 例如，当程序丢弃 os.File 而没有调用 Close 时，该 os.File 对象便可使用一个终结器
// 来关闭与其相关联的操作系统文件描述符，但依赖终结器去刷新一个内存中的I/O缓存是错误的，
// 因为该缓存不会在程序退出时被刷新。
//
// 一个程序的单个Go程会按顺序运行所有的终结器。若某个终结器需要长时间运行，
// 它应当通过开始一个新的Go程来继续。
// TODO: 仍需校对及语句优化
func SetFinalizer(x, f interface{})

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
// It is either a sequence number or, when possible,
// a release tag like "release.2010-03-04".
// A trailing + indicates that the tree had local modifications
// at the time of the build.

// Version 返回Go目录树的版本字符串。
// 它一般是一个序列数字，也可能是一个类似于 "release.2010-03-04" 的发行标注。
// 随后的 + 号表示该源码树在构建时进行了本地的修改。
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
