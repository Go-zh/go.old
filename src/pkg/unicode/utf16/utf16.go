// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package utf16 implements encoding and decoding of UTF-16 sequences.

// utf16 包实现了对UTF-16序列的编码和解码。
package utf16

// The conditions replacementChar==unicode.ReplacementChar and
// maxRune==unicode.MaxRune are verified in the tests.
// Defining them locally avoids this package depending on package unicode.
//
// 条件 RuneError==unicode.ReplacementChar 和 MaxRune==unicode.MaxRune
// 已在测试中验证。局部地定义它们是为了避免对 unicode 包的依赖。

const (
	replacementChar = '\uFFFD'     // Unicode replacement character // Unicode替换字符
	maxRune         = '\U0010FFFF' // Maximum valid Unicode code point. // 最大的有效Unicode码点。
)

const (
	// 0xd800-0xdc00 encodes the high 10 bits of a pair.
	// 0xdc00-0xe000 encodes the low 10 bits of a pair.
	// the value is those 20 bits plus 0x10000.
	//
	// 0xd800-0xdc00 编码了一对值的高10位。
	// 0xdc00-0xe000 编码了一对值的低10位。
	// 其值为这20位加上 0x10000。
	surr1 = 0xd800
	surr2 = 0xdc00
	surr3 = 0xe000

	surrSelf = 0x10000
)

// IsSurrogate returns true if the specified Unicode code point
// can appear in a surrogate pair.

// IsSurrogate 在指定的Unicode码点可出现在替代值对中时返回 true。
func IsSurrogate(r rune) bool {
	return surr1 <= r && r < surr3
}

// DecodeRune returns the UTF-16 decoding of a surrogate pair.
// If the pair is not a valid UTF-16 surrogate pair, DecodeRune returns
// the Unicode replacement code point U+FFFD.

// DecodeRune 返回替代值对的UTF-16解码。
// 若该值对并非有效的UTF-16替代值对，DecodeRune 就会返回Unicode的替换码点U+FFFD。
func DecodeRune(r1, r2 rune) rune {
	if surr1 <= r1 && r1 < surr2 && surr2 <= r2 && r2 < surr3 {
		return (rune(r1)-surr1)<<10 | (rune(r2) - surr2) + 0x10000
	}
	return replacementChar
}

// EncodeRune returns the UTF-16 surrogate pair r1, r2 for the given rune.
// If the rune is not a valid Unicode code point or does not need encoding,
// EncodeRune returns U+FFFD, U+FFFD.

// EncodeRune 返回给定符文的UTF-16替代值对 r1, r2。
// 若该符文并非有效的Unicode码点或无需编码，EncodeRune 就会返回 U+FFFD, U+FFFD。
func EncodeRune(r rune) (r1, r2 rune) {
	if r < surrSelf || r > maxRune || IsSurrogate(r) {
		return replacementChar, replacementChar
	}
	r -= surrSelf
	return surr1 + (r>>10)&0x3ff, surr2 + r&0x3ff
}

// Encode returns the UTF-16 encoding of the Unicode code point sequence s.

// Encode 返回Unicode码点序列 s 的UTF-16编码。
func Encode(s []rune) []uint16 {
	n := len(s)
	for _, v := range s {
		if v >= surrSelf {
			n++
		}
	}

	a := make([]uint16, n)
	n = 0
	for _, v := range s {
		switch {
		case v < 0, surr1 <= v && v < surr3, v > maxRune:
			v = replacementChar
			fallthrough
		case v < surrSelf:
			a[n] = uint16(v)
			n++
		default:
			r1, r2 := EncodeRune(v)
			a[n] = uint16(r1)
			a[n+1] = uint16(r2)
			n += 2
		}
	}
	return a[0:n]
}

// Decode returns the Unicode code point sequence represented
// by the UTF-16 encoding s.

// Decode 返回由UTF-16编码 s 所表示的Unicode码点序列。
func Decode(s []uint16) []rune {
	a := make([]rune, len(s))
	n := 0
	for i := 0; i < len(s); i++ {
		switch r := s[i]; {
		case surr1 <= r && r < surr2 && i+1 < len(s) &&
			surr2 <= s[i+1] && s[i+1] < surr3:
			// valid surrogate sequence
			// 有效的替代序列
			a[n] = DecodeRune(rune(r), rune(s[i+1]))
			i++
			n++
		case surr1 <= r && r < surr3:
			// invalid surrogate sequence
			// 无效的替代序列
			a[n] = replacementChar
			n++
		default:
			// normal rune
			// 一般符文
			a[n] = rune(r)
			n++
		}
	}
	return a[0:n]
}
