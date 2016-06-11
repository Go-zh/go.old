// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io

type multiReader struct {
	readers []Reader
}

func (mr *multiReader) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		// Optimization to flatten nested multiReaders (Issue 13558)
		if len(mr.readers) == 1 {
			if r, ok := mr.readers[0].(*multiReader); ok {
				mr.readers = r.readers
				continue
			}
		}
		n, err = mr.readers[0].Read(p)
		if n > 0 || err != EOF {
			if err == EOF {
				// Don't return EOF yet. There may be more bytes
				// in the remaining readers.
				// 还不能返回 EOF。剩下的读取器中可能还有更多字节。
				err = nil
			}
			return
		}
		mr.readers = mr.readers[1:]
	}
	return 0, EOF
}

// MultiReader returns a Reader that's the logical concatenation of
// the provided input readers. They're read sequentially. Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.

// MultiReader 返回一个 Reader，它是输入 readers 提供的的逻辑拼接。
// 它们按顺序读取。一旦所有的输入返回 EOF，Read 就会返回 EOF。
// 若任何 readers 返回了非 nil 或非 EOF 错误，Read 就会返回该错误。
func MultiReader(readers ...Reader) Reader {
	r := make([]Reader, len(readers))
	copy(r, readers)
	return &multiReader{r}
}

type multiWriter struct {
	writers []Writer
}

func (t *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = ErrShortWrite
			return
		}
	}
	return len(p), nil
}

var _ stringWriter = (*multiWriter)(nil)

func (t *multiWriter) WriteString(s string) (n int, err error) {
	var p []byte // lazily initialized if/when needed
	for _, w := range t.writers {
		if sw, ok := w.(stringWriter); ok {
			n, err = sw.WriteString(s)
		} else {
			if p == nil {
				p = []byte(s)
			}
			n, err = w.Write(p)
		}
		if err != nil {
			return
		}
		if n != len(s) {
			err = ErrShortWrite
			return
		}
	}
	return len(s), nil
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.

// MultiWriter 创建一个 Writer，它将其写入复制到所有提供的 writers 中，类似于Unix的tee(1)命令。
func MultiWriter(writers ...Writer) Writer {
	w := make([]Writer, len(writers))
	copy(w, writers)
	return &multiWriter{w}
}
