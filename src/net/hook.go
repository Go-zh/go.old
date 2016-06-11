// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import "context"

var (
	// if non-nil, overrides dialTCP.
	testHookDialTCP func(ctx context.Context, net string, laddr, raddr *TCPAddr) (*TCPConn, error)

	testHookHostsPath = "/etc/hosts"
	testHookLookupIP  = func(
		ctx context.Context,
		fn func(context.Context, string) ([]IPAddr, error),
		host string,
	) ([]IPAddr, error) {
		return fn(ctx, host)
	}
	testHookSetKeepAlive = func() {}
)
