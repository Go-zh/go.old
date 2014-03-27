// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fmt

import (
	"errors"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync"
	"unicode/utf8"
)

// runeUnreader is the interface to something that can unread runes.
// If the object provided to Scan does not satisfy this interface,
// a local buffer will be used to back up the input, but its contents
// will be lost when Scan returns.

// runeUnreader 接口可用于某些东西反读取符文。
// 若提供给 Scan 的对象不满足此接口，就会使用局部缓存来备份输入，但其内容会在
// Scan 返回时丢失。
type runeUnreader interface {
	UnreadRune() error
}

// ScanState represents the scanner state passed to custom scanners.
// Scanners may do rune-at-a-time scanning or ask the ScanState
// to discover the next space-delimited token.

// ScanState 表示传递给定制扫描器的扫描状态。扫瞄器可一次扫描一个附文或请求
// ScanState 发现下一个以空格分隔的标记。
type ScanState interface {
	// ReadRune reads the next rune (Unicode code point) from the input.
	// If invoked during Scanln, Fscanln, or Sscanln, ReadRune() will
	// return EOF after returning the first '\n' or when reading beyond
	// the specified width.
	//
	// ReadRune 从输入中读取下一个符文（Unicode码点）。若它在 Scanln、Fscanln 或
	// Sscanln 中被调用，ReadRune() 就会在返回第一个“\n”或读取至指定宽度后返回 EOF。
	ReadRune() (r rune, size int, err error)
	// UnreadRune causes the next call to ReadRune to return the same rune.
	// UnreadRune 会使 ReadRune 的下一次调用返回相同的符文。
	UnreadRune() error
	// SkipSpace skips space in the input. Newlines are treated as space
	// unless the scan operation is Scanln, Fscanln or Sscanln, in which case
	// a newline is treated as EOF.
	//
	// SkipSpace 跳过输入中的空格。换行符会被视作空格，除非该扫描操作为 Scanln、
	// Fscanln 或 Sscanln，在这种情况下，换行符会被视作 EOF。
	SkipSpace()
	// Token skips space in the input if skipSpace is true, then returns the
	// run of Unicode code points c satisfying f(c).  If f is nil,
	// !unicode.IsSpace(c) is used; that is, the token will hold non-space
	// characters.  Newlines are treated as space unless the scan operation
	// is Scanln, Fscanln or Sscanln, in which case a newline is treated as
	// EOF.  The returned slice points to shared data that may be overwritten
	// by the next call to Token, a call to a Scan function using the ScanState
	// as input, or when the calling Scan method returns.
	//
	// Token 会在 skipSpace 为 true 时从输入中跳过空格，然后返回一系列满足 f(c)
	// 的Unicode码点 c。若 f 为 nil，就会使用 !unicode.IsSpace(c)；
	// 即，该标记会保留非空格字符。换行符会被视作空格，除非该扫描操作为 Scanln、
	// Fscanln 或 Sscanln，否则在这种情况下，换行符会被视作 EOF。
	// 所返回的指向共享数据的切片可能会在下次调用 Token 时被覆盖，Scan 函数的调用
	// 会将 ScanState 用作输入，或当调用 Scan 方法返回时也会。
	Token(skipSpace bool, f func(rune) bool) (token []byte, err error)
	// Width returns the value of the width option and whether it has been set.
	// The unit is Unicode code points.
	// Width 返回宽度选项的值，并判断它是否被设置。其单元为Unicode码点。
	Width() (wid int, ok bool)
	// Because ReadRune is implemented by the interface, Read should never be
	// called by the scanning routines and a valid implementation of
	// ScanState may choose always to return an error from Read.
	//
	// 由于 ReadRune 被此接口实现，因此 Read 不应当被扫描功能调用，而 ScanState
	// 的有效实现可选择总是从 Read 中返回错误。
	Read(buf []byte) (n int, err error)
}

// Scanner is implemented by any value that has a Scan method, which scans
// the input for the representation of a value and stores the result in the
// receiver, which must be a pointer to be useful.  The Scan method is called
// for any argument to Scan, Scanf, or Scanln that implements it.

// Scanner 由任何拥有 Scan 方法的值实现，它将输入扫描成值的表示，并将其结果存储到接收者中，
// 该接收者必须为可用的指针。Scan 方法会被 Scan、Scanf 或 Scanln 的任何实现了它的实参所调用。
type Scanner interface {
	Scan(state ScanState, verb rune) error
}

// Scan scans text read from standard input, storing successive
// space-separated values into successive arguments.  Newlines count
// as space.  It returns the number of items successfully scanned.
// If that is less than the number of arguments, err will report why.

// Scan 扫描从标准输入中读取的文本，并将连续由空格分隔的值存储为连续的实参。
// 换行符计为空格。它返回成功扫描的条目数。若它少于实参数，err 就会报告原因。
func Scan(a ...interface{}) (n int, err error) {
	return Fscan(os.Stdin, a...)
}

// Scanln is similar to Scan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.

// Scanln 类似于 Scan，但它在换行符处停止扫描，且最后的条目之后必须为换行符或 EOF。
func Scanln(a ...interface{}) (n int, err error) {
	return Fscanln(os.Stdin, a...)
}

// Scanf scans text read from standard input, storing successive
// space-separated values into successive arguments as determined by
// the format.  It returns the number of items successfully scanned.

// Scanf 扫描从标准输入中读取的文本，并将连续由空格分隔的值存储为连续的实参，
// 其格式由 format 决定。它返回成功扫描的条目数。
func Scanf(format string, a ...interface{}) (n int, err error) {
	return Fscanf(os.Stdin, format, a...)
}

type stringReader string

func (r *stringReader) Read(b []byte) (n int, err error) {
	n = copy(b, *r)
	*r = (*r)[n:]
	if n == 0 {
		err = io.EOF
	}
	return
}

// Sscan scans the argument string, storing successive space-separated
// values into successive arguments.  Newlines count as space.  It
// returns the number of items successfully scanned.  If that is less
// than the number of arguments, err will report why.

// Sscan 扫描实参 string，并将连续由空格分隔的值存储为连续的实参。
// 换行符计为空格。它返回成功扫描的条目数。若它少于实参数，err 就会报告原因。
func Sscan(str string, a ...interface{}) (n int, err error) {
	return Fscan((*stringReader)(&str), a...)
}

// Sscanln is similar to Sscan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.

// Sscanln 类似于 Sscan，但它在换行符处停止扫描，且最后的条目之后必须为换行符或 EOF。
func Sscanln(str string, a ...interface{}) (n int, err error) {
	return Fscanln((*stringReader)(&str), a...)
}

// Sscanf scans the argument string, storing successive space-separated
// values into successive arguments as determined by the format.  It
// returns the number of items successfully parsed.

// Scanf 扫描实参 string，并将连续由空格分隔的值存储为连续的实参，
// 其格式由 format 决定。它返回成功解析的条目数。
func Sscanf(str string, format string, a ...interface{}) (n int, err error) {
	return Fscanf((*stringReader)(&str), format, a...)
}

// Fscan scans text read from r, storing successive space-separated
// values into successive arguments.  Newlines count as space.  It
// returns the number of items successfully scanned.  If that is less
// than the number of arguments, err will report why.

// Fscan 扫描从 r 中读取的文本，并将连续由空格分隔的值存储为连续的实参。
// 换行符计为空格。它返回成功扫描的条目数。若它少于实参数，err 就会报告原因。
func Fscan(r io.Reader, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, true, false)
	n, err = s.doScan(a)
	s.free(old)
	return
}

// Fscanln is similar to Fscan, but stops scanning at a newline and
// after the final item there must be a newline or EOF.

// Fscanln 类似于 Sscan，但它在换行符处停止扫描，且最后的条目之后必须为换行符或 EOF。
func Fscanln(r io.Reader, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, false, true)
	n, err = s.doScan(a)
	s.free(old)
	return
}

// Fscanf scans text read from r, storing successive space-separated
// values into successive arguments as determined by the format.  It
// returns the number of items successfully parsed.

// Fscanf 扫描从 r 中读取的文本，并将连续由空格分隔的值存储为连续的实参，
// 其格式由 format 决定。它返回成功解析的条目数。
func Fscanf(r io.Reader, format string, a ...interface{}) (n int, err error) {
	s, old := newScanState(r, false, false)
	n, err = s.doScanf(format, a)
	s.free(old)
	return
}

// scanError represents an error generated by the scanning software.
// It's used as a unique signature to identify such errors when recovering.

// scanError 表示由扫描软件生成的错误。在恢复时，它用作唯一的签名来标识出这类错误。
type scanError struct {
	err error
}

const eof = -1

// ss is the internal implementation of ScanState.

// ss 为 ScanState 的内部实现。
type ss struct {
	rr       io.RuneReader // where to read input            // 读取输入的地方
	buf      buffer        // token accumulator              // 标记累计器
	peekRune rune          // one-rune lookahead             // 前一个符文
	prevRune rune          // last rune returned by ReadRune // ReadRune 上一个返回的符文
	count    int           // runes consumed so far.         // 已消耗的字符数
	atEOF    bool          // already read EOF               // 是否已读到 EOF
	ssave
}

// ssave holds the parts of ss that need to be
// saved and restored on recursive scans.

// ssave 保存了 ss 中需要在递归扫描时保存并恢复的部分。
type ssave struct {
	validSave bool // is or was a part of an actual ss.     // 是或曾是原来 ss 的一部分。
	nlIsEnd   bool // whether newline terminates scan       // 终止扫描的地方是否为换行符
	nlIsSpace bool // whether newline counts as white space // 换行符是否计为空白符
	argLimit  int  // max value of ss.count for this arg; argLimit <= limit // 该字段为 ss.count 的最大值；argLimit <= limit
	limit     int  // max value of ss.count.                // ss.count 的最大值
	maxWid    int  // width of this field.                  // 该字段的宽度。
}

// The Read method is only in ScanState so that ScanState
// satisfies io.Reader. It will never be called when used as
// intended, so there is no need to make it actually work.

// Read 方法只在满足 io.Reader 的 ScanState 中。它故意被设计为永远不会被调用。
// 因此无需让它真正地工作。
func (s *ss) Read(buf []byte) (n int, err error) {
	return 0, errors.New("ScanState's Read should not be called. Use ReadRune")
}

func (s *ss) ReadRune() (r rune, size int, err error) {
	if s.peekRune >= 0 {
		s.count++
		r = s.peekRune
		size = utf8.RuneLen(r)
		s.prevRune = r
		s.peekRune = -1
		return
	}
	if s.atEOF || s.nlIsEnd && s.prevRune == '\n' || s.count >= s.argLimit {
		err = io.EOF
		return
	}

	r, size, err = s.rr.ReadRune()
	if err == nil {
		s.count++
		s.prevRune = r
	} else if err == io.EOF {
		s.atEOF = true
	}
	return
}

func (s *ss) Width() (wid int, ok bool) {
	if s.maxWid == hugeWid {
		return 0, false
	}
	return s.maxWid, true
}

// The public method returns an error; this private one panics.
// If getRune reaches EOF, the return value is EOF (-1).

// 公共的方法返回一个错误；私有的则会panic。
// 若 getRune 读到了 EOF，其返回值就是 EOF (-1)。
func (s *ss) getRune() (r rune) {
	r, _, err := s.ReadRune()
	if err != nil {
		if err == io.EOF {
			return eof
		}
		s.error(err)
	}
	return
}

// mustReadRune turns io.EOF into a panic(io.ErrUnexpectedEOF).
// It is called in cases such as string scanning where an EOF is a
// syntax error.

// mustReadRune 将 io.EOF 转为 panic(io.ErrUnexpectedEOF)。
// 例如在字符串内扫描到 EOF 为语法错误时，它就会被调用。
func (s *ss) mustReadRune() (r rune) {
	r = s.getRune()
	if r == eof {
		s.error(io.ErrUnexpectedEOF)
	}
	return
}

func (s *ss) UnreadRune() error {
	if u, ok := s.rr.(runeUnreader); ok {
		u.UnreadRune()
	} else {
		s.peekRune = s.prevRune
	}
	s.prevRune = -1
	s.count--
	return nil
}

func (s *ss) error(err error) {
	panic(scanError{err})
}

func (s *ss) errorString(err string) {
	panic(scanError{errors.New(err)})
}

func (s *ss) Token(skipSpace bool, f func(rune) bool) (tok []byte, err error) {
	defer func() {
		if e := recover(); e != nil {
			if se, ok := e.(scanError); ok {
				err = se.err
			} else {
				panic(e)
			}
		}
	}()
	if f == nil {
		f = notSpace
	}
	s.buf = s.buf[:0]
	tok = s.token(skipSpace, f)
	return
}

// space is a copy of the unicode.White_Space ranges,
// to avoid depending on package unicode.

// space 为 unicode.White_Space 区域的副本，以此避免对 unicode 包的依赖。
var space = [][2]uint16{
	{0x0009, 0x000d},
	{0x0020, 0x0020},
	{0x0085, 0x0085},
	{0x00a0, 0x00a0},
	{0x1680, 0x1680},
	{0x2000, 0x200a},
	{0x2028, 0x2029},
	{0x202f, 0x202f},
	{0x205f, 0x205f},
	{0x3000, 0x3000},
}

func isSpace(r rune) bool {
	if r >= 1<<16 {
		return false
	}
	rx := uint16(r)
	for _, rng := range space {
		if rx < rng[0] {
			return false
		}
		if rx <= rng[1] {
			return true
		}
	}
	return false
}

// notSpace is the default scanning function used in Token.

// notSpace 为用在 Token 中的默认扫描函数。
func notSpace(r rune) bool {
	return !isSpace(r)
}

// SkipSpace provides Scan methods the ability to skip space and newline
// characters in keeping with the current scanning mode set by format strings
// and Scan/Scanln.

// SkipSpace 让 Scan 方法可根据由当前格式字符串和 Scan/Scanln
// 设置的扫描模式跳过空格和换行符。
func (s *ss) SkipSpace() {
	s.skipSpace(false)
}

// readRune is a structure to enable reading UTF-8 encoded code points
// from an io.Reader.  It is used if the Reader given to the scanner does
// not already implement io.RuneReader.

// readRune 结构体开启从 io.Reader 中读取以UTF-8编码的码点。若给予扫描器的 Reader
// 并未实现 io.RuneReader，就会使用此结构体。
type readRune struct {
	reader  io.Reader
	buf     [utf8.UTFMax]byte // used only inside ReadRune // 只在 ReadRune 内使用。
	pending int               // number of bytes in pendBuf; only >0 for bad UTF-8 // pendBuf 中的字节数，对于错误的 UTF-8 只会 >0。
	pendBuf [utf8.UTFMax]byte // bytes left over           // 剩余的字节
}

// readByte returns the next byte from the input, which may be
// left over from a previous read if the UTF-8 was ill-formed.

// readByte 从输入中返回下一个字节，若它不合UTF-8规范，就会从上一次读取中留下来。
func (r *readRune) readByte() (b byte, err error) {
	if r.pending > 0 {
		b = r.pendBuf[0]
		copy(r.pendBuf[0:], r.pendBuf[1:])
		r.pending--
		return
	}
	n, err := io.ReadFull(r.reader, r.pendBuf[0:1])
	if n != 1 {
		return 0, err
	}
	return r.pendBuf[0], err
}

// unread saves the bytes for the next read.

// unread 为下一次读取保存字节。
func (r *readRune) unread(buf []byte) {
	copy(r.pendBuf[r.pending:], buf)
	r.pending += len(buf)
}

// ReadRune returns the next UTF-8 encoded code point from the
// io.Reader inside r.

// ReadRune 从 r 的 io.Reader 中返回下一个UTF-8编码的码点。
func (r *readRune) ReadRune() (rr rune, size int, err error) {
	r.buf[0], err = r.readByte()
	if err != nil {
		return 0, 0, err
	}
	if r.buf[0] < utf8.RuneSelf { // fast check for common ASCII case // 快速检测常见的ASCII情况
		rr = rune(r.buf[0])
		return
	}
	var n int
	for n = 1; !utf8.FullRune(r.buf[0:n]); n++ {
		r.buf[n], err = r.readByte()
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return
		}
	}
	rr, size = utf8.DecodeRune(r.buf[0:n])
	if size < n { // an error // 一个错误
		r.unread(r.buf[size:n])
	}
	return
}

var ssFree = sync.Pool{
	New: func() interface{} { return new(ss) },
}

// newScanState allocates a new ss struct or grab a cached one.

// newScanState 分配一个新的 ss 结构体或抓取一个已缓存的。
func newScanState(r io.Reader, nlIsSpace, nlIsEnd bool) (s *ss, old ssave) {
	// If the reader is a *ss, then we've got a recursive
	// call to Scan, so re-use the scan state.
	// 若读取器是一个 *ss，那么我们就得到了一个递归调用的 Scan，这样会重新使用扫描状态。
	s, ok := r.(*ss)
	if ok {
		old = s.ssave
		s.limit = s.argLimit
		s.nlIsEnd = nlIsEnd || s.nlIsEnd
		s.nlIsSpace = nlIsSpace
		return
	}

	s = ssFree.Get().(*ss)
	if rr, ok := r.(io.RuneReader); ok {
		s.rr = rr
	} else {
		s.rr = &readRune{reader: r}
	}
	s.nlIsSpace = nlIsSpace
	s.nlIsEnd = nlIsEnd
	s.prevRune = -1
	s.peekRune = -1
	s.atEOF = false
	s.limit = hugeWid
	s.argLimit = hugeWid
	s.maxWid = hugeWid
	s.validSave = true
	s.count = 0
	return
}

// free saves used ss structs in ssFree; avoid an allocation per invocation.

// free 在 ssFree 中保存已使用的 ss 结构体；避免为每次请求都进行分配。
func (s *ss) free(old ssave) {
	// If it was used recursively, just restore the old state.
	// 如果要递归地使用，只需重新存储旧的状态即可。
	if old.validSave {
		s.ssave = old
		return
	}
	// Don't hold on to ss structs with large buffers.
	// 不让 ss 结构体保存大型缓存。
	if cap(s.buf) > 1024 {
		return
	}
	s.buf = s.buf[:0]
	s.rr = nil
	ssFree.Put(s)
}

// skipSpace skips spaces and maybe newlines.

// skipSpace 跳过空格，还可能跳过换行符。
func (s *ss) skipSpace(stopAtNewline bool) {
	for {
		r := s.getRune()
		if r == eof {
			return
		}
		if r == '\r' && s.peek("\n") {
			continue
		}
		if r == '\n' {
			if stopAtNewline {
				break
			}
			if s.nlIsSpace {
				continue
			}
			s.errorString("unexpected newline")
			return
		}
		if !isSpace(r) {
			s.UnreadRune()
			break
		}
	}
}

// token returns the next space-delimited string from the input.  It
// skips white space.  For Scanln, it stops at newlines.  For Scan,
// newlines are treated as spaces.

// token 从输入中返回下一个以空格分隔的字符串。它会跳过空白符。
// 对于 Scanln，它会在换行符处停止。对于 Scan，换行符会被视作空格。
func (s *ss) token(skipSpace bool, f func(rune) bool) []byte {
	if skipSpace {
		s.skipSpace(false)
	}
	// read until white space or newline
	// 读取直到遇见空白符或回车符。
	for {
		r := s.getRune()
		if r == eof {
			break
		}
		if !f(r) {
			s.UnreadRune()
			break
		}
		s.buf.WriteRune(r)
	}
	return s.buf
}

var complexError = errors.New("syntax error scanning complex number")
var boolError = errors.New("syntax error scanning boolean")

func indexRune(s string, r rune) int {
	for i, c := range s {
		if c == r {
			return i
		}
	}
	return -1
}

// consume reads the next rune in the input and reports whether it is in the ok string.
// If accept is true, it puts the character into the input token.

// consume 从输入中读取下一个符文并报告它是否在 ok 字符串中。
func (s *ss) consume(ok string, accept bool) bool {
	r := s.getRune()
	if r == eof {
		return false
	}
	if indexRune(ok, r) >= 0 {
		if accept {
			s.buf.WriteRune(r)
		}
		return true
	}
	if r != eof && accept {
		s.UnreadRune()
	}
	return false
}

// peek reports whether the next character is in the ok string, without consuming it.

// peek 报告下一个字符是否在 ok 字符串中，而不销毁它。
func (s *ss) peek(ok string) bool {
	r := s.getRune()
	if r != eof {
		s.UnreadRune()
	}
	return indexRune(ok, r) >= 0
}

func (s *ss) notEOF() {
	// Guarantee there is data to be read.
	// 保证数据被读取。
	if r := s.getRune(); r == eof {
		panic(io.EOF)
	}
	s.UnreadRune()
}

// accept checks the next rune in the input.  If it's a byte (sic) in the string, it puts it in the
// buffer and returns true. Otherwise it return false.

// accept 从输入中检查下一个符文。若它在此字符串中（的原文）是一个字节，
// 此方法就会将它放到缓存中并返回 true。否则它会返回 false。
func (s *ss) accept(ok string) bool {
	return s.consume(ok, true)
}

// okVerb verifies that the verb is present in the list, setting s.err appropriately if not.

// okVerb 验证占位符是否在列表中，若否，则设置适当的 s.err。
func (s *ss) okVerb(verb rune, okVerbs, typ string) bool {
	for _, v := range okVerbs {
		if v == verb {
			return true
		}
	}
	s.errorString("bad verb %" + string(verb) + " for " + typ)
	return false
}

// scanBool returns the value of the boolean represented by the next token.

// scanBool 返回下一个标记所表示的布尔值。
func (s *ss) scanBool(verb rune) bool {
	s.skipSpace(false)
	s.notEOF()
	if !s.okVerb(verb, "tv", "boolean") {
		return false
	}
	// Syntax-checking a boolean is annoying.  We're not fastidious about case.
	// 对布尔值进行语法检查是很讨厌的。我们并不苛求各种情况。
	switch s.getRune() {
	case '0':
		return false
	case '1':
		return true
	case 't', 'T':
		if s.accept("rR") && (!s.accept("uU") || !s.accept("eE")) {
			s.error(boolError)
		}
		return true
	case 'f', 'F':
		if s.accept("aA") && (!s.accept("lL") || !s.accept("sS") || !s.accept("eE")) {
			s.error(boolError)
		}
		return false
	}
	return false
}

// Numerical elements

// 数值元素。
const (
	binaryDigits      = "01"
	octalDigits       = "01234567"
	decimalDigits     = "0123456789"
	hexadecimalDigits = "0123456789aAbBcCdDeEfF"
	sign              = "+-"
	period            = "."
	exponent          = "eEp"
)

// getBase returns the numeric base represented by the verb and its digit string.

// getBase 返回由占位符及其数字串 digits 所表示的数值进制 base。
func (s *ss) getBase(verb rune) (base int, digits string) {
	s.okVerb(verb, "bdoUxXv", "integer") // sets s.err // 设置 s.err
	base = 10
	digits = decimalDigits
	switch verb {
	case 'b':
		base = 2
		digits = binaryDigits
	case 'o':
		base = 8
		digits = octalDigits
	case 'x', 'X', 'U':
		base = 16
		digits = hexadecimalDigits
	}
	return
}

// scanNumber returns the numerical string with specified digits starting here.

// scanNumber 返回数值字符串及从此处开始的指定的数字 digits。
func (s *ss) scanNumber(digits string, haveDigits bool) string {
	if !haveDigits {
		s.notEOF()
		if !s.accept(digits) {
			s.errorString("expected integer")
		}
	}
	for s.accept(digits) {
	}
	return string(s.buf)
}

// scanRune returns the next rune value in the input.

// scanRune 返回输入中的下一个符文值。
func (s *ss) scanRune(bitSize int) int64 {
	s.notEOF()
	r := int64(s.getRune())
	n := uint(bitSize)
	x := (r << (64 - n)) >> (64 - n)
	if x != r {
		s.errorString("overflow on character value " + string(r))
	}
	return r
}

// scanBasePrefix reports whether the integer begins with a 0 or 0x,
// and returns the base, digit string, and whether a zero was found.
// It is called only if the verb is %v.

// scanBasePrefix 报告该整数是否以 0 或 0x 开头，并返回其进制 base，数字串 digit
// 以及是否找到零 found。
func (s *ss) scanBasePrefix() (base int, digits string, found bool) {
	if !s.peek("0") {
		return 10, decimalDigits, false
	}
	s.accept("0")
	found = true // We've put a digit into the token buffer. // 我们将一个数字放到标记缓存里
	// Special cases for '0' && '0x'
	// “0”和“0x”的特殊情况
	base, digits = 8, octalDigits
	if s.peek("xX") {
		s.consume("xX", false)
		base, digits = 16, hexadecimalDigits
	}
	return
}

// scanInt returns the value of the integer represented by the next
// token, checking for overflow.  Any error is stored in s.err.

// scanInt 返回由下一个标记表示的整数值并检测溢出。任何错误都会存入 s.err。
func (s *ss) scanInt(verb rune, bitSize int) int64 {
	if verb == 'c' {
		return s.scanRune(bitSize)
	}
	s.skipSpace(false)
	s.notEOF()
	base, digits := s.getBase(verb)
	haveDigits := false
	if verb == 'U' {
		if !s.consume("U", false) || !s.consume("+", false) {
			s.errorString("bad unicode format ")
		}
	} else {
		s.accept(sign) // If there's a sign, it will be left in the token buffer. // 如果有符号，它就会被留在标记缓存内。
		if verb == 'v' {
			base, digits, haveDigits = s.scanBasePrefix()
		}
	}
	tok := s.scanNumber(digits, haveDigits)
	i, err := strconv.ParseInt(tok, base, 64)
	if err != nil {
		s.error(err)
	}
	n := uint(bitSize)
	x := (i << (64 - n)) >> (64 - n)
	if x != i {
		s.errorString("integer overflow on token " + tok)
	}
	return i
}

// scanUint returns the value of the unsigned integer represented
// by the next token, checking for overflow.  Any error is stored in s.err.

// scanUint 返回由下一个标记表示的无符号整数值并检测溢出。任何错误都会存入 s.err。
func (s *ss) scanUint(verb rune, bitSize int) uint64 {
	if verb == 'c' {
		return uint64(s.scanRune(bitSize))
	}
	s.skipSpace(false)
	s.notEOF()
	base, digits := s.getBase(verb)
	haveDigits := false
	if verb == 'U' {
		if !s.consume("U", false) || !s.consume("+", false) {
			s.errorString("bad unicode format ")
		}
	} else if verb == 'v' {
		base, digits, haveDigits = s.scanBasePrefix()
	}
	tok := s.scanNumber(digits, haveDigits)
	i, err := strconv.ParseUint(tok, base, 64)
	if err != nil {
		s.error(err)
	}
	n := uint(bitSize)
	x := (i << (64 - n)) >> (64 - n)
	if x != i {
		s.errorString("unsigned integer overflow on token " + tok)
	}
	return i
}

// floatToken returns the floating-point number starting here, no longer than swid
// if the width is specified. It's not rigorous about syntax because it doesn't check that
// we have at least some digits, but Atof will do that.

// floatToken 返回从此处开始的浮点数，若宽度已指定，就不会比 s.wid 更长。
// 它对语法并不严格，因为它不会检查我们有没有数字，不过 convertFloat 会检查它的。
func (s *ss) floatToken() string {
	s.buf = s.buf[:0]
	// NaN?              // 非数值？
	if s.accept("nN") && s.accept("aA") && s.accept("nN") {
		return string(s.buf)
	}
	// leading sign?     // 前导正负号？
	s.accept(sign)
	// Inf?              // 无穷大？
	if s.accept("iI") && s.accept("nN") && s.accept("fF") {
		return string(s.buf)
	}
	// digits?           // 数字？
	for s.accept(decimalDigits) {
	}
	// decimal point?    // 小数点？
	if s.accept(period) {
		// fraction?     // 小数？
		for s.accept(decimalDigits) {
		}
	}
	// exponent?         // 指数？
	if s.accept(exponent) {
		// leading sign? // 前导正负号？
		s.accept(sign)
		// digits?       // 数字？
		for s.accept(decimalDigits) {
		}
	}
	return string(s.buf)
}

// complexTokens returns the real and imaginary parts of the complex number starting here.
// The number might be parenthesized and has the format (N+Ni) where N is a floating-point
// number and there are no spaces within.

// complexTokens 返回从此处开始的复数的实部和虚部。该数字可能被括号括住且格式为
// (N+Ni)，其中 N 是一个浮点数且其间没有空格。
func (s *ss) complexTokens() (real, imag string) {
	// TODO: accept N and Ni independently?
	// TODO: 分别接受 N 和 Ni？
	parens := s.accept("(")
	real = s.floatToken()
	s.buf = s.buf[:0]
	// Must now have a sign.
	// 现在必须有正负号。
	if !s.accept("+-") {
		s.error(complexError)
	}
	// Sign is now in buffer
	// 现在正负号在缓存中
	imagSign := string(s.buf)
	imag = s.floatToken()
	if !s.accept("i") {
		s.error(complexError)
	}
	if parens && !s.accept(")") {
		s.error(complexError)
	}
	return real, imagSign + imag
}

// convertFloat converts the string to a float64value.

// convertFloat 将字符串转换为 float64 值。
func (s *ss) convertFloat(str string, n int) float64 {
	if p := indexRune(str, 'p'); p >= 0 {
		// Atof doesn't handle power-of-2 exponents,
		// but they're easy to evaluate.
		// ParseFloat 不会处理2的幂的指数，但它们很容易求值。
		f, err := strconv.ParseFloat(str[:p], n)
		if err != nil {
			// Put full string into error.
			// 将整个字符串转为错误。
			if e, ok := err.(*strconv.NumError); ok {
				e.Num = str
			}
			s.error(err)
		}
		m, err := strconv.Atoi(str[p+1:])
		if err != nil {
			// Put full string into error.
			// 将整个字符串转为错误。
			if e, ok := err.(*strconv.NumError); ok {
				e.Num = str
			}
			s.error(err)
		}
		return math.Ldexp(f, m)
	}
	f, err := strconv.ParseFloat(str, n)
	if err != nil {
		s.error(err)
	}
	return f
}

// convertComplex converts the next token to a complex128 value.
// The atof argument is a type-specific reader for the underlying type.
// If we're reading complex64, atof will parse float32s and convert them
// to float64's to avoid reproducing this code for each complex type.

// convertComplex 将下一个标记转换为 complex128 值。
// 若我们读取到 complex64，convertFloat 会将它解析成 float32 并转换为 float64，
// 以此避免为每个复数类型重复此代码。
func (s *ss) scanComplex(verb rune, n int) complex128 {
	if !s.okVerb(verb, floatVerbs, "complex") {
		return 0
	}
	s.skipSpace(false)
	s.notEOF()
	sreal, simag := s.complexTokens()
	real := s.convertFloat(sreal, n/2)
	imag := s.convertFloat(simag, n/2)
	return complex(real, imag)
}

// convertString returns the string represented by the next input characters.
// The format of the input is determined by the verb.

// convertString 返回由下一次输入的字符表示的字符串。
// 输入的格式由占位符 verb 决定。
func (s *ss) convertString(verb rune) (str string) {
	if !s.okVerb(verb, "svqx", "string") {
		return ""
	}
	s.skipSpace(false)
	s.notEOF()
	switch verb {
	case 'q':
		str = s.quotedString()
	case 'x':
		str = s.hexString()
	default:
		str = string(s.token(true, notSpace)) // %s and %v just return the next word // %s 和 %v 只返回下一个单词
	}
	return
}

// quotedString returns the double- or back-quoted string represented by the next input characters.

// quotedString 返回由下一次输入的字符表示的，以双引号或反引号围起的字符串。
func (s *ss) quotedString() string {
	s.notEOF()
	quote := s.getRune()
	switch quote {
	case '`':
		// Back-quoted: Anything goes until EOF or back quote.
		// 反引号围绕的：遇到任何东西都会继续直到 EOF 或反引号。
		for {
			r := s.mustReadRune()
			if r == quote {
				break
			}
			s.buf.WriteRune(r)
		}
		return string(s.buf)
	case '"':
		// Double-quoted: Include the quotes and let strconv.Unquote do the backslash escapes.
		// 双引号围绕的：包括该引号并让 strconv.Unquote 进行反斜杠转义。
		s.buf.WriteRune(quote)
		for {
			r := s.mustReadRune()
			s.buf.WriteRune(r)
			if r == '\\' {
				// In a legal backslash escape, no matter how long, only the character
				// immediately after the escape can itself be a backslash or quote.
				// Thus we only need to protect the first character after the backslash.
				//
				// 在一个合法的反斜杠转义中，无论多长，
				// 只有紧跟反斜杠之后的转义本身才可以是反斜杠或引号。
				// 因此我们之需要保证第一个字符在反斜杠之后即可。
				s.buf.WriteRune(s.mustReadRune())
			} else if r == '"' {
				break
			}
		}
		result, err := strconv.Unquote(string(s.buf))
		if err != nil {
			s.error(err)
		}
		return result
	default:
		s.errorString("expected quoted string")
	}
	return ""
}

// hexDigit returns the value of the hexadecimal digit

// hexDigit 返回十六进制数字的值
func (s *ss) hexDigit(d rune) int {
	digit := int(d)
	switch digit {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return digit - '0'
	case 'a', 'b', 'c', 'd', 'e', 'f':
		return 10 + digit - 'a'
	case 'A', 'B', 'C', 'D', 'E', 'F':
		return 10 + digit - 'A'
	}
	s.errorString("illegal hex digit")
	return 0
}

// hexByte returns the next hex-encoded (two-character) byte from the input.
// There must be either two hexadecimal digits or a space character in the input.

// hexByte 从输入中返回下一个以十六进制编码的（两个字符的）字节。
// 输入中必须为两个十六进制数字或一个空格字符。
func (s *ss) hexByte() (b byte, ok bool) {
	rune1 := s.getRune()
	if rune1 == eof {
		return
	}
	if isSpace(rune1) {
		s.UnreadRune()
		return
	}
	rune2 := s.mustReadRune()
	return byte(s.hexDigit(rune1)<<4 | s.hexDigit(rune2)), true
}

// hexString returns the space-delimited hexpair-encoded string.

// hexString 返回以空格分隔的，十六进制对编码的字符串。
func (s *ss) hexString() string {
	s.notEOF()
	for {
		b, ok := s.hexByte()
		if !ok {
			break
		}
		s.buf.WriteByte(b)
	}
	if len(s.buf) == 0 {
		s.errorString("no hex data for %x string")
		return ""
	}
	return string(s.buf)
}

const floatVerbs = "beEfFgGv"

const hugeWid = 1 << 30

// scanOne scans a single value, deriving the scanner from the type of the argument.

// scanOne 扫描单个值，从实参的类型导出扫描器。
func (s *ss) scanOne(verb rune, arg interface{}) {
	s.buf = s.buf[:0]
	var err error
	// If the parameter has its own Scan method, use that.
	// 若该形参有它自己的 Scan 方法，就用它。
	if v, ok := arg.(Scanner); ok {
		err = v.Scan(s, verb)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			s.error(err)
		}
		return
	}

	switch v := arg.(type) {
	case *bool:
		*v = s.scanBool(verb)
	case *complex64:
		*v = complex64(s.scanComplex(verb, 64))
	case *complex128:
		*v = s.scanComplex(verb, 128)
	case *int:
		*v = int(s.scanInt(verb, intBits))
	case *int8:
		*v = int8(s.scanInt(verb, 8))
	case *int16:
		*v = int16(s.scanInt(verb, 16))
	case *int32:
		*v = int32(s.scanInt(verb, 32))
	case *int64:
		*v = s.scanInt(verb, 64)
	case *uint:
		*v = uint(s.scanUint(verb, intBits))
	case *uint8:
		*v = uint8(s.scanUint(verb, 8))
	case *uint16:
		*v = uint16(s.scanUint(verb, 16))
	case *uint32:
		*v = uint32(s.scanUint(verb, 32))
	case *uint64:
		*v = s.scanUint(verb, 64)
	case *uintptr:
		*v = uintptr(s.scanUint(verb, uintptrBits))
	// Floats are tricky because you want to scan in the precision of the result, not
	// scan in high precision and convert, in order to preserve the correct error condition.
	// 浮点数很棘手，因为你为了保存恰当的错误条件，需要在结果的精度中扫描，而不是在高精度中扫描并转换。
	case *float32:
		if s.okVerb(verb, floatVerbs, "float32") {
			s.skipSpace(false)
			s.notEOF()
			*v = float32(s.convertFloat(s.floatToken(), 32))
		}
	case *float64:
		if s.okVerb(verb, floatVerbs, "float64") {
			s.skipSpace(false)
			s.notEOF()
			*v = s.convertFloat(s.floatToken(), 64)
		}
	case *string:
		*v = s.convertString(verb)
	case *[]byte:
		// We scan to string and convert so we get a copy of the data.
		// If we scanned to bytes, the slice would point at the buffer.
		// 我们将它扫描为字符串并转换，因此我们获取该数据的一份副本。
		// 若我们将它扫描为字节，该切片就会指向缓存。
		*v = []byte(s.convertString(verb))
	default:
		val := reflect.ValueOf(v)
		ptr := val
		if ptr.Kind() != reflect.Ptr {
			s.errorString("type not a pointer: " + val.Type().String())
			return
		}
		switch v := ptr.Elem(); v.Kind() {
		case reflect.Bool:
			v.SetBool(s.scanBool(verb))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v.SetInt(s.scanInt(verb, v.Type().Bits()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			v.SetUint(s.scanUint(verb, v.Type().Bits()))
		case reflect.String:
			v.SetString(s.convertString(verb))
		case reflect.Slice:
			// For now, can only handle (renamed) []byte.
			// 现在，只能处理（重命名的）[]byte。
			typ := v.Type()
			if typ.Elem().Kind() != reflect.Uint8 {
				s.errorString("can't scan type: " + val.Type().String())
			}
			str := s.convertString(verb)
			v.Set(reflect.MakeSlice(typ, len(str), len(str)))
			for i := 0; i < len(str); i++ {
				v.Index(i).SetUint(uint64(str[i]))
			}
		case reflect.Float32, reflect.Float64:
			s.skipSpace(false)
			s.notEOF()
			v.SetFloat(s.convertFloat(s.floatToken(), v.Type().Bits()))
		case reflect.Complex64, reflect.Complex128:
			v.SetComplex(s.scanComplex(verb, v.Type().Bits()))
		default:
			s.errorString("can't scan type: " + val.Type().String())
		}
	}
}

// errorHandler turns local panics into error returns.

// errorHandler 将局部panic转为错误返回。
func errorHandler(errp *error) {
	if e := recover(); e != nil {
		if se, ok := e.(scanError); ok { // catch local error // 捕获本地错误
			*errp = se.err
		} else if eof, ok := e.(error); ok && eof == io.EOF { // out of input // 超出输入
			*errp = eof
		} else {
			panic(e)
		}
	}
}

// doScan does the real work for scanning without a format string.

// doScan 进行真正的扫描工作而无需格式字符串。
func (s *ss) doScan(a []interface{}) (numProcessed int, err error) {
	defer errorHandler(&err)
	for _, arg := range a {
		s.scanOne('v', arg)
		numProcessed++
	}
	// Check for newline if required.
	// 根据需要检查换行符。
	if !s.nlIsSpace {
		for {
			r := s.getRune()
			if r == '\n' || r == eof {
				break
			}
			if !isSpace(r) {
				s.errorString("expected newline")
				break
			}
		}
	}
	return
}

// advance determines whether the next characters in the input match
// those of the format.  It returns the number of bytes (sic) consumed
// in the format. Newlines included, all runs of space characters in
// either input or format behave as a single space. This routine also
// handles the %% case.  If the return value is zero, either format
// starts with a % (with no following %) or the input is empty.
// If it is negative, the input did not match the string.

// advance 判断输入中的下一个字符是否匹配那些格式 format。
// 它返回该格式（原文）中消耗的字节数。包括换行符，在输入或格式中的连续空白字符
// 都视为单个空格。此程序也处理 %% 的情况。若返回值为零，那么不是格式起始于
// %（没有后跟 %）就是输入为空。若它是复数，则输入不匹配该字符串。
func (s *ss) advance(format string) (i int) {
	for i < len(format) {
		fmtc, w := utf8.DecodeRuneInString(format[i:])
		if fmtc == '%' {
			// %% acts like a real percent
			nextc, _ := utf8.DecodeRuneInString(format[i+w:]) // will not match % if string is empty // 若字符串为空则不会匹配 %
			if nextc != '%' {
				return
			}
			i += w // skip the first % // 跳过第一个 %
		}
		sawSpace := false
		for isSpace(fmtc) && i < len(format) {
			sawSpace = true
			i += w
			fmtc, w = utf8.DecodeRuneInString(format[i:])
		}
		if sawSpace {
			// There was space in the format, so there should be space (EOF)
			// in the input.
			// 格式中有空格，因此输入中也应该有空格（EOF）
			inputc := s.getRune()
			if inputc == eof || inputc == '\n' {
				// If we've reached a newline, stop now; don't read ahead.
				// 若我们遇到换行符，就立即停止，不继续读取。
				return
			}
			if !isSpace(inputc) {
				// Space in format but not in input: error
				// 格式中有空格但输入中没有：错误
				s.errorString("expected space in input to match format")
			}
			s.skipSpace(true)
			continue
		}
		inputc := s.mustReadRune()
		if fmtc != inputc {
			s.UnreadRune()
			return -1
		}
		i += w
	}
	return
}

// doScanf does the real work when scanning with a format string.
//  At the moment, it handles only pointers to basic types.

// doScanf 根据格式字符串进行真正的扫描工作。
// 目前，它只能处理指向基本类型的指针。
func (s *ss) doScanf(format string, a []interface{}) (numProcessed int, err error) {
	defer errorHandler(&err)
	end := len(format) - 1
	// We process one item per non-trivial format
	// 我们为每个非平凡的格式处理一个条目
	for i := 0; i <= end; {
		w := s.advance(format[i:])
		if w > 0 {
			i += w
			continue
		}
		// Either we failed to advance, we have a percent character, or we ran out of input.
		// 我们要么无法继续，要么有一个百分号，或用尽输入。
		if format[i] != '%' {
			// Can't advance format.  Why not?
			// 不能继续格式化。为啥？因为输入不匹配格式。
			if w < 0 {
				s.errorString("input does not match format")
			}
			// Otherwise at EOF; "too many operands" error handled below
			// 否则就是遇到了 EOF；以下为“太多操作数”的错误处理
			break
		}
		i++ // % is one byte // % 是一个字节

		// do we have 20 (width)?
		// 我们有没有20个（宽度）？
		var widPresent bool
		s.maxWid, widPresent, i = parsenum(format, i, end)
		if !widPresent {
			s.maxWid = hugeWid
		}
		s.argLimit = s.limit
		if f := s.count + s.maxWid; f < s.argLimit {
			s.argLimit = f
		}

		c, w := utf8.DecodeRuneInString(format[i:])
		i += w

		if numProcessed >= len(a) { // out of operands // 超过操作数
			s.errorString("too few operands for format %" + format[i-w:])
			break
		}
		arg := a[numProcessed]

		s.scanOne(c, arg)
		numProcessed++
		s.argLimit = s.limit
	}
	if numProcessed < len(a) {
		s.errorString("too many operands")
	}
	return
}
