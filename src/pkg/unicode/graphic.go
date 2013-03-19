// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unicode

// Bit masks for each code point under U+0100, for fast lookup.

// 在 U+0100 以下的每个码点的位屏蔽，方便查看。
const (
	pC     = 1 << iota // a control character.     // 控制字符
	pP                 // a punctuation character. // 标点字符
	pN                 // a numeral.               // 数值字符
	pS                 // a symbolic character.    // 符号字符
	pZ                 // a spacing character.     // 空白字符
	pLu                // an upper-case letter.    // 大写字母
	pLl                // a lower-case letter.     // 小写字母
	pp                 // a printable character according to Go's definition.        // 根据Go定义的可打印字符。
	pg     = pp | pZ   // a graphical character according to the Unicode definition. // 根据Unicode定义的可显示字符。
	pLo    = pLl | pLu // a letter that is neither upper nor lower case.             // 既非大写也非小写的字母。
	pLmask = pLo
)

// GraphicRanges defines the set of graphic characters according to Unicode.

// GraphicRanges 根据Unicode定义了可显示字符的集合。
var GraphicRanges = []*RangeTable{
	L, M, N, P, S, Zs,
}

// PrintRanges defines the set of printable characters according to Go.
// ASCII space, U+0020, is handled separately.

// PrintRanges 根据Go定义了可打印字符的集合。ASCII空格（即U+0020）另作处理。
var PrintRanges = []*RangeTable{
	L, M, N, P, S,
}

// IsGraphic reports whether the rune is defined as a Graphic by Unicode.
// Such characters include letters, marks, numbers, punctuation, symbols, and
// spaces, from categories L, M, N, P, S, Zs.

// IsGraphic 报告该符文是否为Unicode定义的可显示字符。包括字母、标记、数字、
// 标点、符号和空白这样的，类别为L、M、N、P,、S和Zs的字符。
func IsGraphic(r rune) bool {
	// We convert to uint32 to avoid the extra test for negative,
	// and in the index we convert to uint8 to avoid the range check.
	// 我们将它转换为 uint32 来避免额外的负数测试，在索引中我们将它转换为 uint8
	// 来避免范围检测。
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pg != 0
	}
	return IsOneOf(GraphicRanges, r)
}

// IsPrint reports whether the rune is defined as printable by Go. Such
// characters include letters, marks, numbers, punctuation, symbols, and the
// ASCII space character, from categories L, M, N, P, S and the ASCII space
// character.  This categorization is the same as IsGraphic except that the
// only spacing character is ASCII space, U+0020.

// IsPrint 报告该符文是否为Go定义的可打印字符。包括字母、标记、数字、标点、
// 符号和ASCII空格这样的，类别为L、M、N、P、S和ASCII空格的字符。
// 除空白字符只有ASCII空格（即U+0020）外，其它的类别与 IsGraphic 相同。
func IsPrint(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pp != 0
	}
	return IsOneOf(PrintRanges, r)
}

// IsOneOf reports whether the rune is a member of one of the ranges.

// IsOneOf 报告该符文是否为该范围中的一员。
func IsOneOf(set []*RangeTable, r rune) bool {
	for _, inside := range set {
		if Is(inside, r) {
			return true
		}
	}
	return false
}

// IsControl reports whether the rune is a control character.
// The C (Other) Unicode category includes more code points
// such as surrogates; use Is(C, r) to test for them.

// IsControl 报告该字符是否为控制字符。Unicode的C（其它）
// 类别包括了更多像替代值这样的码点；请使用 Is(C, r) 来测试它们。
func IsControl(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pC != 0
	}
	// All control characters are < Latin1Max.
	// 所有的控制字符都 < Latin1Max。
	return false
}

// IsLetter reports whether the rune is a letter (category L).

// IsLetter 报告该符文是否为字母（类别L）。
func IsLetter(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&(pLmask) != 0
	}
	return isExcludingLatin(Letter, r)
}

// IsMark reports whether the rune is a mark character (category M).

// IsMark 报告该符文是否为标记字符（类别M）。
func IsMark(r rune) bool {
	// There are no mark characters in Latin-1.
	// 标记字符不在Latin-1中。
	return isExcludingLatin(Mark, r)
}

// IsNumber reports whether the rune is a number (category N).

// IsNumber 报告该符文是否为数字（类别N）。
func IsNumber(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pN != 0
	}
	return isExcludingLatin(Number, r)
}

// IsPunct reports whether the rune is a Unicode punctuation character
// (category P).

// IsPunct 报告该符文是否为Unicode标点字符（类别P）。
func IsPunct(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pP != 0
	}
	return Is(Punct, r)
}

// IsSpace reports whether the rune is a space character as defined
// by Unicode's White Space property; in the Latin-1 space
// this is
//	'\t', '\n', '\v', '\f', '\r', ' ', U+0085 (NEL), U+00A0 (NBSP).
// Other definitions of spacing characters are set by category
// Z and property Pattern_White_Space.

// IsSpace 报告该符文是否为Unicode空白字符属性定义的空白符；在Latin-1中的空白为
//	'\t'、'\n'、'\v'、'\f'、'\r'、' '、U+0085 (NEL) 和 U+00A0 (NBSP)。
// 其它空白字符的定义由类别Z和属性 Pattern_White_Space 设置。
func IsSpace(r rune) bool {
	// This property isn't the same as Z; special-case it.
	// 此属性与Z不同，它是特殊情况。
	if uint32(r) <= MaxLatin1 {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			return true
		}
		return false
	}
	return isExcludingLatin(White_Space, r)
}

// IsSymbol reports whether the rune is a symbolic character.

// IsSymbol 报告该符文是否为符号字符。
func IsSymbol(r rune) bool {
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pS != 0
	}
	return isExcludingLatin(Symbol, r)
}
