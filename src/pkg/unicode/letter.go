// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package unicode provides data and functions to test some properties of
// Unicode code points.

// unicode 包提供了一些测试Unicode码点属性的数据和函数.
package unicode

const (
	MaxRune         = '\U0010FFFF' // Maximum valid Unicode code point. // Unicode码点的最大值
	ReplacementChar = '\uFFFD'     // Represents invalid code points.   // 无效码点的表示
	MaxASCII        = '\u007F'     // maximum ASCII value.              // ASCII的最大值
	MaxLatin1       = '\u00FF'     // maximum Latin-1 value.            // Latin-1的最大值
)

// RangeTable defines a set of Unicode code points by listing the ranges of
// code points within the set. The ranges are listed in two slices
// to save space: a slice of 16-bit ranges and a slice of 32-bit ranges.
// The two slices must be in sorted order and non-overlapping.
// Also, R32 should contain only values >= 0x10000 (1<<16).

// RangeTable 通过列出码点范围，定义了Unicode码点的集合。为了节省空间，
// 其范围分别在16位、32位这两个切片中列出。这两个切片必须已经排序且无重叠的部分。
// 此外，R32只包含 >= 0x10000 (1<<16) 的值。
type RangeTable struct {
	R16         []Range16
	R32         []Range32
	LatinOffset int // number of entries in R16 with Hi <= MaxLatin1 // R16 中满足 Hi <= MaxLatin1 的条目数
}

// Range16 represents of a range of 16-bit Unicode code points.  The range runs from Lo to Hi
// inclusive and has the specified stride.

// Range16 表示16位Unicode码点的范围。该范围从 Lo 连续到 Hi 且包括两端，
// 还有一个指定的间距。
type Range16 struct {
	Lo     uint16
	Hi     uint16
	Stride uint16
}

// Range32 represents of a range of Unicode code points and is used when one or
// more of the values will not fit in 16 bits.  The range runs from Lo to Hi
// inclusive and has the specified stride. Lo and Hi must always be >= 1<<16.

// Range32 表示Unicode码点的范围，它在一个或多个值不能用16位容纳时使用。该范围从
// Lo 连续到 Hi 且包括两端，还有一个指定的间距。Lo 和 Hi 都必须满足 >= 1<<16。
type Range32 struct {
	Lo     uint32
	Hi     uint32
	Stride uint32
}

// CaseRange represents a range of Unicode code points for simple (one
// code point to one code point) case conversion.
// The range runs from Lo to Hi inclusive, with a fixed stride of 1.  Deltas
// are the number to add to the code point to reach the code point for a
// different case for that character.  They may be negative.  If zero, it
// means the character is in the corresponding case. There is a special
// case representing sequences of alternating corresponding Upper and Lower
// pairs.  It appears with a fixed Delta of
//	{UpperLower, UpperLower, UpperLower}
// The constant UpperLower has an otherwise impossible delta value.

// CaseRange 表示Unicode码点中，简单的（即一对一的）大小写转换的范围。该范围从
// Lo 连续到 Hi，包括一个固定的间距。Delta 为添加的码点数量，
// 以便于该字符不同写法间的转换。它们可为负数。若为零，即表示该字符的写法一致。
// 还有种特殊的写法，表示一对大小写交替对应的序列。它会与像
// 	{UpperLower, UpperLower, UpperLower}
// 这样固定的 Delta 一同出现。常量 UpperLower 可能拥有其它的 delta 值。
type CaseRange struct {
	Lo    uint32
	Hi    uint32
	Delta d
}

// SpecialCase represents language-specific case mappings such as Turkish.
// Methods of SpecialCase customize (by overriding) the standard mappings.

// SpecialCase 表示语言相关的写法映射，例如土耳其语。SpecialCase 的方法（通过覆盖）
// 来定制标准的映射。
type SpecialCase []CaseRange

// BUG(r): There is no mechanism for full case folding, that is, for
// characters that involve multiple runes in the input or output.

// BUG(r): 现在还没有完整的写法转换机制。具体来说，就是对于涉及到多个符文的字符，
// 其输入或输出的转换机制并不完整。

// Indices into the Delta arrays inside CaseRanges for case mapping.

// CaseRange 中 Delta 数组的下标，以用于写法映射。
const (
	UpperCase = iota
	LowerCase
	TitleCase
	MaxCase
)

type d [MaxCase]rune // to make the CaseRanges text shorter // 让 CaseRange 短一点。

// If the Delta field of a CaseRange is UpperLower or LowerUpper, it means
// this CaseRange represents a sequence of the form (say)
// Upper Lower Upper Lower.

// 若 CaseRange 的 Delta 字段为 UpperLower 或 LowerUpper，则该 CaseRange 即表示
// （所谓的）“Upper Lower Upper Lower”序列。
const (
	UpperLower = MaxRune + 1 // (Cannot be a valid delta.) // 不能算有效的 delta。
)

// linearMax is the maximum size table for linear search for non-Latin1 rune.
// Derived by running 'go test -calibrate'.

// linearMax 是对非Latin1符文进行线性查找的的最大表。
const linearMax = 18

// is16 reports whether r is in the sorted slice of 16-bit ranges.

// is16 报告该符文是否在16位范围内的已排序切片中。
func is16(ranges []Range16, r uint16) bool {
	if len(ranges) <= linearMax || r <= MaxLatin1 {
		for i := range ranges {
			range_ := &ranges[i]
			if r < range_.Lo {
				return false
			}
			if r <= range_.Hi {
				return (r-range_.Lo)%range_.Stride == 0
			}
		}
		return false
	}

	// binary search over ranges
	// 对整个范围进行二分查找
	lo := 0
	hi := len(ranges)
	for lo < hi {
		m := lo + (hi-lo)/2
		range_ := &ranges[m]
		if range_.Lo <= r && r <= range_.Hi {
			return (r-range_.Lo)%range_.Stride == 0
		}
		if r < range_.Lo {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return false
}

// is32 reports whether r is in the sorted slice of 32-bit ranges.

// is32 报告该符文是否在32位范围内的已排序切片中。
func is32(ranges []Range32, r uint32) bool {
	if len(ranges) <= linearMax {
		for i := range ranges {
			range_ := &ranges[i]
			if r < range_.Lo {
				return false
			}
			if r <= range_.Hi {
				return (r-range_.Lo)%range_.Stride == 0
			}
		}
		return false
	}

	// binary search over ranges
	// 对整个范围进行二分查找
	lo := 0
	hi := len(ranges)
	for lo < hi {
		m := lo + (hi-lo)/2
		range_ := ranges[m]
		if range_.Lo <= r && r <= range_.Hi {
			return (r-range_.Lo)%range_.Stride == 0
		}
		if r < range_.Lo {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return false
}

// Is tests whether rune is in the specified table of ranges.

// Is 报告该符文是否在指定范围的表中。
func Is(rangeTab *RangeTable, r rune) bool {
	r16 := rangeTab.R16
	if len(r16) > 0 && r <= rune(r16[len(r16)-1].Hi) {
		return is16(r16, uint16(r))
	}
	r32 := rangeTab.R32
	if len(r32) > 0 && r >= rune(r32[0].Lo) {
		return is32(r32, uint32(r))
	}
	return false
}

func isExcludingLatin(rangeTab *RangeTable, r rune) bool {
	r16 := rangeTab.R16
	if off := rangeTab.LatinOffset; len(r16) > off && r <= rune(r16[len(r16)-1].Hi) {
		return is16(r16[off:], uint16(r))
	}
	r32 := rangeTab.R32
	if len(r32) > 0 && r >= rune(r32[0].Lo) {
		return is32(r32, uint32(r))
	}
	return false
}

// IsUpper reports whether the rune is an upper case letter.

// IsUpper 报告该符文是否为大写字母。
func IsUpper(r rune) bool {
	// See comment in IsGraphic.
	// 见 IsGraphic 的注释。
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pLmask == pLu
	}
	return isExcludingLatin(Upper, r)
}

// IsLower reports whether the rune is a lower case letter.

// IsLower 报告该符文是否为小写字母。
func IsLower(r rune) bool {
	// See comment in IsGraphic.
	if uint32(r) <= MaxLatin1 {
		return properties[uint8(r)]&pLmask == pLl
	}
	return isExcludingLatin(Lower, r)
}

// IsTitle reports whether the rune is a title case letter.

// IsTitle 报告该符文是否为标题字母。
func IsTitle(r rune) bool {
	if r <= MaxLatin1 {
		return false
	}
	return isExcludingLatin(Title, r)
}

// to maps the rune using the specified case mapping.

// to 通过指定的写法映射表来映射该符文。
func to(_case int, r rune, caseRange []CaseRange) rune {
	if _case < 0 || MaxCase <= _case {
		return ReplacementChar // as reasonable an error as any // 对任何字符来说都是个合理的错误
	}
	// binary search over ranges
	// 对整个范围进行二分查找
	lo := 0
	hi := len(caseRange)
	for lo < hi {
		m := lo + (hi-lo)/2
		cr := caseRange[m]
		if rune(cr.Lo) <= r && r <= rune(cr.Hi) {
			delta := rune(cr.Delta[_case])
			if delta > MaxRune {
				// In an Upper-Lower sequence, which always starts with
				// an UpperCase letter, the real deltas always look like:
				//	{0, 1, 0}    UpperCase (Lower is next)
				//	{-1, 0, -1}  LowerCase (Upper, Title are previous)
				// The characters at even offsets from the beginning of the
				// sequence are upper case; the ones at odd offsets are lower.
				// The correct mapping can be done by clearing or setting the low
				// bit in the sequence offset.
				// The constants UpperCase and TitleCase are even while LowerCase
				// is odd so we take the low bit from _case.
				//
				// 在一个“大写-小写”序列中，它总是以大写字母开头，真正的 delta
				// 形式总是这样的：
				//	{0, 1, 0}    UpperCase（下一个为小写字母）
				//	{-1, 0, -1}  LowerCase（上一个为大写、标题字母）
				// 从序列起始处开始，字符偏移量为偶数的是大写；偏移量为奇数的是小写。
				// 正确的映射表可通过在该序列的偏移量中清除或设置低位来得到。
				// 当 LowerCase 为奇数时，常量 UpperCase 和 TitleCase 为偶数，
				// 因此我们可以从 _case 中得到低位。
				return rune(cr.Lo) + ((r-rune(cr.Lo))&^1 | rune(_case&1))
			}
			return r + delta
		}
		if r < rune(cr.Lo) {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return r
}

// To maps the rune to the specified case: UpperCase, LowerCase, or TitleCase.

// To 将该符文映射为指定的写法：UpperCase、LowerCase、或 TitleCase。
func To(_case int, r rune) rune {
	return to(_case, r, CaseRanges)
}

// ToUpper maps the rune to upper case.

// ToUpper 将该符文映射为大写形式。
func ToUpper(r rune) rune {
	if r <= MaxASCII {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		return r
	}
	return To(UpperCase, r)
}

// ToLower maps the rune to lower case.

// ToUpper 将该符文映射为小写形式。
func ToLower(r rune) rune {
	if r <= MaxASCII {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		return r
	}
	return To(LowerCase, r)
}

// ToTitle maps the rune to title case.

// ToTitle 将该符文映射为标题形式。
func ToTitle(r rune) rune {
	if r <= MaxASCII {
		// 对于ASCII来说，标题形式即为大写形式
		if 'a' <= r && r <= 'z' { // title case is upper case for ASCII
			r -= 'a' - 'A'
		}
		return r
	}
	return To(TitleCase, r)
}

// ToUpper maps the rune to upper case giving priority to the special mapping.

// ToUpper 将该符文映射为大写形式，优先考虑特殊的映射。
func (special SpecialCase) ToUpper(r rune) rune {
	r1 := to(UpperCase, r, []CaseRange(special))
	if r1 == r {
		r1 = ToUpper(r)
	}
	return r1
}

// ToTitle maps the rune to title case giving priority to the special mapping.

// ToTitle 将该符文映射为标题形式，优先考虑特殊的映射。
func (special SpecialCase) ToTitle(r rune) rune {
	r1 := to(TitleCase, r, []CaseRange(special))
	if r1 == r {
		r1 = ToTitle(r)
	}
	return r1
}

// ToLower maps the rune to lower case giving priority to the special mapping.

// ToLower 将该符文映射为大写形式，优先考虑特殊的映射。
func (special SpecialCase) ToLower(r rune) rune {
	r1 := to(LowerCase, r, []CaseRange(special))
	if r1 == r {
		r1 = ToLower(r)
	}
	return r1
}

// caseOrbit is defined in tables.go as []foldPair.  Right now all the
// entries fit in uint16, so use uint16.  If that changes, compilation
// will fail (the constants in the composite literal will not fit in uint16)
// and the types here can change to uint32.

// caseOrbit 作为 []foldPair 在 tables.go 中定义。目前所有的条目都符合 uint16，
// 因此使用了 uint16。若这种情况发生了改变，编译就会失败（即复合字面中的常量不符合
// uint16），而这里的类型就必须改成 uint32。
type foldPair struct {
	From uint16
	To   uint16
}

// SimpleFold iterates over Unicode code points equivalent under
// the Unicode-defined simple case folding.  Among the code points
// equivalent to rune (including rune itself), SimpleFold returns the
// smallest rune >= r if one exists, or else the smallest rune >= 0.
//
// For example:
//	SimpleFold('A') = 'a'
//	SimpleFold('a') = 'A'
//
//	SimpleFold('K') = 'k'
//	SimpleFold('k') = '\u212A' (Kelvin symbol, K)
//	SimpleFold('\u212A') = 'K'
//
//	SimpleFold('1') = '1'
//

// SimpleFold 遍历Unicode码点，等价于Unicode定义下的简单写法转换。
// 其中的码点等价于符文（包括符文自身），若存在最小的 >= r 的符文，SimpleFold
// 返回就会返回它，否则就会返回最小的 >= 0 的符文。
//
// 例如：
//	SimpleFold('A') = 'a'
//	SimpleFold('a') = 'A'
//
//	SimpleFold('K') = 'k'
//	SimpleFold('k') = '\u212A' （开尔文符号，K)
//	SimpleFold('\u212A') = 'K'
//
//	SimpleFold('1') = '1'
//
func SimpleFold(r rune) rune {
	// Consult caseOrbit table for special cases.
	// 为特殊的写法查询 caseOrbit 表。
	lo := 0
	hi := len(caseOrbit)
	for lo < hi {
		m := lo + (hi-lo)/2
		if rune(caseOrbit[m].From) < r {
			lo = m + 1
		} else {
			hi = m
		}
	}
	if lo < len(caseOrbit) && rune(caseOrbit[lo].From) == r {
		return rune(caseOrbit[lo].To)
	}

	// No folding specified.  This is a one- or two-element
	// equivalence class containing rune and ToLower(rune)
	// and ToUpper(rune) if they are different from rune.
	//
	// 没有指定的转换。这是一种有一个或两个元素与原符文等价的类型，若该符文的
	// ToLower(rune) 和 ToUpper(rune) 与原符文不同，则三者均包含。
	if l := ToLower(r); l != r {
		return l
	}
	return ToUpper(r)
}
