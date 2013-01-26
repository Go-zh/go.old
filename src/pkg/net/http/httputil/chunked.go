// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The wire protocol for HTTP's "chunked" Transfer-Encoding.

// HTTP网络协议的“chunked”传输编码.

// This code is a duplicate of ../chunked.go with these edits:
//	s/newChunked/NewChunked/g
//	s/package http/package httputil/
// Please make any changes in both files.

// 这个代码是../chunked.go的复制：
//  s/newChunked/NewChunked/g
//	s/package http/package httputil/
// 改动的话请对以上所有文件进行改动。

package httputil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const maxLineLength = 4096 // assumed <= bufio.defaultBufSize

var ErrLineTooLong = errors.New("header line too long")

// NewChunkedReader returns a new chunkedReader that translates the data read from r
// out of HTTP "chunked" format before returning it.
// The chunkedReader returns io.EOF when the final 0-length chunk is read.
//
// NewChunkedReader is not needed by normal applications. The http package
// automatically decodes chunking when reading response bodies.

// NewChunkedReader返回一个新的chunkedReader。这个chunkedReader能翻译从r中的HTTP “chunked”
// 获取到的数据，并且返回数据。
// chunkedReader当读取到最后的0长度的chunk的时候返回io.EOF。
//
// NewChunkedReader在通常的应用中并不需要。http包会在读取回复的消息体的时候自动解码。
func NewChunkedReader(r io.Reader) io.Reader {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &chunkedReader{r: br}
}

type chunkedReader struct {
	r   *bufio.Reader
	n   uint64 // unread bytes in chunk
	err error
	buf [2]byte
}

func (cr *chunkedReader) beginChunk() {
	// chunk-size CRLF
	var line []byte
	line, cr.err = readLine(cr.r)
	if cr.err != nil {
		return
	}
	cr.n, cr.err = parseHexUint(line)
	if cr.err != nil {
		return
	}
	if cr.n == 0 {
		cr.err = io.EOF
	}
}

func (cr *chunkedReader) Read(b []uint8) (n int, err error) {
	if cr.err != nil {
		return 0, cr.err
	}
	if cr.n == 0 {
		cr.beginChunk()
		if cr.err != nil {
			return 0, cr.err
		}
	}
	if uint64(len(b)) > cr.n {
		b = b[0:cr.n]
	}
	n, cr.err = cr.r.Read(b)
	cr.n -= uint64(n)
	if cr.n == 0 && cr.err == nil {
		// end of chunk (CRLF)
		if _, cr.err = io.ReadFull(cr.r, cr.buf[:]); cr.err == nil {
			if cr.buf[0] != '\r' || cr.buf[1] != '\n' {
				cr.err = errors.New("malformed chunked encoding")
			}
		}
	}
	return n, cr.err
}

// Read a line of bytes (up to \n) from b.
// Give up if the line exceeds maxLineLength.
// The returned bytes are a pointer into storage in
// the bufio, so they are only valid until the next bufio read.

// 从b中肚脐眼一行字节（直到\n为止）。
// 如果这行的字节数超过了maxLineLength则自动放弃读取。
// 返回的字节是指向bufio的指针，所以它们直到下个bufio读取的时候才可见。
func readLine(b *bufio.Reader) (p []byte, err error) {
	if p, err = b.ReadSlice('\n'); err != nil {
		// We always know when EOF is coming.
		// If the caller asked for a line, there should be a line.
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		} else if err == bufio.ErrBufferFull {
			err = ErrLineTooLong
		}
		return nil, err
	}
	if len(p) >= maxLineLength {
		return nil, ErrLineTooLong
	}
	return trimTrailingWhitespace(p), nil
}

func trimTrailingWhitespace(b []byte) []byte {
	for len(b) > 0 && isASCIISpace(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return b
}

func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// NewChunkedWriter returns a new chunkedWriter that translates writes into HTTP
// "chunked" format before writing them to w. Closing the returned chunkedWriter
// sends the final 0-length chunk that marks the end of the stream.
//
// NewChunkedWriter is not needed by normal applications. The http
// package adds chunking automatically if handlers don't set a
// Content-Length header. Using NewChunkedWriter inside a handler
// would result in double chunking or chunking with a Content-Length
// length, both of which are wrong.

// NewChunkedWriter返回一个新的chunkWriter，这个chunkWriter会将HTTP的“chunked”格式进行转化
// 然后写入w中。关闭返回的chunkedWriter，发送0长度的chunk就标志着流的结束。
//
// 一般的应用并不需要使用NewChunkedWriter。如果不设置Content-Length头的话，http包会自动增加chunk。
// 在handler中使用NewChunkedWriter会导致重复块，或者有Content-length长度的块，这两种都是错误的。
func NewChunkedWriter(w io.Writer) io.WriteCloser {
	return &chunkedWriter{w}
}

// Writing to chunkedWriter translates to writing in HTTP chunked Transfer
// Encoding wire format to the underlying Wire chunkedWriter.

// 写入到chunkedWriter会在写的时候进行翻译转换。将无线的格式编码转换为底层的chunkedWriter。
type chunkedWriter struct {
	Wire io.Writer
}

// Write the contents of data as one chunk to Wire.
// NOTE: Note that the corresponding chunk-writing procedure in Conn.Write has
// a bug since it does not check for success of io.WriteString

// 将data的内容写成传输过程中的一个个块。
// 注意：注意相关的Conn.Write是有个bug，因为它并没有检查io.WriteString的成功。
func (cw *chunkedWriter) Write(data []byte) (n int, err error) {

	// Don't send 0-length data. It looks like EOF for chunked encoding.
	if len(data) == 0 {
		return 0, nil
	}

	if _, err = fmt.Fprintf(cw.Wire, "%x\r\n", len(data)); err != nil {
		return 0, err
	}
	if n, err = cw.Wire.Write(data); err != nil {
		return
	}
	if n != len(data) {
		err = io.ErrShortWrite
		return
	}
	_, err = io.WriteString(cw.Wire, "\r\n")

	return
}

func (cw *chunkedWriter) Close() error {
	_, err := io.WriteString(cw.Wire, "0\r\n")
	return err
}

func parseHexUint(v []byte) (n uint64, err error) {
	for _, b := range v {
		n <<= 4
		switch {
		case '0' <= b && b <= '9':
			b = b - '0'
		case 'a' <= b && b <= 'f':
			b = b - 'a' + 10
		case 'A' <= b && b <= 'F':
			b = b - 'A' + 10
		default:
			return 0, errors.New("invalid byte in chunk length")
		}
		n |= uint64(b)
	}
	return
}
