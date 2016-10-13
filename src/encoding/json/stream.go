// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"bytes"
	"errors"
	"io"
)

// Decoder 从指定输入流中读取并解析 JSON 值。
type Decoder struct {
	r     io.Reader
	buf   []byte
	d     decodeState
	scanp int // start of unread data in buf
	scan  scanner
	err   error

	tokenState int
	tokenStack []int
}

// NewDecoder 返回一个从 r 读取数据的新 decoder
//
// 该 decoder 有它自己的缓冲区，并且会从 r 中读取 JSON 数据。
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// UseNumber 会将一个数字作为 Number 而不是 float64 解析入一个 interface{} 。
func (dec *Decoder) UseNumber() { dec.d.useNumber = true }

// Decode 从指定的输入中读取 JSON 字符串并解析入 v 中。
//
// 更多 JSON 值到 Go 值之间的解析请参阅 Unmarshal 文档。
func (dec *Decoder) Decode(v interface{}) error {
	if dec.err != nil {
		return dec.err
	}

	if err := dec.tokenPrepareForDecode(); err != nil {
		return err
	}

	if !dec.tokenValueAllowed() {
		return &SyntaxError{msg: "not at beginning of value"}
	}

	// Read whole value into buffer.
	n, err := dec.readValue()
	if err != nil {
		return err
	}
	dec.d.init(dec.buf[dec.scanp : dec.scanp+n])
	dec.scanp += n

	// Don't save err from unmarshal into dec.err:
	// the connection is still usable since we read a complete JSON
	// object from it before the error happened.
	err = dec.d.unmarshal(v)

	// fixup token streaming state
	dec.tokenValueEnd()

	return err
}

// Buffered 返回仍存在于 Decoder 的缓冲区中的数据。返回的 reader 在 Decode 的
// 下一次方法调用之前都是合法的。
func (dec *Decoder) Buffered() io.Reader {
	return bytes.NewReader(dec.buf[dec.scanp:])
}

// readValue reads a JSON value into dec.buf.
// It returns the length of the encoding.
func (dec *Decoder) readValue() (int, error) {
	dec.scan.reset()

	scanp := dec.scanp
	var err error
Input:
	for {
		// Look in the buffer for a new value.
		for i, c := range dec.buf[scanp:] {
			dec.scan.bytes++
			v := dec.scan.step(&dec.scan, c)
			if v == scanEnd {
				scanp += i
				break Input
			}
			// scanEnd is delayed one byte.
			// We might block trying to get that byte from src,
			// so instead invent a space byte.
			if (v == scanEndObject || v == scanEndArray) && dec.scan.step(&dec.scan, ' ') == scanEnd {
				scanp += i + 1
				break Input
			}
			if v == scanError {
				dec.err = dec.scan.err
				return 0, dec.scan.err
			}
		}
		scanp = len(dec.buf)

		// Did the last read have an error?
		// Delayed until now to allow buffer scan.
		if err != nil {
			if err == io.EOF {
				if dec.scan.step(&dec.scan, ' ') == scanEnd {
					break Input
				}
				if nonSpace(dec.buf) {
					err = io.ErrUnexpectedEOF
				}
			}
			dec.err = err
			return 0, err
		}

		n := scanp - dec.scanp
		err = dec.refill()
		scanp = dec.scanp + n
	}
	return scanp - dec.scanp, nil
}

func (dec *Decoder) refill() error {
	// Make room to read more into the buffer.
	// First slide down data already consumed.
	if dec.scanp > 0 {
		n := copy(dec.buf, dec.buf[dec.scanp:])
		dec.buf = dec.buf[:n]
		dec.scanp = 0
	}

	// Grow buffer if not large enough.
	const minRead = 512
	if cap(dec.buf)-len(dec.buf) < minRead {
		newBuf := make([]byte, len(dec.buf), 2*cap(dec.buf)+minRead)
		copy(newBuf, dec.buf)
		dec.buf = newBuf
	}

	// Read. Delay error for next iteration (after scan).
	n, err := dec.r.Read(dec.buf[len(dec.buf):cap(dec.buf)])
	dec.buf = dec.buf[0 : len(dec.buf)+n]

	return err
}

func nonSpace(b []byte) bool {
	for _, c := range b {
		if !isSpace(c) {
			return true
		}
	}
	return false
}

// Encoder 向指定的输出流中写入 JSON 字符串。
type Encoder struct {
	w          io.Writer
	err        error
	escapeHTML bool

	indentBuf    *bytes.Buffer
	indentPrefix string
	indentValue  string
}

// NewEncoder 返回向 w 写入数据的新 encoder 。
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w, escapeHTML: true}
}

// Encode 向指定流中写入 v 的 JSON 字符串表示，以一个换行符结尾。
//
// 更多  Go 值到 JSON 值之间的转换请参阅 Marshal 文档。
func (enc *Encoder) Encode(v interface{}) error {
	if enc.err != nil {
		return enc.err
	}
	e := newEncodeState()
	err := e.marshal(v, encOpts{escapeHTML: enc.escapeHTML})
	if err != nil {
		return err
	}

	// Terminate each value with a newline.
	// This makes the output look a little nicer
	// when debugging, and some kind of space
	// is required if the encoded value was a number,
	// so that the reader knows there aren't more
	// digits coming.
	e.WriteByte('\n')

	b := e.Bytes()
	if enc.indentPrefix != "" || enc.indentValue != "" {
		if enc.indentBuf == nil {
			enc.indentBuf = new(bytes.Buffer)
		}
		enc.indentBuf.Reset()
		err = Indent(enc.indentBuf, b, enc.indentPrefix, enc.indentValue)
		if err != nil {
			return err
		}
		b = enc.indentBuf.Bytes()
	}
	if _, err = enc.w.Write(b); err != nil {
		enc.err = err
	}
	encodeStatePool.Put(e)
	return err
}

// SetIndent 指示 encoder 来格式化每一个解析后的 JSON 序列。如同
// 同包的函数 Indent(dst, src, prefix, indent) 一样。
// 调用 SetIndent("", "") 来禁用缩进。
func (enc *Encoder) SetIndent(prefix, indent string) {
	enc.indentPrefix = prefix
	enc.indentValue = indent
}

// SetEscapeHTML 指示是否需要再 JSON 字符串中转义 HTML 字符。默认行为是将 &，< 和 >
// 转义为 \u0026， \u003c 和 \u003e 来避免一些 HTML 中内嵌 JSON 的安全问题。
//
// 在一些非 HTML 场景下为了提升输出的可读性，可以通过 SetEscapeHTML(false) 来禁止此行为。
func (enc *Encoder) SetEscapeHTML(on bool) {
	enc.escapeHTML = on
}

// RawMessage 是一个未经处理的 JSON 字符串值。
// 它实现了 Marshaler 和 Unmarshaler ，可以用于推迟 JSON 解析或预转换 JSON 字符串。
type RawMessage []byte

// MarshalJSON 返回 m 的 JSON 字符串表示。
func (m *RawMessage) MarshalJSON() ([]byte, error) {
	return *m, nil
}

// UnmarshalJSON 设置 *m 为 data 的拷贝。
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("json.RawMessage: UnmarshalJSON on nil pointer")
	}
	*m = append((*m)[0:0], data...)
	return nil
}

var _ Marshaler = (*RawMessage)(nil)
var _ Unmarshaler = (*RawMessage)(nil)

// Token 保存一个以下类型之一的值：
//
// Delim 表示四个 JSON 分隔符 [ ] { }
// bool 表示 JSON 布尔值
// float64 表示 JSON 数字
// Number 表示 JSON 数字
// string 表示 JSON 字符串字面量
// nil 表示 JSON null
type Token interface{}

const (
	tokenTopValue = iota
	tokenArrayStart
	tokenArrayValue
	tokenArrayComma
	tokenObjectStart
	tokenObjectKey
	tokenObjectColon
	tokenObjectValue
	tokenObjectComma
)

// advance tokenstate from a separator state to a value state
func (dec *Decoder) tokenPrepareForDecode() error {
	// Note: Not calling peek before switch, to avoid
	// putting peek into the standard Decode path.
	// peek is only called when using the Token API.
	switch dec.tokenState {
	case tokenArrayComma:
		c, err := dec.peek()
		if err != nil {
			return err
		}
		if c != ',' {
			return &SyntaxError{"expected comma after array element", 0}
		}
		dec.scanp++
		dec.tokenState = tokenArrayValue
	case tokenObjectColon:
		c, err := dec.peek()
		if err != nil {
			return err
		}
		if c != ':' {
			return &SyntaxError{"expected colon after object key", 0}
		}
		dec.scanp++
		dec.tokenState = tokenObjectValue
	}
	return nil
}

func (dec *Decoder) tokenValueAllowed() bool {
	switch dec.tokenState {
	case tokenTopValue, tokenArrayStart, tokenArrayValue, tokenObjectValue:
		return true
	}
	return false
}

func (dec *Decoder) tokenValueEnd() {
	switch dec.tokenState {
	case tokenArrayStart, tokenArrayValue:
		dec.tokenState = tokenArrayComma
	case tokenObjectValue:
		dec.tokenState = tokenObjectComma
	}
}

// Delim 是一个 JSON 数组或对象分隔符，[ ] { 或 } 之一。
type Delim rune

func (d Delim) String() string {
	return string(d)
}

// Token 返回 JSON 字符串流中的下一个标记（token）。在流的最后，Token 返回
// nil， io.EOF 。
//
// Token 保证返回的分隔符 [ ] { } 是一一对应的：如果发现不对应，则会返回错误。
//
// 输入流由基础的 JSON 值 - 布尔，字符串，数字 null 以及分隔符 [ ] { } 组成。
// 逗号和冒号会被省略。
func (dec *Decoder) Token() (Token, error) {
	for {
		c, err := dec.peek()
		if err != nil {
			return nil, err
		}
		switch c {
		case '[':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenArrayStart
			return Delim('['), nil

		case ']':
			if dec.tokenState != tokenArrayStart && dec.tokenState != tokenArrayComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim(']'), nil

		case '{':
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenStack = append(dec.tokenStack, dec.tokenState)
			dec.tokenState = tokenObjectStart
			return Delim('{'), nil

		case '}':
			if dec.tokenState != tokenObjectStart && dec.tokenState != tokenObjectComma {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = dec.tokenStack[len(dec.tokenStack)-1]
			dec.tokenStack = dec.tokenStack[:len(dec.tokenStack)-1]
			dec.tokenValueEnd()
			return Delim('}'), nil

		case ':':
			if dec.tokenState != tokenObjectColon {
				return dec.tokenError(c)
			}
			dec.scanp++
			dec.tokenState = tokenObjectValue
			continue

		case ',':
			if dec.tokenState == tokenArrayComma {
				dec.scanp++
				dec.tokenState = tokenArrayValue
				continue
			}
			if dec.tokenState == tokenObjectComma {
				dec.scanp++
				dec.tokenState = tokenObjectKey
				continue
			}
			return dec.tokenError(c)

		case '"':
			if dec.tokenState == tokenObjectStart || dec.tokenState == tokenObjectKey {
				var x string
				old := dec.tokenState
				dec.tokenState = tokenTopValue
				err := dec.Decode(&x)
				dec.tokenState = old
				if err != nil {
					clearOffset(err)
					return nil, err
				}
				dec.tokenState = tokenObjectColon
				return x, nil
			}
			fallthrough

		default:
			if !dec.tokenValueAllowed() {
				return dec.tokenError(c)
			}
			var x interface{}
			if err := dec.Decode(&x); err != nil {
				clearOffset(err)
				return nil, err
			}
			return x, nil
		}
	}
}

func clearOffset(err error) {
	if s, ok := err.(*SyntaxError); ok {
		s.Offset = 0
	}
}

func (dec *Decoder) tokenError(c byte) (Token, error) {
	var context string
	switch dec.tokenState {
	case tokenTopValue:
		context = " looking for beginning of value"
	case tokenArrayStart, tokenArrayValue, tokenObjectValue:
		context = " looking for beginning of value"
	case tokenArrayComma:
		context = " after array element"
	case tokenObjectKey:
		context = " looking for beginning of object key string"
	case tokenObjectColon:
		context = " after object key"
	case tokenObjectComma:
		context = " after object key:value pair"
	}
	return nil, &SyntaxError{"invalid character " + quoteChar(c) + " " + context, 0}
}

// More 返回是否在当前被解析的数组和对象中是否还有元素。
func (dec *Decoder) More() bool {
	c, err := dec.peek()
	return err == nil && c != ']' && c != '}'
}

func (dec *Decoder) peek() (byte, error) {
	var err error
	for {
		for i := dec.scanp; i < len(dec.buf); i++ {
			c := dec.buf[i]
			if isSpace(c) {
				continue
			}
			dec.scanp = i
			return c, nil
		}
		// buffer has been scanned, now report any error
		if err != nil {
			return 0, err
		}
		err = dec.refill()
	}
}

/*
TODO

// EncodeToken writes the given JSON token to the stream.
// It returns an error if the delimiters [ ] { } are not properly used.
//
// EncodeToken does not call Flush, because usually it is part of
// a larger operation such as Encode, and those will call Flush when finished.
// Callers that create an Encoder and then invoke EncodeToken directly,
// without using Encode, need to call Flush when finished to ensure that
// the JSON is written to the underlying writer.
func (e *Encoder) EncodeToken(t Token) error  {
	...
}

*/
