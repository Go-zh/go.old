// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package bufio implements buffered I/O.  It wraps an io.Reader or io.Writer
// object, creating another object (Reader or Writer) that also implements
// the interface but provides buffering and some help for textual I/O.

// bufio 包实现了带缓存的I/O操作. 它封装了一个io.Reader或者io.Writer对象，另外创建了一个对象
//（Reader或者Writer），这个对象也实现了一个接口，并提供缓冲和文档读写的帮助。
package bufio

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

const (
	defaultBufSize = 4096
)

var (
	ErrInvalidUnreadByte = errors.New("bufio: invalid use of UnreadByte")
	ErrInvalidUnreadRune = errors.New("bufio: invalid use of UnreadRune")
	ErrBufferFull        = errors.New("bufio: buffer full")
	ErrNegativeCount     = errors.New("bufio: negative count")
)

// Buffered input.

// 缓冲输入。

// Reader implements buffering for an io.Reader object.

// Reader实现了对一个io.Reader对象的缓冲读。
type Reader struct {
	buf          []byte
	rd           io.Reader
	r, w         int
	err          error
	lastByte     int
	lastRuneSize int
}

const minReadBufferSize = 16

// NewReaderSize returns a new Reader whose buffer has at least the specified
// size. If the argument io.Reader is already a Reader with large enough
// size, it returns the underlying Reader.

// NewReaderSize返回了一个新的读取器，这个读取器的缓存大小至少大于制定的大小。
// 如果io.Reader参数已经是一个有足够大缓存的读取器，它就会返回这个Reader了。
func NewReaderSize(rd io.Reader, size int) *Reader {
	// Is it already a Reader?
	b, ok := rd.(*Reader)
	if ok && len(b.buf) >= size {
		return b
	}
	if size < minReadBufferSize {
		size = minReadBufferSize
	}
	return &Reader{
		buf:          make([]byte, size),
		rd:           rd,
		lastByte:     -1,
		lastRuneSize: -1,
	}
}

// NewReader returns a new Reader whose buffer has the default size.

// NewReader返回一个新的Reader，这个Reader的大小是默认的大小。
func NewReader(rd io.Reader) *Reader {
	return NewReaderSize(rd, defaultBufSize)
}

var errNegativeRead = errors.New("bufio: reader returned negative count from Read")

// fill reads a new chunk into the buffer.

// fill读取一个新的块到缓存中。
func (b *Reader) fill() {
	// Slide existing data to beginning.
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}

	// Read new data.
	n, err := b.rd.Read(b.buf[b.w:])
	if n < 0 {
		panic(errNegativeRead)
	}
	b.w += n
	if err != nil {
		b.err = err
	}
}

func (b *Reader) readErr() error {
	err := b.err
	b.err = nil
	return err
}

// Peek returns the next n bytes without advancing the reader. The bytes stop
// being valid at the next read call. If Peek returns fewer than n bytes, it
// also returns an error explaining why the read is short. The error is
// ErrBufferFull if n is larger than b's buffer size.

// Peek返回没有读取的下n个字节。在下个读取的调用前，字节是不可见的。如果Peek返回的字节数少于n，
// 它一定会解释为什么读取的字节数段了。如果n比b的缓冲大小更大，返回的错误是ErrBufferFull。
func (b *Reader) Peek(n int) ([]byte, error) {
	if n < 0 {
		return nil, ErrNegativeCount
	}
	if n > len(b.buf) {
		return nil, ErrBufferFull
	}
	for b.w-b.r < n && b.err == nil {
		b.fill()
	}
	m := b.w - b.r
	if m > n {
		m = n
	}
	var err error
	if m < n {
		err = b.readErr()
		if err == nil {
			err = ErrBufferFull
		}
	}
	return b.buf[b.r : b.r+m], err
}

// Read reads data into p.
// It returns the number of bytes read into p.
// It calls Read at most once on the underlying Reader,
// hence n may be less than len(p).
// At EOF, the count will be zero and err will be io.EOF.

// Read读取数据到p。
// 返回读取到p的字节数。
// 底层读取最多只会调用一次Read，因此n会小于len(p)。
// 在EOF之后，调用这个函数返回的会是0和io.Eof。
func (b *Reader) Read(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return 0, b.readErr()
	}
	if b.w == b.r {
		if b.err != nil {
			return 0, b.readErr()
		}
		if len(p) >= len(b.buf) {
			// Large read, empty buffer.
			// Read directly into p to avoid copy.
			n, b.err = b.rd.Read(p)
			if n > 0 {
				b.lastByte = int(p[n-1])
				b.lastRuneSize = -1
			}
			return n, b.readErr()
		}
		b.fill()
		if b.w == b.r {
			return 0, b.readErr()
		}
	}

	if n > b.w-b.r {
		n = b.w - b.r
	}
	copy(p[0:n], b.buf[b.r:])
	b.r += n
	b.lastByte = int(b.buf[b.r-1])
	b.lastRuneSize = -1
	return n, nil
}

// ReadByte reads and returns a single byte.
// If no byte is available, returns an error.

// ReadByte读取和回复一个单字节。
// 如果没有字节可以读取，返回一个error。
func (b *Reader) ReadByte() (c byte, err error) {
	b.lastRuneSize = -1
	for b.w == b.r {
		if b.err != nil {
			return 0, b.readErr()
		}
		b.fill()
	}
	c = b.buf[b.r]
	b.r++
	b.lastByte = int(c)
	return c, nil
}

// UnreadByte unreads the last byte.  Only the most recently read byte can be unread.

// UnreadByte将最后的字节标志为未读。只有最后的字节才可以被标志为未读。
func (b *Reader) UnreadByte() error {
	b.lastRuneSize = -1
	if b.r == b.w && b.lastByte >= 0 {
		b.w = 1
		b.r = 0
		b.buf[0] = byte(b.lastByte)
		b.lastByte = -1
		return nil
	}
	if b.r <= 0 {
		return ErrInvalidUnreadByte
	}
	b.r--
	b.lastByte = -1
	return nil
}

// ReadRune reads a single UTF-8 encoded Unicode character and returns the
// rune and its size in bytes. If the encoded rune is invalid, it consumes one byte
// and returns unicode.ReplacementChar (U+FFFD) with a size of 1.

// ReadRune读取单个的UTF-8编码的Unicode字节，并且返回rune和它的字节大小。
// 如果编码的rune是可见的，它消耗一个字节并且返回1字节的unicode.ReplacementChar (U+FFFD)。
func (b *Reader) ReadRune() (r rune, size int, err error) {
	for b.r+utf8.UTFMax > b.w && !utf8.FullRune(b.buf[b.r:b.w]) && b.err == nil {
		b.fill()
	}
	b.lastRuneSize = -1
	if b.r == b.w {
		return 0, 0, b.readErr()
	}
	r, size = rune(b.buf[b.r]), 1
	if r >= 0x80 {
		r, size = utf8.DecodeRune(b.buf[b.r:b.w])
	}
	b.r += size
	b.lastByte = int(b.buf[b.r-1])
	b.lastRuneSize = size
	return r, size, nil
}

// UnreadRune unreads the last rune.  If the most recent read operation on
// the buffer was not a ReadRune, UnreadRune returns an error.  (In this
// regard it is stricter than UnreadByte, which will unread the last byte
// from any read operation.)

// UnreadRune将最后一个rune设置为未读。如果最新的在buffer上的操作不是ReadRune，则UnreadRune
// 就返回一个error。（在这个角度上看，这个函数比UnreadByte更严格，UnreadByte会将最后一个读取
// 的byte设置为未读。）
func (b *Reader) UnreadRune() error {
	if b.lastRuneSize < 0 || b.r == 0 {
		return ErrInvalidUnreadRune
	}
	b.r -= b.lastRuneSize
	b.lastByte = -1
	b.lastRuneSize = -1
	return nil
}

// Buffered returns the number of bytes that can be read from the current buffer.

// Buffered返回当前缓存的可读字节数。
func (b *Reader) Buffered() int { return b.w - b.r }

// ReadSlice reads until the first occurrence of delim in the input,
// returning a slice pointing at the bytes in the buffer.
// The bytes stop being valid at the next read call.
// If ReadSlice encounters an error before finding a delimiter,
// it returns all the data in the buffer and the error itself (often io.EOF).
// ReadSlice fails with error ErrBufferFull if the buffer fills without a delim.
// Because the data returned from ReadSlice will be overwritten
// by the next I/O operation, most clients should use
// ReadBytes or ReadString instead.
// ReadSlice returns err != nil if and only if line does not end in delim.

// ReadSlice从输入中读取，直到遇到第一个终止符为止，返回一个指向缓存中字节的slice。
// 在下次调用的时候这些字节就是已经被读取了。如果ReadSlice在找到终止符之前遇到了error，
// 它就会返回缓存中所有的数据和错误本身（经常是 io.EOF）。
// 如果在终止符之前缓存已经被充满了，ReadSlice会返回ErrBufferFull错误。
// 由于ReadSlice返回的数据会被下次的I/O操作重写，因此许多的客户端会选择使用ReadBytes或者ReadString代替。
// 当且仅当数据没有以终止符结束的时候，ReadSlice返回err != nil
func (b *Reader) ReadSlice(delim byte) (line []byte, err error) {
	// Look in buffer.
	if i := bytes.IndexByte(b.buf[b.r:b.w], delim); i >= 0 {
		line1 := b.buf[b.r : b.r+i+1]
		b.r += i + 1
		return line1, nil
	}

	// Read more into buffer, until buffer fills or we find delim.
	for {
		if b.err != nil {
			line := b.buf[b.r:b.w]
			b.r = b.w
			return line, b.readErr()
		}

		n := b.Buffered()
		b.fill()

		// Search new part of buffer
		if i := bytes.IndexByte(b.buf[n:b.w], delim); i >= 0 {
			line := b.buf[0 : n+i+1]
			b.r = n + i + 1
			return line, nil
		}

		// Buffer is full?
		if b.Buffered() >= len(b.buf) {
			b.r = b.w
			return b.buf, ErrBufferFull
		}
	}
	panic("not reached")
}

// ReadLine is a low-level line-reading primitive. Most callers should use
// ReadBytes('\n') or ReadString('\n') instead.
//
// ReadLine tries to return a single line, not including the end-of-line bytes.
// If the line was too long for the buffer then isPrefix is set and the
// beginning of the line is returned. The rest of the line will be returned
// from future calls. isPrefix will be false when returning the last fragment
// of the line. The returned buffer is only valid until the next call to
// ReadLine. ReadLine either returns a non-nil line or it returns an error,
// never both.
//
// The text returned from ReadLine does not include the line end ("\r\n" or "\n").
// No indication or error is given if the input ends without a final line end.
func (b *Reader) ReadLine() (line []byte, isPrefix bool, err error) {
	line, err = b.ReadSlice('\n')
	if err == ErrBufferFull {
		// Handle the case where "\r\n" straddles the buffer.
		if len(line) > 0 && line[len(line)-1] == '\r' {
			// Put the '\r' back on buf and drop it from line.
			// Let the next call to ReadLine check for "\r\n".
			if b.r == 0 {
				// should be unreachable
				panic("bufio: tried to rewind past start of buffer")
			}
			b.r--
			line = line[:len(line)-1]
		}
		return line, true, nil
	}

	if len(line) == 0 {
		if err != nil {
			line = nil
		}
		return
	}
	err = nil

	if line[len(line)-1] == '\n' {
		drop := 1
		if len(line) > 1 && line[len(line)-2] == '\r' {
			drop = 2
		}
		line = line[:len(line)-drop]
	}
	return
}

// ReadBytes reads until the first occurrence of delim in the input,
// returning a slice containing the data up to and including the delimiter.
// If ReadBytes encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadBytes returns err != nil if and only if the returned data does not end in
// delim.
func (b *Reader) ReadBytes(delim byte) (line []byte, err error) {
	// Use ReadSlice to look for array,
	// accumulating full buffers.
	var frag []byte
	var full [][]byte
	err = nil

	for {
		var e error
		frag, e = b.ReadSlice(delim)
		if e == nil { // got final fragment
			break
		}
		if e != ErrBufferFull { // unexpected error
			err = e
			break
		}

		// Make a copy of the buffer.
		buf := make([]byte, len(frag))
		copy(buf, frag)
		full = append(full, buf)
	}

	// Allocate new buffer to hold the full pieces and the fragment.
	n := 0
	for i := range full {
		n += len(full[i])
	}
	n += len(frag)

	// Copy full pieces and fragment in.
	buf := make([]byte, n)
	n = 0
	for i := range full {
		n += copy(buf[n:], full[i])
	}
	copy(buf[n:], frag)
	return buf, err
}

// ReadString reads until the first occurrence of delim in the input,
// returning a string containing the data up to and including the delimiter.
// If ReadString encounters an error before finding a delimiter,
// it returns the data read before the error and the error itself (often io.EOF).
// ReadString returns err != nil if and only if the returned data does not end in
// delim.
func (b *Reader) ReadString(delim byte) (line string, err error) {
	bytes, err := b.ReadBytes(delim)
	return string(bytes), err
}

// WriteTo implements io.WriterTo.
func (b *Reader) WriteTo(w io.Writer) (n int64, err error) {
	n, err = b.writeBuf(w)
	if err != nil {
		return
	}

	if r, ok := b.rd.(io.WriterTo); ok {
		m, err := r.WriteTo(w)
		n += m
		return n, err
	}

	for b.fill(); b.r < b.w; b.fill() {
		m, err := b.writeBuf(w)
		n += m
		if err != nil {
			return n, err
		}
	}

	if b.err == io.EOF {
		b.err = nil
	}

	return n, b.readErr()
}

// writeBuf writes the Reader's buffer to the writer.
func (b *Reader) writeBuf(w io.Writer) (int64, error) {
	n, err := w.Write(b.buf[b.r:b.w])
	b.r += n
	return int64(n), err
}

// buffered output

// Writer implements buffering for an io.Writer object.
// If an error occurs writing to a Writer, no more data will be
// accepted and all subsequent writes will return the error.
type Writer struct {
	err error
	buf []byte
	n   int
	wr  io.Writer
}

// NewWriterSize returns a new Writer whose buffer has at least the specified
// size. If the argument io.Writer is already a Writer with large enough
// size, it returns the underlying Writer.
func NewWriterSize(wr io.Writer, size int) *Writer {
	// Is it already a Writer?
	b, ok := wr.(*Writer)
	if ok && len(b.buf) >= size {
		return b
	}
	if size <= 0 {
		size = defaultBufSize
	}
	b = new(Writer)
	b.buf = make([]byte, size)
	b.wr = wr
	return b
}

// NewWriter returns a new Writer whose buffer has the default size.
func NewWriter(wr io.Writer) *Writer {
	return NewWriterSize(wr, defaultBufSize)
}

// Flush writes any buffered data to the underlying io.Writer.
func (b *Writer) Flush() error {
	if b.err != nil {
		return b.err
	}
	if b.n == 0 {
		return nil
	}
	n, err := b.wr.Write(b.buf[0:b.n])
	if n < b.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < b.n {
			copy(b.buf[0:b.n-n], b.buf[n:b.n])
		}
		b.n -= n
		b.err = err
		return err
	}
	b.n = 0
	return nil
}

// Available returns how many bytes are unused in the buffer.
func (b *Writer) Available() int { return len(b.buf) - b.n }

// Buffered returns the number of bytes that have been written into the current buffer.
func (b *Writer) Buffered() int { return b.n }

// Write writes the contents of p into the buffer.
// It returns the number of bytes written.
// If nn < len(p), it also returns an error explaining
// why the write is short.
func (b *Writer) Write(p []byte) (nn int, err error) {
	for len(p) > b.Available() && b.err == nil {
		var n int
		if b.Buffered() == 0 {
			// Large write, empty buffer.
			// Write directly from p to avoid copy.
			n, b.err = b.wr.Write(p)
		} else {
			n = copy(b.buf[b.n:], p)
			b.n += n
			b.Flush()
		}
		nn += n
		p = p[n:]
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], p)
	b.n += n
	nn += n
	return nn, nil
}

// WriteByte writes a single byte.
func (b *Writer) WriteByte(c byte) error {
	if b.err != nil {
		return b.err
	}
	if b.Available() <= 0 && b.Flush() != nil {
		return b.err
	}
	b.buf[b.n] = c
	b.n++
	return nil
}

// WriteRune writes a single Unicode code point, returning
// the number of bytes written and any error.
func (b *Writer) WriteRune(r rune) (size int, err error) {
	if r < utf8.RuneSelf {
		err = b.WriteByte(byte(r))
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
	if b.err != nil {
		return 0, b.err
	}
	n := b.Available()
	if n < utf8.UTFMax {
		if b.Flush(); b.err != nil {
			return 0, b.err
		}
		n = b.Available()
		if n < utf8.UTFMax {
			// Can only happen if buffer is silly small.
			return b.WriteString(string(r))
		}
	}
	size = utf8.EncodeRune(b.buf[b.n:], r)
	b.n += size
	return size, nil
}

// WriteString writes a string.
// It returns the number of bytes written.
// If the count is less than len(s), it also returns an error explaining
// why the write is short.
func (b *Writer) WriteString(s string) (int, error) {
	nn := 0
	for len(s) > b.Available() && b.err == nil {
		n := copy(b.buf[b.n:], s)
		b.n += n
		nn += n
		s = s[n:]
		b.Flush()
	}
	if b.err != nil {
		return nn, b.err
	}
	n := copy(b.buf[b.n:], s)
	b.n += n
	nn += n
	return nn, nil
}

// ReadFrom implements io.ReaderFrom.
func (b *Writer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.Buffered() == 0 {
		if w, ok := b.wr.(io.ReaderFrom); ok {
			return w.ReadFrom(r)
		}
	}
	var m int
	for {
		m, err = r.Read(b.buf[b.n:])
		if m == 0 {
			break
		}
		b.n += m
		n += int64(m)
		if b.Available() == 0 {
			if err1 := b.Flush(); err1 != nil {
				return n, err1
			}
		}
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return n, err
}

// buffered input and output

// ReadWriter stores pointers to a Reader and a Writer.
// It implements io.ReadWriter.
type ReadWriter struct {
	*Reader
	*Writer
}

// NewReadWriter allocates a new ReadWriter that dispatches to r and w.
func NewReadWriter(r *Reader, w *Writer) *ReadWriter {
	return &ReadWriter{r, w}
}
