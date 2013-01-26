// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package httptest provides utilities for HTTP testing.

// httptest包提供HTTP测试的单元工具.
package httptest

import (
	"bytes"
	"net/http"
)

// ResponseRecorder is an implementation of http.ResponseWriter that
// records its mutations for later inspection in tests.

// ResponseRecorder是http.ResponseWriter的具体实现，它为进一步的观察记录下了任何变化。
type ResponseRecorder struct {
	Code      int           // the HTTP response code from WriteHeader  // 为WriteHeader回复的code
	HeaderMap http.Header   // the HTTP response headers  // HTTP回复的头
	Body      *bytes.Buffer // if non-nil, the bytes.Buffer to append written data to  // 如果是非空，bytes.Buffer要将数据写到这里面
	Flushed   bool

	wroteHeader bool
}

// NewRecorder returns an initialized ResponseRecorder.

// NewRecorder返回一个初始化的ResponseRecorder。
func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
		Code:      200,
	}
}

// DefaultRemoteAddr is the default remote address to return in RemoteAddr if
// an explicit DefaultRemoteAddr isn't set on ResponseRecorder.

// DefaultRemoteAddr是RemoteAddr返回的默认远端地址。如果没有对ResponseRecorder做地址设置的话，
// DefaultRemoteAddr就作为默认值。
const DefaultRemoteAddr = "1.2.3.4"

// Header returns the response headers.

// Header返回回复的header。
func (rw *ResponseRecorder) Header() http.Header {
	m := rw.HeaderMap
	if m == nil {
		m = make(http.Header)
		rw.HeaderMap = m
	}
	return m
}

// Write always succeeds and writes to rw.Body, if not nil.

// Write总是返回成功，并且如果buf非空的话，它会写数据到rw.Body。
func (rw *ResponseRecorder) Write(buf []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}
	if rw.Body != nil {
		rw.Body.Write(buf)
	}
	return len(buf), nil
}

// WriteHeader sets rw.Code.

// WriteHeader设置rw.Code
func (rw *ResponseRecorder) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.Code = code
	}
	rw.wroteHeader = true
}

// Flush sets rw.Flushed to true.

// Flush将rw.Flushed设置为true。
func (rw *ResponseRecorder) Flush() {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}
	rw.Flushed = true
}
