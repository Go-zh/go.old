// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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

	stagingMap http.Header // map that handlers manipulate to set headers
	trailerMap http.Header // lazily filled when Trailers() is called

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
	m := rw.stagingMap
	if m == nil {
		m = make(http.Header)
		rw.stagingMap = m
	}
	return m
}

// writeHeader writes a header if it was not written yet and
// detects Content-Type if needed.
//
// bytes or str are the beginning of the response body.
// We pass both to avoid unnecessarily generate garbage
// in rw.WriteString which was created for performance reasons.
// Non-nil bytes win.
func (rw *ResponseRecorder) writeHeader(b []byte, str string) {
	if rw.wroteHeader {
		return
	}
	if len(str) > 512 {
		str = str[:512]
	}

	m := rw.Header()

	_, hasType := m["Content-Type"]
	hasTE := m.Get("Transfer-Encoding") != ""
	if !hasType && !hasTE {
		if b == nil {
			b = []byte(str)
		}
		m.Set("Content-Type", http.DetectContentType(b))
	}

	rw.WriteHeader(200)
}

// Write always succeeds and writes to rw.Body, if not nil.

// Write总是返回成功，并且如果buf非空的话，它会写数据到rw.Body。
func (rw *ResponseRecorder) Write(buf []byte) (int, error) {
	rw.writeHeader(buf, "")
	if rw.Body != nil {
		rw.Body.Write(buf)
	}
	return len(buf), nil
}

// WriteString always succeeds and writes to rw.Body, if not nil.
func (rw *ResponseRecorder) WriteString(str string) (int, error) {
	rw.writeHeader(nil, str)
	if rw.Body != nil {
		rw.Body.WriteString(str)
	}
	return len(str), nil
}

// WriteHeader sets rw.Code. After it is called, changing rw.Header
// will not affect rw.HeaderMap.
func (rw *ResponseRecorder) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.Code = code
	rw.wroteHeader = true
	if rw.HeaderMap == nil {
		rw.HeaderMap = make(http.Header)
	}
	for k, vv := range rw.stagingMap {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		rw.HeaderMap[k] = vv2
	}
}

// Flush sets rw.Flushed to true.

// Flush将rw.Flushed设置为true。
func (rw *ResponseRecorder) Flush() {
	if !rw.wroteHeader {
		rw.WriteHeader(200)
	}
	rw.Flushed = true
}

// Trailers returns any trailers set by the handler. It must be called
// after the handler finished running.
func (rw *ResponseRecorder) Trailers() http.Header {
	if rw.trailerMap != nil {
		return rw.trailerMap
	}
	trailers, ok := rw.HeaderMap["Trailer"]
	if !ok {
		rw.trailerMap = make(http.Header)
		return rw.trailerMap
	}
	rw.trailerMap = make(http.Header, len(trailers))
	for _, k := range trailers {
		switch k {
		case "Transfer-Encoding", "Content-Length", "Trailer":
			// Ignore since forbidden by RFC 2616 14.40.
			continue
		}
		k = http.CanonicalHeaderKey(k)
		vv, ok := rw.stagingMap[k]
		if !ok {
			continue
		}
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		rw.trailerMap[k] = vv2
	}
	return rw.trailerMap
}
