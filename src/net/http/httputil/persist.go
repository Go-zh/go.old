// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httputil

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"sync"
)

var (
	ErrPersistEOF = &http.ProtocolError{ErrorString: "persistent connection closed"}
	ErrClosed     = &http.ProtocolError{ErrorString: "connection closed by user"}
	ErrPipeline   = &http.ProtocolError{ErrorString: "pipeline error"}
)

// This is an API usage error - the local side is closed.
// ErrPersistEOF (above) reports that the remote side is closed.

// 这是一个API使用的错误 - 本地一边的连接关闭。
// ErrPersistEOF（上文提到的）报告远端的一边连接关闭。
var errClosed = errors.New("i/o operation on closed connection")

// ServerConn is an artifact of Go's early HTTP implementation.
// It is low-level, old, and unused by Go's current HTTP stack.
// We should have deleted it before Go 1.
//
// Deprecated: Use the Server in package net/http instead.

// ServerConn 在底层连接之上读取请求，发送回复，直到HTTP keepalive出现了结束命令。
// ServerConn 允许靠调用Hijack来对底层连接进行劫持，从而得到连接的控制权。
// ServerConn 支持管道连接，例如，当回复发送的时候，请求可以不需要进行同步（但是是在相同的顺序）。
//
// 已弃用：请使用 net/http 包中的 Server 代替。
type ServerConn struct {
	mu              sync.Mutex // read-write protects the following fields
	c               net.Conn
	r               *bufio.Reader
	re, we          error // read/write errors
	lastbody        io.ReadCloser
	nread, nwritten int
	pipereq         map[*http.Request]uint

	pipe textproto.Pipeline
}

// NewServerConn is an artifact of Go's early HTTP implementation.
// It is low-level, old, and unused by Go's current HTTP stack.
// We should have deleted it before Go 1.
//
// Deprecated: Use the Server in package net/http instead.

// NewServerConn返回一个新的ServerConn来读取和写c。如果r非空，则使用缓存对c进行读取。
//
// 已弃用：请使用 net/http 包中的 Server 代替。
func NewServerConn(c net.Conn, r *bufio.Reader) *ServerConn {
	if r == nil {
		r = bufio.NewReader(c)
	}
	return &ServerConn{c: c, r: r, pipereq: make(map[*http.Request]uint)}
}

// Hijack detaches the ServerConn and returns the underlying connection as well
// as the read-side bufio which may have some left over data. Hijack may be
// called before Read has signaled the end of the keep-alive logic. The user
// should not call Hijack while Read or Write is in progress.

// Hijack将ServerConn单独分离出来，并且返回底层的连接，以及可能有一些未读数据的缓存的读取器。
// Hijack会在读取获取到keep-alive结束信号之前被调用。在Read或者Write进行中不可以调用Hijack。
func (sc *ServerConn) Hijack() (net.Conn, *bufio.Reader) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	c := sc.c
	r := sc.r
	sc.c = nil
	sc.r = nil
	return c, r
}

// Close calls Hijack and then also closes the underlying connection.

// Close调用Hijack，并且关闭底层的连接。
func (sc *ServerConn) Close() error {
	c, _ := sc.Hijack()
	if c != nil {
		return c.Close()
	}
	return nil
}

// Read returns the next request on the wire. An ErrPersistEOF is returned if
// it is gracefully determined that there are no more requests (e.g. after the
// first request on an HTTP/1.0 connection, or after a Connection:close on a
// HTTP/1.1 connection).

// Read返回连接上的下个请求。如果确认了没有更多请求之后，将会返回ErrPersistEOF。（例如，在HTTP/1.0
// 的第一个请求之后，或者在HTTP/1.1的Connection:close之后）
func (sc *ServerConn) Read() (*http.Request, error) {
	var req *http.Request
	var err error

	// Ensure ordered execution of Reads and Writes
	id := sc.pipe.Next()
	sc.pipe.StartRequest(id)
	defer func() {
		sc.pipe.EndRequest(id)
		if req == nil {
			sc.pipe.StartResponse(id)
			sc.pipe.EndResponse(id)
		} else {
			// Remember the pipeline id of this request
			sc.mu.Lock()
			sc.pipereq[req] = id
			sc.mu.Unlock()
		}
	}()

	sc.mu.Lock()
	if sc.we != nil { // no point receiving if write-side broken or closed
		defer sc.mu.Unlock()
		return nil, sc.we
	}
	if sc.re != nil {
		defer sc.mu.Unlock()
		return nil, sc.re
	}
	if sc.r == nil { // connection closed by user in the meantime
		defer sc.mu.Unlock()
		return nil, errClosed
	}
	r := sc.r
	lastbody := sc.lastbody
	sc.lastbody = nil
	sc.mu.Unlock()

	// Make sure body is fully consumed, even if user does not call body.Close
	if lastbody != nil {
		// body.Close is assumed to be idempotent and multiple calls to
		// it should return the error that its first invocation
		// returned.
		err = lastbody.Close()
		if err != nil {
			sc.mu.Lock()
			defer sc.mu.Unlock()
			sc.re = err
			return nil, err
		}
	}

	req, err = http.ReadRequest(r)
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err != nil {
		if err == io.ErrUnexpectedEOF {
			// A close from the opposing client is treated as a
			// graceful close, even if there was some unparse-able
			// data before the close.
			sc.re = ErrPersistEOF
			return nil, sc.re
		} else {
			sc.re = err
			return req, err
		}
	}
	sc.lastbody = req.Body
	sc.nread++
	if req.Close {
		sc.re = ErrPersistEOF
		return req, sc.re
	}
	return req, err
}

// Pending returns the number of unanswered requests
// that have been received on the connection.

// Pending返回已经连接上但未应答的请求数。
func (sc *ServerConn) Pending() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.nread - sc.nwritten
}

// Write writes resp in response to req. To close the connection gracefully, set the
// Response.Close field to true. Write should be considered operational until
// it returns an error, regardless of any errors returned on the Read side.

// Write为请求进行回复。为了要更好的关闭连接，该函数将Response.Close设置为true。
// 直到它返回一个错误之前，Write都可以被调用，并且应该要忽略任何读取端的错误。
func (sc *ServerConn) Write(req *http.Request, resp *http.Response) error {

	// Retrieve the pipeline ID of this request/response pair
	sc.mu.Lock()
	id, ok := sc.pipereq[req]
	delete(sc.pipereq, req)
	if !ok {
		sc.mu.Unlock()
		return ErrPipeline
	}
	sc.mu.Unlock()

	// Ensure pipeline order
	sc.pipe.StartResponse(id)
	defer sc.pipe.EndResponse(id)

	sc.mu.Lock()
	if sc.we != nil {
		defer sc.mu.Unlock()
		return sc.we
	}
	if sc.c == nil { // connection closed by user in the meantime
		defer sc.mu.Unlock()
		return ErrClosed
	}
	c := sc.c
	if sc.nread <= sc.nwritten {
		defer sc.mu.Unlock()
		return errors.New("persist server pipe count")
	}
	if resp.Close {
		// After signaling a keep-alive close, any pipelined unread
		// requests will be lost. It is up to the user to drain them
		// before signaling.
		sc.re = ErrPersistEOF
	}
	sc.mu.Unlock()

	err := resp.Write(c)
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if err != nil {
		sc.we = err
		return err
	}
	sc.nwritten++

	return nil
}

// ClientConn is an artifact of Go's early HTTP implementation.
// It is low-level, old, and unused by Go's current HTTP stack.
// We should have deleted it before Go 1.
//
// Deprecated: Use Client or Transport in package net/http instead.

// ClientConn从还保持着HTTP keepalive的底层连接发送请求，并且接收header。
// ClientConn支持调用Hijack来劫持连接用于获取底层网络连接的控制来处理net.Conn。
//
// 已弃用：请使用 net/http 包中的 Client 或 Transport 代替。
type ClientConn struct {
	mu              sync.Mutex // read-write protects the following fields
	c               net.Conn
	r               *bufio.Reader
	re, we          error // read/write errors
	lastbody        io.ReadCloser
	nread, nwritten int
	pipereq         map[*http.Request]uint

	pipe     textproto.Pipeline
	writeReq func(*http.Request, io.Writer) error
}

// NewClientConn is an artifact of Go's early HTTP implementation.
// It is low-level, old, and unused by Go's current HTTP stack.
// We should have deleted it before Go 1.
//
// Deprecated: Use the Client or Transport in package net/http instead.

// NewClientConn返回一个新的ClientConnd对c进行读取和写入。如果r非空，则使用缓存对c进行读取。
//
// 已弃用：请使用 net/http 包中的 Client 或 Transport 代替。
func NewClientConn(c net.Conn, r *bufio.Reader) *ClientConn {
	if r == nil {
		r = bufio.NewReader(c)
	}
	return &ClientConn{
		c:        c,
		r:        r,
		pipereq:  make(map[*http.Request]uint),
		writeReq: (*http.Request).Write,
	}
}

// NewProxyClientConn is an artifact of Go's early HTTP implementation.
// It is low-level, old, and unused by Go's current HTTP stack.
// We should have deleted it before Go 1.
//
// Deprecated: Use the Client or Transport in package net/http instead.

// NewProxyClientConn像NewClientConn一样，不同的是使用Request的WriteProxy方法对请求进行写操作。
//
// 已弃用：请使用 net/http 包中的 Client 或 Transport 代替。
func NewProxyClientConn(c net.Conn, r *bufio.Reader) *ClientConn {
	cc := NewClientConn(c, r)
	cc.writeReq = (*http.Request).WriteProxy
	return cc
}

// Hijack detaches the ClientConn and returns the underlying connection as well
// as the read-side bufio which may have some left over data. Hijack may be
// called before the user or Read have signaled the end of the keep-alive
// logic. The user should not call Hijack while Read or Write is in progress.

// Hijack将ClientConn单独分离出来，并且返回底层的连接，以及可能有一些未读数据的缓存的读取器。
// Hijack会在读取获取到keep-alive结束信号之前被调用。在Read或者Write进行中不可以调用Hijack。
func (cc *ClientConn) Hijack() (c net.Conn, r *bufio.Reader) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	c = cc.c
	r = cc.r
	cc.c = nil
	cc.r = nil
	return
}

// Close calls Hijack and then also closes the underlying connection.

// Close调用Hijack并且关闭底层的连接
func (cc *ClientConn) Close() error {
	c, _ := cc.Hijack()
	if c != nil {
		return c.Close()
	}
	return nil
}

// Write writes a request. An ErrPersistEOF error is returned if the connection
// has been closed in an HTTP keepalive sense. If req.Close equals true, the
// keepalive connection is logically closed after this request and the opposing
// server is informed. An ErrUnexpectedEOF indicates the remote closed the
// underlying TCP connection, which is usually considered as graceful close.

// Write负责写请求。如果HTTP长连接已经被关闭了，ErrPersistEOF错误就会被抛出。如果req.Close设置为true，
// 在通知请求和对应的服务之后，长连接就会被关闭了。ErrUnexpectedEOF则表示TCP连接被远端关闭。
// 在考虑到关闭连接的时候必须考虑到这种情况。
func (cc *ClientConn) Write(req *http.Request) error {
	var err error

	// Ensure ordered execution of Writes
	id := cc.pipe.Next()
	cc.pipe.StartRequest(id)
	defer func() {
		cc.pipe.EndRequest(id)
		if err != nil {
			cc.pipe.StartResponse(id)
			cc.pipe.EndResponse(id)
		} else {
			// Remember the pipeline id of this request
			cc.mu.Lock()
			cc.pipereq[req] = id
			cc.mu.Unlock()
		}
	}()

	cc.mu.Lock()
	if cc.re != nil { // no point sending if read-side closed or broken
		defer cc.mu.Unlock()
		return cc.re
	}
	if cc.we != nil {
		defer cc.mu.Unlock()
		return cc.we
	}
	if cc.c == nil { // connection closed by user in the meantime
		defer cc.mu.Unlock()
		return errClosed
	}
	c := cc.c
	if req.Close {
		// We write the EOF to the write-side error, because there
		// still might be some pipelined reads
		cc.we = ErrPersistEOF
	}
	cc.mu.Unlock()

	err = cc.writeReq(req, c)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if err != nil {
		cc.we = err
		return err
	}
	cc.nwritten++

	return nil
}

// Pending returns the number of unanswered requests
// that have been sent on the connection.

// Pending返回已经被发送出去但是却没有获取到应答的请求数。
func (cc *ClientConn) Pending() int {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	return cc.nwritten - cc.nread
}

// Read reads the next response from the wire. A valid response might be
// returned together with an ErrPersistEOF, which means that the remote
// requested that this be the last request serviced. Read can be called
// concurrently with Write, but not with another Read.

// Read读取连接上的下个请求。回复有可能和ErrPersistEOF一起返回，如果返回了这个错误，
// 则代表远端的请求是最后被服务的请求了。Read可以和Write并发调用，但是却不能和其他Read并发调用。
func (cc *ClientConn) Read(req *http.Request) (resp *http.Response, err error) {
	// Retrieve the pipeline ID of this request/response pair
	cc.mu.Lock()
	id, ok := cc.pipereq[req]
	delete(cc.pipereq, req)
	if !ok {
		cc.mu.Unlock()
		return nil, ErrPipeline
	}
	cc.mu.Unlock()

	// Ensure pipeline order
	cc.pipe.StartResponse(id)
	defer cc.pipe.EndResponse(id)

	cc.mu.Lock()
	if cc.re != nil {
		defer cc.mu.Unlock()
		return nil, cc.re
	}
	if cc.r == nil { // connection closed by user in the meantime
		defer cc.mu.Unlock()
		return nil, errClosed
	}
	r := cc.r
	lastbody := cc.lastbody
	cc.lastbody = nil
	cc.mu.Unlock()

	// Make sure body is fully consumed, even if user does not call body.Close
	if lastbody != nil {
		// body.Close is assumed to be idempotent and multiple calls to
		// it should return the error that its first invocation
		// returned.
		err = lastbody.Close()
		if err != nil {
			cc.mu.Lock()
			defer cc.mu.Unlock()
			cc.re = err
			return nil, err
		}
	}

	resp, err = http.ReadResponse(r, req)
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if err != nil {
		cc.re = err
		return resp, err
	}
	cc.lastbody = resp.Body

	cc.nread++

	if resp.Close {
		cc.re = ErrPersistEOF // don't send any more requests
		return resp, cc.re
	}
	return resp, err
}

// Do is convenience method that writes a request and reads a response.

// Do是一个写请求和读回复很方便的方法。
func (cc *ClientConn) Do(req *http.Request) (*http.Response, error) {
	err := cc.Write(req)
	if err != nil {
		return nil, err
	}
	return cc.Read(req)
}
