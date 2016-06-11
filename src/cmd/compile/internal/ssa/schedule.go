// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import "container/heap"

const (
	ScorePhi = iota // towards top of block
	ScoreVarDef
	ScoreMemory
	ScoreDefault
	ScoreFlags
	ScoreControl // towards bottom of block
)

type ValHeap struct {
	a     []*Value
	score []int8
}

func (h ValHeap) Len() int      { return len(h.a) }
func (h ValHeap) Swap(i, j int) { a := h.a; a[i], a[j] = a[j], a[i] }

func (h *ValHeap) Push(x interface{}) {
	// Push and Pop use pointer receivers because they modify the slice's length,
	// not just its contents.
	v := x.(*Value)
	h.a = append(h.a, v)
}
func (h *ValHeap) Pop() interface{} {
	old := h.a
	n := len(old)
	x := old[n-1]
	h.a = old[0 : n-1]
	return x
}
func (h ValHeap) Less(i, j int) bool {
	x := h.a[i]
	y := h.a[j]
	sx := h.score[x.ID]
	sy := h.score[y.ID]
	if c := sx - sy; c != 0 {
		return c > 0 // higher score comes later.
	}
	if x.Line != y.Line { // Favor in-order line stepping
		return x.Line > y.Line
	}
	if x.Op != OpPhi {
		if c := len(x.Args) - len(y.Args); c != 0 {
			return c < 0 // smaller args comes later
		}
	}
	return x.ID > y.ID
}

// Schedule the Values in each Block. After this phase returns, the
// order of b.Values matters and is the order in which those values
// will appear in the assembly output. For now it generates a
// reasonable valid schedule using a priority queue. TODO(khr):
// schedule smarter.
func schedule(f *Func) {
	// For each value, the number of times it is used in the block
	// by values that have not been scheduled yet.
	uses := make([]int32, f.NumValues())

	// reusable priority queue
	priq := new(ValHeap)

	// "priority" for a value
	score := make([]int8, f.NumValues())

	// scheduling order. We queue values in this list in reverse order.
	var order []*Value

	// maps mem values to the next live memory value
	nextMem := make([]*Value, f.NumValues())
	// additional pretend arguments for each Value. Used to enforce load/store ordering.
	additionalArgs := make([][]*Value, f.NumValues())

	for _, b := range f.Blocks {
		// Compute score. Larger numbers are scheduled closer to the end of the block.
		for _, v := range b.Values {
			switch {
			case v.Op == OpAMD64LoweredGetClosurePtr:
				// We also score GetLoweredClosurePtr as early as possible to ensure that the
				// context register is not stomped. GetLoweredClosurePtr should only appear
				// in the entry block where there are no phi functions, so there is no
				// conflict or ambiguity here.
				if b != f.Entry {
					f.Fatalf("LoweredGetClosurePtr appeared outside of entry block, b=%s", b.String())
				}
				score[v.ID] = ScorePhi
			case v.Op == OpPhi:
				// We want all the phis first.
				score[v.ID] = ScorePhi
			case v.Op == OpVarDef:
				// We want all the vardefs next.
				score[v.ID] = ScoreVarDef
			case v.Type.IsMemory():
				// Schedule stores as early as possible. This tends to
				// reduce register pressure. It also helps make sure
				// VARDEF ops are scheduled before the corresponding LEA.
				score[v.ID] = ScoreMemory
			case v.Type.IsFlags():
				// Schedule flag register generation as late as possible.
				// This makes sure that we only have one live flags
				// value at a time.
				score[v.ID] = ScoreFlags
			default:
				score[v.ID] = ScoreDefault
			}
		}
	}

	for _, b := range f.Blocks {
		// Find store chain for block.
		// Store chains for different blocks overwrite each other, so
		// the calculated store chain is good only for this block.
		for _, v := range b.Values {
			if v.Op != OpPhi && v.Type.IsMemory() {
				for _, w := range v.Args {
					if w.Type.IsMemory() {
						nextMem[w.ID] = v
					}
				}
			}
		}

		// Compute uses.
		for _, v := range b.Values {
			if v.Op == OpPhi {
				// If a value is used by a phi, it does not induce
				// a scheduling edge because that use is from the
				// previous iteration.
				continue
			}
			for _, w := range v.Args {
				if w.Block == b {
					uses[w.ID]++
				}
				// Any load must come before the following store.
				if v.Type.IsMemory() || !w.Type.IsMemory() {
					continue // not a load
				}
				s := nextMem[w.ID]
				if s == nil || s.Block != b {
					continue
				}
				additionalArgs[s.ID] = append(additionalArgs[s.ID], v)
				uses[v.ID]++
			}
		}

		if b.Control != nil && b.Control.Op != OpPhi {
			// Force the control value to be scheduled at the end,
			// unless it is a phi value (which must be first).
			score[b.Control.ID] = ScoreControl

			// Schedule values dependent on the control value at the end.
			// This reduces the number of register spills. We don't find
			// all values that depend on the control, just values with a
			// direct dependency. This is cheaper and in testing there
			// was no difference in the number of spills.
			for _, v := range b.Values {
				if v.Op != OpPhi {
					for _, a := range v.Args {
						if a == b.Control {
							score[v.ID] = ScoreControl
						}
					}
				}
			}
		}

		// To put things into a priority queue
		// The values that should come last are least.
		priq.score = score
		priq.a = priq.a[:0]

		// Initialize priority queue with schedulable values.
		for _, v := range b.Values {
			if uses[v.ID] == 0 {
				heap.Push(priq, v)
			}
		}

		// Schedule highest priority value, update use counts, repeat.
		order = order[:0]
		for {
			// Find highest priority schedulable value.
			// Note that schedule is assembled backwards.

			if priq.Len() == 0 {
				break
			}

			v := heap.Pop(priq).(*Value)

			// Add it to the schedule.
			order = append(order, v)

			// Update use counts of arguments.
			for _, w := range v.Args {
				if w.Block != b {
					continue
				}
				uses[w.ID]--
				if uses[w.ID] == 0 {
					// All uses scheduled, w is now schedulable.
					heap.Push(priq, w)
				}
			}
			for _, w := range additionalArgs[v.ID] {
				uses[w.ID]--
				if uses[w.ID] == 0 {
					// All uses scheduled, w is now schedulable.
					heap.Push(priq, w)
				}
			}
		}
		if len(order) != len(b.Values) {
			f.Fatalf("schedule does not include all values")
		}
		for i := 0; i < len(b.Values); i++ {
			b.Values[i] = order[len(b.Values)-1-i]
		}
	}

	f.scheduled = true
}
