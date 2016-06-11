// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"runtime/internal/atomic"
	"unsafe"
)

// The constant is known to the compiler.
// There is no fundamental theory behind this number.
const tmpStringBufSize = 32

type tmpBuf [tmpStringBufSize]byte

// concatstrings implements a Go string concatenation x+y+z+...
// The operands are passed in the slice a.
// If buf != nil, the compiler has determined that the result does not
// escape the calling function, so the string data can be stored in buf
// if small enough.
func concatstrings(buf *tmpBuf, a []string) string {
	idx := 0
	l := 0
	count := 0
	for i, x := range a {
		n := len(x)
		if n == 0 {
			continue
		}
		if l+n < l {
			throw("string concatenation too long")
		}
		l += n
		count++
		idx = i
	}
	if count == 0 {
		return ""
	}

	// If there is just one string and either it is not on the stack
	// or our result does not escape the calling frame (buf != nil),
	// then we can return that string directly.
	if count == 1 && (buf != nil || !stringDataOnStack(a[idx])) {
		return a[idx]
	}
	s, b := rawstringtmp(buf, l)
	l = 0
	for _, x := range a {
		copy(b[l:], x)
		l += len(x)
	}
	return s
}

func concatstring2(buf *tmpBuf, a [2]string) string {
	return concatstrings(buf, a[:])
}

func concatstring3(buf *tmpBuf, a [3]string) string {
	return concatstrings(buf, a[:])
}

func concatstring4(buf *tmpBuf, a [4]string) string {
	return concatstrings(buf, a[:])
}

func concatstring5(buf *tmpBuf, a [5]string) string {
	return concatstrings(buf, a[:])
}

// Buf is a fixed-size buffer for the result,
// it is not nil if the result does not escape.
func slicebytetostring(buf *tmpBuf, b []byte) string {
	l := len(b)
	if l == 0 {
		// Turns out to be a relatively common case.
		// Consider that you want to parse out data between parens in "foo()bar",
		// you find the indices and convert the subslice to string.
		return ""
	}
	if raceenabled && l > 0 {
		racereadrangepc(unsafe.Pointer(&b[0]),
			uintptr(l),
			getcallerpc(unsafe.Pointer(&buf)),
			funcPC(slicebytetostring))
	}
	if msanenabled && l > 0 {
		msanread(unsafe.Pointer(&b[0]), uintptr(l))
	}
	s, c := rawstringtmp(buf, l)
	copy(c, b)
	return s
}

// stringDataOnStack reports whether the string's data is
// stored on the current goroutine's stack.
func stringDataOnStack(s string) bool {
	ptr := uintptr(stringStructOf(&s).str)
	stk := getg().stack
	return stk.lo <= ptr && ptr < stk.hi
}

func rawstringtmp(buf *tmpBuf, l int) (s string, b []byte) {
	if buf != nil && l <= len(buf) {
		b = buf[:l]
		s = slicebytetostringtmp(b)
	} else {
		s, b = rawstring(l)
	}
	return
}

func slicebytetostringtmp(b []byte) string {
	// Return a "string" referring to the actual []byte bytes.
	// This is only for use by internal compiler optimizations
	// that know that the string form will be discarded before
	// the calling goroutine could possibly modify the original
	// slice or synchronize with another goroutine.
	// First such case is a m[string(k)] lookup where
	// m is a string-keyed map and k is a []byte.
	// Second such case is "<"+string(b)+">" concatenation where b is []byte.
	// Third such case is string(b)=="foo" comparison where b is []byte.

	if raceenabled && len(b) > 0 {
		racereadrangepc(unsafe.Pointer(&b[0]),
			uintptr(len(b)),
			getcallerpc(unsafe.Pointer(&b)),
			funcPC(slicebytetostringtmp))
	}
	if msanenabled && len(b) > 0 {
		msanread(unsafe.Pointer(&b[0]), uintptr(len(b)))
	}
	return *(*string)(unsafe.Pointer(&b))
}

func stringtoslicebyte(buf *tmpBuf, s string) []byte {
	var b []byte
	if buf != nil && len(s) <= len(buf) {
		*buf = tmpBuf{}
		b = buf[:len(s)]
	} else {
		b = rawbyteslice(len(s))
	}
	copy(b, s)
	return b
}

func stringtoslicebytetmp(s string) []byte {
	// Return a slice referring to the actual string bytes.
	// This is only for use by internal compiler optimizations
	// that know that the slice won't be mutated.
	// The only such case today is:
	// for i, c := range []byte(str)

	str := stringStructOf(&s)
	ret := slice{array: str.str, len: str.len, cap: str.len}
	return *(*[]byte)(unsafe.Pointer(&ret))
}

func stringtoslicerune(buf *[tmpStringBufSize]rune, s string) []rune {
	// two passes.
	// unlike slicerunetostring, no race because strings are immutable.
	n := 0
	t := s
	for len(s) > 0 {
		_, k := charntorune(s)
		s = s[k:]
		n++
	}
	var a []rune
	if buf != nil && n <= len(buf) {
		*buf = [tmpStringBufSize]rune{}
		a = buf[:n]
	} else {
		a = rawruneslice(n)
	}
	n = 0
	for len(t) > 0 {
		r, k := charntorune(t)
		t = t[k:]
		a[n] = r
		n++
	}
	return a
}

func slicerunetostring(buf *tmpBuf, a []rune) string {
	if raceenabled && len(a) > 0 {
		racereadrangepc(unsafe.Pointer(&a[0]),
			uintptr(len(a))*unsafe.Sizeof(a[0]),
			getcallerpc(unsafe.Pointer(&buf)),
			funcPC(slicerunetostring))
	}
	if msanenabled && len(a) > 0 {
		msanread(unsafe.Pointer(&a[0]), uintptr(len(a))*unsafe.Sizeof(a[0]))
	}
	var dum [4]byte
	size1 := 0
	for _, r := range a {
		size1 += runetochar(dum[:], r)
	}
	s, b := rawstringtmp(buf, size1+3)
	size2 := 0
	for _, r := range a {
		// check for race
		if size2 >= size1 {
			break
		}
		size2 += runetochar(b[size2:], r)
	}
	return s[:size2]
}

type stringStruct struct {
	str unsafe.Pointer
	len int
}

// Variant with *byte pointer type for DWARF debugging.
type stringStructDWARF struct {
	str *byte
	len int
}

func stringStructOf(sp *string) *stringStruct {
	return (*stringStruct)(unsafe.Pointer(sp))
}

func intstring(buf *[4]byte, v int64) string {
	var s string
	var b []byte
	if buf != nil {
		b = buf[:]
		s = slicebytetostringtmp(b)
	} else {
		s, b = rawstring(4)
	}
	if int64(rune(v)) != v {
		v = runeerror
	}
	n := runetochar(b, rune(v))
	return s[:n]
}

// stringiter returns the index of the next
// rune after the rune that starts at s[k].
func stringiter(s string, k int) int {
	if k >= len(s) {
		// 0 is end of iteration
		return 0
	}

	c := s[k]
	if c < runeself {
		return k + 1
	}

	// multi-char rune
	_, n := charntorune(s[k:])
	return k + n
}

// stringiter2 returns the rune that starts at s[k]
// and the index where the next rune starts.
func stringiter2(s string, k int) (int, rune) {
	if k >= len(s) {
		// 0 is end of iteration
		return 0, 0
	}

	c := s[k]
	if c < runeself {
		return k + 1, rune(c)
	}

	// multi-char rune
	r, n := charntorune(s[k:])
	return k + n, r
}

// rawstring allocates storage for a new string. The returned
// string and byte slice both refer to the same storage.
// The storage is not zeroed. Callers should use
// b to set the string contents and then drop b.
func rawstring(size int) (s string, b []byte) {
	p := mallocgc(uintptr(size), nil, false)

	stringStructOf(&s).str = p
	stringStructOf(&s).len = size

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, size}

	for {
		ms := maxstring
		if uintptr(size) <= ms || atomic.Casuintptr((*uintptr)(unsafe.Pointer(&maxstring)), ms, uintptr(size)) {
			return
		}
	}
}

// rawbyteslice allocates a new byte slice. The byte slice is not zeroed.
func rawbyteslice(size int) (b []byte) {
	cap := roundupsize(uintptr(size))
	p := mallocgc(cap, nil, false)
	if cap != uintptr(size) {
		memclr(add(p, uintptr(size)), cap-uintptr(size))
	}

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, int(cap)}
	return
}

// rawruneslice allocates a new rune slice. The rune slice is not zeroed.
func rawruneslice(size int) (b []rune) {
	if uintptr(size) > _MaxMem/4 {
		throw("out of memory")
	}
	mem := roundupsize(uintptr(size) * 4)
	p := mallocgc(mem, nil, false)
	if mem != uintptr(size)*4 {
		memclr(add(p, uintptr(size)*4), mem-uintptr(size)*4)
	}

	*(*slice)(unsafe.Pointer(&b)) = slice{p, size, int(mem / 4)}
	return
}

// used by cmd/cgo
func gobytes(p *byte, n int) []byte {
	if n == 0 {
		return make([]byte, 0)
	}
	x := make([]byte, n)
	memmove(unsafe.Pointer(&x[0]), unsafe.Pointer(p), uintptr(n))
	return x
}

func gostring(p *byte) string {
	l := findnull(p)
	if l == 0 {
		return ""
	}
	s, b := rawstring(l)
	memmove(unsafe.Pointer(&b[0]), unsafe.Pointer(p), uintptr(l))
	return s
}

func gostringn(p *byte, l int) string {
	if l == 0 {
		return ""
	}
	s, b := rawstring(l)
	memmove(unsafe.Pointer(&b[0]), unsafe.Pointer(p), uintptr(l))
	return s
}

func index(s, t string) int {
	if len(t) == 0 {
		return 0
	}
	for i := 0; i < len(s); i++ {
		if s[i] == t[0] && hasprefix(s[i:], t) {
			return i
		}
	}
	return -1
}

func contains(s, t string) bool {
	return index(s, t) >= 0
}

func hasprefix(s, t string) bool {
	return len(s) >= len(t) && s[:len(t)] == t
}

func atoi(s string) int {
	n := 0
	for len(s) > 0 && '0' <= s[0] && s[0] <= '9' {
		n = n*10 + int(s[0]) - '0'
		s = s[1:]
	}
	return n
}

//go:nosplit
func findnull(s *byte) int {
	if s == nil {
		return 0
	}
	p := (*[_MaxMem/2 - 1]byte)(unsafe.Pointer(s))
	l := 0
	for p[l] != 0 {
		l++
	}
	return l
}

func findnullw(s *uint16) int {
	if s == nil {
		return 0
	}
	p := (*[_MaxMem/2/2 - 1]uint16)(unsafe.Pointer(s))
	l := 0
	for p[l] != 0 {
		l++
	}
	return l
}

var maxstring uintptr = 256 // a hint for print

//go:nosplit
func gostringnocopy(str *byte) string {
	ss := stringStruct{str: unsafe.Pointer(str), len: findnull(str)}
	s := *(*string)(unsafe.Pointer(&ss))
	for {
		ms := maxstring
		if uintptr(len(s)) <= ms || atomic.Casuintptr(&maxstring, ms, uintptr(len(s))) {
			break
		}
	}
	return s
}

func gostringw(strw *uint16) string {
	var buf [8]byte
	str := (*[_MaxMem/2/2 - 1]uint16)(unsafe.Pointer(strw))
	n1 := 0
	for i := 0; str[i] != 0; i++ {
		n1 += runetochar(buf[:], rune(str[i]))
	}
	s, b := rawstring(n1 + 4)
	n2 := 0
	for i := 0; str[i] != 0; i++ {
		// check for race
		if n2 >= n1 {
			break
		}
		n2 += runetochar(b[n2:], rune(str[i]))
	}
	b[n2] = 0 // for luck
	return s[:n2]
}
