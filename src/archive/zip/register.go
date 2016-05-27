// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zip

import (
	"compress/flate"
	"errors"
	"io"
	"io/ioutil"
	"sync"
)

// A Compressor returns a new compressing writer, writing to w.
// The WriteCloser's Close method must be used to flush pending data to w.
// The Compressor itself must be safe to invoke from multiple goroutines
// simultaneously, but each returned writer will be used only by
// one goroutine at a time.

// Compressor 返回一个新的压缩写入器，写入到 w 中。WriteCloser 的 Close
// 方法必须必须被用于将等待的数据刷新到 w 中。Compressor 在多个Go程被同步调用时，
// 其自身必须保证安全，但每个返回的写入器一次只会被一个Go程使用。
type Compressor func(w io.Writer) (io.WriteCloser, error)

// A Decompressor returns a new decompressing reader, reading from r.
// The ReadCloser's Close method must be used to release associated resources.
// The Decompressor itself must be safe to invoke from multiple goroutines
// simultaneously, but each returned reader will be used only by
// one goroutine at a time.

// Decompressor 返回一个新的解压读取器，从 r 中读取。ReadCloser 的 Close
// 方法必须被用于释放相关的资源。Decompressor 在多个Go程被同步调用时，
// 其自身必须保证安全，但每个返回的读取器一次只会被一个Go程使用。
type Decompressor func(r io.Reader) io.ReadCloser

var flateWriterPool sync.Pool

func newFlateWriter(w io.Writer) io.WriteCloser {
	fw, ok := flateWriterPool.Get().(*flate.Writer)
	if ok {
		fw.Reset(w)
	} else {
		fw, _ = flate.NewWriter(w, 5)
	}
	return &pooledFlateWriter{fw: fw}
}

type pooledFlateWriter struct {
	mu sync.Mutex // guards Close and Write
	fw *flate.Writer
}

func (w *pooledFlateWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fw == nil {
		return 0, errors.New("Write after Close")
	}
	return w.fw.Write(p)
}

func (w *pooledFlateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var err error
	if w.fw != nil {
		err = w.fw.Close()
		flateWriterPool.Put(w.fw)
		w.fw = nil
	}
	return err
}

var flateReaderPool sync.Pool

func newFlateReader(r io.Reader) io.ReadCloser {
	fr, ok := flateReaderPool.Get().(io.ReadCloser)
	if ok {
		fr.(flate.Resetter).Reset(r, nil)
	} else {
		fr = flate.NewReader(r)
	}
	return &pooledFlateReader{fr: fr}
}

type pooledFlateReader struct {
	mu sync.Mutex // guards Close and Read
	fr io.ReadCloser
}

func (r *pooledFlateReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fr == nil {
		return 0, errors.New("Read after Close")
	}
	return r.fr.Read(p)
}

func (r *pooledFlateReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var err error
	if r.fr != nil {
		err = r.fr.Close()
		flateReaderPool.Put(r.fr)
		r.fr = nil
	}
	return err
}

var (
	mu sync.RWMutex // guards compressor and decompressor maps

	compressors = map[uint16]Compressor{
		Store:   func(w io.Writer) (io.WriteCloser, error) { return &nopCloser{w}, nil },
		Deflate: func(w io.Writer) (io.WriteCloser, error) { return newFlateWriter(w), nil },
	}

	decompressors = map[uint16]Decompressor{
		Store:   ioutil.NopCloser,
		Deflate: newFlateReader,
	}
)

// RegisterDecompressor allows custom decompressors for a specified method ID.
// The common methods Store and Deflate are built in.

// RegisterDecompressor使用指定的方法ID注册一个Decompressor类型函数。
// 通用方法 Store 和 Deflate 是内建的。
func RegisterDecompressor(method uint16, dcomp Decompressor) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := decompressors[method]; ok {
		panic("decompressor already registered")
	}
	decompressors[method] = dcomp
}

// RegisterCompressor registers custom compressors for a specified method ID.
// The common methods Store and Deflate are built in.

// RegisterCompressor使用指定的方法ID注册一个Compressor类型函数。
// 常用的方法Store和Deflate是内建的。
func RegisterCompressor(method uint16, comp Compressor) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := compressors[method]; ok {
		panic("compressor already registered")
	}
	compressors[method] = comp
}

func compressor(method uint16) Compressor {
	mu.RLock()
	defer mu.RUnlock()
	return compressors[method]
}

func decompressor(method uint16) Decompressor {
	mu.RLock()
	defer mu.RUnlock()
	return decompressors[method]
}
