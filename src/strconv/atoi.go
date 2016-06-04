// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

import "errors"

// ErrRange indicates that a value is out of range for the target type.

// ErrRange 表示值超过了目标类型的范围。
var ErrRange = errors.New("value out of range")

// ErrSyntax indicates that a value does not have the right syntax for the target type.

// ErrSyntax 表示值不符合目标类型的正确格式。
var ErrSyntax = errors.New("invalid syntax")

// A NumError records a failed conversion.

// NumError 用来记录失败的转换。
type NumError struct {
	Func string // the failing function (ParseBool, ParseInt, ParseUint, ParseFloat)
	Num  string // the input
	Err  error  // the reason the conversion failed (ErrRange, ErrSyntax)
}

func (e *NumError) Error() string {
	return "strconv." + e.Func + ": " + "parsing " + Quote(e.Num) + ": " + e.Err.Error()
}

func syntaxError(fn, str string) *NumError {
	return &NumError{fn, str, ErrSyntax}
}

func rangeError(fn, str string) *NumError {
	return &NumError{fn, str, ErrRange}
}

const intSize = 32 << (^uint(0) >> 63)

// IntSize is the size in bits of an int or uint value.

// IntSize 是 int 或是 uint 值的位数。
const IntSize = intSize

const maxUint64 = (1<<64 - 1)

// ParseUint is like ParseInt but for unsigned numbers.

// ParseUint 类似于 ParseInt, 只不过结果是无符号数。
func ParseUint(s string, base int, bitSize int) (uint64, error) {
	var n uint64
	var err error
	var cutoff, maxVal uint64

	if bitSize == 0 {
		bitSize = int(IntSize)
	}

	i := 0
	switch {
	case len(s) < 1:
		err = ErrSyntax
		goto Error

	case 2 <= base && base <= 36:
		// valid base; nothing to do

	case base == 0:
		// Look for octal, hex prefix.
		switch {
		case s[0] == '0' && len(s) > 1 && (s[1] == 'x' || s[1] == 'X'):
			if len(s) < 3 {
				err = ErrSyntax
				goto Error
			}
			base = 16
			i = 2
		case s[0] == '0':
			base = 8
			i = 1
		default:
			base = 10
		}

	default:
		err = errors.New("invalid base " + Itoa(base))
		goto Error
	}

	// Cutoff is the smallest number such that cutoff*base > maxUint64.
	// Use compile-time constants for common cases.
	switch base {
	case 10:
		cutoff = maxUint64/10 + 1
	case 16:
		cutoff = maxUint64/16 + 1
	default:
		cutoff = maxUint64/uint64(base) + 1
	}

	maxVal = 1<<uint(bitSize) - 1

	for ; i < len(s); i++ {
		var v byte
		d := s[i]
		switch {
		case '0' <= d && d <= '9':
			v = d - '0'
		case 'a' <= d && d <= 'z':
			v = d - 'a' + 10
		case 'A' <= d && d <= 'Z':
			v = d - 'A' + 10
		default:
			n = 0
			err = ErrSyntax
			goto Error
		}
		if v >= byte(base) {
			n = 0
			err = ErrSyntax
			goto Error
		}

		if n >= cutoff {
			// n*base overflows
			n = maxUint64
			err = ErrRange
			goto Error
		}
		n *= uint64(base)

		n1 := n + uint64(v)
		if n1 < n || n1 > maxVal {
			// n+v overflows
			n = maxUint64
			err = ErrRange
			goto Error
		}
		n = n1
	}

	return n, nil

Error:
	return n, &NumError{"ParseUint", s, err}
}

// ParseInt interprets a string s in the given base (2 to 36) and
// returns the corresponding value i. If base == 0, the base is
// implied by the string's prefix: base 16 for "0x", base 8 for
// "0", and base 10 otherwise.
//
// The bitSize argument specifies the integer type
// that the result must fit into. Bit sizes 0, 8, 16, 32, and 64
// correspond to int, int8, int16, int32, and int64.
//
// The errors that ParseInt returns have concrete type *NumError
// and include err.Num = s. If s is empty or contains invalid
// digits, err.Err = ErrSyntax and the returned value is 0;
// if the value corresponding to s cannot be represented by a
// signed integer of the given size, err.Err = ErrRange and the
// returned value is the maximum magnitude integer of the
// appropriate bitSize and sign.

// ParseInt 返回 s 表示的以 base (2~36)为基数的值 i 。
// 如果 base == 0, 那么使用字符串的前缀来指定基数, 0x 打头代表基数 16, 0 打头代表基数 8, 其他则基数为 10。
//
// 参数 bitSize 指定转换结果的类型, 0, 8, 16, 32, 64 分别代表 int, int8, int16, int32 和 int 64。
//
// ParseInt 的错误是 *NumError 类型, 其中 err.Num = s。
// 如果 s 为空或是包含非法的数字, 那么 err.Err = ErrSyntax, 返回值为 0。
// 如果字符串的值不能用指定整型表示的话, 那么 err.Err = ErrRange, 返回值是与类型和符合匹配的最大值。
func ParseInt(s string, base int, bitSize int) (i int64, err error) {
	const fnParseInt = "ParseInt"

	if bitSize == 0 {
		bitSize = int(IntSize)
	}

	// Empty string bad.
	if len(s) == 0 {
		return 0, syntaxError(fnParseInt, s)
	}

	// Pick off leading sign.
	s0 := s
	neg := false
	if s[0] == '+' {
		s = s[1:]
	} else if s[0] == '-' {
		neg = true
		s = s[1:]
	}

	// Convert unsigned and check range.
	var un uint64
	un, err = ParseUint(s, base, bitSize)
	if err != nil && err.(*NumError).Err != ErrRange {
		err.(*NumError).Func = fnParseInt
		err.(*NumError).Num = s0
		return 0, err
	}
	cutoff := uint64(1 << uint(bitSize-1))
	if !neg && un >= cutoff {
		return int64(cutoff - 1), rangeError(fnParseInt, s0)
	}
	if neg && un > cutoff {
		return -int64(cutoff), rangeError(fnParseInt, s0)
	}
	n := int64(un)
	if neg {
		n = -n
	}
	return n, nil
}

// Atoi is shorthand for ParseInt(s, 10, 0).

// Atoi 是 ParseInt(s, 10, 0) 的简写。
func Atoi(s string) (int, error) {
	i64, err := ParseInt(s, 10, 0)
	return int(i64), err
}
