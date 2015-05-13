// Derived from Inferno utils/6c/gc.h
// http://code.google.com/p/inferno-os/source/browse/utils/6c/gc.h
//
//	Copyright © 1994-1999 Lucent Technologies Inc.  All rights reserved.
//	Portions Copyright © 1995-1997 C H Forsyth (forsyth@terzarima.net)
//	Portions Copyright © 1997-1999 Vita Nuova Limited
//	Portions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com)
//	Portions Copyright © 2004,2006 Bruce Ellis
//	Portions Copyright © 2005-2007 C H Forsyth (forsyth@terzarima.net)
//	Revisions Copyright © 2000-2007 Lucent Technologies Inc. and others
//	Portions Copyright © 2009 The Go Authors.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// "Portable" optimizations.

package gc

import (
	"cmd/internal/obj"
	"fmt"
	"sort"
	"strings"
)

type OptStats struct {
	Ncvtreg int32
	Nspill  int32
	Nreload int32
	Ndelmov int32
	Nvar    int32
	Naddr   int32
}

var Ostats OptStats

var noreturn_symlist [10]*Sym

// p is a call instruction. Does the call fail to return?
func Noreturn(p *obj.Prog) bool {
	if noreturn_symlist[0] == nil {
		noreturn_symlist[0] = Pkglookup("panicindex", Runtimepkg)
		noreturn_symlist[1] = Pkglookup("panicslice", Runtimepkg)
		noreturn_symlist[2] = Pkglookup("throwinit", Runtimepkg)
		noreturn_symlist[3] = Pkglookup("gopanic", Runtimepkg)
		noreturn_symlist[4] = Pkglookup("panicwrap", Runtimepkg)
		noreturn_symlist[5] = Pkglookup("throwreturn", Runtimepkg)
		noreturn_symlist[6] = Pkglookup("selectgo", Runtimepkg)
		noreturn_symlist[7] = Pkglookup("block", Runtimepkg)
	}

	if p.To.Node == nil {
		return false
	}
	s := ((p.To.Node).(*Node)).Sym
	if s == nil {
		return false
	}
	for i := 0; noreturn_symlist[i] != nil; i++ {
		if s == noreturn_symlist[i] {
			return true
		}
	}
	return false
}

// JMP chasing and removal.
//
// The code generator depends on being able to write out jump
// instructions that it can jump to now but fill in later.
// the linker will resolve them nicely, but they make the code
// longer and more difficult to follow during debugging.
// Remove them.

/* what instruction does a JMP to p eventually land on? */
func chasejmp(p *obj.Prog, jmploop *int) *obj.Prog {
	n := 0
	for p != nil && p.As == obj.AJMP && p.To.Type == obj.TYPE_BRANCH {
		n++
		if n > 10 {
			*jmploop = 1
			break
		}

		p = p.To.Val.(*obj.Prog)
	}

	return p
}

/*
 * reuse reg pointer for mark/sweep state.
 * leave reg==nil at end because alive==nil.
 */
var alive interface{} = nil
var dead interface{} = 1

/* mark all code reachable from firstp as alive */
func mark(firstp *obj.Prog) {
	for p := firstp; p != nil; p = p.Link {
		if p.Opt != dead {
			break
		}
		p.Opt = alive
		if p.As != obj.ACALL && p.To.Type == obj.TYPE_BRANCH && p.To.Val.(*obj.Prog) != nil {
			mark(p.To.Val.(*obj.Prog))
		}
		if p.As == obj.AJMP || p.As == obj.ARET || p.As == obj.AUNDEF {
			break
		}
	}
}

func fixjmp(firstp *obj.Prog) {
	if Debug['R'] != 0 && Debug['v'] != 0 {
		fmt.Printf("\nfixjmp\n")
	}

	// pass 1: resolve jump to jump, mark all code as dead.
	jmploop := 0

	for p := firstp; p != nil; p = p.Link {
		if Debug['R'] != 0 && Debug['v'] != 0 {
			fmt.Printf("%v\n", p)
		}
		if p.As != obj.ACALL && p.To.Type == obj.TYPE_BRANCH && p.To.Val.(*obj.Prog) != nil && p.To.Val.(*obj.Prog).As == obj.AJMP {
			p.To.Val = chasejmp(p.To.Val.(*obj.Prog), &jmploop)
			if Debug['R'] != 0 && Debug['v'] != 0 {
				fmt.Printf("->%v\n", p)
			}
		}

		p.Opt = dead
	}

	if Debug['R'] != 0 && Debug['v'] != 0 {
		fmt.Printf("\n")
	}

	// pass 2: mark all reachable code alive
	mark(firstp)

	// pass 3: delete dead code (mostly JMPs).
	var last *obj.Prog

	for p := firstp; p != nil; p = p.Link {
		if p.Opt == dead {
			if p.Link == nil && p.As == obj.ARET && last != nil && last.As != obj.ARET {
				// This is the final ARET, and the code so far doesn't have one.
				// Let it stay. The register allocator assumes that all live code in
				// the function can be traversed by starting at all the RET instructions
				// and following predecessor links. If we remove the final RET,
				// this assumption will not hold in the case of an infinite loop
				// at the end of a function.
				// Keep the RET but mark it dead for the liveness analysis.
				p.Mode = 1
			} else {
				if Debug['R'] != 0 && Debug['v'] != 0 {
					fmt.Printf("del %v\n", p)
				}
				continue
			}
		}

		if last != nil {
			last.Link = p
		}
		last = p
	}

	last.Link = nil

	// pass 4: elide JMP to next instruction.
	// only safe if there are no jumps to JMPs anymore.
	if jmploop == 0 {
		var last *obj.Prog
		for p := firstp; p != nil; p = p.Link {
			if p.As == obj.AJMP && p.To.Type == obj.TYPE_BRANCH && p.To.Val == p.Link {
				if Debug['R'] != 0 && Debug['v'] != 0 {
					fmt.Printf("del %v\n", p)
				}
				continue
			}

			if last != nil {
				last.Link = p
			}
			last = p
		}

		last.Link = nil
	}

	if Debug['R'] != 0 && Debug['v'] != 0 {
		fmt.Printf("\n")
		for p := firstp; p != nil; p = p.Link {
			fmt.Printf("%v\n", p)
		}
		fmt.Printf("\n")
	}
}

// Control flow analysis. The Flow structures hold predecessor and successor
// information as well as basic loop analysis.
//
//	graph = flowstart(firstp, 0);
//	... use flow graph ...
//	flowend(graph); // free graph
//
// Typical uses of the flow graph are to iterate over all the flow-relevant instructions:
//
//	for(f = graph->start; f != nil; f = f->link)
//
// or, given an instruction f, to iterate over all the predecessors, which is
// f->p1 and this list:
//
//	for(f2 = f->p2; f2 != nil; f2 = f2->p2link)
//
// The size argument to flowstart specifies an amount of zeroed memory
// to allocate in every f->data field, for use by the client.
// If size == 0, f->data will be nil.

var flowmark int

// MaxFlowProg is the maximum size program (counted in instructions)
// for which the flow code will build a graph. Functions larger than this limit
// will not have flow graphs and consequently will not be optimized.
const MaxFlowProg = 50000

func Flowstart(firstp *obj.Prog, newData func() interface{}) *Graph {
	// Count and mark instructions to annotate.
	nf := 0

	for p := firstp; p != nil; p = p.Link {
		p.Opt = nil // should be already, but just in case
		Thearch.Proginfo(p)
		if p.Info.Flags&Skip != 0 {
			continue
		}
		p.Opt = &flowmark
		nf++
	}

	if nf == 0 {
		return nil
	}

	if nf >= MaxFlowProg {
		if Debug['v'] != 0 {
			Warn("%v is too big (%d instructions)", Curfn.Nname.Sym, nf)
		}
		return nil
	}

	// Allocate annotations and assign to instructions.
	graph := new(Graph)
	ff := make([]Flow, nf)
	start := &ff[0]
	id := 0
	var last *Flow
	for p := firstp; p != nil; p = p.Link {
		if p.Opt == nil {
			continue
		}
		f := &ff[0]
		ff = ff[1:]
		p.Opt = f
		f.Prog = p
		if last != nil {
			last.Link = f
		}
		last = f
		if newData != nil {
			f.Data = newData()
		}
		f.Id = int32(id)
		id++
	}

	// Fill in pred/succ information.
	var f1 *Flow
	var p *obj.Prog
	for f := start; f != nil; f = f.Link {
		p = f.Prog
		if p.Info.Flags&Break == 0 {
			f1 = f.Link
			f.S1 = f1
			f1.P1 = f
		}

		if p.To.Type == obj.TYPE_BRANCH {
			if p.To.Val == nil {
				Fatal("pnil %v", p)
			}
			f1 = p.To.Val.(*obj.Prog).Opt.(*Flow)
			if f1 == nil {
				Fatal("fnil %v / %v", p, p.To.Val.(*obj.Prog))
			}
			if f1 == f {
				//fatal("self loop %P", p);
				continue
			}

			f.S2 = f1
			f.P2link = f1.P2
			f1.P2 = f
		}
	}

	graph.Start = start
	graph.Num = nf
	return graph
}

func Flowend(graph *Graph) {
	for f := graph.Start; f != nil; f = f.Link {
		f.Prog.Info.Flags = 0 // drop cached proginfo
		f.Prog.Opt = nil
	}
}

/*
 * find looping structure
 *
 * 1) find reverse postordering
 * 2) find approximate dominators,
 *	the actual dominators if the flow graph is reducible
 *	otherwise, dominators plus some other non-dominators.
 *	See Matthew S. Hecht and Jeffrey D. Ullman,
 *	"Analysis of a Simple Algorithm for Global Data Flow Problems",
 *	Conf.  Record of ACM Symp. on Principles of Prog. Langs, Boston, Massachusetts,
 *	Oct. 1-3, 1973, pp.  207-217.
 * 3) find all nodes with a predecessor dominated by the current node.
 *	such a node is a loop head.
 *	recursively, all preds with a greater rpo number are in the loop
 */
func postorder(r *Flow, rpo2r []*Flow, n int32) int32 {
	r.Rpo = 1
	r1 := r.S1
	if r1 != nil && r1.Rpo == 0 {
		n = postorder(r1, rpo2r, n)
	}
	r1 = r.S2
	if r1 != nil && r1.Rpo == 0 {
		n = postorder(r1, rpo2r, n)
	}
	rpo2r[n] = r
	n++
	return n
}

func rpolca(idom []int32, rpo1 int32, rpo2 int32) int32 {
	if rpo1 == -1 {
		return rpo2
	}
	var t int32
	for rpo1 != rpo2 {
		if rpo1 > rpo2 {
			t = rpo2
			rpo2 = rpo1
			rpo1 = t
		}

		for rpo1 < rpo2 {
			t = idom[rpo2]
			if t >= rpo2 {
				Fatal("bad idom")
			}
			rpo2 = t
		}
	}

	return rpo1
}

func doms(idom []int32, r int32, s int32) bool {
	for s > r {
		s = idom[s]
	}
	return s == r
}

func loophead(idom []int32, r *Flow) bool {
	src := r.Rpo
	if r.P1 != nil && doms(idom, src, r.P1.Rpo) {
		return true
	}
	for r = r.P2; r != nil; r = r.P2link {
		if doms(idom, src, r.Rpo) {
			return true
		}
	}
	return false
}

func loopmark(rpo2r **Flow, head int32, r *Flow) {
	if r.Rpo < head || r.Active == head {
		return
	}
	r.Active = head
	r.Loop += LOOP
	if r.P1 != nil {
		loopmark(rpo2r, head, r.P1)
	}
	for r = r.P2; r != nil; r = r.P2link {
		loopmark(rpo2r, head, r)
	}
}

func flowrpo(g *Graph) {
	g.Rpo = make([]*Flow, g.Num)
	idom := make([]int32, g.Num)

	for r1 := g.Start; r1 != nil; r1 = r1.Link {
		r1.Active = 0
	}

	rpo2r := g.Rpo
	d := postorder(g.Start, rpo2r, 0)
	nr := int32(g.Num)
	if d > nr {
		Fatal("too many reg nodes %d %d", d, nr)
	}
	nr = d
	var r1 *Flow
	for i := int32(0); i < nr/2; i++ {
		r1 = rpo2r[i]
		rpo2r[i] = rpo2r[nr-1-i]
		rpo2r[nr-1-i] = r1
	}

	for i := int32(0); i < nr; i++ {
		rpo2r[i].Rpo = i
	}

	idom[0] = 0
	var me int32
	for i := int32(0); i < nr; i++ {
		r1 = rpo2r[i]
		me = r1.Rpo
		d = -1

		// rpo2r[r->rpo] == r protects against considering dead code,
		// which has r->rpo == 0.
		if r1.P1 != nil && rpo2r[r1.P1.Rpo] == r1.P1 && r1.P1.Rpo < me {
			d = r1.P1.Rpo
		}
		for r1 = r1.P2; r1 != nil; r1 = r1.P2link {
			if rpo2r[r1.Rpo] == r1 && r1.Rpo < me {
				d = rpolca(idom, d, r1.Rpo)
			}
		}
		idom[i] = d
	}

	for i := int32(0); i < nr; i++ {
		r1 = rpo2r[i]
		r1.Loop++
		if r1.P2 != nil && loophead(idom, r1) {
			loopmark(&rpo2r[0], i, r1)
		}
	}

	for r1 := g.Start; r1 != nil; r1 = r1.Link {
		r1.Active = 0
	}
}

func Uniqp(r *Flow) *Flow {
	r1 := r.P1
	if r1 == nil {
		r1 = r.P2
		if r1 == nil || r1.P2link != nil {
			return nil
		}
	} else if r.P2 != nil {
		return nil
	}
	return r1
}

func Uniqs(r *Flow) *Flow {
	r1 := r.S1
	if r1 == nil {
		r1 = r.S2
		if r1 == nil {
			return nil
		}
	} else if r.S2 != nil {
		return nil
	}
	return r1
}

// The compilers assume they can generate temporary variables
// as needed to preserve the right semantics or simplify code
// generation and the back end will still generate good code.
// This results in a large number of ephemeral temporary variables.
// Merge temps with non-overlapping lifetimes and equal types using the
// greedy algorithm in Poletto and Sarkar, "Linear Scan Register Allocation",
// ACM TOPLAS 1999.

type TempVar struct {
	node    *Node
	def     *Flow    // definition of temp var
	use     *Flow    // use list, chained through Flow.data
	merge   *TempVar // merge var with this one
	start   int64    // smallest Prog.pc in live range
	end     int64    // largest Prog.pc in live range
	addr    uint8    // address taken - no accurate end
	removed uint8    // removed from program
}

type startcmp []*TempVar

func (x startcmp) Len() int {
	return len(x)
}

func (x startcmp) Swap(i, j int) {
	x[i], x[j] = x[j], x[i]
}

func (x startcmp) Less(i, j int) bool {
	a := x[i]
	b := x[j]

	if a.start < b.start {
		return true
	}
	if a.start > b.start {
		return false
	}

	// Order what's left by id or symbol name,
	// just so that sort is forced into a specific ordering,
	// so that the result of the sort does not depend on
	// the sort implementation.
	if a.def != b.def {
		return int(a.def.Id-b.def.Id) < 0
	}
	if a.node != b.node {
		return stringsCompare(a.node.Sym.Name, b.node.Sym.Name) < 0
	}
	return false
}

// Is n available for merging?
func canmerge(n *Node) bool {
	return n.Class == PAUTO && strings.HasPrefix(n.Sym.Name, "autotmp")
}

func mergetemp(firstp *obj.Prog) {
	const (
		debugmerge = 0
	)

	g := Flowstart(firstp, nil)
	if g == nil {
		return
	}

	// Build list of all mergeable variables.
	nvar := 0
	for l := Curfn.Func.Dcl; l != nil; l = l.Next {
		if canmerge(l.N) {
			nvar++
		}
	}

	var_ := make([]TempVar, nvar)
	nvar = 0
	var n *Node
	var v *TempVar
	for l := Curfn.Func.Dcl; l != nil; l = l.Next {
		n = l.N
		if canmerge(n) {
			v = &var_[nvar]
			nvar++
			n.Opt = v
			v.node = n
		}
	}

	// Build list of uses.
	// We assume that the earliest reference to a temporary is its definition.
	// This is not true of variables in general but our temporaries are all
	// single-use (that's why we have so many!).
	for f := g.Start; f != nil; f = f.Link {
		p := f.Prog
		if p.From.Node != nil && ((p.From.Node).(*Node)).Opt != nil && p.To.Node != nil && ((p.To.Node).(*Node)).Opt != nil {
			Fatal("double node %v", p)
		}
		v = nil
		n, _ = p.From.Node.(*Node)
		if n != nil {
			v, _ = n.Opt.(*TempVar)
		}
		if v == nil {
			n, _ = p.To.Node.(*Node)
			if n != nil {
				v, _ = n.Opt.(*TempVar)
			}
		}
		if v != nil {
			if v.def == nil {
				v.def = f
			}
			f.Data = v.use
			v.use = f
			if n == p.From.Node && (p.Info.Flags&LeftAddr != 0) {
				v.addr = 1
			}
		}
	}

	if debugmerge > 1 && Debug['v'] != 0 {
		Dumpit("before", g.Start, 0)
	}

	nkill := 0

	// Special case.
	for i := 0; i < len(var_); i++ {
		v = &var_[i]
		if v.addr != 0 {
			continue
		}

		// Used in only one instruction, which had better be a write.
		f := v.use
		if f != nil && f.Data.(*Flow) == nil {
			p := f.Prog
			if p.To.Node == v.node && (p.Info.Flags&RightWrite != 0) && p.Info.Flags&RightRead == 0 {
				p.As = obj.ANOP
				p.To = obj.Addr{}
				v.removed = 1
				if debugmerge > 0 && Debug['v'] != 0 {
					fmt.Printf("drop write-only %v\n", v.node.Sym)
				}
			} else {
				Fatal("temp used and not set: %v", p)
			}
			nkill++
			continue
		}

		// Written in one instruction, read in the next, otherwise unused,
		// no jumps to the next instruction. Happens mainly in 386 compiler.
		f = v.use
		if f != nil && f.Link == f.Data.(*Flow) && (f.Data.(*Flow)).Data.(*Flow) == nil && Uniqp(f.Link) == f {
			p := f.Prog
			p1 := f.Link.Prog
			const (
				SizeAny = SizeB | SizeW | SizeL | SizeQ | SizeF | SizeD
			)
			if p.From.Node == v.node && p1.To.Node == v.node && (p.Info.Flags&Move != 0) && (p.Info.Flags|p1.Info.Flags)&(LeftAddr|RightAddr) == 0 && p.Info.Flags&SizeAny == p1.Info.Flags&SizeAny {
				p1.From = p.From
				Thearch.Excise(f)
				v.removed = 1
				if debugmerge > 0 && Debug['v'] != 0 {
					fmt.Printf("drop immediate-use %v\n", v.node.Sym)
				}
			}

			nkill++
			continue
		}
	}

	// Traverse live range of each variable to set start, end.
	// Each flood uses a new value of gen so that we don't have
	// to clear all the r->active words after each variable.
	gen := int32(0)

	for i := 0; i < len(var_); i++ {
		v = &var_[i]
		gen++
		for f := v.use; f != nil; f = f.Data.(*Flow) {
			mergewalk(v, f, uint32(gen))
		}
		if v.addr != 0 {
			gen++
			for f := v.use; f != nil; f = f.Data.(*Flow) {
				varkillwalk(v, f, uint32(gen))
			}
		}
	}

	// Sort variables by start.
	bystart := make([]*TempVar, len(var_))

	for i := 0; i < len(var_); i++ {
		bystart[i] = &var_[i]
	}
	sort.Sort(startcmp(bystart[:len(var_)]))

	// List of in-use variables, sorted by end, so that the ones that
	// will last the longest are the earliest ones in the array.
	// The tail inuse[nfree:] holds no-longer-used variables.
	// In theory we should use a sorted tree so that insertions are
	// guaranteed O(log n) and then the loop is guaranteed O(n log n).
	// In practice, it doesn't really matter.
	inuse := make([]*TempVar, len(var_))

	ninuse := 0
	nfree := len(var_)
	var t *Type
	var v1 *TempVar
	var j int
	for i := 0; i < len(var_); i++ {
		v = bystart[i]
		if debugmerge > 0 && Debug['v'] != 0 {
			fmt.Printf("consider %v: removed=%d\n", Nconv(v.node, obj.FmtSharp), v.removed)
		}

		if v.removed != 0 {
			continue
		}

		// Expire no longer in use.
		for ninuse > 0 && inuse[ninuse-1].end < v.start {
			ninuse--
			v1 = inuse[ninuse]
			nfree--
			inuse[nfree] = v1
		}

		if debugmerge > 0 && Debug['v'] != 0 {
			fmt.Printf("consider %v: removed=%d nfree=%d nvar=%d\n", Nconv(v.node, obj.FmtSharp), v.removed, nfree, len(var_))
		}

		// Find old temp to reuse if possible.
		t = v.node.Type

		for j = nfree; j < len(var_); j++ {
			v1 = inuse[j]
			if debugmerge > 0 && Debug['v'] != 0 {
				fmt.Printf("consider %v: maybe %v: type=%v,%v addrtaken=%v,%v\n", Nconv(v.node, obj.FmtSharp), Nconv(v1.node, obj.FmtSharp), t, v1.node.Type, v.node.Addrtaken, v1.node.Addrtaken)
			}

			// Require the types to match but also require the addrtaken bits to match.
			// If a variable's address is taken, that disables registerization for the individual
			// words of the variable (for example, the base,len,cap of a slice).
			// We don't want to merge a non-addressed var with an addressed one and
			// inhibit registerization of the former.
			if Eqtype(t, v1.node.Type) && v.node.Addrtaken == v1.node.Addrtaken {
				inuse[j] = inuse[nfree]
				nfree++
				if v1.merge != nil {
					v.merge = v1.merge
				} else {
					v.merge = v1
				}
				nkill++
				break
			}
		}

		// Sort v into inuse.
		j = ninuse
		ninuse++

		for j > 0 && inuse[j-1].end < v.end {
			inuse[j] = inuse[j-1]
			j--
		}

		inuse[j] = v
	}

	if debugmerge > 0 && Debug['v'] != 0 {
		fmt.Printf("%v [%d - %d]\n", Curfn.Nname.Sym, len(var_), nkill)
		var v *TempVar
		for i := 0; i < len(var_); i++ {
			v = &var_[i]
			fmt.Printf("var %v %v %d-%d", Nconv(v.node, obj.FmtSharp), v.node.Type, v.start, v.end)
			if v.addr != 0 {
				fmt.Printf(" addr=1")
			}
			if v.removed != 0 {
				fmt.Printf(" dead=1")
			}
			if v.merge != nil {
				fmt.Printf(" merge %v", Nconv(v.merge.node, obj.FmtSharp))
			}
			if v.start == v.end && v.def != nil {
				fmt.Printf(" %v", v.def.Prog)
			}
			fmt.Printf("\n")
		}

		if debugmerge > 1 && Debug['v'] != 0 {
			Dumpit("after", g.Start, 0)
		}
	}

	// Update node references to use merged temporaries.
	for f := g.Start; f != nil; f = f.Link {
		p := f.Prog
		n, _ = p.From.Node.(*Node)
		if n != nil {
			v, _ = n.Opt.(*TempVar)
			if v != nil && v.merge != nil {
				p.From.Node = v.merge.node
			}
		}
		n, _ = p.To.Node.(*Node)
		if n != nil {
			v, _ = n.Opt.(*TempVar)
			if v != nil && v.merge != nil {
				p.To.Node = v.merge.node
			}
		}
	}

	// Delete merged nodes from declaration list.
	var l *NodeList
	for lp := &Curfn.Func.Dcl; ; {
		l = *lp
		if l == nil {
			break
		}

		Curfn.Func.Dcl.End = l
		n = l.N
		v, _ = n.Opt.(*TempVar)
		if v != nil && (v.merge != nil || v.removed != 0) {
			*lp = l.Next
			continue
		}

		lp = &l.Next
	}

	// Clear aux structures.
	for i := 0; i < len(var_); i++ {
		var_[i].node.Opt = nil
	}

	Flowend(g)
}

func mergewalk(v *TempVar, f0 *Flow, gen uint32) {
	var p *obj.Prog
	var f1 *Flow

	for f1 = f0; f1 != nil; f1 = f1.P1 {
		if uint32(f1.Active) == gen {
			break
		}
		f1.Active = int32(gen)
		p = f1.Prog
		if v.end < p.Pc {
			v.end = p.Pc
		}
		if f1 == v.def {
			v.start = p.Pc
			break
		}
	}

	var f2 *Flow
	for f := f0; f != f1; f = f.P1 {
		for f2 = f.P2; f2 != nil; f2 = f2.P2link {
			mergewalk(v, f2, gen)
		}
	}
}

func varkillwalk(v *TempVar, f0 *Flow, gen uint32) {
	var p *obj.Prog
	var f1 *Flow

	for f1 = f0; f1 != nil; f1 = f1.S1 {
		if uint32(f1.Active) == gen {
			break
		}
		f1.Active = int32(gen)
		p = f1.Prog
		if v.end < p.Pc {
			v.end = p.Pc
		}
		if v.start > p.Pc {
			v.start = p.Pc
		}
		if p.As == obj.ARET || (p.As == obj.AVARKILL && p.To.Node == v.node) {
			break
		}
	}

	for f := f0; f != f1; f = f.S1 {
		varkillwalk(v, f.S2, gen)
	}
}

// Eliminate redundant nil pointer checks.
//
// The code generation pass emits a CHECKNIL for every possibly nil pointer.
// This pass removes a CHECKNIL if every predecessor path has already
// checked this value for nil.
//
// Simple backwards flood from check to definition.
// Run prog loop backward from end of program to beginning to avoid quadratic
// behavior removing a run of checks.
//
// Assume that stack variables with address not taken can be loaded multiple times
// from memory without being rechecked. Other variables need to be checked on
// each load.

var killed int // f->data is either nil or &killed

func nilopt(firstp *obj.Prog) {
	g := Flowstart(firstp, nil)
	if g == nil {
		return
	}

	if Debug_checknil > 1 { /* || strcmp(curfn->nname->sym->name, "f1") == 0 */
		Dumpit("nilopt", g.Start, 0)
	}

	ncheck := 0
	nkill := 0
	var p *obj.Prog
	for f := g.Start; f != nil; f = f.Link {
		p = f.Prog
		if p.As != obj.ACHECKNIL || !Thearch.Regtyp(&p.From) {
			continue
		}
		ncheck++
		if Thearch.Stackaddr(&p.From) {
			if Debug_checknil != 0 && p.Lineno > 1 {
				Warnl(int(p.Lineno), "removed nil check of SP address")
			}
			f.Data = &killed
			continue
		}

		nilwalkfwd(f)
		if f.Data != nil {
			if Debug_checknil != 0 && p.Lineno > 1 {
				Warnl(int(p.Lineno), "removed nil check before indirect")
			}
			continue
		}

		nilwalkback(f)
		if f.Data != nil {
			if Debug_checknil != 0 && p.Lineno > 1 {
				Warnl(int(p.Lineno), "removed repeated nil check")
			}
			continue
		}
	}

	for f := g.Start; f != nil; f = f.Link {
		if f.Data != nil {
			nkill++
			Thearch.Excise(f)
		}
	}

	Flowend(g)

	if Debug_checknil > 1 {
		fmt.Printf("%v: removed %d of %d nil checks\n", Curfn.Nname.Sym, nkill, ncheck)
	}
}

func nilwalkback(fcheck *Flow) {
	for f := fcheck; f != nil; f = Uniqp(f) {
		p := f.Prog
		if (p.Info.Flags&RightWrite != 0) && Thearch.Sameaddr(&p.To, &fcheck.Prog.From) {
			// Found initialization of value we're checking for nil.
			// without first finding the check, so this one is unchecked.
			return
		}

		if f != fcheck && p.As == obj.ACHECKNIL && Thearch.Sameaddr(&p.From, &fcheck.Prog.From) {
			fcheck.Data = &killed
			return
		}
	}
}

// Here is a more complex version that scans backward across branches.
// It assumes fcheck->kill = 1 has been set on entry, and its job is to find a reason
// to keep the check (setting fcheck->kill = 0).
// It doesn't handle copying of aggregates as well as I would like,
// nor variables with their address taken,
// and it's too subtle to turn on this late in Go 1.2. Perhaps for Go 1.3.
/*
for(f1 = f0; f1 != nil; f1 = f1->p1) {
	if(f1->active == gen)
		break;
	f1->active = gen;
	p = f1->prog;

	// If same check, stop this loop but still check
	// alternate predecessors up to this point.
	if(f1 != fcheck && p->as == ACHECKNIL && thearch.sameaddr(&p->from, &fcheck->prog->from))
		break;

	if((p.Info.flags & RightWrite) && thearch.sameaddr(&p->to, &fcheck->prog->from)) {
		// Found initialization of value we're checking for nil.
		// without first finding the check, so this one is unchecked.
		fcheck->kill = 0;
		return;
	}

	if(f1->p1 == nil && f1->p2 == nil) {
		print("lost pred for %P\n", fcheck->prog);
		for(f1=f0; f1!=nil; f1=f1->p1) {
			thearch.proginfo(&info, f1->prog);
			print("\t%P %d %d %D %D\n", r1->prog, info.flags&RightWrite, thearch.sameaddr(&f1->prog->to, &fcheck->prog->from), &f1->prog->to, &fcheck->prog->from);
		}
		fatal("lost pred trail");
	}
}

for(f = f0; f != f1; f = f->p1)
	for(f2 = f->p2; f2 != nil; f2 = f2->p2link)
		nilwalkback(fcheck, f2, gen);
*/

func nilwalkfwd(fcheck *Flow) {
	// If the path down from rcheck dereferences the address
	// (possibly with a small offset) before writing to memory
	// and before any subsequent checks, it's okay to wait for
	// that implicit check. Only consider this basic block to
	// avoid problems like:
	//	_ = *x // should panic
	//	for {} // no writes but infinite loop may be considered visible

	var last *Flow
	for f := Uniqs(fcheck); f != nil; f = Uniqs(f) {
		p := f.Prog
		if (p.Info.Flags&LeftRead != 0) && Thearch.Smallindir(&p.From, &fcheck.Prog.From) {
			fcheck.Data = &killed
			return
		}

		if (p.Info.Flags&(RightRead|RightWrite) != 0) && Thearch.Smallindir(&p.To, &fcheck.Prog.From) {
			fcheck.Data = &killed
			return
		}

		// Stop if another nil check happens.
		if p.As == obj.ACHECKNIL {
			return
		}

		// Stop if value is lost.
		if (p.Info.Flags&RightWrite != 0) && Thearch.Sameaddr(&p.To, &fcheck.Prog.From) {
			return
		}

		// Stop if memory write.
		if (p.Info.Flags&RightWrite != 0) && !Thearch.Regtyp(&p.To) {
			return
		}

		// Stop if we jump backward.
		if last != nil && f.Id <= last.Id {
			return
		}
		last = f
	}
}
