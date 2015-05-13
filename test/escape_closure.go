// errorcheck -0 -m -l

// Copyright 2015 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test escape analysis for closure arguments.

package escape

var sink interface{}

func ClosureCallArgs0() {
	x := 0         // ERROR "moved to heap: x"
	func(p *int) { // ERROR "p does not escape" "func literal does not escape"
		*p = 1
		// BAD: x should not escape to heap here
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs1() {
	x := 0 // ERROR "moved to heap: x"
	for {
		func(p *int) { // ERROR "p does not escape" "func literal does not escape"
			*p = 1
			// BAD: x should not escape to heap here
		}(&x) // ERROR "&x escapes to heap"
	}
}

func ClosureCallArgs2() {
	for {
		// BAD: x should not escape here
		x := 0         // ERROR "moved to heap: x"
		func(p *int) { // ERROR "p does not escape" "func literal does not escape"
			*p = 1
		}(&x) // ERROR "&x escapes to heap"
	}
}

func ClosureCallArgs3() {
	x := 0         // ERROR "moved to heap: x"
	func(p *int) { // ERROR "leaking param: p" "func literal does not escape"
		sink = p // ERROR "p escapes to heap"
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs4() {
	// BAD: x should not leak here
	x := 0                  // ERROR "moved to heap: x"
	_ = func(p *int) *int { // ERROR "leaking param: p to result ~r1" "func literal does not escape"
		return p
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs5() {
	x := 0                     // ERROR "moved to heap: x"
	sink = func(p *int) *int { // ERROR "leaking param: p to result ~r1" "func literal does not escape"
		return p
	}(&x) // ERROR "&x escapes to heap" "\(func literal\)\(&x\) escapes to heap"
}

func ClosureCallArgs6() {
	x := 0         // ERROR "moved to heap: x"
	func(p *int) { // ERROR "moved to heap: p" "func literal does not escape"
		sink = &p // ERROR "&p escapes to heap"
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs7() {
	var pp *int
	for {
		x := 0         // ERROR "moved to heap: x"
		func(p *int) { // ERROR "leaking param: p" "func literal does not escape"
			pp = p
		}(&x) // ERROR "&x escapes to heap"
	}
	_ = pp
}

func ClosureCallArgs8() {
	x := 0               // ERROR "moved to heap: x"
	defer func(p *int) { // ERROR "p does not escape" "func literal does not escape"
		*p = 1
		// BAD: x should not escape to heap here
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs9() {
	// BAD: x should not leak
	x := 0 // ERROR "moved to heap: x"
	for {
		defer func(p *int) { // ERROR "func literal escapes to heap" "p does not escape"
			*p = 1
		}(&x) // ERROR "&x escapes to heap"
	}
}

func ClosureCallArgs10() {
	for {
		x := 0               // ERROR "moved to heap: x"
		defer func(p *int) { // ERROR "func literal escapes to heap" "p does not escape"
			*p = 1
		}(&x) // ERROR "&x escapes to heap"
	}
}

func ClosureCallArgs11() {
	x := 0               // ERROR "moved to heap: x"
	defer func(p *int) { // ERROR "leaking param: p" "func literal does not escape"
		sink = p // ERROR "p escapes to heap"
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs12() {
	// BAD: x should not leak
	x := 0                    // ERROR "moved to heap: x"
	defer func(p *int) *int { // ERROR "leaking param: p to result ~r1" "func literal does not escape"
		return p
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs13() {
	x := 0               // ERROR "moved to heap: x"
	defer func(p *int) { // ERROR "moved to heap: p" "func literal does not escape"
		sink = &p // ERROR "&p escapes to heap"
	}(&x) // ERROR "&x escapes to heap"
}

func ClosureCallArgs14() {
	x := 0 // ERROR "moved to heap: x"
	// BAD: &x should not escape here
	p := &x                  // ERROR "moved to heap: p" "&x escapes to heap"
	_ = func(p **int) *int { // ERROR "leaking param: p to result ~r1 level=1" "func literal does not escape"
		return *p
		// BAD: p should not escape here
	}(&p) // ERROR "&p escapes to heap"
}

func ClosureCallArgs15() {
	x := 0                      // ERROR "moved to heap: x"
	p := &x                     // ERROR "moved to heap: p" "&x escapes to heap"
	sink = func(p **int) *int { // ERROR "leaking param: p to result ~r1 level=1" "func literal does not escape"
		return *p
		// BAD: p should not escape here
	}(&p) // ERROR "&p escapes to heap" "\(func literal\)\(&p\) escapes to heap"
}
