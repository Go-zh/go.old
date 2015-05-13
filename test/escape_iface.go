// errorcheck -0 -m -l

// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test escape analysis for interface conversions.

package escape

var sink interface{}

type M interface {
	M()
}

func mescapes(m M) { // ERROR "leaking param: m"
	sink = m // ERROR "m escapes to heap"
}

func mdoesnotescape(m M) { // ERROR "m does not escape"
}

// Tests for type stored directly in iface and with value receiver method.
type M0 struct {
	p *int
}

func (M0) M() {
}

func efaceEscape0() {
	{
		i := 0
		v := M0{&i} // ERROR "&i does not escape"
		var x M = v // ERROR "v does not escape"
		_ = x
	}
	{
		i := 0      // ERROR "moved to heap: i"
		v := M0{&i} // ERROR "&i escapes to heap"
		var x M = v // ERROR "v escapes to heap"
		sink = x    // ERROR "x escapes to heap"
	}
	{
		i := 0
		v := M0{&i} // ERROR "&i does not escape"
		var x M = v // ERROR "v does not escape"
		v1 := x.(M0)
		_ = v1
	}
	{
		i := 0      // ERROR "moved to heap: i"
		v := M0{&i} // ERROR "&i escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		v1 := x.(M0)
		sink = v1 // ERROR "v1 escapes to heap"
	}
	{
		i := 0      // ERROR "moved to heap: i"
		v := M0{&i} // ERROR "&i escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		x.M()
	}
	{
		i := 0      // ERROR "moved to heap: i"
		v := M0{&i} // ERROR "&i escapes to heap"
		var x M = v // ERROR "v escapes to heap"
		mescapes(x)
	}
	{
		i := 0
		v := M0{&i} // ERROR "&i does not escape"
		var x M = v // ERROR "v does not escape"
		mdoesnotescape(x)
	}
}

// Tests for type stored indirectly in iface and with value receiver method.
type M1 struct {
	p *int
	x int
}

func (M1) M() {
}

func efaceEscape1() {
	{
		i := 0
		v := M1{&i, 0} // ERROR "&i does not escape"
		var x M = v    // ERROR "v does not escape"
		_ = x
	}
	{
		i := 0         // ERROR "moved to heap: i"
		v := M1{&i, 0} // ERROR "&i escapes to heap"
		var x M = v    // ERROR "v escapes to heap"
		sink = x       // ERROR "x escapes to heap"
	}
	{
		i := 0
		v := M1{&i, 0} // ERROR "&i does not escape"
		var x M = v    // ERROR "v does not escape"
		v1 := x.(M1)
		_ = v1
	}
	{
		i := 0         // ERROR "moved to heap: i"
		v := M1{&i, 0} // ERROR "&i escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		v1 := x.(M1)
		sink = v1 // ERROR "v1 escapes to heap"
	}
	{
		i := 0         // ERROR "moved to heap: i"
		v := M1{&i, 0} // ERROR "&i escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		x.M()
	}
	{
		i := 0         // ERROR "moved to heap: i"
		v := M1{&i, 0} // ERROR "&i escapes to heap"
		var x M = v    // ERROR "v escapes to heap"
		mescapes(x)
	}
	{
		i := 0
		v := M1{&i, 0} // ERROR "&i does not escape"
		var x M = v    // ERROR "v does not escape"
		mdoesnotescape(x)
	}
}

// Tests for type stored directly in iface and with pointer receiver method.
type M2 struct {
	p *int
}

func (*M2) M() {
}

func efaceEscape2() {
	{
		i := 0
		v := &M2{&i} // ERROR "&i does not escape" "&M2 literal does not escape"
		var x M = v  // ERROR "v does not escape"
		_ = x
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal escapes to heap"
		var x M = v  // ERROR "v escapes to heap"
		sink = x     // ERROR "x escapes to heap"
	}
	{
		i := 0
		v := &M2{&i} // ERROR "&i does not escape" "&M2 literal does not escape"
		var x M = v  // ERROR "v does not escape"
		v1 := x.(*M2)
		_ = v1
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		v1 := x.(*M2)
		sink = v1 // ERROR "v1 escapes to heap"
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal does not escape"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v does not escape"
		v1 := x.(*M2)
		sink = *v1 // ERROR "v1 escapes to heap"
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal does not escape"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v does not escape"
		v1, ok := x.(*M2)
		sink = *v1 // ERROR "v1 escapes to heap"
		_ = ok
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal escapes to heap"
		// BAD: v does not escape to heap here
		var x M = v // ERROR "v escapes to heap"
		x.M()
	}
	{
		i := 0       // ERROR "moved to heap: i"
		v := &M2{&i} // ERROR "&i escapes to heap" "&M2 literal escapes to heap"
		var x M = v  // ERROR "v escapes to heap"
		mescapes(x)
	}
	{
		i := 0
		v := &M2{&i} // ERROR "&i does not escape" "&M2 literal does not escape"
		var x M = v  // ERROR "v does not escape"
		mdoesnotescape(x)
	}
}
