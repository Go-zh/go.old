// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Trace is a tool for viewing trace files.

Trace files can be generated with:
	- runtime/trace.Start
	- net/http/pprof package
	- go test -trace

Example usage:
Generate a trace file with 'go test':
	go test -trace trace.out pkg
View the trace in a web browser:
	go tool trace trace.out
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"internal/trace"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

const usageMessage = "" +
	`Usage of 'go tool trace':
Given a trace file produced by 'go test':
	go test -trace=trace.out pkg

Open a web browser displaying trace:
	go tool trace [flags] [pkg.test] trace.out
[pkg.test] argument is required for traces produced by Go 1.6 and below.
Go 1.7 does not require the binary argument.

Flags:
	-http=addr: HTTP service address (e.g., ':6060')
`

var (
	httpFlag = flag.String("http", "localhost:0", "HTTP service address (e.g., ':6060')")

	// The binary file name, left here for serveSVGProfile.
	programBinary string
	traceFile     string
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usageMessage)
		os.Exit(2)
	}
	flag.Parse()

	// Go 1.7 traces embed symbol info and does not require the binary.
	// But we optionally accept binary as first arg for Go 1.5 traces.
	switch flag.NArg() {
	case 1:
		traceFile = flag.Arg(0)
	case 2:
		programBinary = flag.Arg(0)
		traceFile = flag.Arg(1)
	default:
		flag.Usage()
	}

	ln, err := net.Listen("tcp", *httpFlag)
	if err != nil {
		dief("failed to create server socket: %v\n", err)
	}

	log.Printf("Parsing trace...")
	events, err := parseEvents()
	if err != nil {
		dief("%v\n", err)
	}

	log.Printf("Serializing trace...")
	params := &traceParams{
		events:  events,
		endTime: int64(1<<63 - 1),
	}
	data := generateTrace(params)

	log.Printf("Splitting trace...")
	ranges = splitTrace(data)

	log.Printf("Opening browser")
	if !startBrowser("http://" + ln.Addr().String()) {
		fmt.Fprintf(os.Stderr, "Trace viewer is listening on http://%s\n", ln.Addr().String())
	}

	// Start http server.
	http.HandleFunc("/", httpMain)
	err = http.Serve(ln, nil)
	dief("failed to start http server: %v\n", err)
}

var ranges []Range

var loader struct {
	once   sync.Once
	events []*trace.Event
	err    error
}

func parseEvents() ([]*trace.Event, error) {
	loader.once.Do(func() {
		tracef, err := os.Open(traceFile)
		if err != nil {
			loader.err = fmt.Errorf("failed to open trace file: %v", err)
			return
		}
		defer tracef.Close()

		// Parse and symbolize.
		events, err := trace.Parse(bufio.NewReader(tracef), programBinary)
		if err != nil {
			loader.err = fmt.Errorf("failed to parse trace: %v", err)
			return
		}
		loader.events = events
	})
	return loader.events, loader.err
}

// httpMain serves the starting page.
func httpMain(w http.ResponseWriter, r *http.Request) {
	if err := templMain.Execute(w, ranges); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

var templMain = template.Must(template.New("").Parse(`
<html>
<body>
{{if $}}
	{{range $e := $}}
		<a href="/trace?start={{$e.Start}}&end={{$e.End}}">View trace ({{$e.Name}})</a><br>
	{{end}}
	<br>
{{else}}
	<a href="/trace">View trace</a><br>
{{end}}
<a href="/goroutines">Goroutine analysis</a><br>
<a href="/io">Network blocking profile</a><br>
<a href="/block">Synchronization blocking profile</a><br>
<a href="/syscall">Syscall blocking profile</a><br>
<a href="/sched">Scheduler latency profile</a><br>
</body>
</html>
`))

// startBrowser tries to open the URL in a browser
// and reports whether it succeeds.
// Note: copied from x/tools/cmd/cover/html.go
func startBrowser(url string) bool {
	// try to start the browser
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		args = []string{"xdg-open"}
	}
	cmd := exec.Command(args[0], append(args[1:], url)...)
	return cmd.Start() == nil
}

func dief(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg, args...)
	os.Exit(1)
}
