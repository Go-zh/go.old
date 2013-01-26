// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package jsonrpc implements a JSON-RPC ClientCodec and ServerCodec
// for the rpc packagea.

// 包jsonrpc使用了rpc的包实现了一个JSON-RPC的客户端解码器和服务端的解码器
package jsonrpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"sync"
)

type clientCodec struct {
	dec *json.Decoder // for reading JSON values  // 为了读取Json值
	enc *json.Encoder // for writing JSON values  // 为了写Json值
	c   io.Closer

	// temporary work space  // 临时工作空间
	req  clientRequest
	resp clientResponse

	// JSON-RPC responses include the request id but not the request method.
	// Package rpc expects both.
	// We save the request method in pending when sending a request
	// and then look it up by request ID when filling out the rpc Response.
	
	// JSON-RPC回复内容包含了请求id，但是不包含请求方法。
	// rpc包却都包含。
	// 我们发送请求的时候将请求方法放到pendding中，并且填写rpc回复的时候根据请求ID来查找。
	mutex   sync.Mutex        // protects pending  // 保护pending
	pending map[uint64]string // map request id to method name  // 将请求id映射到方法名
}

// NewClientCodec returns a new rpc.ClientCodec using JSON-RPC on conn.

// NewClientCodec在连接中使用JSON-RPC返回一个新的rpc.ClientCodec
func NewClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return &clientCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]string),
	}
}

type clientRequest struct {
	Method string         `json:"method"`
	Params [1]interface{} `json:"params"`
	Id     uint64         `json:"id"`
}

func (c *clientCodec) WriteRequest(r *rpc.Request, param interface{}) error {
	c.mutex.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.mutex.Unlock()
	c.req.Method = r.ServiceMethod
	c.req.Params[0] = param
	c.req.Id = r.Seq
	return c.enc.Encode(&c.req)
}

type clientResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

func (r *clientResponse) reset() {
	r.Id = 0
	r.Result = nil
	r.Error = nil
}

func (c *clientCodec) ReadResponseHeader(r *rpc.Response) error {
	c.resp.reset()
	if err := c.dec.Decode(&c.resp); err != nil {
		return err
	}

	c.mutex.Lock()
	r.ServiceMethod = c.pending[c.resp.Id]
	delete(c.pending, c.resp.Id)
	c.mutex.Unlock()

	r.Error = ""
	r.Seq = c.resp.Id
	if c.resp.Error != nil {
		x, ok := c.resp.Error.(string)
		if !ok {
			return fmt.Errorf("invalid error %v", c.resp.Error)
		}
		if x == "" {
			x = "unspecified error"
		}
		r.Error = x
	}
	return nil
}

func (c *clientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(*c.resp.Result, x)
}

func (c *clientCodec) Close() error {
	return c.c.Close()
}

// NewClient returns a new rpc.Client to handle requests to the
// set of services at the other end of the connection.

// NewClient返回新的rpc.Client，用于连接的服务器一端来进行rpc服务。
func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(NewClientCodec(conn))
}

// Dial connects to a JSON-RPC server at the specified network address.

// Dial在指定的网络地址上，连接了一个JSON-RPC服务
func Dial(network, address string) (*rpc.Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), err
}
