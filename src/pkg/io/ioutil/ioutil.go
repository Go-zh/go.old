// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ioutil implements some I/O utility functions.

// ioutil 实现了一些I/O的工具函数。
package ioutil

import (
	"bytes"
	"io"
	"os"
	"sort"
)

// readAll reads from r until an error or EOF and returns the data it read
// from the internal buffer allocated with a specified capacity.

// readAll 从 r 中读取，直至遇到错误或EOF，然后返回它从以指定容量分配的内部缓存中读取的数据。
func readAll(r io.Reader, capacity int64) (b []byte, err error) {
	buf := bytes.NewBuffer(make([]byte, 0, capacity))
	// If the buffer overflows, we will get bytes.ErrTooLarge.
	// Return that as an error. Any other panic remains.
	//
	// 若该缓存溢出，我们会获取 bytes.ErrTooLarge，并将其作为错误返回。
	// 其它任何情况均视作恐慌。
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		if panicErr, ok := e.(error); ok && panicErr == bytes.ErrTooLarge {
			err = panicErr
		} else {
			panic(e)
		}
	}()
	_, err = buf.ReadFrom(r)
	return buf.Bytes(), err
}

// ReadAll reads from r until an error or EOF and returns the data it read.
// A successful call returns err == nil, not err == EOF. Because ReadAll is
// defined to read from src until EOF, it does not treat an EOF from Read
// as an error to be reported.

// ReadAll 从 r 中读取，直至遇到错误或EOF，然后返回它所读取的数据。
// 一次成功的调用应当返回 err == nil，而非 err == 因为 ReadAll 被定义为从 src
// 进行读取直至遇到EOF，它并不会将来自 Read 的EOF视作错误来报告。
func ReadAll(r io.Reader) ([]byte, error) {
	return readAll(r, bytes.MinRead)
}

// ReadFile reads the file named by filename and returns the contents.
// A successful call returns err == nil, not err == EOF. Because ReadFile
// reads the whole file, it does not treat an EOF from Read as an error
// to be reported.

// ReadFile 读取名为 filename 的文件并返回其内容。
// 一次成功的调用应当返回 err == nil，而非 err == EOF。因为 ReadFile 会读取整个文件，
// 它并不会将来自 Read 的EOF视作错误来报告。
func ReadFile(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// It's a good but not certain bet that FileInfo will tell us exactly how much to
	// read, so let's try it but be prepared for the answer to be wrong.
	//
	// 这是个不错的方法，但 FileInfo 不敢打赌它能确切地告诉我们读取了多少，
	// 所以让我们先试试，但要做好得到错误答案的准备。
	var n int64

	if fi, err := f.Stat(); err == nil {
		// Don't preallocate a huge buffer, just in case.
		// 不要预先分配一个巨大的缓存，按需分配就好。
		if size := fi.Size(); size < 1e9 {
			n = size
		}
	}
	// As initial capacity for readAll, use n + a little extra in case Size is zero,
	// and to avoid another allocation after Read has filled the buffer.  The readAll
	// call will read into its allocated internal buffer cheaply.  If the size was
	// wrong, we'll either waste some space off the end or reallocate as needed, but
	// in the overwhelmingly common case we'll get it just right.
	//
	// 根据 readAll 的初始容量，采用 n 加上一点点额外的容量，此时 Size 为零，这样避免了在
	// Read 之后的另一次分配被填入该缓存。readAll 之需要很小的代价就能将数据读取到
	// 其内部分配的缓存中。若该大小错误，我们就会在末尾浪费一点空间或根据需要重新分配，
	// 但在绝大多数情况下，我们会使它恰到好处。
	return readAll(f, n+bytes.MinRead)
}

// WriteFile writes data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm;
// otherwise WriteFile truncates it before writing.

// WriteFile 将数据写入到名为 filename 的文件中。
// 若该文件不存在，WriteFile 就会按照权限 perm 创建它；否则 WriteFile 就会在写入前将其截断。
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	n, err := f.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

// byName implements sort.Interface.

// byName 实现了 sort.Interface。
type byName []os.FileInfo

func (f byName) Len() int           { return len(f) }
func (f byName) Less(i, j int) bool { return f[i].Name() < f[j].Name() }
func (f byName) Swap(i, j int)      { f[i], f[j] = f[j], f[i] }

// ReadDir reads the directory named by dirname and returns
// a list of sorted directory entries.

// ReadDir 读取名为 dirname 的目录并返回一个已排序的目录项列表。
func ReadDir(dirname string) ([]os.FileInfo, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	list, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		return nil, err
	}
	sort.Sort(byName(list))
	return list, nil
}

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

// NopCloser returns a ReadCloser with a no-op Close method wrapping
// the provided Reader r.

// NopCloser 将提供的 Reader r 用空操作 Close 方法包装后作为 ReadCloser 返回。
func NopCloser(r io.Reader) io.ReadCloser {
	return nopCloser{r}
}

type devNull int

// devNull implements ReaderFrom as an optimization so io.Copy to
// ioutil.Discard can avoid doing unnecessary work.

// devNull 为优化实现了 ReaderFrom，因此 io.Copy 到 ioutil.Discard 避免了不必要的工作。
var _ io.ReaderFrom = devNull(0)

func (devNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (devNull) ReadFrom(r io.Reader) (n int64, err error) {
	buf := blackHole()
	defer blackHolePut(buf)
	readSize := 0
	for {
		readSize, err = r.Read(buf)
		n += int64(readSize)
		if err != nil {
			if err == io.EOF {
				return n, nil
			}
			return
		}
	}
}

// Discard is an io.Writer on which all Write calls succeed
// without doing anything.

// Discard 是一个 io.Writer，对它进行的任何 Write 调用都将无条件成功。
var Discard io.Writer = devNull(0)
