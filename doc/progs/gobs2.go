// compile

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
)

type P struct {
	X, Y, Z int
	Name    string
}

type Q struct {
	X, Y *int32
	Name string
}

func main() {
	// 初始化编码器和解码器。通常 enc 和 dec 会被绑定至网络连接，
	// 而编码器和解码器会在不同的进程中运行。
	var network bytes.Buffer        // 代替一个网络连接
	enc := gob.NewEncoder(&network) // 将会写入到网络中
	dec := gob.NewDecoder(&network) // 将会从网络中读取
	// 编码（发送）该值。
	err := enc.Encode(P{3, 4, 5, "Pythagoras"})
	if err != nil {
		log.Fatal("encode error:", err)
	}
	// 解码（接收）该值。
	var q Q
	err = dec.Decode(&q)
	if err != nil {
		log.Fatal("decode error:", err)
	}
	fmt.Printf("%q: {%d,%d}\n", q.Name, *q.X, *q.Y)
}
