// errorcheck

// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

type T	// ERROR "unexpected semicolon or newline in type declaration"
// line below uncommented to avoid follow-up error
// {