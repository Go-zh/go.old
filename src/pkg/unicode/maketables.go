// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// Unicode table generator.
// Data read from the web.

// Unicode 列表生成器。数据读取自Web。

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func main() {
	flag.Parse()
	loadChars() // always needed // 必须的
	loadCasefold()
	printCategories()
	printScriptOrProperty(false)
	printScriptOrProperty(true)
	printCases()
	printLatinProperties()
	printCasefold()
	printSizes()
}

var dataURL = flag.String("data", "", "full URL for UnicodeData.txt; defaults to --url/UnicodeData.txt")
var casefoldingURL = flag.String("casefolding", "", "full URL for CaseFolding.txt; defaults to --url/CaseFolding.txt")
var url = flag.String("url",
	"http://www.unicode.org/Public/6.3.0/ucd/",
	"URL of Unicode database directory")
var tablelist = flag.String("tables",
	"all",
	"comma-separated list of which tables to generate; can be letter")
var scriptlist = flag.String("scripts",
	"all",
	"comma-separated list of which script tables to generate")
var proplist = flag.String("props",
	"all",
	"comma-separated list of which property tables to generate")
var cases = flag.Bool("cases",
	true,
	"generate case tables")
var test = flag.Bool("test",
	false,
	"test existing tables; can be used to compare web data with package data")
var localFiles = flag.Bool("local",
	false,
	"data files have been copied to current directory; for debugging only")

var scriptRe = regexp.MustCompile(`^([0-9A-F]+)(\.\.[0-9A-F]+)? *; ([A-Za-z_]+)$`)
var logger = log.New(os.Stderr, "", log.Lshortfile)

type reader struct {
	*bufio.Reader
	fd   *os.File
	resp *http.Response
}

func open(url string) *reader {
	file := filepath.Base(url)
	if *localFiles {
		fd, err := os.Open(file)
		if err != nil {
			logger.Fatal(err)
		}
		return &reader{bufio.NewReader(fd), fd, nil}
	}
	resp, err := http.Get(url)
	if err != nil {
		logger.Fatal(err)
	}
	if resp.StatusCode != 200 {
		logger.Fatalf("bad GET status for %s: %d", file, resp.Status)
	}
	return &reader{bufio.NewReader(resp.Body), nil, resp}

}

func (r *reader) close() {
	if r.fd != nil {
		r.fd.Close()
	} else {
		r.resp.Body.Close()
	}
}

var category = map[string]bool{
	// Nd Lu etc.
	// We use one-character names to identify merged categories
	// 我们用单个字符名来标识互通的类别。
	"L": true, // Lu Ll Lt Lm Lo
	"P": true, // Pc Pd Ps Pe Pu Pf Po
	"M": true, // Mn Mc Me
	"N": true, // Nd Nl No
	"S": true, // Sm Sc Sk So
	"Z": true, // Zs Zl Zp
	"C": true, // Cc Cf Cs Co Cn
}

// UnicodeData.txt has form:
//	0037;DIGIT SEVEN;Nd;0;EN;;7;7;7;N;;;;;
//	007A;LATIN SMALL LETTER Z;Ll;0;L;;;;;N;;;005A;;005A
// See http://www.unicode.org/reports/tr44/ for a full explanation
// The fields:

// UnicodeData.txt 的形式为：
//	0037;DIGIT SEVEN;Nd;0;EN;;7;7;7;N;;;;;
//	007A;LATIN SMALL LETTER Z;Ll;0;L;;;;;N;;;005A;;005A
// 完整说明见 http://www.unicode.org/reports/tr44/
// The fields:
const (
	FCodePoint = iota
	FName
	FGeneralCategory
	FCanonicalCombiningClass
	FBidiClass
	FDecompositionTypeAndMapping
	FNumericType
	FNumericDigit // If a decimal digit. // 是否为十进制数字。
	FNumericValue // Includes non-decimal, e.g. U+2155=1/5 // 包括非十进制数字，例如 U+2155=⅕
	FBidiMirrored
	FUnicode1Name
	FISOComment
	FSimpleUppercaseMapping
	FSimpleLowercaseMapping
	FSimpleTitlecaseMapping
	NumField

	MaxChar = 0x10FFFF // anything above this shouldn't exist // 大于该码点的应该都不存在。
)

var fieldName = []string{
	FCodePoint:                   "CodePoint",
	FName:                        "Name",
	FGeneralCategory:             "GeneralCategory",
	FCanonicalCombiningClass:     "CanonicalCombiningClass",
	FBidiClass:                   "BidiClass",
	FDecompositionTypeAndMapping: "DecompositionTypeAndMapping",
	FNumericType:                 "NumericType",
	FNumericDigit:                "NumericDigit",
	FNumericValue:                "NumericValue",
	FBidiMirrored:                "BidiMirrored",
	FUnicode1Name:                "Unicode1Name",
	FISOComment:                  "ISOComment",
	FSimpleUppercaseMapping:      "SimpleUppercaseMapping",
	FSimpleLowercaseMapping:      "SimpleLowercaseMapping",
	FSimpleTitlecaseMapping:      "SimpleTitlecaseMapping",
}

// This contains only the properties we're interested in.

// 这里只包含了我们感兴趣的属性
type Char struct {
	field     []string // debugging only; could be deleted if we take out char.dump() // 仅用于调试；若我们去掉了 char.dump()，就能删除它。
	codePoint rune     // if zero, this index is not a valid code point. // 若为零，该索引即为非法的码点。
	category  string
	upperCase rune
	lowerCase rune
	titleCase rune
	foldCase  rune // simple case folding // 简单的写法转换
	caseOrbit rune // next in simple case folding orbit // 简单写法转换轨道中的的下一个字符
}

// Scripts.txt has form:
//	A673          ; Cyrillic # Po       SLAVONIC ASTERISK
//	A67C..A67D    ; Cyrillic # Mn   [2] COMBINING CYRILLIC KAVYKA..COMBINING CYRILLIC PAYEROK
// See http://www.unicode.org/Public/5.1.0/ucd/UCD.html for full explanation

// Scripts.txt 的形式为：
//	A673          ; Cyrillic # Po       SLAVONIC ASTERISK
//	A67C..A67D    ; Cyrillic # Mn   [2] COMBINING CYRILLIC KAVYKA..COMBINING CYRILLIC PAYEROK
// 完整说明见 http://www.unicode.org/Public/5.1.0/ucd/UCD.html

type Script struct {
	lo, hi uint32 // range of code points // 码点范围
	script string
}

var chars = make([]Char, MaxChar+1)
var scripts = make(map[string][]Script)
var props = make(map[string][]Script) // a property looks like a script; can share the format // props 类似于 scripts，它们的格式可以共享

var lastChar rune = 0

// In UnicodeData.txt, some ranges are marked like this:
//	3400;<CJK Ideograph Extension A, First>;Lo;0;L;;;;;N;;;;;
//	4DB5;<CJK Ideograph Extension A, Last>;Lo;0;L;;;;;N;;;;;
// parseCategory returns a state variable indicating the weirdness.

// 在 UnicodeData.txt 中，一些转换复幅度被标记成这样：
//	3400;<CJK Ideograph Extension A, First>;Lo;0;L;;;;;N;;;;;
//	4DB5;<CJK Ideograph Extension A, Last>;Lo;0;L;;;;;N;;;;;
// parseCategory 会返回一个 state 变量来指示它的类别。
type State int

const (
	SNormal State = iota // known to be zero for the type // 已知该类型的值为零
	SFirst
	SLast
	SMissing
)

func parseCategory(line string) (state State) {
	field := strings.Split(line, ";")
	if len(field) != NumField {
		logger.Fatalf("%5s: %d fields (expected %d)\n", line, len(field), NumField)
	}
	point, err := strconv.ParseUint(field[FCodePoint], 16, 64)
	if err != nil {
		logger.Fatalf("%.5s...: %s", line, err)
	}
	lastChar = rune(point)
	if point == 0 {
		return // not interesting and we use 0 as unset // 没什么意思，我们用 0 表示未设置
	}
	if point > MaxChar {
		return
	}
	char := &chars[point]
	char.field = field
	if char.codePoint != 0 {
		logger.Fatalf("point %U reused", point)
	}
	char.codePoint = lastChar
	char.category = field[FGeneralCategory]
	category[char.category] = true
	switch char.category {
	case "Nd":
		// Decimal digit
		// 十进制数字
		_, err := strconv.Atoi(field[FNumericValue])
		if err != nil {
			logger.Fatalf("%U: bad numeric field: %s", point, err)
		}
	case "Lu":
		char.letter(field[FCodePoint], field[FSimpleLowercaseMapping], field[FSimpleTitlecaseMapping])
	case "Ll":
		char.letter(field[FSimpleUppercaseMapping], field[FCodePoint], field[FSimpleTitlecaseMapping])
	case "Lt":
		char.letter(field[FSimpleUppercaseMapping], field[FSimpleLowercaseMapping], field[FCodePoint])
	default:
		char.letter(field[FSimpleUppercaseMapping], field[FSimpleLowercaseMapping], field[FSimpleTitlecaseMapping])
	}
	switch {
	case strings.Index(field[FName], ", First>") > 0:
		state = SFirst
	case strings.Index(field[FName], ", Last>") > 0:
		state = SLast
	}
	return
}

func (char *Char) dump(s string) {
	fmt.Print(s, " ")
	for i := 0; i < len(char.field); i++ {
		fmt.Printf("%s:%q ", fieldName[i], char.field[i])
	}
	fmt.Print("\n")
}

func (char *Char) letter(u, l, t string) {
	char.upperCase = char.letterValue(u, "U")
	char.lowerCase = char.letterValue(l, "L")
	char.titleCase = char.letterValue(t, "T")
}

func (char *Char) letterValue(s string, cas string) rune {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		char.dump(cas)
		logger.Fatalf("%U: bad letter(%s): %s", char.codePoint, s, err)
	}
	return rune(v)
}

func allCategories() []string {
	a := make([]string, 0, len(category))
	for k := range category {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

func all(scripts map[string][]Script) []string {
	a := make([]string, 0, len(scripts))
	for k := range scripts {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

func allCatFold(m map[string]map[rune]bool) []string {
	a := make([]string, 0, len(m))
	for k := range m {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

// Extract the version number from the URL

// 从URL提取版本号
func version() string {
	// Break on slashes and look for the first numeric field
	// 从斜杠处断开，并查看第一个数值段
	fields := strings.Split(*url, "/")
	for _, f := range fields {
		if len(f) > 0 && '0' <= f[0] && f[0] <= '9' {
			return f
		}
	}
	logger.Fatal("unknown version")
	return "Unknown"
}

func categoryOp(code rune, class uint8) bool {
	category := chars[code].category
	return len(category) > 0 && category[0] == class
}

func loadChars() {
	if *dataURL == "" {
		flag.Set("data", *url+"UnicodeData.txt")
	}
	input := open(*dataURL)
	defer input.close()
	scanner := bufio.NewScanner(input)
	var first rune = 0
	for scanner.Scan() {
		switch parseCategory(scanner.Text()) {
		case SNormal:
			if first != 0 {
				logger.Fatalf("bad state normal at %U", lastChar)
			}
		case SFirst:
			if first != 0 {
				logger.Fatalf("bad state first at %U", lastChar)
			}
			first = lastChar
		case SLast:
			if first == 0 {
				logger.Fatalf("bad state last at %U", lastChar)
			}
			for i := first + 1; i <= lastChar; i++ {
				chars[i] = chars[first]
				chars[i].codePoint = i
			}
			first = 0
		}
	}
	if scanner.Err() != nil {
		logger.Fatal(scanner.Err())
	}
}

func loadCasefold() {
	if *casefoldingURL == "" {
		flag.Set("casefolding", *url+"CaseFolding.txt")
	}
	input := open(*casefoldingURL)
	defer input.close()
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' || len(strings.TrimSpace(line)) == 0 {
			continue
		}
		field := strings.Split(line, "; ")
		if len(field) != 4 {
			logger.Fatalf("CaseFolding.txt %.5s...: %d fields (expected %d)\n", line, len(field), 4)
		}
		kind := field[1]
		if kind != "C" && kind != "S" {
			// Only care about 'common' and 'simple' foldings.
			// 只关心“常见”和“简单”的写法转换情况。
			continue
		}
		p1, err := strconv.ParseUint(field[0], 16, 64)
		if err != nil {
			logger.Fatalf("CaseFolding.txt %.5s...: %s", line, err)
		}
		p2, err := strconv.ParseUint(field[2], 16, 64)
		if err != nil {
			logger.Fatalf("CaseFolding.txt %.5s...: %s", line, err)
		}
		chars[p1].foldCase = rune(p2)
	}
	if scanner.Err() != nil {
		logger.Fatal(scanner.Err())
	}
}
/*
const progHeader = `// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Generated by running
//	maketables --tables=%s --data=%s --casefolding=%s
// DO NOT EDIT

package unicode

`
*/

const progHeader = `// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 生成自
//	maketables --tables=%s --data=%s --casefolding=%s
// 请勿编辑！

package unicode

`

func printCategories() {
	if *tablelist == "" {
		return
	}
	// Find out which categories to dump
	// 找出转储的类别
	list := strings.Split(*tablelist, ",")
	if *tablelist == "all" {
		list = allCategories()
	}
	if *test {
		fullCategoryTest(list)
		return
	}
	fmt.Printf(progHeader, *tablelist, *dataURL, *casefoldingURL)
	// fmt.Println("// Version is the Unicode edition from which the tables are derived.")
	fmt.Println("// Version 为得到此表所用的Unicode版本。")
	fmt.Printf("const Version = %q\n\n", version())

	if *tablelist == "all" {
		// fmt.Println("// Categories is the set of Unicode category tables.")
		fmt.Println("// Categories 为Unicode类别表的集合。")
		fmt.Println("var Categories = map[string] *RangeTable {")
		for _, k := range allCategories() {
			fmt.Printf("\t%q: %s,\n", k, k)
		}
		fmt.Print("}\n\n")
	}

	decl := make(sort.StringSlice, len(list))
	ndecl := 0
	for _, name := range list {
		if _, ok := category[name]; !ok {
			logger.Fatal("unknown category", name)
		}
		// We generate an UpperCase name to serve as concise documentation and an _UnderScored
		// name to store the data.  This stops godoc dumping all the tables but keeps them
		// available to clients.
		// Cases deserving special comments
		//
		// 我们将首字母大写（UpperCase）的名称用于简练的说明，将带前缀下划线
		// （_UnderScored）的名称用于存储数据。
		varDecl := ""
		/*
			switch name {
			case "C":
				varDecl = "\tOther = _C;	// Other/C is the set of Unicode control and special characters, category C.\n"
				varDecl += "\tC = _C\n"
			case "L":
				varDecl = "\tLetter = _L;	// Letter/L is the set of Unicode letters, category L.\n"
				varDecl += "\tL = _L\n"
			case "M":
				varDecl = "\tMark = _M;	// Mark/M is the set of Unicode mark characters, category M.\n"
				varDecl += "\tM = _M\n"
			case "N":
				varDecl = "\tNumber = _N;	// Number/N is the set of Unicode number characters, category N.\n"
				varDecl += "\tN = _N\n"
			case "P":
				varDecl = "\tPunct = _P;	// Punct/P is the set of Unicode punctuation characters, category P.\n"
				varDecl += "\tP = _P\n"
			case "S":
				varDecl = "\tSymbol = _S;	// Symbol/S is the set of Unicode symbol characters, category S.\n"
				varDecl += "\tS = _S\n"
			case "Z":
				varDecl = "\tSpace = _Z;	// Space/Z is the set of Unicode space characters, category Z.\n"
				varDecl += "\tZ = _Z\n"
			case "Nd":
				varDecl = "\tDigit = _Nd;	// Digit is the set of Unicode characters with the \"decimal digit\" property.\n"
			case "Lu":
				varDecl = "\tUpper = _Lu;	// Upper is the set of Unicode upper case letters.\n"
			case "Ll":
				varDecl = "\tLower = _Ll;	// Lower is the set of Unicode lower case letters.\n"
			case "Lt":
				varDecl = "\tTitle = _Lt;	// Title is the set of Unicode title case letters.\n"
			}
		*/
		switch name {
		case "C":
			varDecl = "\tOther = _C;	// Other/C 为类别 C 中的Unicode控制和特殊字符集合。\n"
			varDecl += "\tC = _C\n"
		case "L":
			varDecl = "\tLetter = _L;	// Letter/L 为类别 L 中的Unicode字母字符集合。\n"
			varDecl += "\tL = _L\n"
		case "M":
			varDecl = "\tMark = _M;	// Mark/M 为类别 M 中的Unicode标记字符集合。\n"
			varDecl += "\tM = _M\n"
		case "N":
			varDecl = "\tNumber = _N;	// Number/N 为类别 N 中的Unicode数字字符集合。\n"
			varDecl += "\tN = _N\n"
		case "P":
			varDecl = "\tPunct = _P;	// Punct/P 为类别 P 中的Unicode标点字符集合。\n"
			varDecl += "\tP = _P\n"
		case "S":
			varDecl = "\tSymbol = _S;	// Symbol/S 为类别 S 中的Unicode符号字符集合。\n"
			varDecl += "\tS = _S\n"
		case "Z":
			varDecl = "\tSpace = _Z;	// Space/Z 为类别 Z 中的Unicode空白字符集合。\n"
			varDecl += "\tZ = _Z\n"
		case "Nd":
			varDecl = "\tDigit = _Nd;	// Digit 为带属性“十进制数字”的Unicode字符集合。\n"
		case "Lu":
			varDecl = "\tUpper = _Lu;	// Upper 为Unicode大写字母集合。\n"
		case "Ll":
			varDecl = "\tLower = _Ll;	// Lower 为Unicode小写字母集合。\n"
		case "Lt":
			varDecl = "\tTitle = _Lt;	// Title 为Unicode标题字母集合。\n"
		}

		if len(name) > 1 {
			varDecl += fmt.Sprintf(
				// "\t%s = _%s;	// %s is the set of Unicode characters in category %s.\n",
				"\t%s = _%s;	// %s 为类别 %s 中的Unicode字符集合。\n",
				name, name, name, name)
		}
		decl[ndecl] = varDecl
		ndecl++
		if len(name) == 1 { // unified categories // 统一的类别
			decl := fmt.Sprintf("var _%s = &RangeTable{\n", name)
			dumpRange(
				decl,
				func(code rune) bool { return categoryOp(code, name[0]) })
			continue
		}
		dumpRange(
			fmt.Sprintf("var _%s = &RangeTable{\n", name),
			func(code rune) bool { return chars[code].category == name })
	}
	decl.Sort()
	// fmt.Println("// These variables have type *RangeTable.")
	fmt.Println("// 这些变量的类型为 *RangeTable。")
	fmt.Println("var (")
	for _, d := range decl {
		fmt.Print(d)
	}
	fmt.Print(")\n\n")
}

type Op func(code rune) bool

const format = "\t\t{0x%04x, 0x%04x, %d},\n"

func dumpRange(header string, inCategory Op) {
	fmt.Print(header)
	next := rune(0)
	latinOffset := 0
	fmt.Print("\tR16: []Range16{\n")
	// one Range for each iteration
	// 每次都迭代同一个范围
	count := &range16Count
	size := 16
	for {
		// look for start of range
		// 查找范围的起始处
		for next < rune(len(chars)) && !inCategory(next) {
			next++
		}
		if next >= rune(len(chars)) {
			// no characters remain
			// 没有剩余的字符了
			break
		}

		// start of range
		// 范围的起始处
		lo := next
		hi := next
		stride := rune(1)
		// accept lo
		// 接受 lo
		next++
		// look for another character to set the stride
		// 查找另一个字符来设置跨度
		for next < rune(len(chars)) && !inCategory(next) {
			next++
		}
		if next >= rune(len(chars)) {
			// no more characters
			// 没有更多字符了
			fmt.Printf(format, lo, hi, stride)
			break
		}
		// set stride
		// 设置跨度
		stride = next - lo
		// check for length of run. next points to first jump in stride
		// 检查连续的长度。下次按照跨度指向第一次跳跃
		for i := next; i < rune(len(chars)); i++ {
			if inCategory(i) == (((i - lo) % stride) == 0) {
				// accept
				// 接受
				if inCategory(i) {
					hi = i
				}
			} else {
				// no more characters in this run
				// 在这一串中没有更多字符了
				break
			}
		}
		if uint32(hi) <= unicode.MaxLatin1 {
			latinOffset++
		}
		size, count = printRange(uint32(lo), uint32(hi), uint32(stride), size, count)
		// next range: start looking where this range ends
		// 下一个范围：从这个范围的终止处开始
		next = hi + 1
	}
	fmt.Print("\t},\n")
	if latinOffset > 0 {
		fmt.Printf("\tLatinOffset: %d,\n", latinOffset)
	}
	fmt.Print("}\n\n")
}

func printRange(lo, hi, stride uint32, size int, count *int) (int, *int) {
	if size == 16 && hi >= 1<<16 {
		if lo < 1<<16 {
			if lo+stride != hi {
				logger.Fatalf("unexpected straddle: %U %U %d", lo, hi, stride)
			}
			// No range contains U+FFFF as an instance, so split
			// the range into two entries. That way we can maintain
			// the invariant that R32 contains only >= 1<<16.
			//
			// 没有任何范围将 U+FFFF 作为一个实例，因此可以用它将一个范围分成两部分。
			// 这样我们可以让只包含 >= 1<<16 的 R32 保持不变。
			fmt.Printf(format, lo, lo, 1)
			lo = hi
			stride = 1
			*count++
		}
		fmt.Print("\t},\n")
		fmt.Print("\tR32: []Range32{\n")
		size = 32
		count = &range32Count
	}
	fmt.Printf(format, lo, hi, stride)
	*count++
	return size, count
}

func fullCategoryTest(list []string) {
	for _, name := range list {
		if _, ok := category[name]; !ok {
			logger.Fatal("unknown category", name)
		}
		r, ok := unicode.Categories[name]
		if !ok && len(name) > 1 {
			logger.Fatalf("unknown table %q", name)
		}
		if len(name) == 1 {
			verifyRange(name, func(code rune) bool { return categoryOp(code, name[0]) }, r)
		} else {
			verifyRange(
				name,
				func(code rune) bool { return chars[code].category == name },
				r)
		}
	}
}

func verifyRange(name string, inCategory Op, table *unicode.RangeTable) {
	count := 0
	for j := range chars {
		i := rune(j)
		web := inCategory(i)
		pkg := unicode.Is(table, i)
		if web != pkg {
			fmt.Fprintf(os.Stderr, "%s: %U: web=%t pkg=%t\n", name, i, web, pkg)
			count++
			if count > 10 {
				break
			}
		}
	}
}

func parseScript(line string, scripts map[string][]Script) {
	comment := strings.Index(line, "#")
	if comment >= 0 {
		line = line[0:comment]
	}
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	field := strings.Split(line, ";")
	if len(field) != 2 {
		logger.Fatalf("%s: %d fields (expected 2)\n", line, len(field))
	}
	matches := scriptRe.FindStringSubmatch(line)
	if len(matches) != 4 {
		logger.Fatalf("%s: %d matches (expected 3)\n", line, len(matches))
	}
	lo, err := strconv.ParseUint(matches[1], 16, 64)
	if err != nil {
		logger.Fatalf("%.5s...: %s", line, err)
	}
	hi := lo
	if len(matches[2]) > 2 { // ignore leading .. // 忽略前导 ..
		hi, err = strconv.ParseUint(matches[2][2:], 16, 64)
		if err != nil {
			logger.Fatalf("%.5s...: %s", line, err)
		}
	}
	name := matches[3]
	scripts[name] = append(scripts[name], Script{uint32(lo), uint32(hi), name})
}

// The script tables have a lot of adjacent elements. Fold them together.

// 书写表单拥有大量相邻的元素。将它们合在一起。
func foldAdjacent(r []Script) []unicode.Range32 {
	s := make([]unicode.Range32, 0, len(r))
	j := 0
	for i := 0; i < len(r); i++ {
		if j > 0 && r[i].lo == s[j-1].Hi+1 {
			s[j-1].Hi = r[i].hi
		} else {
			s = s[0 : j+1]
			s[j] = unicode.Range32{
				Lo:     uint32(r[i].lo),
				Hi:     uint32(r[i].hi),
				Stride: 1,
			}
			j++
		}
	}
	return s
}

func fullScriptTest(list []string, installed map[string]*unicode.RangeTable, scripts map[string][]Script) {
	for _, name := range list {
		if _, ok := scripts[name]; !ok {
			logger.Fatal("unknown script", name)
		}
		_, ok := installed[name]
		if !ok {
			logger.Fatal("unknown table", name)
		}
		for _, script := range scripts[name] {
			for r := script.lo; r <= script.hi; r++ {
				if !unicode.Is(installed[name], rune(r)) {
					fmt.Fprintf(os.Stderr, "%U: not in script %s\n", r, name)
				}
			}
		}
	}
}

// PropList.txt has the same format as Scripts.txt so we can share its parser.

// PropList.txt 与 Scripts.txt 拥有相同的格式，因此我们可以共享它的解析器。
func printScriptOrProperty(doProps bool) {
	flag := "scripts"
	flaglist := *scriptlist
	file := "Scripts.txt"
	table := scripts
	installed := unicode.Scripts
	if doProps {
		flag = "props"
		flaglist = *proplist
		file = "PropList.txt"
		table = props
		installed = unicode.Properties
	}
	if flaglist == "" {
		return
	}
	input := open(*url + file)
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		parseScript(scanner.Text(), table)
	}
	if scanner.Err() != nil {
		logger.Fatal(scanner.Err())
	}
	input.close()

	// Find out which scripts to dump
	// 找出要转储的书写系统。
	list := strings.Split(flaglist, ",")
	if flaglist == "all" {
		list = all(table)
	}
	if *test {
		fullScriptTest(list, installed, table)
		return
	}

	fmt.Printf(
		/*
			"// Generated by running\n"+
				"//	maketables --%s=%s --url=%s\n"+
				// "// DO NOT EDIT\n\n",
		*/
		"// 生成自\n"+
			"//	maketables --%s=%s --url=%s\n"+
			"// 请勿编辑！\n\n",
		flag,
		flaglist,
		*url)
	if flaglist == "all" {
		if doProps {
			// fmt.Println("// Properties is the set of Unicode property tables.")
			fmt.Println("// Properties 为Unicode属性表的集合。")
			fmt.Println("var Properties = map[string] *RangeTable{")
		} else {
			// fmt.Println("// Scripts is the set of Unicode script tables.")
			fmt.Println("// Scripts 为Unicode书写表的集合。")
			fmt.Println("var Scripts = map[string] *RangeTable{")
		}
		for _, k := range all(table) {
			fmt.Printf("\t%q: %s,\n", k, k)
		}
		fmt.Print("}\n\n")
	}

	decl := make(sort.StringSlice, len(list))
	ndecl := 0
	for _, name := range list {
		if doProps {
			decl[ndecl] = fmt.Sprintf(
				// "\t%s = _%s;\t// %s is the set of Unicode characters with property %s.\n",
				"\t%s = _%s;\t// %s 为带属性 %s 的Unicode字符集合。\n",
				name, name, name, name)
		} else {
			decl[ndecl] = fmt.Sprintf(
				// "\t%s = _%s;\t// %s is the set of Unicode characters in script %s.\n",
				"\t%s = _%s;\t// %s 为书写系统 %s 中的Unicode字符集合。\n",
				name, name, name, name)
		}
		ndecl++
		fmt.Printf("var _%s = &RangeTable {\n", name)
		ranges := foldAdjacent(table[name])
		fmt.Print("\tR16: []Range16{\n")
		size := 16
		count := &range16Count
		for _, s := range ranges {
			size, count = printRange(s.Lo, s.Hi, s.Stride, size, count)
		}
		fmt.Print("\t},\n")
		if off := findLatinOffset(ranges); off > 0 {
			fmt.Printf("\tLatinOffset: %d,\n", off)
		}
		fmt.Print("}\n\n")
	}
	decl.Sort()
	// fmt.Println("// These variables have type *RangeTable.")
	fmt.Println("// 这些变量的类型为 *RangeTable。")
	fmt.Println("var (")
	for _, d := range decl {
		fmt.Print(d)
	}
	fmt.Print(")\n\n")
}

func findLatinOffset(ranges []unicode.Range32) int {
	i := 0
	for i < len(ranges) && ranges[i].Hi <= unicode.MaxLatin1 {
		i++
	}
	return i
}

const (
	CaseUpper = 1 << iota
	CaseLower
	CaseTitle
	CaseNone    = 0  // must be zero // 必须为零
	CaseMissing = -1 // character not present; not a valid case state // 字符不存在；无效字符状态
)

type caseState struct {
	point        rune
	_case        int
	deltaToUpper rune
	deltaToLower rune
	deltaToTitle rune
}

// Is d a continuation of the state of c?

// d 是 c 的 state 后续么？
func (c *caseState) adjacent(d *caseState) bool {
	if d.point < c.point {
		c, d = d, c
	}
	switch {
	case d.point != c.point+1: // code points not adjacent (shouldn't happen) // 码点不相邻（应该不会发生的）
		return false
	case d._case != c._case: // different cases // 不同的写法
		return c.upperLowerAdjacent(d)
	case c._case == CaseNone:
		return false
	case c._case == CaseMissing:
		return false
	case d.deltaToUpper != c.deltaToUpper:
		return false
	case d.deltaToLower != c.deltaToLower:
		return false
	case d.deltaToTitle != c.deltaToTitle:
		return false
	}
	return true
}

// Is d the same as c, but opposite in upper/lower case? this would make it
// an element of an UpperLower sequence.

// d 是与 c 相同且是与之对应的大/小写么？这会让它变成单个“大写-小写”序列的元素。
func (c *caseState) upperLowerAdjacent(d *caseState) bool {
	// check they're a matched case pair.  we know they have adjacent values
	// 检查它们的大小写是否配对。我们知道它们拥有相近的值。
	switch {
	case c._case == CaseUpper && d._case != CaseLower:
		return false
	case c._case == CaseLower && d._case != CaseUpper:
		return false
	}
	// matched pair (at least in upper/lower).  make the order Upper Lower
	// 配对（至少在大小写上）。将它们排为大小写的顺序
	if c._case == CaseLower {
		c, d = d, c
	}
	// for an Upper Lower sequence the deltas have to be in order
	//	c: 0 1 0
	//	d: -1 0 -1
	//
	// 对于大小写序列，三者的顺序应为
	//	c: 0 1 0
	//	d: -1 0 -1
	switch {
	case c.deltaToUpper != 0:
		return false
	case c.deltaToLower != 1:
		return false
	case c.deltaToTitle != 0:
		return false
	case d.deltaToUpper != -1:
		return false
	case d.deltaToLower != 0:
		return false
	case d.deltaToTitle != -1:
		return false
	}
	return true
}

// Does this character start an UpperLower sequence?

// 该字符是以一个“大写-小写”的序列开始的么？
func (c *caseState) isUpperLower() bool {
	// for an Upper Lower sequence the deltas have to be in order
	//	c: 0 1 0
	//
	// 对于大小序列，三者的顺序应为
	//	c: 0 1 0
	switch {
	case c.deltaToUpper != 0:
		return false
	case c.deltaToLower != 1:
		return false
	case c.deltaToTitle != 0:
		return false
	}
	return true
}

// Does this character start a LowerUpper sequence?

// 该字符是以一个“小写-大写”的序列开始的么？
func (c *caseState) isLowerUpper() bool {
	// for an Upper Lower sequence the deltas have to be in order
	//	c: -1 0 -1
	//
	// 对于大小序列，三者的顺序应为
	//	c: -1 0 -1
	switch {
	case c.deltaToUpper != -1:
		return false
	case c.deltaToLower != 0:
		return false
	case c.deltaToTitle != -1:
		return false
	}
	return true
}

func getCaseState(i rune) (c *caseState) {
	c = &caseState{point: i, _case: CaseNone}
	ch := &chars[i]
	switch ch.codePoint {
	case 0:
		c._case = CaseMissing // Will get NUL wrong but that doesn't matter // 会得到NUL错误，不过没关系
		return
	case ch.upperCase:
		c._case = CaseUpper
	case ch.lowerCase:
		c._case = CaseLower
	case ch.titleCase:
		c._case = CaseTitle
	}
	// Some things such as roman numeral U+2161 don't describe themselves
	// as upper case, but have a lower case.  Second-guess them.
	// 像罗马数字 U+2161 这样的并不称作大写形式，但它们有小写形式。它们都事后诸葛亮。
	if c._case == CaseNone && ch.lowerCase != 0 {
		c._case = CaseUpper
	}
	// Same in the other direction.
	// 反过来说。
	if c._case == CaseNone && ch.upperCase != 0 {
		c._case = CaseLower
	}

	if ch.upperCase != 0 {
		c.deltaToUpper = ch.upperCase - i
	}
	if ch.lowerCase != 0 {
		c.deltaToLower = ch.lowerCase - i
	}
	if ch.titleCase != 0 {
		c.deltaToTitle = ch.titleCase - i
	}
	return
}

func printCases() {
	if !*cases {
		return
	}
	if *test {
		fullCaseTest()
		return
	}
	fmt.Printf(
		/*
			"// Generated by running\n"+
				"//	maketables --data=%s --casefolding=%s\n"+
				"// DO NOT EDIT\n\n"+
				"// CaseRanges is the table describing case mappings for all letters with\n"+
				"// non-self mappings.\n"+
				"var CaseRanges = _CaseRanges\n"+
				"var _CaseRanges = []CaseRange {\n",
		*/
		"// 生成自\n"+
			"//	maketables --data=%s --casefolding=%s\n"+
			"// 请勿编辑！\n\n"+
			"// CaseRanges 是描述所有“非自我映射字母”的写法映射表。\n"+
			"var CaseRanges = _CaseRanges\n"+
			"var _CaseRanges = []CaseRange {\n",
		*dataURL, *casefoldingURL)

	var startState *caseState    // the start of a run; nil for not active // 一连串的开始；未激活则为nil
	var prevState = &caseState{} // the state of the previous character    // 上一个字符的状态
	for i := range chars {
		state := getCaseState(rune(i))
		if state.adjacent(prevState) {
			prevState = state
			continue
		}
		// end of run (possibly)
		// 一连串（可能的）末尾
		printCaseRange(startState, prevState)
		startState = nil
		if state._case != CaseMissing && state._case != CaseNone {
			startState = state
		}
		prevState = state
	}
	fmt.Print("}\n")
}

func printCaseRange(lo, hi *caseState) {
	if lo == nil {
		return
	}
	if lo.deltaToUpper == 0 && lo.deltaToLower == 0 && lo.deltaToTitle == 0 {
		// character represents itself in all cases - no need to mention it
		// 字符表示它自己所有的形式 - 无需提及它
		return
	}
	switch {
	case hi.point > lo.point && lo.isUpperLower():
		fmt.Printf("\t{0x%04X, 0x%04X, d{UpperLower, UpperLower, UpperLower}},\n",
			lo.point, hi.point)
	case hi.point > lo.point && lo.isLowerUpper():
		logger.Fatalf("LowerUpper sequence: should not happen: %U.  If it's real, need to fix To()", lo.point)
		fmt.Printf("\t{0x%04X, 0x%04X, d{LowerUpper, LowerUpper, LowerUpper}},\n",
			lo.point, hi.point)
	default:
		fmt.Printf("\t{0x%04X, 0x%04X, d{%d, %d, %d}},\n",
			lo.point, hi.point,
			lo.deltaToUpper, lo.deltaToLower, lo.deltaToTitle)
	}
}

// If the cased value in the Char is 0, it means use the rune itself.

// 若 cased 的值为 0，即表示用该字符本身。
func caseIt(r, cased rune) rune {
	if cased == 0 {
		return r
	}
	return cased
}

func fullCaseTest() {
	for j, c := range chars {
		i := rune(j)
		lower := unicode.ToLower(i)
		want := caseIt(i, c.lowerCase)
		if lower != want {
			fmt.Fprintf(os.Stderr, "lower %U should be %U is %U\n", i, want, lower)
		}
		upper := unicode.ToUpper(i)
		want = caseIt(i, c.upperCase)
		if upper != want {
			fmt.Fprintf(os.Stderr, "upper %U should be %U is %U\n", i, want, upper)
		}
		title := unicode.ToTitle(i)
		want = caseIt(i, c.titleCase)
		if title != want {
			fmt.Fprintf(os.Stderr, "title %U should be %U is %U\n", i, want, title)
		}
	}
}

func printLatinProperties() {
	if *test {
		return
	}
	fmt.Println("var properties = [MaxLatin1+1]uint8{")
	for code := 0; code <= unicode.MaxLatin1; code++ {
		var property string
		switch chars[code].category {
		case "Cc", "": // NUL has no category. // NUL没有类别
			property = "pC"
		case "Cf": // soft hyphen, unique category, not printable. // 软连字符，唯一的类别，不可打印。
			property = "0"
		case "Ll":
			property = "pLl | pp"
		case "Lo":
			property = "pLo | pp"
		case "Lu":
			property = "pLu | pp"
		case "Nd", "No":
			property = "pN | pp"
		case "Pc", "Pd", "Pe", "Pf", "Pi", "Po", "Ps":
			property = "pP | pp"
		case "Sc", "Sk", "Sm", "So":
			property = "pS | pp"
		case "Zs":
			property = "pZ"
		default:
			logger.Fatalf("%U has unknown category %q", code, chars[code].category)
		}
		// Special case
		// 特殊情况
		if code == ' ' {
			property = "pZ | pp"
		}
		fmt.Printf("\t0x%02X: %s, // %q\n", code, property, code)
	}
	fmt.Printf("}\n\n")
}

type runeSlice []rune

func (p runeSlice) Len() int           { return len(p) }
func (p runeSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p runeSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func printCasefold() {
	// Build list of case-folding groups attached to each canonical folded char (typically lower case).
	// 构建写法转换组的列表，该列表会附加到每一个典型的转换字符（一般为小写）。
	var caseOrbit = make([][]rune, MaxChar+1)
	for j := range chars {
		i := rune(j)
		c := &chars[i]
		if c.foldCase == 0 {
			continue
		}
		orb := caseOrbit[c.foldCase]
		if orb == nil {
			orb = append(orb, c.foldCase)
		}
		caseOrbit[c.foldCase] = append(orb, i)
	}

	// Insert explicit 1-element groups when assuming [lower, upper] would be wrong.
	// 当假定 [lower, upper] 会发生错误时，插入显式的有一个元素的组。
	for j := range chars {
		i := rune(j)
		c := &chars[i]
		f := c.foldCase
		if f == 0 {
			f = i
		}
		orb := caseOrbit[f]
		if orb == nil && (c.upperCase != 0 && c.upperCase != i || c.lowerCase != 0 && c.lowerCase != i) {
			// Default assumption of [upper, lower] is wrong.
			// 默认对 [lower, upper] 的假定为错误。
			caseOrbit[i] = []rune{i}
		}
	}

	// Delete the groups for which assuming [lower, upper] is right.
	// 为假定 [lower, upper] 是正确的删除组。
	for i, orb := range caseOrbit {
		if len(orb) == 2 && chars[orb[0]].upperCase == orb[1] && chars[orb[1]].lowerCase == orb[0] {
			caseOrbit[i] = nil
		}
	}

	// Record orbit information in chars.
	// 记录 chars 中的轨道信息。
	for _, orb := range caseOrbit {
		if orb == nil {
			continue
		}
		sort.Sort(runeSlice(orb))
		c := orb[len(orb)-1]
		for _, d := range orb {
			chars[c].caseOrbit = d
			c = d
		}
	}

	printCaseOrbit()

	// Tables of category and script folding exceptions: code points
	// that must be added when interpreting a particular category/script
	// in a case-folding context.
	//
	// 类别与书写表的转换例外：当在写法转换的上下文中解释特别的类别/书写时，
	// 该码点必须被添加。
	cat := make(map[string]map[rune]bool)
	for name := range category {
		if x := foldExceptions(inCategory(name)); len(x) > 0 {
			cat[name] = x
		}
	}

	scr := make(map[string]map[rune]bool)
	for name := range scripts {
		if x := foldExceptions(inScript(name)); len(x) > 0 {
			cat[name] = x
		}
	}

	printCatFold("FoldCategory", cat)
	printCatFold("FoldScript", scr)
}

// inCategory returns a list of all the runes in the category.

// inCategory 返回该类别中所有符文的列表。
func inCategory(name string) []rune {
	var x []rune
	for j := range chars {
		i := rune(j)
		c := &chars[i]
		if c.category == name || len(name) == 1 && len(c.category) > 1 && c.category[0] == name[0] {
			x = append(x, i)
		}
	}
	return x
}

// inScript returns a list of all the runes in the script.

// inScript 返回该书写系统中所有符文的列表。
func inScript(name string) []rune {
	var x []rune
	for _, s := range scripts[name] {
		for c := s.lo; c <= s.hi; c++ {
			x = append(x, rune(c))
		}
	}
	return x
}

// foldExceptions returns a list of all the runes fold-equivalent
// to runes in class but not in class themselves.

// foldExceptions 返回分类中可等价转换，但在其自身分类中不能的所有符文列表。
func foldExceptions(class []rune) map[rune]bool {
	// Create map containing class and all fold-equivalent chars.
	// 创建包含分类和所有等价转换的字符的映射。
	m := make(map[rune]bool)
	for _, r := range class {
		c := &chars[r]
		if c.caseOrbit == 0 {
			// Just upper and lower.
			// 只是转为大写或小写。
			if u := c.upperCase; u != 0 {
				m[u] = true
			}
			if l := c.lowerCase; l != 0 {
				m[l] = true
			}
			m[r] = true
			continue
		}
		// Otherwise walk orbit.
		// 否则按轨道继续。
		r0 := r
		for {
			m[r] = true
			r = chars[r].caseOrbit
			if r == r0 {
				break
			}
		}
	}

	// Remove class itself.
	// 移除分类自身。
	for _, r := range class {
		delete(m, r)
	}

	// What's left is the exceptions.
	// 剩下的就是例外。
	return m
}

var comment = map[string]string{
	/*
		"FoldCategory": "// FoldCategory maps a category name to a table of\n" +
			"// code points outside the category that are equivalent under\n" +
			"// simple case folding to code points inside the category.\n" +
			"// If there is no entry for a category name, there are no such points.\n",
	*/
	"FoldCategory": "// FoldCategory 将一个类别名映射到该类别外的码点表上，\n" +
		"// 这相当于在简单的情况下对该类别内的码点进行转换。\n" +
		"// 若一个类别名没有对应的条目，则该码点不存在。\n",
	/*
		"FoldScript": "// FoldScript maps a script name to a table of\n" +
			"// code points outside the script that are equivalent under\n" +
			"// simple case folding to code points inside the script.\n" +
			"// If there is no entry for a script name, there are no such points.\n",
	*/
	"FoldScript": "// FoldCategory 将一个书写系统名映射到该书写系统外的码点表上，\n" +
		"// 这相当于在简单的情况下对该书写系统内的码点进行转换。\n" +
		"// 若一个书写系统名没有对应的条目，则该码点不存在。\n",
}

func printCaseOrbit() {
	if *test {
		for j := range chars {
			i := rune(j)
			c := &chars[i]
			f := c.caseOrbit
			if f == 0 {
				if c.lowerCase != i && c.lowerCase != 0 {
					f = c.lowerCase
				} else if c.upperCase != i && c.upperCase != 0 {
					f = c.upperCase
				} else {
					f = i
				}
			}
			if g := unicode.SimpleFold(i); g != f {
				fmt.Fprintf(os.Stderr, "unicode.SimpleFold(%#U) = %#U, want %#U\n", i, g, f)
			}
		}
		return
	}

	fmt.Printf("var caseOrbit = []foldPair{\n")
	for i := range chars {
		c := &chars[i]
		if c.caseOrbit != 0 {
			fmt.Printf("\t{0x%04X, 0x%04X},\n", i, c.caseOrbit)
			foldPairCount++
		}
	}
	fmt.Printf("}\n\n")
}

func printCatFold(name string, m map[string]map[rune]bool) {
	if *test {
		var pkgMap map[string]*unicode.RangeTable
		if name == "FoldCategory" {
			pkgMap = unicode.FoldCategory
		} else {
			pkgMap = unicode.FoldScript
		}
		if len(pkgMap) != len(m) {
			fmt.Fprintf(os.Stderr, "unicode.%s has %d elements, want %d\n", name, len(pkgMap), len(m))
			return
		}
		for k, v := range m {
			t, ok := pkgMap[k]
			if !ok {
				fmt.Fprintf(os.Stderr, "unicode.%s[%q] missing\n", name, k)
				continue
			}
			n := 0
			for _, r := range t.R16 {
				for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
					if !v[c] {
						fmt.Fprintf(os.Stderr, "unicode.%s[%q] contains %#U, should not\n", name, k, c)
					}
					n++
				}
			}
			for _, r := range t.R32 {
				for c := rune(r.Lo); c <= rune(r.Hi); c += rune(r.Stride) {
					if !v[c] {
						fmt.Fprintf(os.Stderr, "unicode.%s[%q] contains %#U, should not\n", name, k, c)
					}
					n++
				}
			}
			if n != len(v) {
				fmt.Fprintf(os.Stderr, "unicode.%s[%q] has %d code points, want %d\n", name, k, n, len(v))
			}
		}
		return
	}

	fmt.Print(comment[name])
	fmt.Printf("var %s = map[string]*RangeTable{\n", name)
	for _, name := range allCatFold(m) {
		fmt.Printf("\t%q: fold%s,\n", name, name)
	}
	fmt.Printf("}\n\n")
	for _, name := range allCatFold(m) {
		class := m[name]
		dumpRange(
			fmt.Sprintf("var fold%s = &RangeTable{\n", name),
			func(code rune) bool { return class[code] })
	}
}

var range16Count = 0  // Number of entries in the 16-bit range tables. // 16位范围表中的条目数。
var range32Count = 0  // Number of entries in the 32-bit range tables. // 32位范围表中的条目数。
var foldPairCount = 0 // Number of fold pairs in the exception tables. // 例外表中转换的对数。

func printSizes() {
	if *test {
		return
	}
	fmt.Println()
	// fmt.Printf("// Range entries: %d 16-bit, %d 32-bit, %d total.\n", range16Count, range32Count,
	fmt.Printf("// 范围条目数：%d 16-bit，%d 32-bit，%d 总数。\n", range16Count, range32Count, range16Count+range32Count)
	range16Bytes := range16Count * 3 * 2
	range32Bytes := range32Count * 3 * 4
	// fmt.Printf("// Range bytes: %d 16-bit, %d 32-bit, %d total.\n", range16Bytes, range32Bytes,
	fmt.Printf("// 范围字节数：%d 16-bit，%d 32-bit，%d 总数。\n", range16Bytes, range32Bytes, range16Bytes+range32Bytes)
	fmt.Println()
	// fmt.Printf("// Fold orbit bytes: %d pairs, %d bytes\n", foldPairCount, foldPairCount*2*2)
	fmt.Printf("// 转换轨道字节数: %d 对，%d 字节\n", foldPairCount, foldPairCount*2*2)
}
