// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

import (
	"errors"
	"io"
	"os"
	"reflect"
	"sync"
	"unicode/utf8"
)

// Some constants in the form of bytes, to avoid string overhead.
// Needlessly fastidious, I suppose.

// 将一些内容以字节的形式存储，以此避免字符串开销。我觉得无可挑剔了。
var (
	commaSpaceBytes  = []byte(", ")
	nilAngleBytes    = []byte("<nil>")
	nilParenBytes    = []byte("(nil)")
	nilBytes         = []byte("nil")
	mapBytes         = []byte("map[")
	percentBangBytes = []byte("%!")
	missingBytes     = []byte("(MISSING)")
	badIndexBytes    = []byte("(BADINDEX)")
	panicBytes       = []byte("(PANIC=")
	extraBytes       = []byte("%!(EXTRA ")
	irparenBytes     = []byte("i)")
	bytesBytes       = []byte("[]byte{")
	badWidthBytes    = []byte("%!(BADWIDTH)")
	badPrecBytes     = []byte("%!(BADPREC)")
	noVerbBytes      = []byte("%!(NOVERB)")
)

// State represents the printer state passed to custom formatters.
// It provides access to the io.Writer interface plus information about
// the flags and options for the operand's format specifier.

// State 表示传递给格式化器的打印器的状态。
// 它提供了访问 io.Writer 接口及关于标记的信息，以及操作数的格式说明符选项。
type State interface {
	// Write is the function to call to emit formatted output to be printed.

	// Write 函数用于打印出已格式化的输出。
	Write(b []byte) (ret int, err error)
	// Width returns the value of the width option and whether it has been set.

	// Width 返回宽度选项的值以及它是否已被设置。
	Width() (wid int, ok bool)
	// Precision returns the value of the precision option and whether it has been set.

	// Precision 返回精度选项的值以及它是否已被设置。
	Precision() (prec int, ok bool)

	// Flag reports whether the flag c, a character, has been set.

	// Flag 返回标记 c（一个字符）是否已被设置。
	Flag(c int) bool
}

// Formatter is the interface implemented by values with a custom formatter.
// The implementation of Format may call Sprint(f) or Fprint(f) etc.
// to generate its output.

// Formatter 接口由带有定制的格式化器的值所实现。
// Format 的实现可调用 Sprintf 或 Fprintf(f) 等函数来生成其输出。
type Formatter interface {
	Format(f State, c rune)
}

// Stringer is implemented by any value that has a String method,
// which defines the ``native'' format for that value.
// The String method is used to print values passed as an operand
// to any format that accepts a string or to an unformatted printer
// such as Print.

// Stringer 接口由任何拥有 String 方法的值所实现，该方法定义了该值的“原生”格式。
// String 方法用于打印值，该值可作为操作数传至任何接受字符串的格式，或像
// Print 这样的未格式化打印器。
type Stringer interface {
	String() string
}

// GoStringer is implemented by any value that has a GoString method,
// which defines the Go syntax for that value.
// The GoString method is used to print values passed as an operand
// to a %#v format.

// GoStringer 接口由任何拥有 GoString 方法的值所实现，该方法定义了该值的Go语法格式。
// GoString 方法用于打印作为操作数传至 %#v 进行格式化的值。
type GoStringer interface {
	GoString() string
}

// Use simple []byte instead of bytes.Buffer to avoid large dependency.

// 使用 []byte 而非 bytes.Buffer 以避免大量的依赖。
type buffer []byte

func (b *buffer) Write(p []byte) (n int, err error) {
	*b = append(*b, p...)
	return len(p), nil
}

func (b *buffer) WriteString(s string) (n int, err error) {
	*b = append(*b, s...)
	return len(s), nil
}

func (b *buffer) WriteByte(c byte) error {
	*b = append(*b, c)
	return nil
}

func (bp *buffer) WriteRune(r rune) error {
	if r < utf8.RuneSelf {
		*bp = append(*bp, byte(r))
		return nil
	}

	b := *bp
	n := len(b)
	for n+utf8.UTFMax > cap(b) {
		b = append(b, 0)
	}
	w := utf8.EncodeRune(b[n:n+utf8.UTFMax], r)
	*bp = b[:n+w]
	return nil
}

type pp struct {
	n         int
	panicking bool
	erroring  bool // printing an error condition // 打印错误条件
	buf       buffer
	// arg holds the current item, as an interface{}.
	// arg 将当前条目作为 interface{} 类型的值保存。
	arg interface{}
	// value holds the current item, as a reflect.Value, and will be
	// the zero Value if the item has not been reflected.
	// value 将当前条目作为 reflect.Value 类型的值保存；若该条目未被反射，则为
	// Value 类型的零值。
	value reflect.Value
	// reordered records whether the format string used argument reordering.
	// reordered 记录该格式字符串是否用实参来重新排序。
	reordered bool
	// goodArgNum records whether the most recent reordering directive was valid.
	// goodArgNum 记录最近重新排序的指令是否有效。
	goodArgNum bool
	runeBuf    [utf8.UTFMax]byte
	fmt        fmt
}

var ppFree = sync.Pool {
	New: func() interface{} { return new(pp) },
}

// newPrinter allocates a new pp struct or grab a cached one.

// newPrinter 分配一个新的，或抓取一个已缓存的 pp 结构体。
func newPrinter() *pp {
	p := ppFree.Get().(*pp)
	p.panicking = false
	p.erroring = false
	p.fmt.init(&p.buf)
	return p
}

// free saves used pp structs in ppFree; avoids an allocation per invocation.

// free 将已使用的 pp 结构体保存到 ppFree 中，以此避免为每个请求都分配。
func (p *pp) free() {
	// Don't hold on to pp structs with large buffers.
	// 不保存拥有大缓存的 pp 结构体。
	if cap(p.buf) > 1024 {
		return
	}
	p.buf = p.buf[:0]
	p.arg = nil
	p.value = reflect.Value{}
	ppFree.Put(p)
}

func (p *pp) Width() (wid int, ok bool) { return p.fmt.wid, p.fmt.widPresent }

func (p *pp) Precision() (prec int, ok bool) { return p.fmt.prec, p.fmt.precPresent }

func (p *pp) Flag(b int) bool {
	switch b {
	case '-':
		return p.fmt.minus
	case '+':
		return p.fmt.plus
	case '#':
		return p.fmt.sharp
	case ' ':
		return p.fmt.space
	case '0':
		return p.fmt.zero
	}
	return false
}

func (p *pp) add(c rune) {
	p.buf.WriteRune(c)
}

// Implement Write so we can call Fprintf on a pp (through State), for
// recursive use in custom verbs.

// Write 实现后，我们就可以在 pp 上（通过 State）调用 Fprintf，递归地使用定制的占位符了。
func (p *pp) Write(b []byte) (ret int, err error) {
	return p.buf.Write(b)
}

// These routines end in 'f' and take a format string.
// 这些以“f”结尾的程序接受格式字符串。

// Fprintf formats according to a format specifier and writes to w.
// It returns the number of bytes written and any write error encountered.

// Fprintf 根据于格式说明符进行格式化并写入到 w。
// 它返回写入的字节数以及任何遇到的写入错误。
func Fprintf(w io.Writer, format string, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrintf(format, a)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Printf formats according to a format specifier and writes to standard output.
// It returns the number of bytes written and any write error encountered.

// Printf 根据于格式说明符进行格式化并写入到标准输出。
// 它返回写入的字节数以及任何遇到的写入错误。
func Printf(format string, a ...interface{}) (n int, err error) {
	return Fprintf(os.Stdout, format, a...)
}

// Sprintf formats according to a format specifier and returns the resulting string.

// Fprintf 根据于格式说明符进行格式化并返回其结果字符串。
func Sprintf(format string, a ...interface{}) string {
	p := newPrinter()
	p.doPrintf(format, a)
	s := string(p.buf)
	p.free()
	return s
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.

// Errorf 根据于格式说明符进行格式化并将字符串作为满足 error 的值返回。
func Errorf(format string, a ...interface{}) error {
	return errors.New(Sprintf(format, a...))
}

// These routines do not take a format string
// 这些程序不接受格式字符串

// Fprint formats using the default formats for its operands and writes to w.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.

// Fprint 使用其操作数的默认格式进行格式化并写入到 w。
// 当两个连续的操作数均不为字符串时，它们之间就会添加空格。
// 它返回写入的字节数以及任何遇到的错误。
func Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrint(a, false, false)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Print formats using the default formats for its operands and writes to standard output.
// Spaces are added between operands when neither is a string.
// It returns the number of bytes written and any write error encountered.

// Print 使用其操作数的默认格式进行格式化并写入到标准输出。
// 当两个连续的操作数均不为字符串时，它们之间就会添加空格。
// 它返回写入的字节数以及任何遇到的错误。
func Print(a ...interface{}) (n int, err error) {
	return Fprint(os.Stdout, a...)
}

// Sprint formats using the default formats for its operands and returns the resulting string.
// Spaces are added between operands when neither is a string.

// Sprint 使用其操作数的默认格式进行格式化并返回其结果字符串。
// 当两个连续的操作数均不为字符串时，它们之间就会添加空格。
func Sprint(a ...interface{}) string {
	p := newPrinter()
	p.doPrint(a, false, false)
	s := string(p.buf)
	p.free()
	return s
}

// These routines end in 'ln', do not take a format string,
// always add spaces between operands, and add a newline
// after the last operand.
// 这些程序以“ln”结尾，它们不接受格式字符串，总是在操作数之间添加空格，
// 且总在最后一个操作数之后添加一个换行符。

// Fprintln formats using the default formats for its operands and writes to w.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.

// Fprintln 使用其操作数的默认格式进行格式化并写入到 w。
// 其操作数之间总是添加空格，且总在最后追加一个换行符。
// 它返回写入的字节数以及任何遇到的错误。
func Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	p := newPrinter()
	p.doPrint(a, true, true)
	n, err = w.Write(p.buf)
	p.free()
	return
}

// Println formats using the default formats for its operands and writes to standard output.
// Spaces are always added between operands and a newline is appended.
// It returns the number of bytes written and any write error encountered.

// Fprintln 使用其操作数的默认格式进行格式化并写入到标准输出。
// 其操作数之间总是添加空格，且总在最后追加一个换行符。
// 它返回写入的字节数以及任何遇到的错误。
func Println(a ...interface{}) (n int, err error) {
	return Fprintln(os.Stdout, a...)
}

// Sprintln formats using the default formats for its operands and returns the resulting string.
// Spaces are always added between operands and a newline is appended.

// Fprintln 使用其操作数的默认格式进行格式化并写返回其结果字符串。
// 其操作数之间总是添加空格，且总在最后追加一个换行符。
func Sprintln(a ...interface{}) string {
	p := newPrinter()
	p.doPrint(a, true, true)
	s := string(p.buf)
	p.free()
	return s
}

// getField gets the i'th field of the struct value.
// If the field is itself is an interface, return a value for
// the thing inside the interface, not the interface itself.

// 获取结构值的第 i 个实参。
// 若实参本身为接口，则返回该接口中的值，而非该接口本身。
func getField(v reflect.Value, i int) reflect.Value {
	val := v.Field(i)
	if val.Kind() == reflect.Interface && !val.IsNil() {
		val = val.Elem()
	}
	return val
}

// parsenum converts ASCII to integer.  num is 0 (and isnum is false) if no number present.

// parsenum 将 ASCII 转换为整数。若不存在数字，则 num 为 0（且isnum 为false）。
func parsenum(s string, start, end int) (num int, isnum bool, newi int) {
	if start >= end {
		return 0, false, end
	}
	for newi = start; newi < end && '0' <= s[newi] && s[newi] <= '9'; newi++ {
		num = num*10 + int(s[newi]-'0')
		isnum = true
	}
	return
}

func (p *pp) unknownType(v interface{}) {
	if v == nil {
		p.buf.Write(nilAngleBytes)
		return
	}
	p.buf.WriteByte('?')
	p.buf.WriteString(reflect.TypeOf(v).String())
	p.buf.WriteByte('?')
}

func (p *pp) badVerb(verb rune) {
	p.erroring = true
	p.add('%')
	p.add('!')
	p.add(verb)
	p.add('(')
	switch {
	case p.arg != nil:
		p.buf.WriteString(reflect.TypeOf(p.arg).String())
		p.add('=')
		p.printArg(p.arg, 'v', false, false, 0)
	case p.value.IsValid():
		p.buf.WriteString(p.value.Type().String())
		p.add('=')
		p.printValue(p.value, 'v', false, false, 0)
	default:
		p.buf.Write(nilAngleBytes)
	}
	p.add(')')
	p.erroring = false
}

func (p *pp) fmtBool(v bool, verb rune) {
	switch verb {
	case 't', 'v':
		p.fmt.fmt_boolean(v)
	default:
		p.badVerb(verb)
	}
}

// fmtC formats a rune for the 'c' format.

// fmtC 将 c 格式化为“c”格式的符文。
func (p *pp) fmtC(c int64) {
	r := rune(c) // Check for overflow. // 溢出检查。
	if int64(r) != c {
		r = utf8.RuneError
	}
	w := utf8.EncodeRune(p.runeBuf[0:utf8.UTFMax], r)
	p.fmt.pad(p.runeBuf[0:w])
}

func (p *pp) fmtInt64(v int64, verb rune) {
	switch verb {
	case 'b':
		p.fmt.integer(v, 2, signed, ldigits)
	case 'c':
		p.fmtC(v)
	case 'd', 'v':
		p.fmt.integer(v, 10, signed, ldigits)
	case 'o':
		p.fmt.integer(v, 8, signed, ldigits)
	case 'q':
		if 0 <= v && v <= utf8.MaxRune {
			p.fmt.fmt_qc(v)
		} else {
			p.badVerb(verb)
		}
	case 'x':
		p.fmt.integer(v, 16, signed, ldigits)
	case 'U':
		p.fmtUnicode(v)
	case 'X':
		p.fmt.integer(v, 16, signed, udigits)
	default:
		p.badVerb(verb)
	}
}

// fmt0x64 formats a uint64 in hexadecimal and prefixes it with 0x or
// not, as requested, by temporarily setting the sharp flag.

// fmt0x64 将一个 uint64 值格式化为带 0x 前缀的十六进制数或不进行格式化，
// 它会根据需要临时设置 # 号标记。
func (p *pp) fmt0x64(v uint64, leading0x bool) {
	sharp := p.fmt.sharp
	p.fmt.sharp = leading0x
	p.fmt.integer(int64(v), 16, unsigned, ldigits)
	p.fmt.sharp = sharp
}

// fmtUnicode formats a uint64 in U+1234 form by
// temporarily turning on the unicode flag and tweaking the precision.

// fmtUnicode 通过临时开启Unicode标记并调整精度来将一个
// uint64 值格式化为 U+1234 这样的形式。
func (p *pp) fmtUnicode(v int64) {
	precPresent := p.fmt.precPresent
	sharp := p.fmt.sharp
	p.fmt.sharp = false
	prec := p.fmt.prec
	if !precPresent {
		// If prec is already set, leave it alone; otherwise 4 is minimum.
		// 若 prec 已经设置，就保留它，否则就将 4 作为最小值精度。
		p.fmt.prec = 4
		p.fmt.precPresent = true
	}
	p.fmt.unicode = true // turn on U+ // 开启 U+ 标记
	p.fmt.uniQuote = sharp
	p.fmt.integer(int64(v), 16, unsigned, udigits)
	p.fmt.unicode = false
	p.fmt.uniQuote = false
	p.fmt.prec = prec
	p.fmt.precPresent = precPresent
	p.fmt.sharp = sharp
}

func (p *pp) fmtUint64(v uint64, verb rune, goSyntax bool) {
	switch verb {
	case 'b':
		p.fmt.integer(int64(v), 2, unsigned, ldigits)
	case 'c':
		p.fmtC(int64(v))
	case 'd':
		p.fmt.integer(int64(v), 10, unsigned, ldigits)
	case 'v':
		if goSyntax {
			p.fmt0x64(v, true)
		} else {
			p.fmt.integer(int64(v), 10, unsigned, ldigits)
		}
	case 'o':
		p.fmt.integer(int64(v), 8, unsigned, ldigits)
	case 'q':
		if 0 <= v && v <= utf8.MaxRune {
			p.fmt.fmt_qc(int64(v))
		} else {
			p.badVerb(verb)
		}
	case 'x':
		p.fmt.integer(int64(v), 16, unsigned, ldigits)
	case 'X':
		p.fmt.integer(int64(v), 16, unsigned, udigits)
	case 'U':
		p.fmtUnicode(int64(v))
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtFloat32(v float32, verb rune) {
	switch verb {
	case 'b':
		p.fmt.fmt_fb32(v)
	case 'e':
		p.fmt.fmt_e32(v)
	case 'E':
		p.fmt.fmt_E32(v)
	case 'f':
		p.fmt.fmt_f32(v)
	case 'g', 'v':
		p.fmt.fmt_g32(v)
	case 'G':
		p.fmt.fmt_G32(v)
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtFloat64(v float64, verb rune) {
	switch verb {
	case 'b':
		p.fmt.fmt_fb64(v)
	case 'e':
		p.fmt.fmt_e64(v)
	case 'E':
		p.fmt.fmt_E64(v)
	case 'f':
		p.fmt.fmt_f64(v)
	case 'g', 'v':
		p.fmt.fmt_g64(v)
	case 'G':
		p.fmt.fmt_G64(v)
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtComplex64(v complex64, verb rune) {
	switch verb {
	case 'b', 'e', 'E', 'f', 'F', 'g', 'G':
		p.fmt.fmt_c64(v, verb)
	case 'v':
		p.fmt.fmt_c64(v, 'g')
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtComplex128(v complex128, verb rune) {
	switch verb {
	case 'b', 'e', 'E', 'f', 'F', 'g', 'G':
		p.fmt.fmt_c128(v, verb)
	case 'v':
		p.fmt.fmt_c128(v, 'g')
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtString(v string, verb rune, goSyntax bool) {
	switch verb {
	case 'v':
		if goSyntax {
			p.fmt.fmt_q(v)
		} else {
			p.fmt.fmt_s(v)
		}
	case 's':
		p.fmt.fmt_s(v)
	case 'x':
		p.fmt.fmt_sx(v, ldigits)
	case 'X':
		p.fmt.fmt_sx(v, udigits)
	case 'q':
		p.fmt.fmt_q(v)
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtBytes(v []byte, verb rune, goSyntax bool, typ reflect.Type, depth int) {
	if verb == 'v' || verb == 'd' {
		if goSyntax {
			if typ == nil {
				p.buf.Write(bytesBytes)
			} else {
				p.buf.WriteString(typ.String())
				p.buf.WriteByte('{')
			}
		} else {
			p.buf.WriteByte('[')
		}
		for i, c := range v {
			if i > 0 {
				if goSyntax {
					p.buf.Write(commaSpaceBytes)
				} else {
					p.buf.WriteByte(' ')
				}
			}
			p.printArg(c, 'v', p.fmt.plus, goSyntax, depth+1)
		}
		if goSyntax {
			p.buf.WriteByte('}')
		} else {
			p.buf.WriteByte(']')
		}
		return
	}
	switch verb {
	case 's':
		p.fmt.fmt_s(string(v))
	case 'x':
		p.fmt.fmt_bx(v, ldigits)
	case 'X':
		p.fmt.fmt_bx(v, udigits)
	case 'q':
		p.fmt.fmt_q(string(v))
	default:
		p.badVerb(verb)
	}
}

func (p *pp) fmtPointer(value reflect.Value, verb rune, goSyntax bool) {
	use0x64 := true
	switch verb {
	case 'p', 'v':
		// ok
	case 'b', 'd', 'o', 'x', 'X':
		use0x64 = false
		// ok
	default:
		p.badVerb(verb)
		return
	}

	var u uintptr
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.Slice, reflect.UnsafePointer:
		u = value.Pointer()
	default:
		p.badVerb(verb)
		return
	}

	if goSyntax {
		p.add('(')
		p.buf.WriteString(value.Type().String())
		p.add(')')
		p.add('(')
		if u == 0 {
			p.buf.Write(nilBytes)
		} else {
			p.fmt0x64(uint64(u), true)
		}
		p.add(')')
	} else if verb == 'v' && u == 0 {
		p.buf.Write(nilAngleBytes)
	} else {
		if use0x64 {
			p.fmt0x64(uint64(u), !p.fmt.sharp)
		} else {
			p.fmtUint64(uint64(u), verb, false)
		}
	}
}

var (
	intBits     = reflect.TypeOf(0).Bits()
	uintptrBits = reflect.TypeOf(uintptr(0)).Bits()
)

func (p *pp) catchPanic(arg interface{}, verb rune) {
	if err := recover(); err != nil {
		// If it's a nil pointer, just say "<nil>". The likeliest causes are a
		// Stringer that fails to guard against nil or a nil pointer for a
		// value receiver, and in either case, "<nil>" is a nice result.
		//
		// 若它是一个 nil 指针，只需显示“<nil>”即可。最可能的原因就是一个 Stringer
		// 未能防止 nil 或值接收器的 nil 指针，这两种情况下，“<nil>”是个不错的结果。
		if v := reflect.ValueOf(arg); v.Kind() == reflect.Ptr && v.IsNil() {
			p.buf.Write(nilAngleBytes)
			return
		}
		// Otherwise print a concise panic message. Most of the time the panic
		// value will print itself nicely.
		// 否则打印一个简明的panic消息。多数情况下panic值自己会打印得很好。
		if p.panicking {
			// Nested panics; the recursion in printArg cannot succeed.
			// 嵌套 printArg 中的递归无法成功。
			panic(err)
		}
		p.buf.Write(percentBangBytes)
		p.add(verb)
		p.buf.Write(panicBytes)
		p.panicking = true
		p.printArg(err, 'v', false, false, 0)
		p.panicking = false
		p.buf.WriteByte(')')
	}
}

func (p *pp) handleMethods(verb rune, plus, goSyntax bool, depth int) (wasString, handled bool) {
	if p.erroring {
		return
	}
	// Is it a Formatter?
	// 判断是否为 Formatter。
	if formatter, ok := p.arg.(Formatter); ok {
		handled = true
		wasString = false
		defer p.catchPanic(p.arg, verb)
		formatter.Format(p, verb)
		return
	}
	// Must not touch flags before Formatter looks at them.
	// 决不能在 Formatter 处理标记之前触及它们。
	if plus {
		p.fmt.plus = false
	}
	// If we're doing Go syntax and the argument knows how to supply it, take care of it now.
	// 如果我们正在处理Go语法而实参知道如何提供它，那就现在弄好它。
	if goSyntax {
		p.fmt.sharp = false
		if stringer, ok := p.arg.(GoStringer); ok {
			wasString = false
			handled = true
			defer p.catchPanic(p.arg, verb)
			// Print the result of GoString unadorned.
			// 纯粹地打印 GoString 的值。
			p.fmtString(stringer.GoString(), 's', false)
			return
		}
	} else {
		// If a string is acceptable according to the format, see if
		// the value satisfies one of the string-valued interfaces.
		// Println etc. set verb to %v, which is "stringable".
		//
		// 若一个字符串是否可以接受取决于其格式，就看它的值是否满足其中一种字符串值的接口。
		// Println 等函数会将占位符设置为 %v，它是“可字符串化”的。
		switch verb {
		case 'v', 's', 'x', 'X', 'q':
			// Is it an error or Stringer?
			// The duplication in the bodies is necessary:
			// setting wasString and handled, and deferring catchPanic,
			// must happen before calling the method.
			//
			// 它是 error 还是 Stringer？一下主体中的重复是必须的：
			// 设置 wasString 和 handled 并推迟 catchPanic 必须发生在调用此方法之前。
			switch v := p.arg.(type) {
			case error:
				wasString = false
				handled = true
				defer p.catchPanic(p.arg, verb)
				p.printArg(v.Error(), verb, plus, false, depth)
				return

			case Stringer:
				wasString = false
				handled = true
				defer p.catchPanic(p.arg, verb)
				p.printArg(v.String(), verb, plus, false, depth)
				return
			}
		}
	}
	handled = false
	return
}

func (p *pp) printArg(arg interface{}, verb rune, plus, goSyntax bool, depth int) (wasString bool) {
	p.arg = arg
	p.value = reflect.Value{}

	if arg == nil {
		if verb == 'T' || verb == 'v' {
			p.fmt.pad(nilAngleBytes)
		} else {
			p.badVerb(verb)
		}
		return false
	}

	// Special processing considerations.
	// %T (the value's type) and %p (its address) are special; we always do them first.
	// 对特殊处理的考虑。
	// %T（值的类型）与 %p（其地址）是特殊的；我们总是首先处理它。
	switch verb {
	case 'T':
		p.printArg(reflect.TypeOf(arg).String(), 's', false, false, 0)
		return false
	case 'p':
		p.fmtPointer(reflect.ValueOf(arg), verb, goSyntax)
		return false
	}

	// Clear flags for base formatters.
	// handleMethods needs them, so we must restore them later.
	// We could call handleMethods here and avoid this work, but
	// handleMethods is expensive enough to be worth delaying.
	//
	// 为基础的格式化器清理标记。
	// handleMethods 需要它们，因此我们必须稍后重新存储它们。我们可以在此处调用
	// handleMethods 并避免它工作，但对于是否值得来说 handleMethods 的代价够高了。
	oldPlus := p.fmt.plus
	oldSharp := p.fmt.sharp
	if plus {
		p.fmt.plus = false
	}
	if goSyntax {
		p.fmt.sharp = false
	}

	// Some types can be done without reflection.
	// 有些类型可以不用反射就能完成。
	switch f := arg.(type) {
	case bool:
		p.fmtBool(f, verb)
	case float32:
		p.fmtFloat32(f, verb)
	case float64:
		p.fmtFloat64(f, verb)
	case complex64:
		p.fmtComplex64(f, verb)
	case complex128:
		p.fmtComplex128(f, verb)
	case int:
		p.fmtInt64(int64(f), verb)
	case int8:
		p.fmtInt64(int64(f), verb)
	case int16:
		p.fmtInt64(int64(f), verb)
	case int32:
		p.fmtInt64(int64(f), verb)
	case int64:
		p.fmtInt64(f, verb)
	case uint:
		p.fmtUint64(uint64(f), verb, goSyntax)
	case uint8:
		p.fmtUint64(uint64(f), verb, goSyntax)
	case uint16:
		p.fmtUint64(uint64(f), verb, goSyntax)
	case uint32:
		p.fmtUint64(uint64(f), verb, goSyntax)
	case uint64:
		p.fmtUint64(f, verb, goSyntax)
	case uintptr:
		p.fmtUint64(uint64(f), verb, goSyntax)
	case string:
		p.fmtString(f, verb, goSyntax)
		wasString = verb == 's' || verb == 'v'
	case []byte:
		p.fmtBytes(f, verb, goSyntax, nil, depth)
		wasString = verb == 's'
	default:
		// Restore flags in case handleMethods finds a Formatter.
		// 在 handleMethods 找到 Formatter 的情况下重新存储标记。
		p.fmt.plus = oldPlus
		p.fmt.sharp = oldSharp
		// If the type is not simple, it might have methods.
		// 若该类型不简单，它可能拥有方法。
		if isString, handled := p.handleMethods(verb, plus, goSyntax, depth); handled {
			return isString
		}
		// Need to use reflection
		// 需要使用反射。
		return p.printReflectValue(reflect.ValueOf(arg), verb, plus, goSyntax, depth)
	}
	p.arg = nil
	return
}

// printValue is like printArg but starts with a reflect value, not an interface{} value.

// printValue 类似于 printArg，但它以一个反射值开始，而非 interface{} 值。
func (p *pp) printValue(value reflect.Value, verb rune, plus, goSyntax bool, depth int) (wasString bool) {
	if !value.IsValid() {
		if verb == 'T' || verb == 'v' {
			p.buf.Write(nilAngleBytes)
		} else {
			p.badVerb(verb)
		}
		return false
	}

	// Special processing considerations.
	// %T (the value's type) and %p (its address) are special; we always do them first.
	// 对特殊处理的考虑。
	// %T（值的类型）与 %p（其地址）是特殊的；我们总是首先处理它。
	switch verb {
	case 'T':
		p.printArg(value.Type().String(), 's', false, false, 0)
		return false
	case 'p':
		p.fmtPointer(value, verb, goSyntax)
		return false
	}

	// Handle values with special methods.
	// Call always, even when arg == nil, because handleMethods clears p.fmt.plus for us.
	// 用特殊的方法处理值。
	// 即使 arg == nil 时也总是调用，因为 handleMethods 为我们清理了 p.fmt.plus。
	p.arg = nil // Make sure it's cleared, for safety. // 为安全起见，确认它是否已被清理。
	if value.CanInterface() {
		p.arg = value.Interface()
	}
	if isString, handled := p.handleMethods(verb, plus, goSyntax, depth); handled {
		return isString
	}

	return p.printReflectValue(value, verb, plus, goSyntax, depth)
}

// printReflectValue is the fallback for both printArg and printValue.
// It uses reflect to print the value.

// printReflectValue 是 printField 和 printValue 二者的备用方案。
// 它使用反射来打印值。
func (p *pp) printReflectValue(value reflect.Value, verb rune, plus, goSyntax bool, depth int) (wasString bool) {
	oldValue := p.value
	p.value = value
BigSwitch:
	switch f := value; f.Kind() {
	case reflect.Bool:
		p.fmtBool(f.Bool(), verb)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		p.fmtInt64(f.Int(), verb)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		p.fmtUint64(f.Uint(), verb, goSyntax)
	case reflect.Float32, reflect.Float64:
		if f.Type().Size() == 4 {
			p.fmtFloat32(float32(f.Float()), verb)
		} else {
			p.fmtFloat64(f.Float(), verb)
		}
	case reflect.Complex64, reflect.Complex128:
		if f.Type().Size() == 8 {
			p.fmtComplex64(complex64(f.Complex()), verb)
		} else {
			p.fmtComplex128(f.Complex(), verb)
		}
	case reflect.String:
		p.fmtString(f.String(), verb, goSyntax)
	case reflect.Map:
		if goSyntax {
			p.buf.WriteString(f.Type().String())
			if f.IsNil() {
				p.buf.WriteString("(nil)")
				break
			}
			p.buf.WriteByte('{')
		} else {
			p.buf.Write(mapBytes)
		}
		keys := f.MapKeys()
		for i, key := range keys {
			if i > 0 {
				if goSyntax {
					p.buf.Write(commaSpaceBytes)
				} else {
					p.buf.WriteByte(' ')
				}
			}
			p.printValue(key, verb, plus, goSyntax, depth+1)
			p.buf.WriteByte(':')
			p.printValue(f.MapIndex(key), verb, plus, goSyntax, depth+1)
		}
		if goSyntax {
			p.buf.WriteByte('}')
		} else {
			p.buf.WriteByte(']')
		}
	case reflect.Struct:
		if goSyntax {
			p.buf.WriteString(value.Type().String())
		}
		p.add('{')
		v := f
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			if i > 0 {
				if goSyntax {
					p.buf.Write(commaSpaceBytes)
				} else {
					p.buf.WriteByte(' ')
				}
			}
			if plus || goSyntax {
				if f := t.Field(i); f.Name != "" {
					p.buf.WriteString(f.Name)
					p.buf.WriteByte(':')
				}
			}
			p.printValue(getField(v, i), verb, plus, goSyntax, depth+1)
		}
		p.buf.WriteByte('}')
	case reflect.Interface:
		value := f.Elem()
		if !value.IsValid() {
			if goSyntax {
				p.buf.WriteString(f.Type().String())
				p.buf.Write(nilParenBytes)
			} else {
				p.buf.Write(nilAngleBytes)
			}
		} else {
			wasString = p.printValue(value, verb, plus, goSyntax, depth+1)
		}
	case reflect.Array, reflect.Slice:
		// Byte slices are special.
		// 字节切片比较特殊。
		if typ := f.Type(); typ.Elem().Kind() == reflect.Uint8 {
			var bytes []byte
			if f.Kind() == reflect.Slice {
				bytes = f.Bytes()
			} else if f.CanAddr() {
				bytes = f.Slice(0, f.Len()).Bytes()
			} else {
				// We have an array, but we cannot Slice() a non-addressable array,
				// so we build a slice by hand. This is a rare case but it would be nice
				// if reflection could help a little more.
				//
				// 我们有一个数组，但我们不能让不可寻址的数组调用 Slice()，
				// 因此我们手动构建了一个切片。这是一种非常罕见的情况，
				// 但如果反射能再多点帮助，那就再好不过了。
				bytes = make([]byte, f.Len())
				for i := range bytes {
					bytes[i] = byte(f.Index(i).Uint())
				}
			}
			p.fmtBytes(bytes, verb, goSyntax, typ, depth)
			wasString = verb == 's'
			break
		}
		if goSyntax {
			p.buf.WriteString(value.Type().String())
			if f.Kind() == reflect.Slice && f.IsNil() {
				p.buf.WriteString("(nil)")
				break
			}
			p.buf.WriteByte('{')
		} else {
			p.buf.WriteByte('[')
		}
		for i := 0; i < f.Len(); i++ {
			if i > 0 {
				if goSyntax {
					p.buf.Write(commaSpaceBytes)
				} else {
					p.buf.WriteByte(' ')
				}
			}
			p.printValue(f.Index(i), verb, plus, goSyntax, depth+1)
		}
		if goSyntax {
			p.buf.WriteByte('}')
		} else {
			p.buf.WriteByte(']')
		}
	case reflect.Ptr:
		v := f.Pointer()
		// pointer to array or slice or struct?  ok at top level
		// but not embedded (avoid loops)
		// 指向数组还是切片还是结构体？在顶层它没啥问题，但嵌入后（避免循环）就不行了。
		if v != 0 && depth == 0 {
			switch a := f.Elem(); a.Kind() {
			case reflect.Array, reflect.Slice:
				p.buf.WriteByte('&')
				p.printValue(a, verb, plus, goSyntax, depth+1)
				break BigSwitch
			case reflect.Struct:
				p.buf.WriteByte('&')
				p.printValue(a, verb, plus, goSyntax, depth+1)
				break BigSwitch
			}
		}
		fallthrough
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		p.fmtPointer(value, verb, goSyntax)
	default:
		p.unknownType(f)
	}
	p.value = oldValue
	return wasString
}

// intFromArg gets the argNumth element of a. On return, isInt reports whether the argument has type int.

// intFromArg 获取 a 中的 argNumth 元素，isInt 报告了该实参是否拥有 int 类型。
func intFromArg(a []interface{}, argNum int) (num int, isInt bool, newArgNum int) {
	newArgNum = argNum
	if argNum < len(a) {
		num, isInt = a[argNum].(int)
		newArgNum = argNum + 1
	}
	return
}

// parseArgNumber returns the value of the bracketed number, minus 1
// (explicit argument numbers are one-indexed but we want zero-indexed).
// The opening bracket is known to be present at format[0].
// The returned values are the index, the number of bytes to consume
// up to the closing paren, if present, and whether the number parsed
// ok. The bytes to consume will be 1 if no closing paren is present.
func parseArgNumber(format string) (index int, wid int, ok bool) {
	// Find closing bracket.
	for i := 1; i < len(format); i++ {
		if format[i] == ']' {
			width, ok, newi := parsenum(format, 1, i)
			if !ok || newi != i {
				return 0, i + 1, false
			}
			return width - 1, i + 1, true // arg numbers are one-indexed and skip paren.
		}
	}
	return 0, 1, false
}

// argNumber returns the next argument to evaluate, which is either the value of the passed-in
// argNum or the value of the bracketed integer that begins format[i:]. It also returns
// the new value of i, that is, the index of the next byte of the format to process.
func (p *pp) argNumber(argNum int, format string, i int, numArgs int) (newArgNum, newi int, found bool) {
	if len(format) <= i || format[i] != '[' {
		return argNum, i, false
	}
	p.reordered = true
	index, wid, ok := parseArgNumber(format[i:])
	if ok && 0 <= index && index < numArgs {
		return index, i + wid, true
	}
	p.goodArgNum = false
	return argNum, i + wid, true
}

func (p *pp) doPrintf(format string, a []interface{}) {
	end := len(format)
	argNum := 0         // we process one argument per non-trivial format // 我们为每个非平凡格式都处理一个实参。
	afterIndex := false // previous item in format was an index like [3]. // 格式中的前一项是否为类似于 [3] 的下标。
	p.reordered = false
	for i := 0; i < end; {
		p.goodArgNum = true
		lasti := i
		for i < end && format[i] != '%' {
			i++
		}
		if i > lasti {
			p.buf.WriteString(format[lasti:i])
		}
		if i >= end {
			// done processing format string // 处理格式字符串完成
			break
		}

		// Process one verb // 处理个占位符
		i++
		// Do we have flags? // 是否有标记？
		p.fmt.clearflags()
	F:
		for ; i < end; i++ {
			switch format[i] {
			case '#':
				p.fmt.sharp = true
			case '0':
				p.fmt.zero = true
			case '+':
				p.fmt.plus = true
			case '-':
				p.fmt.minus = true
			case ' ':
				p.fmt.space = true
			default:
				break F
			}
		}

		// Do we have an explicit argument index?
		// 有显式的实参下标么？
		argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a))

		// Do we have width?
		// 有宽度不？
		if i < end && format[i] == '*' {
			i++
			p.fmt.wid, p.fmt.widPresent, argNum = intFromArg(a, argNum)
			if !p.fmt.widPresent {
				p.buf.Write(badWidthBytes)
			}
			afterIndex = false
		} else {
			p.fmt.wid, p.fmt.widPresent, i = parsenum(format, i, end)
			if afterIndex && p.fmt.widPresent { // "%[3]2d"
				p.goodArgNum = false
			}
		}
		// Do we have precision?
		// 有精度不？
		if i+1 < end && format[i] == '.' {
			i++
			if afterIndex { // "%[3].2d"
				p.goodArgNum = false
			}
			argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a))
			if format[i] == '*' {
				i++
				p.fmt.prec, p.fmt.precPresent, argNum = intFromArg(a, argNum)
				if !p.fmt.precPresent {
					p.buf.Write(badPrecBytes)
				}
				afterIndex = false
			} else {
				p.fmt.prec, p.fmt.precPresent, i = parsenum(format, i, end)
				if !p.fmt.precPresent {
					p.fmt.prec = 0
					p.fmt.precPresent = true
				}
			}
		}

		if !afterIndex {
			argNum, i, afterIndex = p.argNumber(argNum, format, i, len(a))
		}

		if i >= end {
			p.buf.Write(noVerbBytes)
			continue
		}
		c, w := utf8.DecodeRuneInString(format[i:])
		i += w
		// percent is special - absorbs no operand
		// 百分号是特殊的 —— 它不接受操作数
		if c == '%' {
			p.buf.WriteByte('%') // We ignore width and prec. // 我们忽略宽度和精度。
			continue
		}
		if !p.goodArgNum {
			p.buf.Write(percentBangBytes)
			p.add(c)
			p.buf.Write(badIndexBytes)
			continue
		} else if argNum >= len(a) { // out of operands // 超过操作数
			p.buf.Write(percentBangBytes)
			p.add(c)
			p.buf.Write(missingBytes)
			continue
		}
		arg := a[argNum]
		argNum++

		goSyntax := c == 'v' && p.fmt.sharp
		plus := c == 'v' && p.fmt.plus
		p.printArg(arg, c, plus, goSyntax, 0)
	}

	// Check for extra arguments unless the call accessed the arguments
	// out of order, in which case it's too expensive to detect if they've all
	// been used and arguably OK if they're not.
	if !p.reordered && argNum < len(a) {
		p.buf.Write(extraBytes)
		for ; argNum < len(a); argNum++ {
			arg := a[argNum]
			if arg != nil {
				p.buf.WriteString(reflect.TypeOf(arg).String())
				p.buf.WriteByte('=')
			}
			p.printArg(arg, 'v', false, false, 0)
			if argNum+1 < len(a) {
				p.buf.Write(commaSpaceBytes)
			}
		}
		p.buf.WriteByte(')')
	}
}

func (p *pp) doPrint(a []interface{}, addspace, addnewline bool) {
	prevString := false
	for argNum := 0; argNum < len(a); argNum++ {
		p.fmt.clearflags()
		// always add spaces if we're doing Println
		// 若我们执行 Println 就总是添加空格
		arg := a[argNum]
		if argNum > 0 {
			isString := arg != nil && reflect.TypeOf(arg).Kind() == reflect.String
			if addspace || !isString && !prevString {
				p.buf.WriteByte(' ')
			}
		}
		prevString = p.printArg(arg, 'v', false, false, 0)
	}
	if addnewline {
		p.buf.WriteByte('\n')
	}
}
