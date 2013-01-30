// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package rpc provides access to the exported methods of an object across a
	network or other I/O connection.  A server registers an object, making it visible
	as a service with the name of the type of the object.  After registration, exported
	methods of the object will be accessible remotely.  A server may register multiple
	objects (services) of different types but it is an error to register multiple
	objects of the same type.

	Only methods that satisfy these criteria will be made available for remote access;
	other methods will be ignored:

		- the method is exported.
		- the method has two arguments, both exported (or builtin) types.
		- the method's second argument is a pointer.
		- the method has return type error.

	In effect, the method must look schematically like

		func (t *T) MethodName(argType T1, replyType *T2) error

	where T, T1 and T2 can be marshaled by encoding/gob.
	These requirements apply even if a different codec is used.
	(In the future, these requirements may soften for custom codecs.)

	The method's first argument represents the arguments provided by the caller; the
	second argument represents the result parameters to be returned to the caller.
	The method's return value, if non-nil, is passed back as a string that the client
	sees as if created by errors.New.  If an error is returned, the reply parameter
	will not be sent back to the client.

	The server may handle requests on a single connection by calling ServeConn.  More
	typically it will create a network listener and call Accept or, for an HTTP
	listener, HandleHTTP and http.Serve.

	A client wishing to use the service establishes a connection and then invokes
	NewClient on the connection.  The convenience function Dial (DialHTTP) performs
	both steps for a raw network connection (an HTTP connection).  The resulting
	Client object has two methods, Call and Go, that specify the service and method to
	call, a pointer containing the arguments, and a pointer to receive the result
	parameters.

	The Call method waits for the remote call to complete while the Go method
	launches the call asynchronously and signals completion using the Call
	structure's Done channel.

	Unless an explicit codec is set up, package encoding/gob is used to
	transport the data.

	Here is a simple example.  A server wishes to export an object of type Arith:

		package server

		type Args struct {
			A, B int
		}

		type Quotient struct {
			Quo, Rem int
		}

		type Arith int

		func (t *Arith) Multiply(args *Args, reply *int) error {
			*reply = args.A * args.B
			return nil
		}

		func (t *Arith) Divide(args *Args, quo *Quotient) error {
			if args.B == 0 {
				return errors.New("divide by zero")
			}
			quo.Quo = args.A / args.B
			quo.Rem = args.A % args.B
			return nil
		}

	The server calls (for HTTP service):

		arith := new(Arith)
		rpc.Register(arith)
		rpc.HandleHTTP()
		l, e := net.Listen("tcp", ":1234")
		if e != nil {
			log.Fatal("listen error:", e)
		}
		go http.Serve(l, nil)

	At this point, clients can see a service "Arith" with methods "Arith.Multiply" and
	"Arith.Divide".  To invoke one, a client first dials the server:

		client, err := rpc.DialHTTP("tcp", serverAddress + ":1234")
		if err != nil {
			log.Fatal("dialing:", err)
		}

	Then it can make a remote call:

		// Synchronous call
		args := &server.Args{7,8}
		var reply int
		err = client.Call("Arith.Multiply", args, &reply)
		if err != nil {
			log.Fatal("arith error:", err)
		}
		fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)

	or

		// Asynchronous call
		quotient := new(Quotient)
		divCall := client.Go("Arith.Divide", args, quotient, nil)
		replyCall := <-divCall.Done	// will be equal to divCall
		// check errors, print, etc.

	A server implementation will often provide a simple, type-safe wrapper for the
	client.
*/

/*
	rpc 包提供了一个方法来通过网络或者其他的I/O连接进入对象的外部方法. 一个server注册一个对象，
	标记它成为可见对象类型名字的服务。注册后，对象的外部方法就可以远程调用了。一个server可以注册多个
	不同类型的对象，但是却不可以注册多个相同类型的对象。

	只有满足这些标准的方法才会被远程调用视为可见；其他的方法都会被忽略：

		- 方法是外部可见的。
		- 方法有两个参数，参数的类型都是外部可见的。
		- 方法的第二个参数是一个指针。
		- 方法有返回类型错误

	事实上，方法必须看起来类似这样

		func (t *T) MethodName(argType T1, replyType *T2) error

	T，T1和T2可以被encoding/gob序列化。
	不管使用什么编解码，这些要求都要满足。
	（在未来，这些要求可能对自定义的编解码会放宽）

	方法的第一个参数代表调用者提供的参数；第二个参数代表返回给调用者的参数。方法的返回值，如果是非空的话
	就会被作为一个string返回，客户端会error像是被errors.New调用返回的一样。如果error返回的话，
	返回的参数将会被送回给客户端。

	服务断可以使用ServeConn来处理单个连接上的请求。更通用的方法，服务器可以制造一个网络监听，然后调用
	Accept，或者对一个HTTP监听，处理HandleHTTP和http.Serve。

	客户端希望使用服务来建立连接，然后在连接上调用NewClient来建立连接。更方便的方法就是调用Dial(DialHTTP)
	来建立一个新的网络连接（一个HTTP连接）。客户端获得到的对象有两个方法，Call和Go，指定的参数有：服务和方法
	指向参数的指针，接受返回结果的指针。

	call方法等待远程调用完成，但Go方法是异步调用call方法，使用Call通道来标志调用完成。

	除非有明确制定编解码器，否则默认使用encoding/gob来传输数据。

	这是个简单的例子，服务器希望对外服务出Arith对象：

		package server

		type Args struct {
			A, B int
		}

		type Quotient struct {
			Quo, Rem int
		}

		type Arith int

		func (t *Arith) Multiply(args *Args, reply *int) error {
			*reply = args.A * args.B
			return nil
		}

		func (t *Arith) Divide(args *Args, quo *Quotient) error {
			if args.B == 0 {
				return errors.New("divide by zero")
			}
			quo.Quo = args.A / args.B
			quo.Rem = args.A % args.B
			return nil
		}

	服务端调用（使用HTTP服务）：

		arith := new(Arith)
		rpc.Register(arith)
		rpc.HandleHTTP()
		l, e := net.Listen("tcp", ":1234")
		if e != nil {
			log.Fatal("listen error:", e)
		}
		go http.Serve(l, nil)

	在这个时候，客户端可以看见服务“Arith”，并且有“Arith.Multiply”方法和“Arith.Divide”方法。
	调用其中一个，客户端首先连接服务：

		client, err := rpc.DialHTTP("tcp", serverAddress + ":1234")
		if err != nil {
			log.Fatal("dialing:", err)
		}

	当它要调用远程服务的时候：

		// Synchronous call
		args := &server.Args{7,8}
		var reply int
		err = client.Call("Arith.Multiply", args, &reply)
		if err != nil {
			log.Fatal("arith error:", err)
		}
		fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)

	or

		// Asynchronous call
		quotient := new(Quotient)
		divCall := client.Go("Arith.Divide", args, quotient, nil)
		replyCall := <-divCall.Done	// will be equal to divCall
		// check errors, print, etc.

	服务端的实现需要为客户端提供一个简单的，类型安全服务。
*/
package rpc

import (
	"bufio"
	"encoding/gob"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	// Defaults used by HandleHTTP  //默认被HandleHTPP使用
	DefaultRPCPath   = "/_goRPC_"
	DefaultDebugPath = "/debug/rpc"
)

// Precompute the reflect type for error.  Can't use error directly
// because Typeof takes an empty interface value.  This is annoying.

// 预先计算error的类型。不能直接使用error的原因是Typeof使用的是一个空的接口值。这个是非常不爽的。
var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

type methodType struct {
	sync.Mutex // protects counters
	method     reflect.Method
	ArgType    reflect.Type
	ReplyType  reflect.Type
	numCalls   uint
}

type service struct {
	name   string                 // name of service  // service的名字
	rcvr   reflect.Value          // receiver of methods for the service  // service的接收的方法
	typ    reflect.Type           // type of the receiver  // 接收者的类型
	method map[string]*methodType // registered methods  //注册方法
}

// Request is a header written before every RPC call.  It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.

// Request是在每个RPC调用之前使用的header。它是内部使用的，写在这里是为了调试用，例如分析网络的流量等。
type Request struct {
	ServiceMethod string   // format: "Service.Method"  // 格式：“Service.Method”
	Seq           uint64   // sequence number chosen by client  // 客户端序列化的数
	next          *Request // for free list in Server // 给服务端的request list使用
}

// Response is a header written before every RPC return.  It is used internally
// but documented here as an aid to debugging, such as when analyzing
// network traffic.

// Response是在每个RPC回复之前被写在头里面的。它是内部使用的，写在这里是为了调试用，例如分析网络的流量等。
type Response struct {
	ServiceMethod string    // echoes that of the Request  // 每个Request的相应方法
	Seq           uint64    // echoes that of the request  // 每个request的相应
	Error         string    // error, if any.  // 如果有的话，表示错误
	next          *Response // for free list in Server  // 个服务端的response list使用
}

// Server represents an RPC Server.

// Server代表一个RPC服务。
type Server struct {
	mu         sync.RWMutex // protects the serviceMap  // 保护serviceMap
	serviceMap map[string]*service
	reqLock    sync.Mutex // protects freeReq  // 保护freeReq
	freeReq    *Request
	respLock   sync.Mutex // protects freeResp  // 保护freeResp
	freeResp   *Response
}

// NewServer returns a new Server.

// NewServer返回一个新的Server
func NewServer() *Server {
	return &Server{serviceMap: make(map[string]*service)}
}

// DefaultServer is the default instance of *Server.

// DefaultServer是默认的*Server实例。
var DefaultServer = NewServer()

// Is this an exported - upper case - name?

// 是否这个是一个对外可见的 - 名字是否是首字母大写？
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// Is this type exported or a builtin?

// 是否这个类型是对外可见的？或者是一个内部类型？
func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}

// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method
//	- two arguments, both pointers to exported structs
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no methods or unsuitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.

// Register发布服务器的一系列方法，接受器必须满足这几个条件：
//	- 对外可见的方法
//	- 两个参数，都指向对外可见的结构
//	- 一个error类型返回值
// 如果接收者不是一个对外可见的类型，或者没有任何方法，或者没有满足条件的方法，都会返回error。
// 它也会使用log包来记录错误。客户端进入每个方法使用字符串格式形如“Type.Method”,
// 这里Type是接收者的具体的类型。
func (server *Server) Register(rcvr interface{}) error {
	return server.register(rcvr, "", false)
}

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.

// RegisterName像Register，但是为type使用提供的名字，而不是使用receivers的具体类型。
func (server *Server) RegisterName(name string, rcvr interface{}) error {
	return server.register(rcvr, name, true)
}

func (server *Server) register(rcvr interface{}, name string, useName bool) error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.serviceMap == nil {
		server.serviceMap = make(map[string]*service)
	}
	s := new(service)
	s.typ = reflect.TypeOf(rcvr)
	s.rcvr = reflect.ValueOf(rcvr)
	sname := reflect.Indirect(s.rcvr).Type().Name()
	if useName {
		sname = name
	}
	if sname == "" {
		log.Fatal("rpc: no service name for type", s.typ.String())
	}
	if !isExported(sname) && !useName {
		s := "rpc Register: type " + sname + " is not exported"
		log.Print(s)
		return errors.New(s)
	}
	if _, present := server.serviceMap[sname]; present {
		return errors.New("rpc: service already defined: " + sname)
	}
	s.name = sname
	s.method = make(map[string]*methodType)

	// Install the methods
	s.method = suitableMethods(s.typ, true)

	if len(s.method) == 0 {
		str := ""
		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PtrTo(s.typ), false)
		if len(method) != 0 {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		log.Print(str)
		return errors.New(str)
	}
	server.serviceMap[s.name] = s
	return nil
}

// suitableMethods returns suitable Rpc methods of typ, it will report
// error using log if reportErr is true.

// 合适的方法返回对应的Rpc方法，如果reportError设置为true的话，它会使用log来报告error。
func suitableMethods(typ reflect.Type, reportErr bool) map[string]*methodType {
	methods := make(map[string]*methodType)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type
		mname := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 3 {
			if reportErr {
				log.Println("method", mname, "has wrong number of ins:", mtype.NumIn())
			}
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(1)
		if !isExportedOrBuiltinType(argType) {
			if reportErr {
				log.Println(mname, "argument type not exported:", argType)
			}
			continue
		}
		// Second arg must be a pointer.
		replyType := mtype.In(2)
		if replyType.Kind() != reflect.Ptr {
			if reportErr {
				log.Println("method", mname, "reply type not a pointer:", replyType)
			}
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			if reportErr {
				log.Println("method", mname, "reply type not exported:", replyType)
			}
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			if reportErr {
				log.Println("method", mname, "has wrong number of outs:", mtype.NumOut())
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			if reportErr {
				log.Println("method", mname, "returns", returnType.String(), "not error")
			}
			continue
		}
		methods[mname] = &methodType{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.

// 当服务端收到一个不合法的请求的时候，就会发送invalidRequest作为占位符回复给客户端。它并不需要解码，
// 因为Response也同时包含了一个error。
var invalidRequest = struct{}{}

func (server *Server) sendResponse(sending *sync.Mutex, req *Request, reply interface{}, codec ServerCodec, errmsg string) {
	resp := server.getResponse()
	// Encode the response header
	resp.ServiceMethod = req.ServiceMethod
	if errmsg != "" {
		resp.Error = errmsg
		reply = invalidRequest
	}
	resp.Seq = req.Seq
	sending.Lock()
	err := codec.WriteResponse(resp, reply)
	if err != nil {
		log.Println("rpc: writing response:", err)
	}
	sending.Unlock()
	server.freeResponse(resp)
}

func (m *methodType) NumCalls() (n uint) {
	m.Lock()
	n = m.numCalls
	m.Unlock()
	return n
}

func (s *service) call(server *Server, sending *sync.Mutex, mtype *methodType, req *Request, argv, replyv reflect.Value, codec ServerCodec) {
	mtype.Lock()
	mtype.numCalls++
	mtype.Unlock()
	function := mtype.method.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{s.rcvr, argv, replyv})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	errmsg := ""
	if errInter != nil {
		errmsg = errInter.(error).Error()
	}
	server.sendResponse(sending, req, replyv.Interface(), codec, errmsg)
	server.freeRequest(req)
}

type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func (c *gobServerCodec) ReadRequestHeader(r *Request) error {
	return c.dec.Decode(r)
}

func (c *gobServerCodec) ReadRequestBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobServerCodec) WriteResponse(r *Response, body interface{}) (err error) {
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if err = c.enc.Encode(body); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobServerCodec) Close() error {
	return c.rwc.Close()
}

// ServeConn runs the server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection.  To use an alternate codec, use ServeCodec.

// ServeConn在单个连接上跑server。
// ServeConn阻塞，知道客户端关闭之后才继续服务其他连接。
// 调用者一般在go语句中调用ServeConn。
// ServeConn在连接传输的时候使用gob格式（参考gob包）。可以使用自定义编码器，ServeCodec。
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	buf := bufio.NewWriter(conn)
	srv := &gobServerCodec{conn, gob.NewDecoder(conn), gob.NewEncoder(buf), buf}
	server.ServeCodec(srv)
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.

// ServerCodec和ServeConn相似，但是使用自定义的编解码器来解码请求和编码回复。
func (server *Server) ServeCodec(codec ServerCodec) {
	sending := new(sync.Mutex)
	for {
		service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
		if err != nil {
			if err != io.EOF {
				log.Println("rpc:", err)
			}
			if !keepReading {
				break
			}
			// send a response if we actually managed to read a header.
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err.Error())
				server.freeRequest(req)
			}
			continue
		}
		go service.call(server, sending, mtype, req, argv, replyv, codec)
	}
	codec.Close()
}

// ServeRequest is like ServeCodec but synchronously serves a single request.
// It does not close the codec upon completion.

// ServerRequest和ServeCodec相似，但是是同步地服务单个请求。
// 它在结束的时候不会关闭编解码器。
func (server *Server) ServeRequest(codec ServerCodec) error {
	sending := new(sync.Mutex)
	service, mtype, req, argv, replyv, keepReading, err := server.readRequest(codec)
	if err != nil {
		if !keepReading {
			return err
		}
		// send a response if we actually managed to read a header.
		if req != nil {
			server.sendResponse(sending, req, invalidRequest, codec, err.Error())
			server.freeRequest(req)
		}
		return err
	}
	service.call(server, sending, mtype, req, argv, replyv, codec)
	return nil
}

func (server *Server) getRequest() *Request {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(Request)
	} else {
		server.freeReq = req.next
		*req = Request{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *Request) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) getResponse() *Response {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(Response)
	} else {
		server.freeResp = resp.next
		*resp = Response{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *Response) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) readRequest(codec ServerCodec) (service *service, mtype *methodType, req *Request, argv, replyv reflect.Value, keepReading bool, err error) {
	service, mtype, req, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		codec.ReadRequestBody(nil)
		return
	}

	// Decode the argument value.
	argIsValue := false // if true, need to indirect before calling.
	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType.Elem())
	} else {
		argv = reflect.New(mtype.ArgType)
		argIsValue = true
	}
	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}
	if argIsValue {
		argv = argv.Elem()
	}

	replyv = reflect.New(mtype.ReplyType.Elem())
	return
}

func (server *Server) readRequestHeader(codec ServerCodec) (service *service, mtype *methodType, req *Request, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = errors.New("rpc: server cannot decode request: " + err.Error())
		return
	}

	// We read the header successfully.  If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	serviceMethod := strings.Split(req.ServiceMethod, ".")
	if len(serviceMethod) != 2 {
		err = errors.New("rpc: service/method request ill-formed: " + req.ServiceMethod)
		return
	}
	// Look up the request.
	server.mu.RLock()
	service = server.serviceMap[serviceMethod[0]]
	server.mu.RUnlock()
	if service == nil {
		err = errors.New("rpc: can't find service " + req.ServiceMethod)
		return
	}
	mtype = service.method[serviceMethod[1]]
	if mtype == nil {
		err = errors.New("rpc: can't find method " + req.ServiceMethod)
	}
	return
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection.  Accept blocks; the caller typically
// invokes it in a go statement.

// Accept接收连接，为每个连接监听和服务请求。Accept是阻塞的，调用者一般在go语句中使用它。
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Fatal("rpc.Serve: accept:", err.Error()) // TODO(r): exit?
		}
		go server.ServeConn(conn)
	}
}

// Register publishes the receiver's methods in the DefaultServer.

// Register在DefaultServer中发布接收者的方法
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.

// RegisterName就像Register，但是为类型使用自定义的名字而不是接收者定义的名字。
func RegisterName(name string, rcvr interface{}) error {
	return DefaultServer.RegisterName(name, rcvr)
}

// A ServerCodec implements reading of RPC requests and writing of
// RPC responses for the server side of an RPC session.
// The server calls ReadRequestHeader and ReadRequestBody in pairs
// to read requests from the connection, and it calls WriteResponse to
// write a response back.  The server calls Close when finished with the
// connection. ReadRequestBody may be called with a nil
// argument to force the body of the request to be read and discarded.

// ServerCodec实现了为RPC会话提供读RPC请求和写PRC回复的服务端的方法。服务端调用
// ReadRequestHeader和ReadRequestBody来读取连接上的请求，然后调用WriteResponse来
// 写回复。服务端当结束连接的时候调用Close。ReadRequestBody可能会调用一个nil参数来强迫
// 读取请求内容并忽略。
type ServerCodec interface {
	ReadRequestHeader(*Request) error
	ReadRequestBody(interface{}) error
	WriteResponse(*Response, interface{}) error

	Close() error
}

// ServeConn runs the DefaultServer on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
// The caller typically invokes ServeConn in a go statement.
// ServeConn uses the gob wire format (see package gob) on the
// connection.  To use an alternate codec, use ServeCodec.

// ServeConn在单个连接上调用DefaultServer。
// ServeConn阻塞，服务连接，直到客户端关闭。
// 调用者一般在go语句中调用ServeConn。ServeConn在连接上使用gob格式（参考gob包）。
// 要使用自定义的编解码，使用ServeCodec.
func ServeConn(conn io.ReadWriteCloser) {
	DefaultServer.ServeConn(conn)
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.

// ServeCodec和ServeConn一样，但是使用特定的codec来解码请求，编码回复。
func ServeCodec(codec ServerCodec) {
	DefaultServer.ServeCodec(codec)
}

// ServeRequest is like ServeCodec but synchronously serves a single request.
// It does not close the codec upon completion.

// ServeRequest和ServeCodec相似，但是同步地服务单个请求。
// 它直到完成了才关闭codec。
func ServeRequest(codec ServerCodec) error {
	return DefaultServer.ServeRequest(codec)
}

// Accept accepts connections on the listener and serves requests
// to DefaultServer for each incoming connection.
// Accept blocks; the caller typically invokes it in a go statement.

// Accept在连接上监听和服务请求，为每个连接调用DefaultServer。
// Accept是阻塞的，调用者一般是在go语句中调用。
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }

// Can connect to RPC service using HTTP CONNECT to rpcPath.

// 可以使用HTTP和rpcPaht，连接到RPC服务
var connected = "200 Connected to Go RPC"

// ServeHTTP implements an http.Handler that answers RPC requests.

// ServeHTTP实现了http.Handler，并且回复RPC请求。
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	server.ServeConn(conn)
}

// HandleHTTP registers an HTTP handler for RPC messages on rpcPath,
// and a debugging handler on debugPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.

// HandleHTTP在rpcPath上为RPC消息注册一个HTTP处理器，并在debugPath注册一个debugging处理器。
// 它仍然需要调用http.Serve()，一般是在go语句中使用。
func (server *Server) HandleHTTP(rpcPath, debugPath string) {
	http.Handle(rpcPath, server)
	http.Handle(debugPath, debugHTTP{server})
}

// HandleHTTP registers an HTTP handler for RPC messages to DefaultServer
// on DefaultRPCPath and a debugging handler on DefaultDebugPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.

// HandleHTTP在DefaultRPCPath上为RPC消息注册了一个HTTP的处理器到DefaultServer上，并且在
// DefaultDebugPath上注册了一个debuggin处理器。
// 它仍然需要调用http.Serve()，一般是在go语句中。
func HandleHTTP() {
	DefaultServer.HandleHTTP(DefaultRPCPath, DefaultDebugPath)
}
