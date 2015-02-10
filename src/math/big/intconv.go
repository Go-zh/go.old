// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements int-to-string conversion functions.
// 本文件实现了 int 到 string 的转换函数。

package big

import (
	"errors"
	"fmt"
	"io"
)

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
func (z *Int) scan(r io.ByteScanner, base int) (*Int, int, error) {
	// determine sign
	// 确定符号
	neg, err := scanSign(r)
	if err != nil {
		return nil, 0, err
	}

	// determine mantissa
	// 确定尾数
	z.abs, base, _, err = z.abs.scan(r, base, false)
	if err != nil {
		return nil, base, err
	}
	// 0 没有符号
	z.neg = len(z.abs) > 0 && neg // 0 has no sign

	return z, base, nil
}

func scanSign(r io.ByteScanner) (neg bool, err error) {
	var ch byte
	if ch, err = r.ReadByte(); err != nil {
		return false, err
	}
	switch ch {
	case '-':
		neg = true
	case '+':
		// nothing to do
		// 啥也不做
	default:
		r.UnreadByte()
	}
	return
}

// byteReader is a local wrapper around fmt.ScanState;
// it implements the ByteReader interface.

// byteReader 是对 fmt.ScanState 的局部封装，它实现了 ByteReader 接口
type byteReader struct {
	fmt.ScanState
}

func (r byteReader) ReadByte() (byte, error) {
	ch, size, err := r.ReadRune()
	if size != 1 && err == nil {
		err = fmt.Errorf("invalid rune %#U", ch)
	}
	return byte(ch), err
}

func (r byteReader) UnreadByte() error {
	return r.UnreadRune()
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
		// 通过扫描确定进制
	default:
		return errors.New("Int.Scan: invalid verb")
	}
	_, _, err := z.scan(byteReader{s}, base)
	return err
}
