// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements signed multi-precision integers.

// 此文件实现了带符号的多精度整数。

package big

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
)

// An Int represents a signed multi-precision integer.
// The zero value for an Int represents the value 0.

// Int 表示一个带符号多精度整数。
// Int 的零值为值 0。
type Int struct {
	neg bool // sign                          // 符号
	abs nat  // absolute value of the integer // 整数的绝对值
}

var intOne = &Int{false, natOne}

// Sign returns:
//
//	-1 if x <  0
//	 0 if x == 0
//	+1 if x >  0
//

// 符号返回：
//
//	若 x <  0 则为 -1
//	若 x == 0 则为  0
//	若 x >  0 则为 +1
//
func (x *Int) Sign() int {
	if len(x.abs) == 0 {
		return 0
	}
	if x.neg {
		return -1
	}
	return 1
}

// SetInt64 sets z to x and returns z.

// SetInt64 将 z 置为 x 并返回 z。
func (z *Int) SetInt64(x int64) *Int {
	neg := false
	if x < 0 {
		neg = true
		x = -x
	}
	z.abs = z.abs.setUint64(uint64(x))
	z.neg = neg
	return z
}

// SetUint64 sets z to x and returns z.

// SetUint64 将 z 置为 x 并返回 z。
func (z *Int) SetUint64(x uint64) *Int {
	z.abs = z.abs.setUint64(x)
	z.neg = false
	return z
}

// NewInt allocates and returns a new Int set to x.

// NewInt 为 x 分配并返回一个新的 Int。
func NewInt(x int64) *Int {
	return new(Int).SetInt64(x)
}

// Set sets z to x and returns z.

// Set 将 z 置为 x 并返回 z。
func (z *Int) Set(x *Int) *Int {
	if z != x {
		z.abs = z.abs.set(x.abs)
		z.neg = x.neg
	}
	return z
}

// Bits provides raw (unchecked but fast) access to x by returning its
// absolute value as a little-endian Word slice. The result and x share
// the same underlying array.
// Bits is intended to support implementation of missing low-level Int
// functionality outside this package; it should be avoided otherwise.

// Bits 提供了对 z 的原始访问（未经检查但很快）。它通过将其绝对值作为小端序的 Word
// 切片返回来实现。其结果与 x 共享同一底层数组。Bits 旨在支持此包外缺失的底层 Int
// 功能的实现，除此之外应尽量避免。
func (x *Int) Bits() []Word {
	return x.abs
}

// SetBits provides raw (unchecked but fast) access to z by setting its
// value to abs, interpreted as a little-endian Word slice, and returning
// z. The result and abs share the same underlying array.
// SetBits is intended to support implementation of missing low-level Int
// functionality outside this package; it should be avoided otherwise.

// SetBits 提供了对 z 的原始访问（未经检查但很快）。它通过将其值设为
// abs，解释为小端序的 Word 切片，并返回 z 来实现。SetBits 旨在支持此包外缺失的底层
// Int 功能的实现，除此之外应尽量避免。
func (z *Int) SetBits(abs []Word) *Int {
	z.abs = nat(abs).norm()
	z.neg = false
	return z
}

// Abs sets z to |x| (the absolute value of x) and returns z.

// Abs 将 z 置为 |x|（即 x 的绝对值）并返回 z。
func (z *Int) Abs(x *Int) *Int {
	z.Set(x)
	z.neg = false
	return z
}

// Neg sets z to -x and returns z.

// Neg 将 z 置为 -x 并返回 z。
func (z *Int) Neg(x *Int) *Int {
	z.Set(x)
	z.neg = len(z.abs) > 0 && !z.neg // 0 has no sign // 0 没有符号
	return z
}

// Add sets z to the sum x+y and returns z.

// Add 将 z 置为 x+y 的和并返回 z。
func (z *Int) Add(x, y *Int) *Int {
	neg := x.neg
	if x.neg == y.neg {
		// x + y == x + y
		// (-x) + (-y) == -(x + y)
		z.abs = z.abs.add(x.abs, y.abs)
	} else {
		// x + (-y) == x - y == -(y - x)
		// (-x) + y == y - x == -(x - y)
		if x.abs.cmp(y.abs) >= 0 {
			z.abs = z.abs.sub(x.abs, y.abs)
		} else {
			neg = !neg
			z.abs = z.abs.sub(y.abs, x.abs)
		}
	}
	z.neg = len(z.abs) > 0 && neg // 0 has no sign // 0 没有符号
	return z
}

// Sub sets z to the difference x-y and returns z.

// Sub 将 z 置为 x-y 的差并返回 z。
func (z *Int) Sub(x, y *Int) *Int {
	neg := x.neg
	if x.neg != y.neg {
		// x - (-y) == x + y
		// (-x) - y == -(x + y)
		z.abs = z.abs.add(x.abs, y.abs)
	} else {
		// x - y == x - y == -(y - x)
		// (-x) - (-y) == y - x == -(x - y)
		if x.abs.cmp(y.abs) >= 0 {
			z.abs = z.abs.sub(x.abs, y.abs)
		} else {
			neg = !neg
			z.abs = z.abs.sub(y.abs, x.abs)
		}
	}
	z.neg = len(z.abs) > 0 && neg // 0 has no sign // 0 没有符号
	return z
}

// Mul sets z to the product x*y and returns z.

// Mul 将 z 置为 x*y 的积并返回 z。
func (z *Int) Mul(x, y *Int) *Int {
	// x * y == x * y
	// x * (-y) == -(x * y)
	// (-x) * y == -(x * y)
	// (-x) * (-y) == x * y
	z.abs = z.abs.mul(x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg != y.neg // 0 has no sign // 0 没有符号
	return z
}

// MulRange sets z to the product of all integers
// in the range [a, b] inclusively and returns z.
// If a > b (empty range), the result is 1.

// MulRange 将 z 置为闭区间 [a, b] 内所有整数的积并返回 z。
// 若 a > b（空区间），则其结果为 1。
func (z *Int) MulRange(a, b int64) *Int {
	switch {
	case a > b:
		return z.SetInt64(1) // empty range // 空区间
	case a <= 0 && b >= 0:
		return z.SetInt64(0) // range includes 0 // 区间包括 0
	}
	// a <= b && (b < 0 || a > 0)

	neg := false
	if a < 0 {
		neg = (b-a)&1 == 0
		a, b = -b, -a
	}

	z.abs = z.abs.mulRange(uint64(a), uint64(b))
	z.neg = neg
	return z
}

// Binomial sets z to the binomial coefficient of (n, k) and returns z.

// Binomial 将 z 置为 (n, k) 的二项式系数并返回 z。
func (z *Int) Binomial(n, k int64) *Int {
	var a, b Int
	a.MulRange(n-k+1, n)
	b.MulRange(1, k)
	return z.Quo(&a, &b)
}

// Quo sets z to the quotient x/y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Quo implements truncated division (like Go); see QuoRem for more details.

// Quo 在 y != 0 时，将 z 置为 x/y 的商并返回 z。
// 若 y == 0，就会产生一个除以零的运行时派错。
// Quo 实现了截断式除法（与Go相同），更多详情见 QuoRem。
func (z *Int) Quo(x, y *Int) *Int {
	z.abs, _ = z.abs.div(nil, x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg != y.neg // 0 has no sign // 0 没有符号
	return z
}

// Rem sets z to the remainder x%y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Rem implements truncated modulus (like Go); see QuoRem for more details.

// Rem 在 y != 0 时，将 z 置为 x%y 的余数并返回 z。
// 若 y == 0，就会产生一个除以零的运行时派错。
// Rem 实现了截断式取模（与Go相同），更多详情见 QuoRem。
func (z *Int) Rem(x, y *Int) *Int {
	_, z.abs = nat(nil).div(z.abs, x.abs, y.abs)
	z.neg = len(z.abs) > 0 && x.neg // 0 has no sign
	return z
}

// QuoRem sets z to the quotient x/y and r to the remainder x%y
// and returns the pair (z, r) for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
//
// QuoRem implements T-division and modulus (like Go):
//
//	q = x/y      with the result truncated to zero
//	r = x - y*q
//
// (See Daan Leijen, ``Division and Modulus for Computer Scientists''.)
// See DivMod for Euclidean division and modulus (unlike Go).
//

// QuoRem 在 y != 0 时，将 z 置为 x/y 的商，将 r 置为 x%y 的余数并返回值对 (z, r)。
// 若 y == 0，就会产生一个除以零的运行时派错。
//
// QuoRem 实现了截断式除法和取模（与Go相同）：
//
//	q = x/y      // 其结果向零截断
//	r = x - y*q
//
// （详见 Daan Leijen，《计算机科学家的除法和取模》。）
// 欧氏除法和取模（与Go不同）见 DivMod。
//
func (z *Int) QuoRem(x, y, r *Int) (*Int, *Int) {
	z.abs, r.abs = z.abs.div(r.abs, x.abs, y.abs)
	z.neg, r.neg = len(z.abs) > 0 && x.neg != y.neg, len(r.abs) > 0 && x.neg // 0 has no sign
	return z, r
}

// Div sets z to the quotient x/y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Div implements Euclidean division (unlike Go); see DivMod for more details.

// Div 在 y != 0 时，将 z 置为 x/y 的商并返回 z。
// 若 y == 0，就会产生一个除以零的运行时派错。
// Div 实现了欧氏除法（与Go不同），更多详情见 DivMod。
func (z *Int) Div(x, y *Int) *Int {
	y_neg := y.neg // z may be an alias for y // z 可能是 y 的别名
	var r Int
	z.QuoRem(x, y, &r)
	if r.neg {
		if y_neg {
			z.Add(z, intOne)
		} else {
			z.Sub(z, intOne)
		}
	}
	return z
}

// Mod sets z to the modulus x%y for y != 0 and returns z.
// If y == 0, a division-by-zero run-time panic occurs.
// Mod implements Euclidean modulus (unlike Go); see DivMod for more details.

// Mod 在 y != 0 时，将 z 置为 x%y 的余数并返回 z。
// 若 y == 0，就会产生一个除以零的运行时派错。
// Mod 实现了欧氏取模（与Go不同），更多详情见 DivMod。
func (z *Int) Mod(x, y *Int) *Int {
	y0 := y // save y
	if z == y || alias(z.abs, y.abs) {
		y0 = new(Int).Set(y)
	}
	var q Int
	q.QuoRem(x, y, z)
	if z.neg {
		if y0.neg {
			z.Sub(z, y0)
		} else {
			z.Add(z, y0)
		}
	}
	return z
}

// DivMod sets z to the quotient x div y and m to the modulus x mod y
// and returns the pair (z, m) for y != 0.
// If y == 0, a division-by-zero run-time panic occurs.
//
// DivMod implements Euclidean division and modulus (unlike Go):
//
//	q = x div y  such that
//	m = x - y*q  with 0 <= m < |q|
//
// (See Raymond T. Boute, ``The Euclidean definition of the functions
// div and mod''. ACM Transactions on Programming Languages and
// Systems (TOPLAS), 14(2):127-144, New York, NY, USA, 4/1992.
// ACM press.)
// See QuoRem for T-division and modulus (like Go).
//

// DivMod 在 y != 0 时，将 z 置为 x 除以 y 的商，将 m 置为 x 取模 y 的模数并返回值对 (z, m)。
// 若 y == 0，就会产生一个除以零的运行时派错。
//
// DivMod 实现了截断式除法和取模（与Go不同）：
//
//	q = x div y // 使得
//	m = x - y*q // 其中
//	0 <= m < |q|
//
// （详见 Raymond T. Boute，《函数 div 和 mod 的欧氏定义》以及《ACM编程语言与系统会议记录》
// （TOPLAS），14(2):127-144, New York, NY, USA, 4/1992. ACM 出版社。）
// 截断式除法和取模（与Go相同）见 QuoRem。
//
func (z *Int) DivMod(x, y, m *Int) (*Int, *Int) {
	y0 := y // save y // 保存 y
	if z == y || alias(z.abs, y.abs) {
		y0 = new(Int).Set(y)
	}
	z.QuoRem(x, y, m)
	if m.neg {
		if y0.neg {
			z.Add(z, intOne)
			m.Sub(m, y0)
		} else {
			z.Sub(z, intOne)
			m.Add(m, y0)
		}
	}
	return z, m
}

// Cmp compares x and y and returns:
//
//   -1 if x <  y
//    0 if x == y
//   +1 if x >  y
//

// Cmp 比较 x 和 y 并返回：
//
//	若 x <  y 则为 -1
//	若 x == y 则为  0
//	若 x >  y 则为 +1
//
func (x *Int) Cmp(y *Int) (r int) {
	// x cmp y == x cmp y
	// x cmp (-y) == x
	// (-x) cmp y == y
	// (-x) cmp (-y) == -(x cmp y)
	switch {
	case x.neg == y.neg:
		r = x.abs.cmp(y.abs)
		if x.neg {
			r = -r
		}
	case x.neg:
		r = -1
	default:
		r = 1
	}
	return
}

func (x *Int) String() string {
	switch {
	case x == nil:
		return "<nil>"
	case x.neg:
		return "-" + x.abs.decimalString()
	}
	return x.abs.decimalString()
}

func charset(ch rune) string {
	switch ch {
	case 'b':
		return lowercaseDigits[0:2]
	case 'o':
		return lowercaseDigits[0:8]
	case 'd', 's', 'v':
		return lowercaseDigits[0:10]
	case 'x':
		return lowercaseDigits[0:16]
	case 'X':
		return uppercaseDigits[0:16]
	}
	return "" // unknown format // 未知格式
}

// write count copies of text to s

// 将 count 份 text 的副本写入 s
func writeMultiple(s fmt.State, text string, count int) {
	if len(text) > 0 {
		b := []byte(text)
		for ; count > 0; count-- {
			s.Write(b)
		}
	}
}

// Format is a support routine for fmt.Formatter. It accepts
// the formats 'b' (binary), 'o' (octal), 'd' (decimal), 'x'
// (lowercase hexadecimal), and 'X' (uppercase hexadecimal).
// Also supported are the full suite of package fmt's format
// verbs for integral types, including '+', '-', and ' '
// for sign control, '#' for leading zero in octal and for
// hexadecimal, a leading "0x" or "0X" for "%#x" and "%#X"
// respectively, specification of minimum digits precision,
// output field width, space or zero padding, and left or
// right justification.
//

// Format 是 fmt.Formatter 的一个支持函数。它接受 'b'（二进制）、'o'（八进制）、
// 'd'（十进制）、'x'（小写十六进制）和 'X'（大写十六进制）的格式。也同样支持 fmt
// 包的一整套类型的格式占位符，包括用于符号控制的 '+'、'-' 和 ' '，用于八进制前导零的
// '#'，分别用于十六进制前导 "0x" 或 "0X" 的 "%#x" 和 "%#X"，用于最小数字精度的规范，
// 输出字段的宽度，空格或零的填充，以及左右对齐。
//
func (x *Int) Format(s fmt.State, ch rune) {
	cs := charset(ch)

	// special cases
	// 特殊情况
	switch {
	case cs == "":
		// unknown format
		// 未知格式
		fmt.Fprintf(s, "%%!%c(big.Int=%s)", ch, x.String())
		return
	case x == nil:
		fmt.Fprint(s, "<nil>")
		return
	}

	// determine sign character
	// 决定符号的字符
	sign := ""
	switch {
	case x.neg:
		sign = "-"
	// 当二者都指定时取代 ' '
	case s.Flag('+'): // supersedes ' ' when both specified
		sign = "+"
	case s.Flag(' '):
		sign = " "
	}

	// determine prefix characters for indicating output base
	// 决定前缀字符来指示输出的进制
	prefix := ""
	if s.Flag('#') {
		switch ch {
		case 'o': // octal // 八进制
			prefix = "0"
		case 'x': // hexadecimal // 十六进制
			prefix = "0x"
		case 'X':
			prefix = "0X"
		}
	}

	// determine digits with base set by len(cs) and digit characters from cs
	// 根据 len(cs) 和 cs 的数字字符来决定其所在的进制数字集合。
	digits := x.abs.string(cs)

	// number of characters for the three classes of number padding
	// 三种数字填充的字符数
	// left：  右对齐数字左侧的空白字符数 ("%8d")
	// zeroes：零字符（实际上的 cs[0]）作为最左边的数字 ("%8d")
	// right： 左对齐数字右侧的空白字符数 ("%-8d")
	var left int   // space characters to left of digits for right justification ("%8d")
	var zeroes int // zero characters (actually cs[0]) as left-most digits ("%.8d")
	var right int  // space characters to right of digits for left justification ("%-8d")

	// determine number padding from precision: the least number of digits to output
	// 根据精度决定填充数：输出最少的数字
	precision, precisionSet := s.Precision()
	if precisionSet {
		switch {
		case len(digits) < precision:
			zeroes = precision - len(digits) // count of zero padding // 记录零填充数
		case digits == "0" && precision == 0:
			// 若为零值 (x == 0) 或零精度 ("." 或 ".0") 则不打印
			return // print nothing if zero value (x == 0) and zero precision ("." or ".0")
		}
	}

	// determine field pad from width: the least number of characters to output
	// 根据宽度决定字段的填充：输出最少的字符数
	length := len(sign) + len(prefix) + zeroes + len(digits)
	// 填充为指定的宽度
	if width, widthSet := s.Width(); widthSet && length < width { // pad as specified
		switch d := width - length; {
		case s.Flag('-'):
			// pad on the right with spaces; supersedes '0' when both specified
			// 在右侧以空格填充；当二者都指定时用 '0' 取代
			right = d
		case s.Flag('0') && !precisionSet:
			// pad with zeroes unless precision also specified
			// 除非也指定了精度，否者用零填充
			zeroes = d
		default:
			// pad on the left with spaces
			// 在左侧以空格填充
			left = d
		}
	}

	// print number as [left pad][sign][prefix][zero pad][digits][right pad]
	// 将数字以 [左填充][符号][前缀][零填充][数字][右填充] 的形式打印出来
	writeMultiple(s, " ", left)
	writeMultiple(s, sign, 1)
	writeMultiple(s, prefix, 1)
	writeMultiple(s, "0", zeroes)
	writeMultiple(s, digits, 1)
	writeMultiple(s, " ", right)
}

// scan sets z to the integer value corresponding to the longest possible prefix
// read from r representing a signed integer number in a given conversion base.
// It returns z, the actual conversion base used, and an error, if any. In the
// error case, the value of z is undefined but the returned value is nil. The
// syntax follows the syntax of integer literals in Go.
//
// The base argument must be 0 or a value from 2 through MaxBase. If the base
// is 0, the string prefix determines the actual conversion base. A prefix of
// ``0x'' or ``0X'' selects base 16; the ``0'' prefix selects base 8, and a
// ``0b'' or ``0B'' prefix selects base 2. Otherwise the selected base is 10.
//

// scan 将 z 置为一个整数值，该整数值对应于从 r 中读取的最长可能的前缀数，这里的 r
// 为按给定转换进制 base 表示的带符号整数。它返回实际使用的转换进制 z，和一个可能的错误。
// 在有错误的情况下，z 的值为未定义，但其返回值为 nil。其语法遵循Go中整数字面的语法。
//
// 进制实参 base 必须为 0 或从 2 到 MaxBase 的值。若 base 为 0，则其实际的转换进制由
// 该字符串的前缀决定。前缀“0x”或“0X”会选择16进制，前缀“0”会选择8进制，前缀“0b”或“0B”
// 会选择2进制。其它情况则选择10进制。
func (z *Int) scan(r io.RuneScanner, base int) (*Int, int, error) {
	// determine sign
	// 决定符号
	ch, _, err := r.ReadRune()
	if err != nil {
		return nil, 0, err
	}
	neg := false
	switch ch {
	case '-':
		neg = true
	// 啥也不做
	case '+': // nothing to do
	default:
		r.UnreadRune()
	}

	// determine mantissa
	// 决定尾数
	z.abs, base, err = z.abs.scan(r, base)
	if err != nil {
		return nil, base, err
	}
	// 0 没有符号
	z.neg = len(z.abs) > 0 && neg // 0 has no sign

	return z, base, nil
}

// Scan is a support routine for fmt.Scanner; it sets z to the value of
// the scanned number. It accepts the formats 'b' (binary), 'o' (octal),
// 'd' (decimal), 'x' (lowercase hexadecimal), and 'X' (uppercase hexadecimal).

// Scan 是 fmt.Scanner 的一个支持函数；它将 z 置为已扫描数字的值。它接受格式'b'（二进制）、
// 'o'（八进制）、'd'（十进制）、'x'（小写十六进制）及'X'（大写十六进制）。
func (z *Int) Scan(s fmt.ScanState, ch rune) error {
	// 跳过前导的空格符
	s.SkipSpace() // skip leading space characters
	base := 0
	switch ch {
	case 'b':
		base = 2
	case 'o':
		base = 8
	case 'd':
		base = 10
	case 'x', 'X':
		base = 16
	case 's', 'v':
		// let scan determine the base
		// 通过扫描决定进制
	default:
		return errors.New("Int.Scan: invalid verb")
	}
	_, _, err := z.scan(s, base)
	return err
}

// Int64 returns the int64 representation of x.
// If x cannot be represented in an int64, the result is undefined.

// Int64 返回 x 的 int64 表示。
// 若 x 不能被表示为 int64，则其结果是未定义的。
func (x *Int) Int64() int64 {
	v := int64(x.Uint64())
	if x.neg {
		v = -v
	}
	return v
}

// Uint64 returns the uint64 representation of x.
// If x cannot be represented in a uint64, the result is undefined.

// Uint64 返回 x 的 uint64 表示。
// 若 x 不能被表示为 uint64，则其结果是未定义的。
func (x *Int) Uint64() uint64 {
	if len(x.abs) == 0 {
		return 0
	}
	v := uint64(x.abs[0])
	if _W == 32 && len(x.abs) > 1 {
		v |= uint64(x.abs[1]) << 32
	}
	return v
}

// SetString sets z to the value of s, interpreted in the given base,
// and returns z and a boolean indicating success. If SetString fails,
// the value of z is undefined but the returned value is nil.
//
// The base argument must be 0 or a value from 2 through MaxBase. If the base
// is 0, the string prefix determines the actual conversion base. A prefix of
// ``0x'' or ``0X'' selects base 16; the ``0'' prefix selects base 8, and a
// ``0b'' or ``0B'' prefix selects base 2. Otherwise the selected base is 10.
//

// SetString 将 z 置为 s 的值，按给定的进制 base 解释并返回 z 和一个指示是否成功的布尔值。
// 若 SetString 失败，则 z 的值是未定义的，其返回值则为 nil。
//
// 进制实参 base 必须为 0 或从 2 到 MaxBase 的值。若 base 为 0，则其实际的转换进制由
// 该字符串的前缀决定。前缀“0x”或“0X”会选择16进制，前缀“0”会选择8进制，前缀“0b”或“0B”
// 会选择2进制。其它情况则选择10进制。
func (z *Int) SetString(s string, base int) (*Int, bool) {
	r := strings.NewReader(s)
	_, _, err := z.scan(r, base)
	if err != nil {
		return nil, false
	}
	_, _, err = r.ReadRune()
	if err != io.EOF {
		return nil, false
	}
	// err == io.EOF => 已扫描完 s 中的所有字符。
	return z, true // err == io.EOF => scan consumed all of s
}

// SetBytes interprets buf as the bytes of a big-endian unsigned
// integer, sets z to that value, and returns z.

// SetBytes 将 buf 解释为大端序的无符号整数字节，置 z 为该值后返回 z。
func (z *Int) SetBytes(buf []byte) *Int {
	z.abs = z.abs.setBytes(buf)
	z.neg = false
	return z
}

// Bytes returns the absolute value of z as a big-endian byte slice.
// Bytes 将 z 的绝对值作为大端序的字节切片返回。
func (x *Int) Bytes() []byte {
	buf := make([]byte, len(x.abs)*_S)
	return buf[x.abs.bytes(buf):]
}

// BitLen returns the length of the absolute value of z in bits.
// The bit length of 0 is 0.

// BitLen 返回 z 的绝对值的位数长度。0 的位长为 0.
func (x *Int) BitLen() int {
	return x.abs.bitLen()
}

// Exp sets z = x**y mod |m| (i.e. the sign of m is ignored), and returns z.
// If y <= 0, the result is 1; if m == nil or m == 0, z = x**y.
// See Knuth, volume 2, section 4.6.3.

// Exp 置 z = x**y mod |m|（换言之，m 的符号被忽略），并返回 z。
// 若 y <=0，则其结果为 1，若 m == nil 或 m == 0，则 z = x**y。
// 见 Knuth《计算机程序设计艺术》，卷 2，章节 4.6.3。
func (z *Int) Exp(x, y, m *Int) *Int {
	if y.neg || len(y.abs) == 0 {
		return z.SetInt64(1)
	}
	// y > 0

	var mWords nat
	if m != nil {
		// 对于 m == 0，m.abs 可能为 nil
		mWords = m.abs // m.abs may be nil for m == 0
	}

	z.abs = z.abs.expNN(x.abs, y.abs, mWords)
	z.neg = len(z.abs) > 0 && x.neg && y.abs[0]&1 == 1 // 0 has no sign // 0 没有符号
	return z
}

// GCD sets z to the greatest common divisor of a and b, which both must
// be > 0, and returns z.
// If x and y are not nil, GCD sets x and y such that z = a*x + b*y.
// If either a or b is <= 0, GCD sets z = x = y = 0.

// GCD 将 z 置为 a 和 b 的最大公约数，二者必须均 > 0，并返回 z。
// 若 x 或 y 非 nil，GCD 会设置 x 与 y 的值使得 z = a*x + b*y。
// 若 a 或 b <= 0，GCD就会置 z = x = y = 0。
func (z *Int) GCD(x, y, a, b *Int) *Int {
	if a.Sign() <= 0 || b.Sign() <= 0 {
		z.SetInt64(0)
		if x != nil {
			x.SetInt64(0)
		}
		if y != nil {
			y.SetInt64(0)
		}
		return z
	}
	if x == nil && y == nil {
		return z.binaryGCD(a, b)
	}

	A := new(Int).Set(a)
	B := new(Int).Set(b)

	X := new(Int)
	Y := new(Int).SetInt64(1)

	lastX := new(Int).SetInt64(1)
	lastY := new(Int)

	q := new(Int)
	temp := new(Int)

	for len(B.abs) > 0 {
		r := new(Int)
		q, r = q.QuoRem(A, B, r)

		A, B = B, r

		temp.Set(X)
		X.Mul(X, q)
		X.neg = !X.neg
		X.Add(X, lastX)
		lastX.Set(temp)

		temp.Set(Y)
		Y.Mul(Y, q)
		Y.neg = !Y.neg
		Y.Add(Y, lastY)
		lastY.Set(temp)
	}

	if x != nil {
		*x = *lastX
	}

	if y != nil {
		*y = *lastY
	}

	*z = *A
	return z
}

// binaryGCD sets z to the greatest common divisor of a and b, which both must
// be > 0, and returns z.
// See Knuth, The Art of Computer Programming, Vol. 2, Section 4.5.2, Algorithm B.

// binaryBCD 将 z 置为 a 和 b 的最大公约数，二者必须均 > 0，并返回 z。
// 见 Knuth《计算机程序设计艺术》卷 2，章节 4.5.2，算法 B。
func (z *Int) binaryGCD(a, b *Int) *Int {
	u := z
	v := new(Int)

	// use one Euclidean iteration to ensure that u and v are approx. the same size
	// 通过欧几里得迭代来确认 u 和 v 的大小大致相同。
	switch {
	case len(a.abs) > len(b.abs):
		u.Set(b)
		v.Rem(a, b)
	case len(a.abs) < len(b.abs):
		u.Set(a)
		v.Rem(b, a)
	default:
		u.Set(a)
		v.Set(b)
	}

	// v might be 0 now
	// v 现在可能为 0
	if len(v.abs) == 0 {
		return u
	}
	// u > 0 && v > 0

	// determine largest k such that u = u' << k, v = v' << k
	// 决定最大的 k 使得 u = u' << k，v = v' << k
	k := u.abs.trailingZeroBits()
	if vk := v.abs.trailingZeroBits(); vk < k {
		k = vk
	}
	u.Rsh(u, k)
	v.Rsh(v, k)

	// determine t (we know that u > 0)
	// 决定 t（我们知道 u > 0）
	t := new(Int)
	if u.abs[0]&1 != 0 {
		// u is odd
		// u 为奇数
		t.Neg(v)
	} else {
		t.Set(u)
	}

	for len(t.abs) > 0 {
		// reduce t
		// 减少 t
		t.Rsh(t, t.abs.trailingZeroBits())
		if t.neg {
			v, t = t, v
			// 0 没有符号
			v.neg = len(v.abs) > 0 && !v.neg // 0 has no sign
		} else {
			u, t = t, u
		}
		t.Sub(u, v)
	}

	return z.Lsh(u, k)
}

// ProbablyPrime performs n Miller-Rabin tests to check whether x is prime.
// If it returns true, x is prime with probability 1 - 1/4^n.
// If it returns false, x is not prime.

// ProbablyPrime 通过执行 n 次Miller-Rabin测试来检查 x 是否为质数。
// 若它返回 true，x 有 1 - 1/4^n 的可能性为质数。
// 若它返回 false，则 x 不是质数。
func (x *Int) ProbablyPrime(n int) bool {
	return !x.neg && x.abs.probablyPrime(n)
}

// Rand sets z to a pseudo-random number in [0, n) and returns z.

// Rand 将 z 置为区间 [0, n) 中的一个伪随机数并返回 z。
func (z *Int) Rand(rnd *rand.Rand, n *Int) *Int {
	z.neg = false
	if n.neg == true || len(n.abs) == 0 {
		z.abs = nil
		return z
	}
	z.abs = z.abs.random(rnd, n.abs, n.abs.bitLen())
	return z
}

// ModInverse sets z to the multiplicative inverse of g in the group ℤ/pℤ (where
// p is a prime) and returns z.

// ModInverse 将 z 置为 g 在群组 ℤ/pℤ 中的乘法逆元素（其中 p 为质数）并返回 z。
func (z *Int) ModInverse(g, p *Int) *Int {
	var d Int
	d.GCD(z, nil, g, p)
	// x and y are such that g*x + p*y = d. Since p is prime, d = 1. Taking
	// that modulo p results in g*x = 1, therefore x is the inverse element.
	// x 和 y 满足 g*x + p*y = d。由于 p 为质数，d = 1。获取 g*x = 1 时以 p
	// 为模的结果，因此 x 是所求的反转元素。
	if z.neg {
		z.Add(z, p)
	}
	return z
}

// Lsh sets z = x << n and returns z.

// Lsh 置 z = x << n 并返回 z。
func (z *Int) Lsh(x *Int, n uint) *Int {
	z.abs = z.abs.shl(x.abs, n)
	z.neg = x.neg
	return z
}

// Rsh sets z = x >> n and returns z.

// Rsh 置 z = x >> n 并返回 z。
func (z *Int) Rsh(x *Int, n uint) *Int {
	if x.neg {
		// (-x) >> s == ^(x-1) >> s == ^((x-1) >> s) == -(((x-1) >> s) + 1)
		// 不会向下溢出，因为 |x| > 0
		t := z.abs.sub(x.abs, natOne) // no underflow because |x| > 0
		t = t.shr(t, n)
		z.abs = t.add(t, natOne)
		// 若 x 为负数，则 z 不能为零
		z.neg = true // z cannot be zero if x is negative
		return z
	}

	z.abs = z.abs.shr(x.abs, n)
	z.neg = false
	return z
}

// Bit returns the value of the i'th bit of x. That is, it
// returns (x>>i)&1. The bit index i must be >= 0.

// Bit 返回 x 第 i 位的值。换言之，它返回 (x>>i)&1。位下标 i 必须 >= 0。
func (x *Int) Bit(i int) uint {
	if i == 0 {
		// optimization for common case: odd/even test of x
		// 一般情况的优化：x 的奇/偶性测试
		if len(x.abs) > 0 {
			// 位 0 与 -x 相同
			return uint(x.abs[0] & 1) // bit 0 is same for -x
		}
		return 0
	}
	if i < 0 {
		panic("negative bit index")
	}
	if x.neg {
		t := nat(nil).sub(x.abs, natOne)
		return t.bit(uint(i)) ^ 1
	}

	return x.abs.bit(uint(i))
}

// SetBit sets z to x, with x's i'th bit set to b (0 or 1).
// That is, if b is 1 SetBit sets z = x | (1 << i);
// if b is 0 SetBit sets z = x &^ (1 << i). If b is not 0 or 1,
// SetBit will panic.

// SetBit 将 z 置为 x，将 x 的第 i 位置为 b（0 或 1）。
// 换言之，若 b 为 1，SetBit 会置 z = x | (1 << i)；若 b 为 0，SetBit
// 会置 z = x &^ (1 << i)。若 b 非 0 或 1，SetBit 就会引发派错。
func (z *Int) SetBit(x *Int, i int, b uint) *Int {
	if i < 0 {
		panic("negative bit index")
	}
	if x.neg {
		t := z.abs.sub(x.abs, natOne)
		t = t.setBit(t, uint(i), b^1)
		z.abs = t.add(t, natOne)
		z.neg = len(z.abs) > 0
		return z
	}
	z.abs = z.abs.setBit(x.abs, uint(i), b)
	z.neg = false
	return z
}

// And sets z = x & y and returns z.

// And 置 z = x & y 并返回 z。
func (z *Int) And(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) & (-y) == ^(x-1) & ^(y-1) == ^((x-1) | (y-1)) == -(((x-1) | (y-1)) + 1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.add(z.abs.or(x1, y1), natOne)
			// 若 x 和 y 为负数，则 z 不能为零。
			z.neg = true // z cannot be zero if x and y are negative
			return z
		}

		// x & y == x & y
		z.abs = z.abs.and(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		// & 是对称的
		x, y = y, x // & is symmetric
	}

	// x & (-y) == x & ^(y-1) == x &^ (y-1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.andNot(x.abs, y1)
	z.neg = false
	return z
}

// AndNot sets z = x &^ y and returns z.

// AndNot 置 z = x &^ y 并返回 z。
func (z *Int) AndNot(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) &^ (-y) == ^(x-1) &^ ^(y-1) == ^(x-1) & (y-1) == (y-1) &^ (x-1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.andNot(y1, x1)
			z.neg = false
			return z
		}

		// x &^ y == x &^ y
		z.abs = z.abs.andNot(x.abs, y.abs)
		z.neg = false
		return z
	}

	if x.neg {
		// (-x) &^ y == ^(x-1) &^ y == ^(x-1) & ^y == ^((x-1) | y) == -(((x-1) | y) + 1)
		x1 := nat(nil).sub(x.abs, natOne)
		z.abs = z.abs.add(z.abs.or(x1, y.abs), natOne)
		// 若 x 为负数且 y 为正数，则 z 不能为零。
		z.neg = true // z cannot be zero if x is negative and y is positive
		return z
	}

	// x &^ (-y) == x &^ ^(y-1) == x & (y-1)
	y1 := nat(nil).add(y.abs, natOne)
	z.abs = z.abs.and(x.abs, y1)
	z.neg = false
	return z
}

// Or sets z = x | y and returns z.

// Or 置 z = x | y 并返回 z。
func (z *Int) Or(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) | (-y) == ^(x-1) | ^(y-1) == ^((x-1) & (y-1)) == -(((x-1) & (y-1)) + 1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.add(z.abs.and(x1, y1), natOne)
			// 若 x 和 y 为负数，则 z 不能为零。
			z.neg = true // z cannot be zero if x and y are negative
			return z
		}

		// x | y == x | y
		z.abs = z.abs.or(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		// | 是对称的
		x, y = y, x // | is symmetric
	}

	// x | (-y) == x | ^(y-1) == ^((y-1) &^ x) == -(^((y-1) &^ x) + 1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.add(z.abs.andNot(y1, x.abs), natOne)
	// 若 x 或 y 之一为负数，则 z 不能为零
	z.neg = true // z cannot be zero if one of x or y is negative
	return z
}

// Xor sets z = x ^ y and returns z.

// Xor 置 z = x ^ y 并返回 z。
func (z *Int) Xor(x, y *Int) *Int {
	if x.neg == y.neg {
		if x.neg {
			// (-x) ^ (-y) == ^(x-1) ^ ^(y-1) == (x-1) ^ (y-1)
			x1 := nat(nil).sub(x.abs, natOne)
			y1 := nat(nil).sub(y.abs, natOne)
			z.abs = z.abs.xor(x1, y1)
			z.neg = false
			return z
		}

		// x ^ y == x ^ y
		z.abs = z.abs.xor(x.abs, y.abs)
		z.neg = false
		return z
	}

	// x.neg != y.neg
	if x.neg {
		// ^ 是对称的
		x, y = y, x // ^ is symmetric
	}

	// x ^ (-y) == x ^ ^(y-1) == ^(x ^ (y-1)) == -((x ^ (y-1)) + 1)
	y1 := nat(nil).sub(y.abs, natOne)
	z.abs = z.abs.add(z.abs.xor(x.abs, y1), natOne)
	// 若 x 或 y 中只有一个为负数，则 z 不能为零。
	z.neg = true // z cannot be zero if only one of x or y is negative
	return z
}

// Not sets z = ^x and returns z.

// Not 置 z = ^x 并返回 z。
func (z *Int) Not(x *Int) *Int {
	if x.neg {
		// ^(-x) == ^(^(x-1)) == x-1
		z.abs = z.abs.sub(x.abs, natOne)
		z.neg = false
		return z
	}

	// ^x == -x-1 == -(x+1)
	z.abs = z.abs.add(x.abs, natOne)
	// 若 x 为正数，则 z 不能为零
	z.neg = true // z cannot be zero if x is positive
	return z
}

// Gob codec version. Permits backward-compatible changes to the encoding.

// Gob 编解码器版本。允许对编码进行向前兼容的更改。
const intGobVersion byte = 1

// GobEncode implements the gob.GobEncoder interface.

// GobEncode 实现了 gob.GobEncoder 接口。
func (x *Int) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}
	buf := make([]byte, 1+len(x.abs)*_S) // extra byte for version and sign bit // 版本和符号位的扩展字节
	i := x.abs.bytes(buf) - 1            // i >= 0
	b := intGobVersion << 1              // make space for sign bit // 为符号位留下空间
	if x.neg {
		b |= 1
	}
	buf[i] = b
	return buf[i:], nil
}

// GobDecode implements the gob.GobDecoder interface.

// GobDecode 实现了 gob.GobDecoder 接口。
func (z *Int) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Int{}
		return nil
	}
	b := buf[0]
	if b>>1 != intGobVersion {
		return errors.New(fmt.Sprintf("Int.GobDecode: encoding version %d not supported", b>>1))
	}
	z.neg = b&1 != 0
	z.abs = z.abs.setBytes(buf[1:])
	return nil
}

// MarshalJSON implements the json.Marshaler interface.

// MarshalJSON 实现了 json.Marshaler 接口。
func (x *Int) MarshalJSON() ([]byte, error) {
	// TODO(gri): get rid of the []byte/string conversions
	return []byte(x.String()), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.

// UnmarshalJSON 实现了 json.Unmarshaler 接口。
func (z *Int) UnmarshalJSON(x []byte) error {
	// TODO(gri): get rid of the []byte/string conversions
	_, ok := z.SetString(string(x), 0)
	if !ok {
		return fmt.Errorf("math/big: cannot unmarshal %s into a *big.Int", x)
	}
	return nil
}
