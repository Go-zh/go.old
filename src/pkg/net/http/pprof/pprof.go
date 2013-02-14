// Copyright 2010 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pprof serves via its HTTP server runtime profiling data
// in the format expected by the pprof visualization tool.
// For more information about pprof, see
// http://code.google.com/p/google-perftools/.
//
// The package is typically only imported for the side effect of
// registering its HTTP handlers.
// The handled paths all begin with /debug/pprof/.
//
// To use pprof, link this package into your program:
//	import _ "net/http/pprof"
//
// If your application is not already running an http server, you
// need to start one.  Add "net/http" and "log" to your imports and
// the following code to your main function:
//
// 	go func() {
// 		log.Println(http.ListenAndServe("localhost:6060", nil))
// 	}()
//
// Then use the pprof tool to look at the heap profile:
//
//	go tool pprof http://localhost:6060/debug/pprof/heap
//
// Or to look at a 30-second CPU profile:
//
//	go tool pprof http://localhost:6060/debug/pprof/profile
//
// Or to look at the goroutine blocking profile:
//
//	go tool pprof http://localhost:6060/debug/pprof/block
//
// To view all available profiles, open http://localhost:6060/debug/pprof/
// in your browser.
//
// For a study of the facility in action, visit
//
//	http://blog.golang.org/2011/06/profiling-go-programs.html
//

// pprof 包通过提供HTTP服务返回runtime的统计数据，这个数据是以pprof可视化工具规定的返回格式返回的.
// 了解更多的pprof的知识，请参考：http://code.google.com/p/google-perftools/。
//
// 导入这个包就会在HTTP服务端的handlers注册这个包自己的处理器。
// 处理的路径全是以/debug/pprof/开头的。
//
// 使用pprof，只要将这个包引入到你的程序中：
//	import _ "net/http/pprof"
//
// 如果你的应用并不是运行在http server上，你需要开启一个http服务。
// 开启方法就是增加"net/http"和"log"包到你的imports中，然后增加下面的方法到你的main函数中。
//
// 	go func() {
// 		log.Println(http.ListenAndServe("localhost:6060", nil))
// 	}()
//
// 然后使用pprof工具查看heap统计：
//
//	go tool pprof http://localhost:6060/debug/pprof/heap
//
// 或者查看30秒内的CPU统计：
//
//	go tool pprof http://localhost:6060/debug/pprof/profile
//
// 也可以查看阻塞中的goroutine统计：
//
//	go tool pprof http://localhost:6060/debug/pprof/block
//
// 要查看所有的可查看的统计，在浏览器中打开http://localhost:6060/debug/pprof/
//
// 要进一步了解profile的功能，请浏览
//
//	http://blog.golang.org/2011/06/profiling-go-programs.html
//
package pprof

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

func init() {
	http.Handle("/debug/pprof/", http.HandlerFunc(Index))
	http.Handle("/debug/pprof/cmdline", http.HandlerFunc(Cmdline))
	http.Handle("/debug/pprof/profile", http.HandlerFunc(Profile))
	http.Handle("/debug/pprof/symbol", http.HandlerFunc(Symbol))
}

// Cmdline responds with the running program's
// command line, with arguments separated by NUL bytes.
// The package initialization registers it as /debug/pprof/cmdline.

// Cmdline返回正在运行的程序的命令行结构，包括被空字节分隔的参数。
// 这个包的初始化函数将这个函数注册为/debug/pprof/cmdline的处理函数。
func Cmdline(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, strings.Join(os.Args, "\x00"))
}

// Profile responds with the pprof-formatted cpu profile.
// The package initialization registers it as /debug/pprof/profile.

// Profile返回pprof格式的cpu统计信息。
// 这个包的初始化函数将这个函数注册为/debug/pprof/profile的处理函数。
func Profile(w http.ResponseWriter, r *http.Request) {
	sec, _ := strconv.ParseInt(r.FormValue("seconds"), 10, 64)
	if sec == 0 {
		sec = 30
	}

	// Set Content Type assuming StartCPUProfile will work,
	// because if it does it starts writing.
	w.Header().Set("Content-Type", "application/octet-stream")
	if err := pprof.StartCPUProfile(w); err != nil {
		// StartCPUProfile failed, so no writes yet.
		// Can change header back to text content
		// and send error code.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Could not enable CPU profiling: %s\n", err)
		return
	}
	time.Sleep(time.Duration(sec) * time.Second)
	pprof.StopCPUProfile()
}

// Symbol looks up the program counters listed in the request,
// responding with a table mapping program counters to function names.
// The package initialization registers it as /debug/pprof/symbol.

// Symbol查询当前请求的程序计数器（PC），
// 返回的是一个程序计数器和函数名字的映射表。
// 这个包的初始化函数将这个函数注册为/debug/pprof/symbol的处理函数。
func Symbol(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	// We have to read the whole POST body before
	// writing any output.  Buffer the output here.
	var buf bytes.Buffer

	// We don't know how many symbols we have, but we
	// do have symbol information.  Pprof only cares whether
	// this number is 0 (no symbols available) or > 0.
	fmt.Fprintf(&buf, "num_symbols: 1\n")

	var b *bufio.Reader
	if r.Method == "POST" {
		b = bufio.NewReader(r.Body)
	} else {
		b = bufio.NewReader(strings.NewReader(r.URL.RawQuery))
	}

	for {
		word, err := b.ReadSlice('+')
		if err == nil {
			word = word[0 : len(word)-1] // trim +
		}
		pc, _ := strconv.ParseUint(string(word), 0, 64)
		if pc != 0 {
			f := runtime.FuncForPC(uintptr(pc))
			if f != nil {
				fmt.Fprintf(&buf, "%#x %s\n", pc, f.Name())
			}
		}

		// Wait until here to check for err; the last
		// symbol will have an err because it doesn't end in +.
		if err != nil {
			if err != io.EOF {
				fmt.Fprintf(&buf, "reading request: %v\n", err)
			}
			break
		}
	}

	w.Write(buf.Bytes())
}

// Handler returns an HTTP handler that serves the named profile.

// Handler返回HTTP handler，这个handler会处理参数name的统计。
func Handler(name string) http.Handler {
	return handler(name)
}

type handler string

func (name handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	debug, _ := strconv.Atoi(r.FormValue("debug"))
	p := pprof.Lookup(string(name))
	if p == nil {
		w.WriteHeader(404)
		fmt.Fprintf(w, "Unknown profile: %s\n", name)
		return
	}
	p.WriteTo(w, debug)
	return
}

// Index responds with the pprof-formatted profile named by the request.
// For example, "/debug/pprof/heap" serves the "heap" profile.
// Index responds to a request for "/debug/pprof/" with an HTML page
// listing the available profiles.

// Index 返回请求处理中格式化的pprof的统计数据。
// 例如，“/debug/pprof/heap” 展示的是“heap”统计信息。
// Index对请求“/debug/pprof/”返回一个HTML页面，这个页面展示了所有可见的统计。
func Index(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
		name := strings.TrimPrefix(r.URL.Path, "/debug/pprof/")
		if name != "" {
			handler(name).ServeHTTP(w, r)
			return
		}
	}

	profiles := pprof.Profiles()
	if err := indexTmpl.Execute(w, profiles); err != nil {
		log.Print(err)
	}
}

var indexTmpl = template.Must(template.New("index").Parse(`<html>
<head>
<title>/debug/pprof/</title>
</head>
/debug/pprof/<br>
<br>
<body>
profiles:<br>
<table>
{{range .}}
<tr><td align=right>{{.Count}}<td><a href="/debug/pprof/{{.Name}}?debug=1">{{.Name}}</a>
{{end}}
</table>
<br>
<a href="/debug/pprof/goroutine?debug=2">full goroutine stack dump</a><br>
</body>
</html>
`))
