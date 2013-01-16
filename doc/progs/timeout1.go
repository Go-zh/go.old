// compile

// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package timeout

import (
	"time"
)

func Timeout() {
	ch := make(chan bool, 1)
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(1 * time.Second)
		timeout <- true
	}()

	// STOP OMIT

	select {
	case <-ch:
		// 从 ch 的读取已发生
	case <-timeout:
		// 从 ch 的读取已超时
	}

	// STOP OMIT
}
