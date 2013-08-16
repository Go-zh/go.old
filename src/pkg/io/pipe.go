// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Pipe adapter to connect code expecting an io.Reader
// with code expecting an io.Writer.

// Pipe 匹配器将代码预期的 io.Reader 连接到代码预期的 io.Writer。

package io

import (
	"errors"
	"sync"
)

// ErrClosedPipe is the error used for read or write operations on a closed pipe.

// ErrClosedPipe 错误用于在已关闭的管道上进行读取或写入操作。
var ErrClosedPipe = errors.New("io: read/write on closed pipe")

type pipeResult struct {
	n   int
	err error
}

// A pipe is the shared pipe structure underlying PipeReader and PipeWriter.

// pipe 是 PipeReader 和 PipeWriter 共享的底层管道结构。
type pipe struct {
	rl    sync.Mutex // gates readers one at a time         // 控制一次一个读取器
	wl    sync.Mutex // gates writers one at a time         // 控制一次一个写入器
	l     sync.Mutex // protects remaining fields           // 保护剩余的字段
	data  []byte     // data remaining in pending write     // 剩余的待写入数据
	rwait sync.Cond  // waiting reader                      // 等待读取器
	wwait sync.Cond  // waiting writer                      // 等待写入器
	rerr  error      // if reader closed, error to give writes  // 若读取器关闭，就给写入操作一个错误
	werr  error      // if writer closed, error to give reads   // 若写入器关闭，就给读取操作一个错误
}

func (p *pipe) read(b []byte) (n int, err error) {
	// One reader at a time.    // 一次一个读取器
	p.rl.Lock()
	defer p.rl.Unlock()

	p.l.Lock()
	defer p.l.Unlock()
	for {
		if p.rerr != nil {
			return 0, ErrClosedPipe
		}
		if p.data != nil {
			break
		}
		if p.werr != nil {
			return 0, p.werr
		}
		p.rwait.Wait()
	}
	n = copy(b, p.data)
	p.data = p.data[n:]
	if len(p.data) == 0 {
		p.data = nil
		p.wwait.Signal()
	}
	return
}

var zero [0]byte

func (p *pipe) write(b []byte) (n int, err error) {
	// pipe uses nil to mean not available  // 管道使用 nil 表示可用
	if b == nil {
		b = zero[:]
	}

	// One writer at a time.    // 一次一个写入器
	p.wl.Lock()
	defer p.wl.Unlock()

	p.l.Lock()
	defer p.l.Unlock()
	if p.werr != nil {
		err = ErrClosedPipe
		return
	}
	p.data = b
	p.rwait.Signal()
	for {
		if p.data == nil {
			break
		}
		if p.rerr != nil {
			err = p.rerr
			break
		}
		if p.werr != nil {
			err = ErrClosedPipe
		}
		p.wwait.Wait()
	}
	n = len(b) - len(p.data)
	p.data = nil // in case of rerr or werr // 在 rerr 或 werr 的情况下
	return
}

func (p *pipe) rclose(err error) {
	if err == nil {
		err = ErrClosedPipe
	}
	p.l.Lock()
	defer p.l.Unlock()
	p.rerr = err
	p.rwait.Signal()
	p.wwait.Signal()
}

func (p *pipe) wclose(err error) {
	if err == nil {
		err = EOF
	}
	p.l.Lock()
	defer p.l.Unlock()
	p.werr = err
	p.rwait.Signal()
	p.wwait.Signal()
}

// A PipeReader is the read half of a pipe.

// PipeReader 是管道的读取端。
type PipeReader struct {
	p *pipe
}

// Read implements the standard Read interface:
// it reads data from the pipe, blocking until a writer
// arrives or the write end is closed.
// If the write end is closed with an error, that error is
// returned as err; otherwise err is EOF.

// Read 实现了标准的 Read 接口：
// 它从管道中读取数据并阻塞，直到写入器开始写入或写入端被关闭。
// 若写入端带错误关闭，该错误将作为 err 返回；否则 err 为 EOF。
func (r *PipeReader) Read(data []byte) (n int, err error) {
	return r.p.read(data)
}

// Close closes the reader; subsequent writes to the
// write half of the pipe will return the error ErrClosedPipe.

// Close 关闭读取器；关闭后如果对管道的写入端进行写入操作，就会返回 ErrClosedPipe 错误。
func (r *PipeReader) Close() error {
	return r.CloseWithError(nil)
}

// CloseWithError closes the reader; subsequent writes
// to the write half of the pipe will return the error err.

// CloseWithError 关闭读取器；关闭后如果对管道的写入端进行写入操作，就会返回 err 错误。
func (r *PipeReader) CloseWithError(err error) error {
	r.p.rclose(err)
	return nil
}

// A PipeWriter is the write half of a pipe.

// PipeReader 是管道的写入端。
type PipeWriter struct {
	p *pipe
}

// Write implements the standard Write interface:
// it writes data to the pipe, blocking until readers
// have consumed all the data or the read end is closed.
// If the read end is closed with an error, that err is
// returned as err; otherwise err is ErrClosedPipe.

// Write 实现了标准的 Write 接口：
// 它将数据写入到管道中并阻塞，直到读取器读完所有的数据或读取端被关闭。
// 若读取端带错误关闭，该错误将作为 err 返回；否则 err 为 ErrClosedPipe。
func (w *PipeWriter) Write(data []byte) (n int, err error) {
	return w.p.write(data)
}

// Close closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and EOF.

// Close 关闭写入器；关闭后如果对管道的读取端进行读取操作，就会返回 EOF 而不返回字节。
func (w *PipeWriter) Close() error {
	return w.CloseWithError(nil)
}

// CloseWithError closes the writer; subsequent reads from the
// read half of the pipe will return no bytes and the error err.

// CloseWithError 关闭写入器；关闭后如果对管道的读取端进行读取操作，就会返回错误 err 而不返回字节。
func (w *PipeWriter) CloseWithError(err error) error {
	w.p.wclose(err)
	return nil
}

// Pipe creates a synchronous in-memory pipe.
// It can be used to connect code expecting an io.Reader
// with code expecting an io.Writer.
// Reads on one end are matched with writes on the other,
// copying data directly between the two; there is no internal buffering.
// It is safe to call Read and Write in parallel with each other or with
// Close. Close will complete once pending I/O is done. Parallel calls to
// Read, and parallel calls to Write, are also safe:
// the individual calls will be gated sequentially.

// Pipe 创建同步的内存管道。
// 它可用于将代码预期的 io.Reader 连接到代码预期的 io.Writer。
// 一端的读取匹配另一端的写入，直接在这两端之间复制数据；它没有内部缓存。
// 它对于并行调用 Read 和 Write 以及其它函数或 Close 来说都是安全的。
// 一旦等待的I/O结束，Close 就会完成。并行调用 Read 或并行调用 Write 也同样安全：
// 同种类的调用将按顺序进行控制。
func Pipe() (*PipeReader, *PipeWriter) {
	p := new(pipe)
	p.rwait.L = &p.l
	p.wwait.L = &p.l
	r := &PipeReader{p}
	w := &PipeWriter{p}
	return r, w
}
