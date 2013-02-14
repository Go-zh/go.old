// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Implementation of Server

// 实现Server

package httptest

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
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
}

// historyListener keeps track of all connections that it's ever
// accepted.

// historyListener保持追踪服务接收过的所有请求。
type historyListener struct {
	net.Listener
	sync.Mutex // protects history  // 保护history
	history    []net.Conn
}

func (hs *historyListener) Accept() (c net.Conn, err error) {
	c, err = hs.Listener.Accept()
	if err == nil {
		hs.Lock()
		hs.history = append(hs.history, c)
		hs.Unlock()
	}
	return
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
	s.Listener = &historyListener{Listener: s.Listener}
	s.URL = "http://" + s.Listener.Addr().String()
	s.wrapHandler()
	go s.Config.Serve(s.Listener)
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
	cert, err := tls.X509KeyPair(localhostCert, localhostKey)
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
	tlsListener := tls.NewListener(s.Listener, s.TLS)

	s.Listener = &historyListener{Listener: tlsListener}
	s.URL = "https://" + s.Listener.Addr().String()
	s.wrapHandler()
	go s.Config.Serve(s.Listener)
}

func (s *Server) wrapHandler() {
	h := s.Config.Handler
	if h == nil {
		h = http.DefaultServeMux
	}
	s.Config.Handler = &waitGroupHandler{
		s: s,
		h: h,
	}
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

// Close shuts down the server and blocks until all outstanding
// requests on this server have completed.

// Close关闭server并且阻塞server，知道所有的请求完成之后才继续。
func (s *Server) Close() {
	s.Listener.Close()
	s.wg.Wait()
	s.CloseClientConnections()
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
}

// CloseClientConnections closes any currently open HTTP connections
// to the test Server.

// CloseClientConnections关闭任何现有打开的HTTP连接到测试服务器上。
func (s *Server) CloseClientConnections() {
	hl, ok := s.Listener.(*historyListener)
	if !ok {
		return
	}
	hl.Lock()
	for _, conn := range hl.history {
		conn.Close()
	}
	hl.Unlock()
}

// waitGroupHandler wraps a handler, incrementing and decrementing a
// sync.WaitGroup on each request, to enable Server.Close to block
// until outstanding requests are finished.

// waitGroupHandler封装了一下handler，在每个请求中进行增加和减少sync.WaitGroup操作，
// 调用Server.Close来阻塞server，知道server上所有的请求都结束才继续。
type waitGroupHandler struct {
	s *Server
	h http.Handler // non-nil
}

func (h *waitGroupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.s.wg.Add(1)
	defer h.s.wg.Done() // a defer, in case ServeHTTP below panics
	h.h.ServeHTTP(w, r)
}

// localhostCert is a PEM-encoded TLS cert with SAN DNS names
// "127.0.0.1" and "[::1]", expiring at the last second of 2049 (the end
// of ASN.1 time).

// localCert是一个PEM-编码的TLS证书，它有的SAN DNS名字是“127.0.0.1” 和 “[::1]”,
// 过期事件是2049年的最后一秒（以ASN.1时间计算）
var localhostCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBTTCB+qADAgECAgEAMAsGCSqGSIb3DQEBBTAAMB4XDTcwMDEwMTAwMDAwMFoX
DTQ5MTIzMTIzNTk1OVowADBaMAsGCSqGSIb3DQEBAQNLADBIAkEAsuA5mAFMj6Q7
qoBzcvKzIq4kzuT5epSp2AkcQfyBHm7K13Ws7u+0b5Vb9gqTf5cAiIKcrtrXVqkL
8i1UQF6AzwIDAQABo2MwYTAOBgNVHQ8BAf8EBAMCACQwEgYDVR0TAQH/BAgwBgEB
/wIBATANBgNVHQ4EBgQEAQIDBDAPBgNVHSMECDAGgAQBAgMEMBsGA1UdEQQUMBKC
CTEyNy4wLjAuMYIFWzo6MV0wCwYJKoZIhvcNAQEFA0EAj1Jsn/h2KHy7dgqutZNB
nCGlNN+8vw263Bax9MklR85Ti6a0VWSvp/fDQZUADvmFTDkcXeA24pqmdUxeQDWw
Pg==
-----END CERTIFICATE-----`)

// localhostKey is the private key for localhostCert.

// localhostKey是localhostCert的私钥。
var localhostKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBPQIBAAJBALLgOZgBTI+kO6qAc3LysyKuJM7k+XqUqdgJHEH8gR5uytd1rO7v
tG+VW/YKk3+XAIiCnK7a11apC/ItVEBegM8CAwEAAQJBAI5sxq7naeR9ahyqRkJi
SIv2iMxLuPEHaezf5CYOPWjSjBPyVhyRevkhtqEjF/WkgL7C2nWpYHsUcBDBQVF0
3KECIQDtEGB2ulnkZAahl3WuJziXGLB+p8Wgx7wzSM6bHu1c6QIhAMEp++CaS+SJ
/TrU0zwY/fW4SvQeb49BPZUF3oqR8Xz3AiEA1rAJHBzBgdOQKdE3ksMUPcnvNJSN
poCcELmz2clVXtkCIQCLytuLV38XHToTipR4yMl6O+6arzAjZ56uq7m7ZRV0TwIh
AM65XAOw8Dsg9Kq78aYXiOEDc5DL0sbFUu/SlmRcCg93
-----END RSA PRIVATE KEY-----
`)
