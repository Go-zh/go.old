// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"
)

func init() {
	register("GCFairness", GCFairness)
	register("GCFairness2", GCFairness2)
	register("GCSys", GCSys)
}

func GCSys() {
	runtime.GOMAXPROCS(1)
	memstats := new(runtime.MemStats)
	runtime.GC()
	runtime.ReadMemStats(memstats)
	sys := memstats.Sys

	runtime.MemProfileRate = 0 // disable profiler

	itercount := 100000
	for i := 0; i < itercount; i++ {
		workthegc()
	}

	// Should only be using a few MB.
	// We allocated 100 MB or (if not short) 1 GB.
	runtime.ReadMemStats(memstats)
	if sys > memstats.Sys {
		sys = 0
	} else {
		sys = memstats.Sys - sys
	}
	if sys > 16<<20 {
		fmt.Printf("using too much memory: %d bytes\n", sys)
		return
	}
	fmt.Printf("OK\n")
}

func workthegc() []byte {
	return make([]byte, 1029)
}

func GCFairness() {
	runtime.GOMAXPROCS(1)
	f, err := os.Open("/dev/null")
	if os.IsNotExist(err) {
		// This test tests what it is intended to test only if writes are fast.
		// If there is no /dev/null, we just don't execute the test.
		fmt.Println("OK")
		return
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for i := 0; i < 2; i++ {
		go func() {
			for {
				f.Write([]byte("."))
			}
		}()
	}
	time.Sleep(10 * time.Millisecond)
	fmt.Println("OK")
}

func GCFairness2() {
	// Make sure user code can't exploit the GC's high priority
	// scheduling to make scheduling of user code unfair. See
	// issue #15706.
	runtime.GOMAXPROCS(1)
	debug.SetGCPercent(1)
	var count [3]int64
	var sink [3]interface{}
	for i := range count {
		go func(i int) {
			for {
				sink[i] = make([]byte, 1024)
				atomic.AddInt64(&count[i], 1)
			}
		}(i)
	}
	// Note: If the unfairness is really bad, it may not even get
	// past the sleep.
	//
	// If the scheduling rules change, this may not be enough time
	// to let all goroutines run, but for now we cycle through
	// them rapidly.
	time.Sleep(30 * time.Millisecond)
	for i := range count {
		if atomic.LoadInt64(&count[i]) == 0 {
			fmt.Printf("goroutine %d did not run\n", i)
			return
		}
	}
	fmt.Println("OK")
}
