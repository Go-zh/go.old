// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Implementation of Server

// 实现Server

package httptest

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/internal"
	"os"
	"runtime"
	"sync"
	"time"
)

// A Server is an HTTP server listening on a system-chosen port on the
// local loopback interface, for use in end-to-end HTTP tests.

// Server 是一个HTTP服务，它在系统选择的端口上监听请求，并且是在本地的接口监听，
// 它完全是为了点到点的HTTP测试而出现。
type Server struct {
	URL      string // base URL of form http://ipaddr:port with no trailing slash  // 基本的http://ipaddr:port的URL样式。没有进行尾部删减。
	Listener net.Listener

	// TLS is the optional TLS configuration, populated with a new config
	// after TLS is started. If set on an unstarted server before StartTLS
	// is called, existing fields are copied into the new config.
	//
	// TLS 为可选的 TLS 配置，它在 TLS 启动后，与新的配置一起构成。若在 StartTLS
	// 被调用前设置了一个未启动的服务，既有的字段就会被复制为新的配置。
	TLS *tls.Config

	// Config may be changed after calling NewUnstartedServer and
	// before Start or StartTLS.
	//
	// Config可能在调用NewUnstartedServer之后或者在Start和StartTLS之前被改变。
	Config *http.Server

	// wg counts the number of outstanding HTTP requests on this server.
	// Close blocks until all requests are finished.
	//
	// wg 计算服务上的HTTP请求数。只有当全部请求都结束之后才关闭阻塞。
	wg sync.WaitGroup

	mu     sync.Mutex // guards closed and conns
	closed bool
	conns  map[net.Conn]http.ConnState // except terminal states
}

func newLocalListener() net.Listener {
	if *serve != "" {
		l, err := net.Listen("tcp", *serve)
		if err != nil {
			panic(fmt.Sprintf("httptest: failed to listen on %v: %v", *serve, err))
		}
		return l
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			panic(fmt.Sprintf("httptest: failed to listen on a port: %v", err))
		}
	}
	return l
}

// When debugging a particular http server-based test,
// this flag lets you run
//	go test -run=BrokenTest -httptest.serve=127.0.0.1:8000
// to start the broken server so you can interact with it manually.

// 当调试一个特别的http基于服务的测试的时候，这个flag就会允许你运行：
//	go test -run=BrokenTest -httptest.serve=127.0.0.1:8000
// 来开启服务以便于你可以和它进行手动交互。
var serve = flag.String("httptest.serve", "", "if non-empty, httptest.NewServer serves on this address and blocks")

// NewServer starts and returns a new Server.
// The caller should call Close when finished, to shut it down.

// NewServer开启并且返回一个新的Server。
// 调用者应该当结束的时候调用Close来关闭Server。
func NewServer(handler http.Handler) *Server {
	ts := NewUnstartedServer(handler)
	ts.Start()
	return ts
}

// NewUnstartedServer returns a new Server but doesn't start it.
//
// After changing its configuration, the caller should call Start or
// StartTLS.
//
// The caller should call Close when finished, to shut it down.

// NewUnstartedServer返回一个新的Server实例，但是并不启动这个Server。
//
// 在改变了配置之后，调用者应该调用Start或者StartTLS。
//
// 调用者应该在结束之后调用Close来关闭Server。
func NewUnstartedServer(handler http.Handler) *Server {
	return &Server{
		Listener: newLocalListener(),
		Config:   &http.Server{Handler: handler},
	}
}

// Start starts a server from NewUnstartedServer.

// Start开启从NewUnstartedServer获取到的server。
func (s *Server) Start() {
	if s.URL != "" {
		panic("Server already started")
	}
	s.URL = "http://" + s.Listener.Addr().String()
	s.wrap()
	s.goServe()
	if *serve != "" {
		fmt.Fprintln(os.Stderr, "httptest: serving on", s.URL)
		select {}
	}
}

// StartTLS starts TLS on a server from NewUnstartedServer.

// StartTLS开启从NewUnstartedServer获取到的server的TLS。
func (s *Server) StartTLS() {
	if s.URL != "" {
		panic("Server already started")
	}
	cert, err := tls.X509KeyPair(internal.LocalhostCert, internal.LocalhostKey)
	if err != nil {
		panic(fmt.Sprintf("httptest: NewTLSServer: %v", err))
	}

	existingConfig := s.TLS
	s.TLS = new(tls.Config)
	if existingConfig != nil {
		*s.TLS = *existingConfig
	}
	if s.TLS.NextProtos == nil {
		s.TLS.NextProtos = []string{"http/1.1"}
	}
	if len(s.TLS.Certificates) == 0 {
		s.TLS.Certificates = []tls.Certificate{cert}
	}
	s.Listener = tls.NewListener(s.Listener, s.TLS)
	s.URL = "https://" + s.Listener.Addr().String()
	s.wrap()
	s.goServe()
}

// NewTLSServer starts and returns a new Server using TLS.
// The caller should call Close when finished, to shut it down.

// NewTLSServer 开启并且返回了一个使用TLS的新的Server。
// 调用者应该在结束的时候调用Close来关闭它。
func NewTLSServer(handler http.Handler) *Server {
	ts := NewUnstartedServer(handler)
	ts.StartTLS()
	return ts
}

type closeIdleTransport interface {
	CloseIdleConnections()
}

// Close shuts down the server and blocks until all outstanding
// requests on this server have completed.

// Close关闭server并且阻塞server，知道所有的请求完成之后才继续。
func (s *Server) Close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.Listener.Close()
		s.Config.SetKeepAlivesEnabled(false)
		for c, st := range s.conns {
			// Force-close any idle connections (those between
			// requests) and new connections (those which connected
			// but never sent a request). StateNew connections are
			// super rare and have only been seen (in
			// previously-flaky tests) in the case of
			// socket-late-binding races from the http Client
			// dialing this server and then getting an idle
			// connection before the dial completed. There is thus
			// a connected connection in StateNew with no
			// associated Request. We only close StateIdle and
			// StateNew because they're not doing anything. It's
			// possible StateNew is about to do something in a few
			// milliseconds, but a previous CL to check again in a
			// few milliseconds wasn't liked (early versions of
			// https://golang.org/cl/15151) so now we just
			// forcefully close StateNew. The docs for Server.Close say
			// we wait for "outstanding requests", so we don't close things
			// in StateActive.
			if st == http.StateIdle || st == http.StateNew {
				s.closeConn(c)
			}
		}
		// If this server doesn't shut down in 5 seconds, tell the user why.
		t := time.AfterFunc(5*time.Second, s.logCloseHangDebugInfo)
		defer t.Stop()
	}
	s.mu.Unlock()

	// Not part of httptest.Server's correctness, but assume most
	// users of httptest.Server will be using the standard
	// transport, so help them out and close any idle connections for them.
	if t, ok := http.DefaultTransport.(closeIdleTransport); ok {
		t.CloseIdleConnections()
	}

	s.wg.Wait()
}

func (s *Server) logCloseHangDebugInfo() {
	s.mu.Lock()
	defer s.mu.Unlock()
	var buf bytes.Buffer
	buf.WriteString("httptest.Server blocked in Close after 5 seconds, waiting for connections:\n")
	for c, st := range s.conns {
		fmt.Fprintf(&buf, "  %T %p %v in state %v\n", c, c, c.RemoteAddr(), st)
	}
	log.Print(buf.String())
}

// CloseClientConnections closes any open HTTP connections to the test Server.
func (s *Server) CloseClientConnections() {
	s.mu.Lock()
	nconn := len(s.conns)
	ch := make(chan struct{}, nconn)
	for c := range s.conns {
		s.closeConnChan(c, ch)
	}
	s.mu.Unlock()

	// Wait for outstanding closes to finish.
	//
	// Out of paranoia for making a late change in Go 1.6, we
	// bound how long this can wait, since golang.org/issue/14291
	// isn't fully understood yet. At least this should only be used
	// in tests.
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for i := 0; i < nconn; i++ {
		select {
		case <-ch:
		case <-timer.C:
			// Too slow. Give up.
			return
		}
	}
}

func (s *Server) goServe() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.Config.Serve(s.Listener)
	}()
}

// wrap installs the connection state-tracking hook to know which
// connections are idle.
func (s *Server) wrap() {
	oldHook := s.Config.ConnState
	s.Config.ConnState = func(c net.Conn, cs http.ConnState) {
		s.mu.Lock()
		defer s.mu.Unlock()
		switch cs {
		case http.StateNew:
			s.wg.Add(1)
			if _, exists := s.conns[c]; exists {
				panic("invalid state transition")
			}
			if s.conns == nil {
				s.conns = make(map[net.Conn]http.ConnState)
			}
			s.conns[c] = cs
			if s.closed {
				// Probably just a socket-late-binding dial from
				// the default transport that lost the race (and
				// thus this connection is now idle and will
				// never be used).
				s.closeConn(c)
			}
		case http.StateActive:
			if oldState, ok := s.conns[c]; ok {
				if oldState != http.StateNew && oldState != http.StateIdle {
					panic("invalid state transition")
				}
				s.conns[c] = cs
			}
		case http.StateIdle:
			if oldState, ok := s.conns[c]; ok {
				if oldState != http.StateActive {
					panic("invalid state transition")
				}
				s.conns[c] = cs
			}
			if s.closed {
				s.closeConn(c)
			}
		case http.StateHijacked, http.StateClosed:
			s.forgetConn(c)
		}
		if oldHook != nil {
			oldHook(c, cs)
		}
	}
}

// closeConn closes c.
// s.mu must be held.
func (s *Server) closeConn(c net.Conn) { s.closeConnChan(c, nil) }

// closeConnChan is like closeConn, but takes an optional channel to receive a value
// when the goroutine closing c is done.
func (s *Server) closeConnChan(c net.Conn, done chan<- struct{}) {
	if runtime.GOOS == "plan9" {
		// Go's Plan 9 net package isn't great at unblocking reads when
		// their underlying TCP connections are closed. Don't trust
		// that that the ConnState state machine will get to
		// StateClosed. Instead, just go there directly. Plan 9 may leak
		// resources if the syscall doesn't end up returning. Oh well.
		s.forgetConn(c)
	}

	c.Close()
	if done != nil {
		done <- struct{}{}
	}
}

// forgetConn removes c from the set of tracked conns and decrements it from the
// waitgroup, unless it was previously removed.
// s.mu must be held.
func (s *Server) forgetConn(c net.Conn) {
	if _, ok := s.conns[c]; ok {
		delete(s.conns, c)
		s.wg.Done()
	}
}
