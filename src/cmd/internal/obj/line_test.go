// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package obj

import (
	"fmt"
	"testing"
)

func TestLineHist(t *testing.T) {
	ctxt := new(Link)
	ctxt.Hash = make(map[SymVer]*LSym)

	Linklinehist(ctxt, 1, "a.c", 0)
	Linklinehist(ctxt, 3, "a.h", 0)
	Linklinehist(ctxt, 5, "<pop>", 0)
	Linklinehist(ctxt, 7, "linedir", 2)
	Linklinehist(ctxt, 9, "<pop>", 0)
	Linklinehist(ctxt, 11, "b.c", 0)
	Linklinehist(ctxt, 13, "<pop>", 0)

	var expect = []string{
		0:  "??:0",
		1:  "a.c:1",
		2:  "a.c:2",
		3:  "a.h:1",
		4:  "a.h:2",
		5:  "a.c:3",
		6:  "a.c:4",
		7:  "linedir:2",
		8:  "linedir:3",
		9:  "??:0",
		10: "??:0",
		11: "b.c:1",
		12: "b.c:2",
		13: "??:0",
		14: "??:0",
	}

	for i, want := range expect {
		var f *LSym
		var l int32
		linkgetline(ctxt, int32(i), &f, &l)
		have := fmt.Sprintf("%s:%d", f.Name, l)
		if have != want {
			t.Errorf("linkgetline(%d) = %q, want %q", i, have, want)
		}
	}
}
