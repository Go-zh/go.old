// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync_test

import (
	"fmt"
	"net/http"
	"sync"
)

// This example fetches several URLs concurrently,
// using a WaitGroup to block until all the fetches are complete.

// 本例子并行地取回几个URL，使用 WaitGroup 进行阻塞，直到所有的取回操作完成。
func ExampleWaitGroup() {
	var wg sync.WaitGroup
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	for _, url := range urls {
		// Increment the WaitGroup counter.
		// 递增 WaitGroup 计数器。
		wg.Add(1)
		// Launch a goroutine to fetch the URL.
		// 启动一个Go程来取回URL。
		go func(url string) {
			// Fetch the URL.
			// 取回URL
			http.Get(url)
			// Decrement the counter.
			// 递减计数器
			wg.Done()
		}(url)
	}
	// Wait for all HTTP fetches to complete.
	// 等待所有的HTTP取回操作完成。
	wg.Wait()
}

func ExampleOnce() {
	var once sync.Once
	onceBody := func() {
		fmt.Printf("Only once\n")
	}
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			once.Do(onceBody)
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	// Output:
	// Only once
}
