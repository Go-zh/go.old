// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sha1

// featureCheck reports whether the CPU supports the
// SHA1 compute intermediate message digest (KIMD)
// function code.
func featureCheck() bool

var useAsm = featureCheck()
