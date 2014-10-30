// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errors implements functions to manipulate errors.

// error 包实现了用于错误处理的函数.
package errors

// New returns an error that formats as the given text.

// New 返回一个给定文本格式的错误。
func New(text string) error {
	return &errorString{text}
}

// errorString is a trivial implementation of error.

// errorString 是 error 的一个琐碎的实现。
type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}
