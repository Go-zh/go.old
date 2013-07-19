// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package flag implements command-line flag parsing.

	Usage:

	Define flags using flag.String(), Bool(), Int(), etc.

	This declares an integer flag, -flagname, stored in the pointer ip, with type *int.
		import "flag"
		var ip = flag.Int("flagname", 1234, "help message for flagname")
	If you like, you can bind the flag to a variable using the Var() functions.
		var flagvar int
		func init() {
			flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
		}
	Or you can create custom flags that satisfy the Value interface (with
	pointer receivers) and couple them to flag parsing by
		flag.Var(&flagVal, "name", "help message for flagname")
	For such flags, the default value is just the initial value of the variable.

	After all flags are defined, call
		flag.Parse()
	to parse the command line into the defined flags.

	Flags may then be used directly. If you're using the flags themselves,
	they are all pointers; if you bind to variables, they're values.
		fmt.Println("ip has value ", *ip)
		fmt.Println("flagvar has value ", flagvar)

	After parsing, the arguments after the flag are available as the
	slice flag.Args() or individually as flag.Arg(i).
	The arguments are indexed from 0 through flag.NArg()-1.

	Command line flag syntax:
		-flag
		-flag=x
		-flag x  // non-boolean flags only
	One or two minus signs may be used; they are equivalent.
	The last form is not permitted for boolean flags because the
	meaning of the command
		cmd -x *
	will change if there is a file called 0, false, etc.  You must
	use the -flag=false form to turn off a boolean flag.

	Flag parsing stops just before the first non-flag argument
	("-" is a non-flag argument) or after the terminator "--".

	Integer flags accept 1234, 0664, 0x1234 and may be negative.
	Boolean flags may be 1, 0, t, f, true, false, TRUE, FALSE, True, False.
	Duration flags accept any input valid for time.ParseDuration.

	The default set of command-line flags is controlled by
	top-level functions.  The FlagSet type allows one to define
	independent sets of flags, such as to implement subcommands
	in a command-line interface. The methods of FlagSet are
	analogous to the top-level functions for the command-line
	flag set.
*/

/*
	flag 包实现命令行标签解析.

	使用：

	定义标签需要使用flag.String(),Bool(),Int()等方法。

	下面的代码定义了一个interger标签，标签名是flagname，标签解析的结果存放在ip指针（*int）指向的值中
		import "flag"
		var ip = flag.Int("flagname", 1234, "help message for flagname")
	你还可以选择使用Var()函数将标签绑定到指定变量中
		var flagvar int
		func init() {
			flag.IntVar(&flagvar, "flagname", 1234, "help message for flagname")
		}
	你也可以传入自定义类型的标签，只要标签满足对应的值接口（接收指针指向的接收者）。像下面代码一样定义标签
		flag.Var(&flagVal, "name", "help message for flagname")
	这样的标签，默认值就是自定义类型的初始值。

	所有的标签都定义好了，就可以调用
		flag.Parse()
	来解析命令行参数并传入到定义好的标签了。

	标签可以被用来直接使用。如果你直接使用标签（没有绑定变量），那他们都是指针类型。如果你将他们绑定到变量上，他们就是值类型。
		fmt.Println("ip has value ", *ip)
		fmt.Println("flagvar has value ", flagvar)

	在解析之后，标签对应的参数可以从flag.Args()获取到，它返回的slice，也可以使用flag.Arg(i)来获取单个参数。
	参数列的索引是从0到flag.NArg()-1。

	命令行标签格式：
		-flag
		-flag=x
		-flag x  // 只有非boolean标签能这么用
	减号可以使用一个或者两个，效果是一样的。
	上面最后一种方式不能被boolean类型的标签使用。因为当有个文件的名字是0或者false这样的词的话，下面的命令
		cmd -x *
	的原意会被改变。你必须使用-flag=false的方式来解析boolean标签。

	一个标签的解析会在下次出现第一个非标签参数（“-”就是一个非标签参数）的时候停止，或者是在终止符号“--”的时候停止。

	Interger标签接受如1234，0664，0x1234和负数这样的值。
	Boolean标签接受1，0，t，f，true，false，TRUE，FALSE，True，False。
	Duration标签接受任何可被time.ParseDuration解析的值。

	默认的命令行标签是由最高层的函数来控制的。FlagSet类型允许每个包定义独立的标签集合，例如在命令行接口中实现子命令。
	FlagSet的方法就是模拟使用最高层函数来控制命令行标签集的行为的。
*/
package flag

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"
)

// ErrHelp is the error returned if the flag -help is invoked but no such flag is defined.

// ErrHelp的使用场景是：标签 -help被调用，但没有定义help标签。
var ErrHelp = errors.New("flag: help requested")

// -- bool Value

// -- bool值
type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) Get() interface{} { return bool(*b) }

func (b *boolValue) String() string { return fmt.Sprintf("%v", *b) }

func (b *boolValue) IsBoolFlag() bool { return true }

// optional interface to indicate boolean flags that can be
// supplied without "=value" text
type boolFlag interface {
	Value
	IsBoolFlag() bool
}

// -- int Value

// -- int值
type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (i *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = intValue(v)
	return err
}

func (i *intValue) Get() interface{} { return int(*i) }

func (i *intValue) String() string { return fmt.Sprintf("%v", *i) }

// -- int64 Value

// -- int64值
type int64Value int64

func newInt64Value(val int64, p *int64) *int64Value {
	*p = val
	return (*int64Value)(p)
}

func (i *int64Value) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = int64Value(v)
	return err
}

func (i *int64Value) Get() interface{} { return int64(*i) }

func (i *int64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- uint Value

// -- uint值
type uintValue uint

func newUintValue(val uint, p *uint) *uintValue {
	*p = val
	return (*uintValue)(p)
}

func (i *uintValue) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uintValue(v)
	return err
}

func (i *uintValue) Get() interface{} { return uint(*i) }

func (i *uintValue) String() string { return fmt.Sprintf("%v", *i) }

// -- uint64 Value

// -- uint64值
type uint64Value uint64

func newUint64Value(val uint64, p *uint64) *uint64Value {
	*p = val
	return (*uint64Value)(p)
}

func (i *uint64Value) Set(s string) error {
	v, err := strconv.ParseUint(s, 0, 64)
	*i = uint64Value(v)
	return err
}

func (i *uint64Value) Get() interface{} { return uint64(*i) }

func (i *uint64Value) String() string { return fmt.Sprintf("%v", *i) }

// -- string Value

// -- string值
type stringValue string

func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) Get() interface{} { return string(*s) }

func (s *stringValue) String() string { return fmt.Sprintf("%s", *s) }

// -- float64 Value

// -- float64值
type float64Value float64

func newFloat64Value(val float64, p *float64) *float64Value {
	*p = val
	return (*float64Value)(p)
}

func (f *float64Value) Set(s string) error {
	v, err := strconv.ParseFloat(s, 64)
	*f = float64Value(v)
	return err
}

func (f *float64Value) Get() interface{} { return float64(*f) }

func (f *float64Value) String() string { return fmt.Sprintf("%v", *f) }

// -- time.Duration Value

// -- time.Duration值
type durationValue time.Duration

func newDurationValue(val time.Duration, p *time.Duration) *durationValue {
	*p = val
	return (*durationValue)(p)
}

func (d *durationValue) Set(s string) error {
	v, err := time.ParseDuration(s)
	*d = durationValue(v)
	return err
}

func (d *durationValue) Get() interface{} { return time.Duration(*d) }

func (d *durationValue) String() string { return (*time.Duration)(d).String() }

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
//
// If a Value has an IsBoolFlag() bool method returning true,
// the command-line parser makes -name equivalent to -name=true
// rather than using the next command-line argument.

// Value接口是定义了标签对应的具体的参数值。
// （默认值是string类型）
//
// 若 Value 拥有的 IsBoolFlag() bool 方法返回 ture，则命令行解析器会使 -name
// 等价于 -name=true，而非使用下一个命令行实参。
type Value interface {
	String() string
	Set(string) error
}

// Getter is an interface that allows the contents of a Value to be retrieved.
// It wraps the Value interface, rather than being part of it, because it
// appeared after Go 1 and its compatibility rules. All Value types provided
// by this package satisfy the Getter interface.
type Getter interface {
	Value
	Get() interface{}
}

// ErrorHandling defines how to handle flag parsing errors.

// ErrorHandling定义了如何处理标签解析的错误
type ErrorHandling int

const (
	ContinueOnError ErrorHandling = iota
	ExitOnError
	PanicOnError
)

// A FlagSet represents a set of defined flags.

// FlagSet是已经定义好的标签的集合
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler.

	// 当解析标签出现错误的时候，Usage就会被调用。这个字段是一个函数（不是一个方法），它可以指向
	// 用户自己定义的错误处理函数。
	Usage func()

	name          string
	parsed        bool
	actual        map[string]*Flag
	formal        map[string]*Flag
	args          []string // arguments after flags  // flags后面的参数
	exitOnError   bool     // does the program exit if there's an error?  // 是否当错误发生的时候，程序会退出？
	errorHandling ErrorHandling
	output        io.Writer // nil means stderr; use out() accessor  // nil代表控制台输出，使用out()来访问这个字段
}

// A Flag represents the state of a flag.

// Flag表示标签的状态
type Flag struct {
	Name     string // name as it appears on command line  // 标签在命令行显示的名字
	Usage    string // help message  // 帮助信息
	Value    Value  // value as set  // 标签的值
	DefValue string // default value (as text); for usage message  // 默认值（文本格式）；这也是一个用法的信息说明
}

// sortFlags returns the flags as a slice in lexicographical sorted order.

// sortFlags返回按字典顺序排序的slice类型的标签集合。
func sortFlags(flags map[string]*Flag) []*Flag {
	list := make(sort.StringSlice, len(flags))
	i := 0
	for _, f := range flags {
		list[i] = f.Name
		i++
	}
	list.Sort()
	result := make([]*Flag, len(list))
	for i, name := range list {
		result[i] = flags[name]
	}
	return result
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.

// SetOutput设置了用法和错误信息的输出目的地。
// 如果output是nil，输出目的地就会使用os.Stderr。
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// VisitAll visits the flags in lexicographical order, calling fn for each.
// It visits all flags, even those not set.

// VisitAll按照字典顺序遍历标签，并且对每个标签调用fn。
// 这个函数会遍历所有标签，包括那些没有定义的标签。
func (f *FlagSet) VisitAll(fn func(*Flag)) {
	for _, flag := range sortFlags(f.formal) {
		fn(flag)
	}
}

// VisitAll visits the command-line flags in lexicographical order, calling
// fn for each.  It visits all flags, even those not set.

// VisitAll按照字典顺序遍历控制台标签，并且对每个标签调用fn。
// 这个函数会遍历所有标签，包括那些没有定义的标签。
func VisitAll(fn func(*Flag)) {
	commandLine.VisitAll(fn)
}

// Visit visits the flags in lexicographical order, calling fn for each.
// It visits only those flags that have been set.

// Visit按照字典顺序遍历标签，并且对每个标签调用fn。
// 这个函数只遍历定义过的标签。
func (f *FlagSet) Visit(fn func(*Flag)) {
	for _, flag := range sortFlags(f.actual) {
		fn(flag)
	}
}

// Visit visits the command-line flags in lexicographical order, calling fn
// for each.  It visits only those flags that have been set.

// Visit按照字典顺序遍历命令行标签，并且对每个标签调用fn。
// 这个函数只遍历定义过的标签。
func Visit(fn func(*Flag)) {
	commandLine.Visit(fn)
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.

// Lookup返回已经定义过的标签，如果标签不存在的话，返回nil。
func (f *FlagSet) Lookup(name string) *Flag {
	return f.formal[name]
}

// Lookup returns the Flag structure of the named command-line flag,
// returning nil if none exists.

// Lookup返回命令行已经定义过的标签，如果标签不存在的话，返回nil。
func Lookup(name string) *Flag {
	return commandLine.formal[name]
}

// Set sets the value of the named flag.

// Set设置定义过的标签的值
func (f *FlagSet) Set(name, value string) error {
	flag, ok := f.formal[name]
	if !ok {
		return fmt.Errorf("no such flag -%v", name)
	}
	err := flag.Value.Set(value)
	if err != nil {
		return err
	}
	if f.actual == nil {
		f.actual = make(map[string]*Flag)
	}
	f.actual[name] = flag
	return nil
}

// Set sets the value of the named command-line flag.

// Set设置命令行中已经定义过的标签的值。
func Set(name, value string) error {
	return commandLine.Set(name, value)
}

// PrintDefaults prints, to standard error unless configured
// otherwise, the default values of all defined flags in the set.

// 除非有特别配置，否则PrintDefault会将内容输出到标准输出控制台中。
// PrintDefault会输出集合中所有定义好的标签的默认信息
func (f *FlagSet) PrintDefaults() {
	f.VisitAll(func(flag *Flag) {
		format := "  -%s=%s: %s\n"
		if _, ok := flag.Value.(*stringValue); ok {
			// put quotes on the value
			format = "  -%s=%q: %s\n"
		}
		fmt.Fprintf(f.out(), format, flag.Name, flag.DefValue, flag.Usage)
	})
}

// PrintDefaults prints to standard error the default values of all defined command-line flags.

// PrintDefaults打印出标准错误，就是所有命令行中定义好的标签的默认信息。
func PrintDefaults() {
	commandLine.PrintDefaults()
}

// defaultUsage is the default function to print a usage message.

// defaultUsage是打印出用法的默认方法。
func defaultUsage(f *FlagSet) {
	fmt.Fprintf(f.out(), "Usage of %s:\n", f.name)
	f.PrintDefaults()
}

// NOTE: Usage is not just defaultUsage(commandLine)
// because it serves (via godoc flag Usage) as the example
// for how to write your own usage function.

// 注意：Usage并不是只能使用自带的defaultUsage（或者是命令行版本的defaultUsage）
// 你可以看例子（godoc的flag使用）了解如何写你自己的usage函数。

// Usage prints to standard error a usage message documenting all defined command-line flags.
// The function is a variable that may be changed to point to a custom function.

// Usage打印出标准的错误信息，包含所有定义过的命令行标签说明。
// 这个函数赋值到一个变量上去，当然也可以将这个变量指向到自定义的函数。
var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	PrintDefaults()
}

// NFlag returns the number of flags that have been set.

// NFlag返回解析过的标签的数量。
func (f *FlagSet) NFlag() int { return len(f.actual) }

// NFlag returns the number of command-line flags that have been set.

// NFlag返回解析过的命令行标签的数量。
func NFlag() int { return len(commandLine.actual) }

// Arg returns the i'th argument.  Arg(0) is the first remaining argument
// after flags have been processed.

// Arg返回第i个参数。当有标签被解析之后，Arg(0)就成为了保留参数。
func (f *FlagSet) Arg(i int) string {
	if i < 0 || i >= len(f.args) {
		return ""
	}
	return f.args[i]
}

// Arg returns the i'th command-line argument.  Arg(0) is the first remaining argument
// after flags have been processed.

// Arg返回第i个命令行参数。当有标签被解析之后，Arg(0)就成为了保留参数。
func Arg(i int) string {
	return commandLine.Arg(i)
}

// NArg is the number of arguments remaining after flags have been processed.

// 在标签被解析之后，NArg就返回解析后参数的个数。
func (f *FlagSet) NArg() int { return len(f.args) }

// NArg is the number of arguments remaining after flags have been processed.

// 在命令行标签被解析之后，NArg就返回解析后参数的个数。
func NArg() int { return len(commandLine.args) }

// Args returns the non-flag arguments.

// Args返回非标签的参数。
func (f *FlagSet) Args() []string { return f.args }

// Args returns the non-flag command-line arguments.

// Args返回非标签的命令行参数。
func Args() []string { return commandLine.args }

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.

// BoolVar定义了一个有指定名字，默认值，和用法说明的标签。
// 参数p指向一个存储标签值的bool变量。
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) {
	f.Var(newBoolValue(value, p), name, usage)
}

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.

// BoolVar定义了一个有指定名字，默认值，和用法说明的bool标签。
// 参数p指向一个存储标签解析值的bool变量。
func BoolVar(p *bool, name string, value bool, usage string) {
	commandLine.Var(newBoolValue(value, p), name, usage)
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.

// Bool定义了一个有指定名字，默认值，和用法说明的bool标签。
// 返回值是一个存储标签解析值的bool变量地址。
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVar(p, name, value, usage)
	return p
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.

// Bool定义了一个有指定名字，默认值，和用法说明的bool标签。
// 返回值是一个存储标签解析值的bool变量地址。
func Bool(name string, value bool, usage string) *bool {
	return commandLine.Bool(name, value, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.

// IntVar定义了一个有指定名字，默认值，和用法说明的int标签。
// 参数p指向一个存储标签解析值的int变量。
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.Var(newIntValue(value, p), name, usage)
}

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.

// IntVar定义了一个有指定名字，默认值，和用法说明的int标签。
// 参数p指向一个存储标签解析值的int变量。
func IntVar(p *int, name string, value int, usage string) {
	commandLine.Var(newIntValue(value, p), name, usage)
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.

// Int定义了一个有指定名字，默认值，和用法说明的int标签。
// 返回值是一个存储标签解析值的int变量地址。
func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVar(p, name, value, usage)
	return p
}

// Int defines an int flag with specified name, default value, and usage string.
// The return value is the address of an int variable that stores the value of the flag.

// Int定义了一个有指定名字，默认值，和用法说明的int标签。
// 返回值是一个存储标签解析值的int变量地址。
func Int(name string, value int, usage string) *int {
	return commandLine.Int(name, value, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.

// Int64Var定义了一个有指定名字，默认值，和用法说明的int64标签。
// 参数p指向一个存储标签解析值的int64变量。
func (f *FlagSet) Int64Var(p *int64, name string, value int64, usage string) {
	f.Var(newInt64Value(value, p), name, usage)
}

// Int64Var defines an int64 flag with specified name, default value, and usage string.
// The argument p points to an int64 variable in which to store the value of the flag.

// Int64Var定义了一个有指定名字，默认值，和用法说明的int64标签。
// 参数p指向一个存储标签解析值的int64变量。
func Int64Var(p *int64, name string, value int64, usage string) {
	commandLine.Var(newInt64Value(value, p), name, usage)
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.

// Int64定义了一个有指定名字，默认值，和用法说明的int64标签。
// 返回值是一个存储标签解析值的int64变量地址。
func (f *FlagSet) Int64(name string, value int64, usage string) *int64 {
	p := new(int64)
	f.Int64Var(p, name, value, usage)
	return p
}

// Int64 defines an int64 flag with specified name, default value, and usage string.
// The return value is the address of an int64 variable that stores the value of the flag.

// Int64定义了一个有指定名字，默认值，和用法说明的int64标签。
// 返回值是一个存储标签解析值的int64变量地址。
func Int64(name string, value int64, usage string) *int64 {
	return commandLine.Int64(name, value, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint variable in which to store the value of the flag.

// UintVar定义了一个有指定名字，默认值，和用法说明的uint标签。
// 参数p指向一个存储标签解析值的uint变量。
func (f *FlagSet) UintVar(p *uint, name string, value uint, usage string) {
	f.Var(newUintValue(value, p), name, usage)
}

// UintVar defines a uint flag with specified name, default value, and usage string.
// The argument p points to a uint  variable in which to store the value of the flag.

// UintVar定义了一个有指定名字，默认值，和用法说明的uint标签。
// 参数p指向一个存储标签解析值的uint变量。
func UintVar(p *uint, name string, value uint, usage string) {
	commandLine.Var(newUintValue(value, p), name, usage)
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.

// Uint定义了一个有指定名字，默认值，和用法说明的uint标签。
// 返回值是一个存储标签解析值的uint变量地址。
func (f *FlagSet) Uint(name string, value uint, usage string) *uint {
	p := new(uint)
	f.UintVar(p, name, value, usage)
	return p
}

// Uint defines a uint flag with specified name, default value, and usage string.
// The return value is the address of a uint  variable that stores the value of the flag.

// Uint定义了一个有指定名字，默认值，和用法说明的uint标签。
// 返回值是一个存储标签解析值的uint变量地址。
func Uint(name string, value uint, usage string) *uint {
	return commandLine.Uint(name, value, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.

// Uint64Var定义了一个有指定名字，默认值，和用法说明的uint64标签。
// 参数p指向一个存储标签解析值的uint64变量。
func (f *FlagSet) Uint64Var(p *uint64, name string, value uint64, usage string) {
	f.Var(newUint64Value(value, p), name, usage)
}

// Uint64Var defines a uint64 flag with specified name, default value, and usage string.
// The argument p points to a uint64 variable in which to store the value of the flag.

// Uint64Var定义了一个有指定名字，默认值，和用法说明的uint64标签。
// 参数p指向一个存储标签解析值的uint64变量。
func Uint64Var(p *uint64, name string, value uint64, usage string) {
	commandLine.Var(newUint64Value(value, p), name, usage)
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.

// Uint64定义了一个有指定名字，默认值，和用法说明的uint64标签。
// 返回值是一个存储标签解析值的uint64变量地址。
func (f *FlagSet) Uint64(name string, value uint64, usage string) *uint64 {
	p := new(uint64)
	f.Uint64Var(p, name, value, usage)
	return p
}

// Uint64 defines a uint64 flag with specified name, default value, and usage string.
// The return value is the address of a uint64 variable that stores the value of the flag.

// Uint64定义了一个有指定名字，默认值，和用法说明的uint64标签。
// 返回值是一个存储标签解析值的uint64变量地址。
func Uint64(name string, value uint64, usage string) *uint64 {
	return commandLine.Uint64(name, value, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.

// StringVar定义了一个有指定名字，默认值，和用法说明的string标签。
// 参数p指向一个存储标签解析值的string变量。
func (f *FlagSet) StringVar(p *string, name string, value string, usage string) {
	f.Var(newStringValue(value, p), name, usage)
}

// StringVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a string variable in which to store the value of the flag.

// StringVar定义了一个有指定名字，默认值，和用法说明的string标签。
// 参数p指向一个存储标签解析值的string变量。
func StringVar(p *string, name string, value string, usage string) {
	commandLine.Var(newStringValue(value, p), name, usage)
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.

// String定义了一个有指定名字，默认值，和用法说明的string标签。
// 返回值是一个存储标签解析值的string变量地址。
func (f *FlagSet) String(name string, value string, usage string) *string {
	p := new(string)
	f.StringVar(p, name, value, usage)
	return p
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.

// String定义了一个有指定名字，默认值，和用法说明的string标签。
// 返回值是一个存储标签解析值的string变量地址。
func String(name string, value string, usage string) *string {
	return commandLine.String(name, value, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.

// Float64Var定义了一个有指定名字，默认值，和用法说明的float64标签。
// 参数p指向一个存储标签解析值的float64变量。
func (f *FlagSet) Float64Var(p *float64, name string, value float64, usage string) {
	f.Var(newFloat64Value(value, p), name, usage)
}

// Float64Var defines a float64 flag with specified name, default value, and usage string.
// The argument p points to a float64 variable in which to store the value of the flag.

// Float64Var定义了一个有指定名字，默认值，和用法说明的float64标签。
// 参数p指向一个存储标签解析值的float64变量。
func Float64Var(p *float64, name string, value float64, usage string) {
	commandLine.Var(newFloat64Value(value, p), name, usage)
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.

// Float64定义了一个有指定名字，默认值，和用法说明的float64标签。
// 返回值是一个存储标签解析值的float64变量地址。
func (f *FlagSet) Float64(name string, value float64, usage string) *float64 {
	p := new(float64)
	f.Float64Var(p, name, value, usage)
	return p
}

// Float64 defines a float64 flag with specified name, default value, and usage string.
// The return value is the address of a float64 variable that stores the value of the flag.

// Float64定义了一个有指定名字，默认值，和用法说明的float64标签。
// 返回值是一个存储标签解析值的float64变量地址。
func Float64(name string, value float64, usage string) *float64 {
	return commandLine.Float64(name, value, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.

// DurationVar定义了一个有指定名字，默认值，和用法说明的time.Duration标签。
// 参数p指向一个存储标签解析值的time.Duration变量。
func (f *FlagSet) DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	f.Var(newDurationValue(value, p), name, usage)
}

// DurationVar defines a time.Duration flag with specified name, default value, and usage string.
// The argument p points to a time.Duration variable in which to store the value of the flag.

// DurationVar定义了一个有指定名字，默认值，和用法说明的time.Duration标签。
// 参数p指向一个存储标签解析值的time.Duration变量。
func DurationVar(p *time.Duration, name string, value time.Duration, usage string) {
	commandLine.Var(newDurationValue(value, p), name, usage)
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.

// Duration定义了一个有指定名字，默认值，和用法说明的time.Duration标签。
// 返回值是一个存储标签解析值的time.Duration变量地址。
func (f *FlagSet) Duration(name string, value time.Duration, usage string) *time.Duration {
	p := new(time.Duration)
	f.DurationVar(p, name, value, usage)
	return p
}

// Duration defines a time.Duration flag with specified name, default value, and usage string.
// The return value is the address of a time.Duration variable that stores the value of the flag.

// Duration定义了一个有指定名字，默认值，和用法说明的time.Duration标签。
// 返回值是一个存储标签解析值的time.Duration变量地址。
func Duration(name string, value time.Duration, usage string) *time.Duration {
	return commandLine.Duration(name, value, usage)
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.

// Var定义了一个有指定名字和用法说明的标签。标签的类型和值是由第一个参数指定的，这个参数
// 是Value类型，并且是用户自定义的实现了Value接口的类型。举个例子，调用者可以定义一种标签，这种标签会把
// 逗号分隔的字符串变成字符串slice，并提供出这种转换的方法。这样，Set（FlagSet）就会将逗号分隔
// 的字符串转换成为slice。
func (f *FlagSet) Var(value Value, name string, usage string) {
	// Remember the default value as a string; it won't change.
	flag := &Flag{name, usage, value, value.String()}
	_, alreadythere := f.formal[name]
	if alreadythere {
		msg := fmt.Sprintf("%s flag redefined: %s", f.name, name)
		fmt.Fprintln(f.out(), msg)
		panic(msg) // Happens only if flags are declared with identical names
	}
	if f.formal == nil {
		f.formal = make(map[string]*Flag)
	}
	f.formal[name] = flag
}

// Var defines a flag with the specified name and usage string. The type and
// value of the flag are represented by the first argument, of type Value, which
// typically holds a user-defined implementation of Value. For instance, the
// caller could create a flag that turns a comma-separated string into a slice
// of strings by giving the slice the methods of Value; in particular, Set would
// decompose the comma-separated string into the slice.

// Var定义了一个有指定名字和用法说明的标签。标签的类型和值是由第一个参数指定的，这个参数
// 是Value类型，并且是用户自定义的实现了Value接口的类型。举个例子，调用者可以定义一种标签，这种标签会把
// 逗号分隔的字符串变成字符串slice，并提供出这种转换的方法。这样，Set（FlagSet）就会将逗号分隔
// 的字符串转换成为slice。
func Var(value Value, name string, usage string) {
	commandLine.Var(value, name, usage)
}

// failf prints to standard error a formatted error and usage message and
// returns the error.

// failf输出错误信息，包含格式错误和用法，并且返回error
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(f.out(), err)
	f.usage()
	return err
}

// usage calls the Usage method for the flag set, or the usage function if
// the flag set is commandLine.

// 如果有FlagSet的话，usage调用FlagSet的Usage方法。否则，当flag set是命令行的话，调用命令行的usage函数。
func (f *FlagSet) usage() {
	if f == commandLine {
		Usage()
	} else if f.Usage == nil {
		defaultUsage(f)
	} else {
		f.Usage()
	}
}

// parseOne parses one flag. It returns whether a flag was seen.

// parseOne解析一个标签，它返回是否这个标签能解析。
func (f *FlagSet) parseOne() (bool, error) {
	if len(f.args) == 0 {
		return false, nil
	}
	s := f.args[0]
	if len(s) == 0 || s[0] != '-' || len(s) == 1 {
		return false, nil
	}
	num_minuses := 1
	if s[1] == '-' {
		num_minuses++
		if len(s) == 2 { // "--" terminates the flags
			f.args = f.args[1:]
			return false, nil
		}
	}
	name := s[num_minuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, f.failf("bad flag syntax: %s", s)
	}

	// it's a flag. does it have an argument?
	f.args = f.args[1:]
	has_value := false
	value := ""
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			value = name[i+1:]
			has_value = true
			name = name[0:i]
			break
		}
	}
	m := f.formal
	flag, alreadythere := m[name] // BUG
	if !alreadythere {
		if name == "help" || name == "h" { // special case for nice help message.
			f.usage()
			return false, ErrHelp
		}
		return false, f.failf("flag provided but not defined: -%s", name)
	}
	if fv, ok := flag.Value.(boolFlag); ok && fv.IsBoolFlag() { // special case: doesn't need an arg
		if has_value {
			if err := fv.Set(value); err != nil {
				return false, f.failf("invalid boolean value %q for  -%s: %v", value, name, err)
			}
		} else {
			fv.Set("true")
		}
	} else {
		// It must have a value, which might be the next argument.
		if !has_value && len(f.args) > 0 {
			// value is the next arg
			has_value = true
			value, f.args = f.args[0], f.args[1:]
		}
		if !has_value {
			return false, f.failf("flag needs an argument: -%s", name)
		}
		if err := flag.Value.Set(value); err != nil {
			return false, f.failf("invalid value %q for flag -%s: %v", value, name, err)
		}
	}
	if f.actual == nil {
		f.actual = make(map[string]*Flag)
	}
	f.actual[name] = flag
	return true, nil
}

// Parse parses flag definitions from the argument list, which should not
// include the command name.  Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help was set but not defined.

// Parse从参数列表中解析定义的标签，这个参数列表并不包含执行的命令名字。
// 这个方法调用时间点必须在FlagSet的所有标签都定义之后，程序访问这些标签之前。
// 当-help标签没有定义却被调用了的时候，这个方法返回ErrHelp。
func (f *FlagSet) Parse(arguments []string) error {
	f.parsed = true
	f.args = arguments
	for {
		seen, err := f.parseOne()
		if seen {
			continue
		}
		if err == nil {
			break
		}
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

// Parsed reports whether f.Parse has been called.

// Parsed返回是否f.Parse已经被调用过。
func (f *FlagSet) Parsed() bool {
	return f.parsed
}

// Parse parses the command-line flags from os.Args[1:].  Must be called
// after all flags are defined and before flags are accessed by the program.

// Parse从参数os.Args[1:]中解析命令行标签。
// 这个方法调用时间点必须在FlagSet的所有标签都定义之后，程序访问这些标签之前。
func Parse() {
	// Ignore errors; commandLine is set for ExitOnError.
	commandLine.Parse(os.Args[1:])
}

// Parsed returns true if the command-line flags have been parsed.

// Parsed 返回是否命令行标签已经被解析过。
func Parsed() bool {
	return commandLine.Parsed()
}

// The default set of command-line flags, parsed from os.Args.

// 默认的命令行标签集合，被os.Args解析。
var commandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name and
// error handling property.

// NewFlagSet通过设置一个特定的名字和错误处理属性，返回一个新的，空的FlagSet。
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{
		name:          name,
		errorHandling: errorHandling,
	}
	return f
}

// Init sets the name and error handling property for a flag set.
// By default, the zero FlagSet uses an empty name and the
// ContinueOnError error handling policy.

// Init设置名字和错误处理标签集合的属性。
// 空标签集合默认使用一个空名字和ContinueOnError的错误处理属性。
func (f *FlagSet) Init(name string, errorHandling ErrorHandling) {
	f.name = name
	f.errorHandling = errorHandling
}
