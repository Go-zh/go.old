// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package utf8 implements functions and constants to support text encoded in
// UTF-8. It includes functions to translate between runes and UTF-8 byte sequences.

// utf8 包实现了支持UTF-8文本编码的函数和常量.
// 其中包括了在符文和UTF-8字节序列之间进行转译的函数。
package utf8

// The conditions RuneError==unicode.ReplacementChar and
// MaxRune==unicode.MaxRune are verified in the tests.
// Defining them locally avoids this package depending on package unicode.
//
// 条件 RuneError==unicode.ReplacementChar 和 MaxRune==unicode.MaxRune
// 已在测试中验证。局部地定义它们是为了避免对 unicode 包的依赖。

// Numbers fundamental to the encoding.

// 用于编码的基本数值。
const (
	RuneError = '\uFFFD'     // the "error" Rune or "Unicode replacement character" // “错误”符文或“Unicode替换字符”
	RuneSelf  = 0x80         // characters below Runeself are represented as themselves in a single byte. // RuneSelf 值以下的字符以单个字节表示其自身。
	MaxRune   = '\U0010FFFF' // Maximum valid Unicode code point. // 最大的有效Unicode码点。
	UTFMax    = 4            // maximum number of bytes of a UTF-8 encoded Unicode character. // UTF-8编码的Unicode字符的最大字节数。
)

// Code points in the surrogate range are not valid for UTF-8.

// 替代范围内的码点是无效的UTF-8编码。
const (
	surrogateMin = 0xD800
	surrogateMax = 0xDFFF
)

const (
	t1 = 0x00 // 0000 0000
	tx = 0x80 // 1000 0000
	t2 = 0xC0 // 1100 0000
	t3 = 0xE0 // 1110 0000
	t4 = 0xF0 // 1111 0000
	t5 = 0xF8 // 1111 1000

	maskx = 0x3F // 0011 1111
	mask2 = 0x1F // 0001 1111
	mask3 = 0x0F // 0000 1111
	mask4 = 0x07 // 0000 0111

	rune1Max = 1<<7 - 1
	rune2Max = 1<<11 - 1
	rune3Max = 1<<16 - 1
)

func decodeRuneInternal(p []byte) (r rune, size int, short bool) {
	n := len(p)
	if n < 1 {
		return RuneError, 0, true
	}
	c0 := p[0]

	// 1-byte, 7-bit sequence?
	// 是否为1字节，7位的序列？
	if c0 < tx {
		return rune(c0), 1, false
	}

	// unexpected continuation byte?
	// 是否为意外的延续字节？
	if c0 < t2 {
		return RuneError, 1, false
	}

	// need first continuation byte
	// 需要第一个延续字节
	if n < 2 {
		return RuneError, 1, true
	}
	c1 := p[1]
	if c1 < tx || t2 <= c1 {
		return RuneError, 1, false
	}

	// 2-byte, 11-bit sequence?
	// 是否为2字节，11位的序列？
	if c0 < t3 {
		r = rune(c0&mask2)<<6 | rune(c1&maskx)
		if r <= rune1Max {
			return RuneError, 1, false
		}
		return r, 2, false
	}

	// need second continuation byte
	// 需要第二个延续字节
	if n < 3 {
		return RuneError, 1, true
	}
	c2 := p[2]
	if c2 < tx || t2 <= c2 {
		return RuneError, 1, false
	}

	// 3-byte, 16-bit sequence?
	// 是否为3字节，16位的序列？
	if c0 < t4 {
		r = rune(c0&mask3)<<12 | rune(c1&maskx)<<6 | rune(c2&maskx)
		if r <= rune2Max {
			return RuneError, 1, false
		}
		if surrogateMin <= r && r <= surrogateMax {
			return RuneError, 1, false
		}
		return r, 3, false
	}

	// need third continuation byte
	// 需要第三个延续字节
	if n < 4 {
		return RuneError, 1, true
	}
	c3 := p[3]
	if c3 < tx || t2 <= c3 {
		return RuneError, 1, false
	}

	// 4-byte, 21-bit sequence?
	// 是否为4字节，21位的序列？
	if c0 < t5 {
		r = rune(c0&mask4)<<18 | rune(c1&maskx)<<12 | rune(c2&maskx)<<6 | rune(c3&maskx)
		if r <= rune3Max || MaxRune < r {
			return RuneError, 1, false
		}
		return r, 4, false
	}

	// error
	// 错误
	return RuneError, 1, false
}

func decodeRuneInStringInternal(s string) (r rune, size int, short bool) {
	n := len(s)
	if n < 1 {
		return RuneError, 0, true
	}
	c0 := s[0]

	// 1-byte, 7-bit sequence?
	// 是否为1字节，7位的序列？
	if c0 < tx {
		return rune(c0), 1, false
	}

	// unexpected continuation byte?
	// 是否为意外的延续字节？
	if c0 < t2 {
		return RuneError, 1, false
	}

	// need first continuation byte
	// 需要第一个延续字节
	if n < 2 {
		return RuneError, 1, true
	}
	c1 := s[1]
	if c1 < tx || t2 <= c1 {
		return RuneError, 1, false
	}

	// 2-byte, 11-bit sequence?
	// 是否为2字节，11位的序列？
	if c0 < t3 {
		r = rune(c0&mask2)<<6 | rune(c1&maskx)
		if r <= rune1Max {
			return RuneError, 1, false
		}
		return r, 2, false
	}

	// need second continuation byte
	// 需要第二个延续字节
	if n < 3 {
		return RuneError, 1, true
	}
	c2 := s[2]
	if c2 < tx || t2 <= c2 {
		return RuneError, 1, false
	}

	// 3-byte, 16-bit sequence?
	// 是否为3字节，16位的序列？
	if c0 < t4 {
		r = rune(c0&mask3)<<12 | rune(c1&maskx)<<6 | rune(c2&maskx)
		if r <= rune2Max {
			return RuneError, 1, false
		}
		if surrogateMin <= r && r <= surrogateMax {
			return RuneError, 1, false
		}
		return r, 3, false
	}

	// need third continuation byte
	// 需要第三个延续字节
	if n < 4 {
		return RuneError, 1, true
	}
	c3 := s[3]
	if c3 < tx || t2 <= c3 {
		return RuneError, 1, false
	}

	// 4-byte, 21-bit sequence?
	// 是否为4字节，21位的序列？
	if c0 < t5 {
		r = rune(c0&mask4)<<18 | rune(c1&maskx)<<12 | rune(c2&maskx)<<6 | rune(c3&maskx)
		if r <= rune3Max || MaxRune < r {
			return RuneError, 1, false
		}
		return r, 4, false
	}

	// error
	// 错误
	return RuneError, 1, false
}

// FullRune reports whether the bytes in p begin with a full UTF-8 encoding of a rune.
// An invalid encoding is considered a full Rune since it will convert as a width-1 error rune.

// FullRune 报告 p 中的字节是否以全UTF-8编码的符文开始。
// 无效的编码取被视作一个完整的符文，因为它会转换成宽度为1的错误符文。
func FullRune(p []byte) bool {
	_, _, short := decodeRuneInternal(p)
	return !short
}

// FullRuneInString is like FullRune but its input is a string.

// FullRuneInString 类似于 FullRune，但其输入为字符串。
func FullRuneInString(s string) bool {
	_, _, short := decodeRuneInStringInternal(s)
	return !short
}

// DecodeRune unpacks the first UTF-8 encoding in p and returns the rune and its width in bytes.
// If the encoding is invalid, it returns (RuneError, 1), an impossible result for correct UTF-8.
// An encoding is invalid if it is incorrect UTF-8, encodes a rune that is
// out of range, or is not the shortest possible UTF-8 encoding for the
// value. No other validation is performed.

// DecodeRune 解包 p 中的第一个UTF-8编码，并返回该符文及其字节宽度。
// 若此编码无效，它就会返回 (RuneError, 1)，即一个对于正确的UTF-8来说不可能的值。
// 若一个编码为错误的UTF-8值，或该符文的编码超出范围，或不是该值可能的最短UTF-8编码，
// 那么它就是无效的。除此之外，并不进行其它的验证。
func DecodeRune(p []byte) (r rune, size int) {
	r, size, _ = decodeRuneInternal(p)
	return
}

// DecodeRuneInString is like DecodeRune but its input is a string.
// If the encoding is invalid, it returns (RuneError, 1), an impossible result for correct UTF-8.
// An encoding is invalid if it is incorrect UTF-8, encodes a rune that is
// out of range, or is not the shortest possible UTF-8 encoding for the
// value. No other validation is performed.

// DecodeRuneInString 类似于 DecodeRune，但其输入为字符串。
// 若此编码无效，它就会返回 (RuneError, 1)，即一个对于正确的UTF-8来说不可能的值。
// 若一个编码为错误的UTF-8值，或该符文的编码超出范围，或不是该值可能的最短UTF-8编码，
// 那么它就是无效的。除此之外，并不进行其它的验证。
func DecodeRuneInString(s string) (r rune, size int) {
	r, size, _ = decodeRuneInStringInternal(s)
	return
}

// DecodeLastRune unpacks the last UTF-8 encoding in p and returns the rune and its width in bytes.
// If the encoding is invalid, it returns (RuneError, 1), an impossible result for correct UTF-8.
// An encoding is invalid if it is incorrect UTF-8, encodes a rune that is
// out of range, or is not the shortest possible UTF-8 encoding for the
// value. No other validation is performed.

// DecodeLastRune 解包 p 中的最后一个UTF-8编码，并返回该符文及其字节宽度。
// 若此编码无效，它就会返回 (RuneError, 1)，即一个对于正确的UTF-8来说不可能的值。
// 若一个编码为错误的UTF-8值，或该符文的编码超出范围，或不是该值可能的最短UTF-8编码，
// 那么它就是无效的。除此之外，并不进行其它的验证。
func DecodeLastRune(p []byte) (r rune, size int) {
	end := len(p)
	if end == 0 {
		return RuneError, 0
	}
	start := end - 1
	r = rune(p[start])
	if r < RuneSelf {
		return r, 1
	}
	// guard against O(n^2) behavior when traversing
	// backwards through strings with long sequences of
	// invalid UTF-8.
	//
	// 当向前遍历一长串无效的UTF-8字符串时，防止复杂度为 O(n^2) 的行为。
	lim := end - UTFMax
	if lim < 0 {
		lim = 0
	}
	for start--; start >= lim; start-- {
		if RuneStart(p[start]) {
			break
		}
	}
	if start < 0 {
		start = 0
	}
	r, size = DecodeRune(p[start:end])
	if start+size != end {
		return RuneError, 1
	}
	return r, size
}

// DecodeLastRuneInString is like DecodeLastRune but its input is a string.
// If the encoding is invalid, it returns (RuneError, 1), an impossible result for correct UTF-8.
// An encoding is invalid if it is incorrect UTF-8, encodes a rune that is
// out of range, or is not the shortest possible UTF-8 encoding for the
// value. No other validation is performed.

// DecodeLastRuneInString 类似于 DecodeLastRune，但其输入为字符串。
// 若此编码无效，它就会返回 (RuneError, 1)，即一个对于正确的UTF-8来说不可能的值。
// 若一个编码为错误的UTF-8值，或该符文的编码超出范围，或不是该值可能的最短UTF-8编码，
// 那么它就是无效的。除此之外，并不进行其它的验证。
func DecodeLastRuneInString(s string) (r rune, size int) {
	end := len(s)
	if end == 0 {
		return RuneError, 0
	}
	start := end - 1
	r = rune(s[start])
	if r < RuneSelf {
		return r, 1
	}
	// guard against O(n^2) behavior when traversing
	// backwards through strings with long sequences of
	// invalid UTF-8.
	//
	// 当向前遍历一长串无效的UTF-8字符串时，防止复杂度为 O(n^2) 的行为。
	lim := end - UTFMax
	if lim < 0 {
		lim = 0
	}
	for start--; start >= lim; start-- {
		if RuneStart(s[start]) {
			break
		}
	}
	if start < 0 {
		start = 0
	}
	r, size = DecodeRuneInString(s[start:end])
	if start+size != end {
		return RuneError, 1
	}
	return r, size
}

// RuneLen returns the number of bytes required to encode the rune.
// It returns -1 if the rune is not a valid value to encode in UTF-8.

// RuneLen 返回编码该符文所需的字节数。
// 若该符文并非有效的UTF-8编码值，就返回 -1。
func RuneLen(r rune) int {
	switch {
	case r < 0:
		return -1
	case r <= rune1Max:
		return 1
	case r <= rune2Max:
		return 2
	case surrogateMin <= r && r <= surrogateMax:
		return -1
	case r <= rune3Max:
		return 3
	case r <= MaxRune:
		return 4
	}
	return -1
}

// EncodeRune writes into p (which must be large enough) the UTF-8 encoding of the rune.
// It returns the number of bytes written.

// EncodeRune 将该符文的UTF-8编码写入到 p 中（它必须足够大）。
// 它返回写入的字节数。
func EncodeRune(p []byte, r rune) int {
	// Negative values are erroneous.  Making it unsigned addresses the problem.
	// 负值是错误的。将它变成无符号数值来解决此问题。
	if uint32(r) <= rune1Max {
		p[0] = byte(r)
		return 1
	}

	if uint32(r) <= rune2Max {
		p[0] = t2 | byte(r>>6)
		p[1] = tx | byte(r)&maskx
		return 2
	}

	if uint32(r) > MaxRune {
		r = RuneError
	}

	if surrogateMin <= r && r <= surrogateMax {
		r = RuneError
	}

	if uint32(r) <= rune3Max {
		p[0] = t3 | byte(r>>12)
		p[1] = tx | byte(r>>6)&maskx
		p[2] = tx | byte(r)&maskx
		return 3
	}

	p[0] = t4 | byte(r>>18)
	p[1] = tx | byte(r>>12)&maskx
	p[2] = tx | byte(r>>6)&maskx
	p[3] = tx | byte(r)&maskx
	return 4
}

// RuneCount returns the number of runes in p.  Erroneous and short
// encodings are treated as single runes of width 1 byte.

// RuneCount 返回 p 中的符文数。
// 错误编码和短编码将被视作宽度为1字节的单个符文。
func RuneCount(p []byte) int {
	i := 0
	var n int
	for n = 0; i < len(p); n++ {
		if p[i] < RuneSelf {
			i++
		} else {
			_, size := DecodeRune(p[i:])
			i += size
		}
	}
	return n
}

// RuneCountInString is like RuneCount but its input is a string.

// RuneCountInString 类似于 RuneCount，但其输入为字符串。
func RuneCountInString(s string) (n int) {
	for _ = range s {
		n++
	}
	return
}

// RuneStart reports whether the byte could be the first byte of
// an encoded rune.  Second and subsequent bytes always have the top
// two bits set to 10.

// RuneStart 报告该字节是否为符文编码的第一个字节。
// 第二个及后续字节的最高两位总是置为 10。
func RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// Valid reports whether p consists entirely of valid UTF-8-encoded runes.

// Valid 报告 p 是否完全由有效的，UTF-8编码的符文构成。
func Valid(p []byte) bool {
	i := 0
	for i < len(p) {
		if p[i] < RuneSelf {
			i++
		} else {
			_, size := DecodeRune(p[i:])
			if size == 1 {
				// All valid runes of size 1 (those
				// below RuneSelf) were handled above.
				// This must be a RuneError.
				//
				// 上面处理了所有大小为 1 的（即小于 RuneSelf
				// 的）有效符文。这肯定是一个 RuneError。
				return false
			}
			i += size
		}
	}
	return true
}

// ValidString reports whether s consists entirely of valid UTF-8-encoded runes.

// ValidString 报告 s 是否完全由有效的，UTF-8编码的符文构成。
func ValidString(s string) bool {
	for i, r := range s {
		if r == RuneError {
			// The RuneError value can be an error
			// sentinel value (if it's size 1) or the same
			// value encoded properly. Decode it to see if
			// it's the 1 byte sentinel value.
			//
			// RuneError 值可作为错误哨兵值（如果其大小为 1）
			// 或正确编码的相同值。通过解码来查看其是否为
			// 1 字节的哨兵值。
			_, size := DecodeRuneInString(s[i:])
			if size == 1 {
				return false
			}
		}
	}
	return true
}

// ValidRune reports whether r can be legally encoded as UTF-8.
// Code points that are out of range or a surrogate half are illegal.

// ValidRune 报告 r 是否能合法地作为UTF-8编码。
// 超出返回或半替代值的码点是非法的。
func ValidRune(r rune) bool {
	switch {
	case r < 0:
		return false
	case surrogateMin <= r && r <= surrogateMax:
		return false
	case r > MaxRune:
		return false
	}
	return true
}
