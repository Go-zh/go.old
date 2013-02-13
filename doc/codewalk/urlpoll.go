// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net/http"
	"time"
)

const (
	numPollers     = 2                // number of Poller goroutines to launch // Poller Go程的启动数
	pollInterval   = 60 * time.Second // how often to poll each URL            // 轮询每一个URL的频率
	statusInterval = 10 * time.Second // how often to log status to stdout     // 将状态记录到标准输出的频率
	errTimeout     = 10 * time.Second // back-off timeout on error             // 回退超时的错误
)

var urls = []string{
	"http://www.google.com/",
	"http://golang.org/",
	"http://blog.golang.org/",
}

// State represents the last-known state of a URL.

// State 表示一个URL最后的已知状态。
type State struct {
	url    string
	status string
}

// StateMonitor maintains a map that stores the state of the URLs being
// polled, and prints the current state every updateInterval nanoseconds.
// It returns a chan State to which resource state should be sent.

// StateMonitor 维护了一个映射，它存储了URL被轮询的状态，并每隔 updateInterval
// 纳秒打印出其当前的状态。它向资源状态的接收者返回一个 chan State。
func StateMonitor(updateInterval time.Duration) chan<- State {
	updates := make(chan State)
	urlStatus := make(map[string]string)
	ticker := time.NewTicker(updateInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				logState(urlStatus)
			case s := <-updates:
				urlStatus[s.url] = s.status
			}
		}
	}()
	return updates
}

// logState prints a state map.

// logState 打印出一个状态映射。
func logState(s map[string]string) {
	log.Println("Current state:")
	for k, v := range s {
		log.Printf(" %s %s", k, v)
	}
}

// Resource represents an HTTP URL to be polled by this program.

// Resource 表示一个被此程序轮询的HTTP URL。
type Resource struct {
	url      string
	errCount int
}

// Poll executes an HTTP HEAD request for url
// and returns the HTTP status string or an error string.

// Poll 为 url 执行一个HTTP HEAD请求，并返回HTTP的状态字符串或一个错误字符串。
func (r *Resource) Poll() string {
	resp, err := http.Head(r.url)
	if err != nil {
		log.Println("Error", r.url, err)
		r.errCount++
		return err.Error()
	}
	r.errCount = 0
	return resp.Status
}

// Sleep sleeps for an appropriate interval (dependant on error state)
// before sending the Resource to done.

// Sleep 在将 Resource 发送到 done 之前休眠一段适当的时间（取决于错误状态）。
func (r *Resource) Sleep(done chan<- *Resource) {
	time.Sleep(pollInterval + errTimeout*time.Duration(r.errCount))
	done <- r
}

func Poller(in <-chan *Resource, out chan<- *Resource, status chan<- State) {
	for r := range in {
		s := r.Poll()
		status <- State{r.url, s}
		out <- r
	}
}

func main() {
	// Create our input and output channels.
	// 创建我们的输入和输出信道。
	pending, complete := make(chan *Resource), make(chan *Resource)

	// Launch the StateMonitor.
	// 启动 StateMonitor。
	status := StateMonitor(statusInterval)

	// Launch some Poller goroutines.
	// 启动一些 Poller Go程。
	for i := 0; i < numPollers; i++ {
		go Poller(pending, complete, status)
	}

	// Send some Resources to the pending queue.
	// 将一些 Resource 发送至 pending 序列。
	go func() {
		for _, url := range urls {
			pending <- &Resource{url: url}
		}
	}()

	for r := range complete {
		go r.Sleep(pending)
	}
}
