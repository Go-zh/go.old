// Derived from Inferno utils/6c/reg.c
// http://code.google.com/p/inferno-os/source/browse/utils/6c/reg.c
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

#include <u.h>
#include <libc.h>
#include "gg.h"
#include "opt.h"

#define	NREGVAR	32	/* 16 general + 16 floating */
#define	REGBITS	((uint64)0xffffffffull)
/*c2go enum {
	NREGVAR = 32,
	REGBITS = 0xffffffff,
};
*/

static	Reg*	firstr;
static	int	first	= 1;

int
rcmp(const void *a1, const void *a2)
{
	Rgn *p1, *p2;
	int c1, c2;

	p1 = (Rgn*)a1;
	p2 = (Rgn*)a2;
	c1 = p2->cost;
	c2 = p1->cost;
	if(c1 -= c2)
		return c1;
	return p2->varno - p1->varno;
}

static void
setaddrs(Bits bit)
{
	int i, n;
	Var *v;
	Node *node;

	while(bany(&bit)) {
		// convert each bit to a variable
		i = bnum(bit);
		node = var[i].node;
		n = var[i].name;
		biclr(&bit, i);

		// disable all pieces of that variable
		for(i=0; i<nvar; i++) {
			v = var+i;
			if(v->node == node && v->name == n)
				v->addr = 2;
		}
	}
}

static char* regname[] = {
	".AX",
	".CX",
	".DX",
	".BX",
	".SP",
	".BP",
	".SI",
	".DI",
	".R8",
	".R9",
	".R10",
	".R11",
	".R12",
	".R13",
	".R14",
	".R15",
	".X0",
	".X1",
	".X2",
	".X3",
	".X4",
	".X5",
	".X6",
	".X7",
	".X8",
	".X9",
	".X10",
	".X11",
	".X12",
	".X13",
	".X14",
	".X15",
};

static Node* regnodes[NREGVAR];

static void walkvardef(Node *n, Reg *r, int active);

void
regopt(Prog *firstp)
{
	Reg *r, *r1;
	Prog *p;
	Graph *g;
	ProgInfo info;
	int i, z, active;
	uint32 vreg;
	Bits bit;

	if(first) {
		fmtinstall('Q', Qconv);
		exregoffset = REG_R15;
		first = 0;
	}

	mergetemp(firstp);

	/*
	 * control flow is more complicated in generated go code
	 * than in generated c code.  define pseudo-variables for
	 * registers, so we have complete register usage information.
	 */
	nvar = NREGVAR;
	memset(var, 0, NREGVAR*sizeof var[0]);
	for(i=0; i<NREGVAR; i++) {
		if(regnodes[i] == N)
			regnodes[i] = newname(lookup(regname[i]));
		var[i].node = regnodes[i];
	}

	regbits = RtoB(REG_SP);
	for(z=0; z<BITS; z++) {
		externs.b[z] = 0;
		params.b[z] = 0;
		consts.b[z] = 0;
		addrs.b[z] = 0;
		ivar.b[z] = 0;
		ovar.b[z] = 0;
	}

	/*
	 * pass 1
	 * build aux data structure
	 * allocate pcs
	 * find use and set of variables
	 */
	g = flowstart(firstp, sizeof(Reg));
	if(g == nil) {
		for(i=0; i<nvar; i++)
			var[i].node->opt = nil;
		return;
	}

	firstr = (Reg*)g->start;

	for(r = firstr; r != R; r = (Reg*)r->f.link) {
		p = r->f.prog;
		if(p->as == AVARDEF || p->as == AVARKILL)
			continue;
		proginfo(&info, p);

		// Avoid making variables for direct-called functions.
		if(p->as == ACALL && p->to.type == TYPE_MEM && p->to.name == NAME_EXTERN)
			continue;

		r->use1.b[0] |= info.reguse | info.regindex;
		r->set.b[0] |= info.regset;

		bit = mkvar(r, &p->from);
		if(bany(&bit)) {
			if(info.flags & LeftAddr)
				setaddrs(bit);
			if(info.flags & LeftRead)
				for(z=0; z<BITS; z++)
					r->use1.b[z] |= bit.b[z];
			if(info.flags & LeftWrite)
				for(z=0; z<BITS; z++)
					r->set.b[z] |= bit.b[z];
		}

		bit = mkvar(r, &p->to);
		if(bany(&bit)) {	
			if(info.flags & RightAddr)
				setaddrs(bit);
			if(info.flags & RightRead)
				for(z=0; z<BITS; z++)
					r->use2.b[z] |= bit.b[z];
			if(info.flags & RightWrite)
				for(z=0; z<BITS; z++)
					r->set.b[z] |= bit.b[z];
		}
	}

	for(i=0; i<nvar; i++) {
		Var *v = var+i;
		if(v->addr) {
			bit = blsh(i);
			for(z=0; z<BITS; z++)
				addrs.b[z] |= bit.b[z];
		}

		if(debug['R'] && debug['v'])
			print("bit=%2d addr=%d et=%-6E w=%-2d s=%N + %lld\n",
				i, v->addr, v->etype, v->width, v->node, v->offset);
	}

	if(debug['R'] && debug['v'])
		dumpit("pass1", &firstr->f, 1);

	/*
	 * pass 2
	 * find looping structure
	 */
	flowrpo(g);

	if(debug['R'] && debug['v'])
		dumpit("pass2", &firstr->f, 1);

	/*
	 * pass 2.5
	 * iterate propagating fat vardef covering forward
	 * r->act records vars with a VARDEF since the last CALL.
	 * (r->act will be reused in pass 5 for something else,
	 * but we'll be done with it by then.)
	 */
	active = 0;
	for(r = firstr; r != R; r = (Reg*)r->f.link) {
		r->f.active = 0;
		r->act = zbits;
	}
	for(r = firstr; r != R; r = (Reg*)r->f.link) {
		p = r->f.prog;
		if(p->as == AVARDEF && isfat(((Node*)(p->to.node))->type) && ((Node*)(p->to.node))->opt != nil) {
			active++;
			walkvardef(p->to.node, r, active);
		}
	}

	/*
	 * pass 3
	 * iterate propagating usage
	 * 	back until flow graph is complete
	 */
loop1:
	change = 0;
	for(r = firstr; r != R; r = (Reg*)r->f.link)
		r->f.active = 0;
	for(r = firstr; r != R; r = (Reg*)r->f.link)
		if(r->f.prog->as == ARET)
			prop(r, zbits, zbits);
loop11:
	/* pick up unreachable code */
	i = 0;
	for(r = firstr; r != R; r = r1) {
		r1 = (Reg*)r->f.link;
		if(r1 && r1->f.active && !r->f.active) {
			prop(r, zbits, zbits);
			i = 1;
		}
	}
	if(i)
		goto loop11;
	if(change)
		goto loop1;

	if(debug['R'] && debug['v'])
		dumpit("pass3", &firstr->f, 1);

	/*
	 * pass 4
	 * iterate propagating register/variable synchrony
	 * 	forward until graph is complete
	 */
loop2:
	change = 0;
	for(r = firstr; r != R; r = (Reg*)r->f.link)
		r->f.active = 0;
	synch(firstr, zbits);
	if(change)
		goto loop2;

	if(debug['R'] && debug['v'])
		dumpit("pass4", &firstr->f, 1);

	/*
	 * pass 4.5
	 * move register pseudo-variables into regu.
	 */
	for(r = firstr; r != R; r = (Reg*)r->f.link) {
		r->regu = (r->refbehind.b[0] | r->set.b[0]) & REGBITS;

		r->set.b[0] &= ~REGBITS;
		r->use1.b[0] &= ~REGBITS;
		r->use2.b[0] &= ~REGBITS;
		r->refbehind.b[0] &= ~REGBITS;
		r->refahead.b[0] &= ~REGBITS;
		r->calbehind.b[0] &= ~REGBITS;
		r->calahead.b[0] &= ~REGBITS;
		r->regdiff.b[0] &= ~REGBITS;
		r->act.b[0] &= ~REGBITS;
	}

	/*
	 * pass 5
	 * isolate regions
	 * calculate costs (paint1)
	 */
	r = firstr;
	if(r) {
		for(z=0; z<BITS; z++)
			bit.b[z] = (r->refahead.b[z] | r->calahead.b[z]) &
			  ~(externs.b[z] | params.b[z] | addrs.b[z] | consts.b[z]);
		if(bany(&bit) && !r->f.refset) {
			// should never happen - all variables are preset
			if(debug['w'])
				print("%L: used and not set: %Q\n", r->f.prog->lineno, bit);
			r->f.refset = 1;
		}
	}
	for(r = firstr; r != R; r = (Reg*)r->f.link)
		r->act = zbits;
	rgp = region;
	nregion = 0;
	for(r = firstr; r != R; r = (Reg*)r->f.link) {
		for(z=0; z<BITS; z++)
			bit.b[z] = r->set.b[z] &
			  ~(r->refahead.b[z] | r->calahead.b[z] | addrs.b[z]);
		if(bany(&bit) && !r->f.refset) {
			if(debug['w'])
				print("%L: set and not used: %Q\n", r->f.prog->lineno, bit);
			r->f.refset = 1;
			excise(&r->f);
		}
		for(z=0; z<BITS; z++)
			bit.b[z] = LOAD(r) & ~(r->act.b[z] | addrs.b[z]);
		while(bany(&bit)) {
			i = bnum(bit);
			rgp->enter = r;
			rgp->varno = i;
			change = 0;
			paint1(r, i);
			biclr(&bit, i);
			if(change <= 0)
				continue;
			rgp->cost = change;
			nregion++;
			if(nregion >= NRGN) {
				if(debug['R'] && debug['v'])
					print("too many regions\n");
				goto brk;
			}
			rgp++;
		}
	}
brk:
	qsort(region, nregion, sizeof(region[0]), rcmp);

	if(debug['R'] && debug['v'])
		dumpit("pass5", &firstr->f, 1);

	/*
	 * pass 6
	 * determine used registers (paint2)
	 * replace code (paint3)
	 */
	rgp = region;
	if(debug['R'] && debug['v'])
		print("\nregisterizing\n");
	for(i=0; i<nregion; i++) {
		if(debug['R'] && debug['v'])
			print("region %d: cost %d varno %d enter %lld\n", i, rgp->cost, rgp->varno, rgp->enter->f.prog->pc);
		bit = blsh(rgp->varno);
		vreg = paint2(rgp->enter, rgp->varno, 0);
		vreg = allreg(vreg, rgp);
		if(rgp->regno != 0) {
			if(debug['R'] && debug['v']) {
				Var *v;

				v = var + rgp->varno;
				print("registerize %N+%lld (bit=%2d et=%2E) in %R\n",
						v->node, v->offset, rgp->varno, v->etype, rgp->regno);
			}
			paint3(rgp->enter, rgp->varno, vreg, rgp->regno);
		}
		rgp++;
	}

	/*
	 * free aux structures. peep allocates new ones.
	 */
	for(i=0; i<nvar; i++)
		var[i].node->opt = nil;
	flowend(g);
	firstr = R;

	if(debug['R'] && debug['v']) {
		// Rebuild flow graph, since we inserted instructions
		g = flowstart(firstp, sizeof(Reg));
		firstr = (Reg*)g->start;
		dumpit("pass6", &firstr->f, 1);
		flowend(g);
		firstr = R;
	}

	/*
	 * pass 7
	 * peep-hole on basic block
	 */
	if(!debug['R'] || debug['P'])
		peep(firstp);

	/*
	 * eliminate nops
	 */
	for(p=firstp; p!=P; p=p->link) {
		while(p->link != P && p->link->as == ANOP)
			p->link = p->link->link;
		if(p->to.type == TYPE_BRANCH)
			while(p->to.u.branch != P && p->to.u.branch->as == ANOP)
				p->to.u.branch = p->to.u.branch->link;
	}

	if(debug['R']) {
		if(ostats.ncvtreg ||
		   ostats.nspill ||
		   ostats.nreload ||
		   ostats.ndelmov ||
		   ostats.nvar ||
		   ostats.naddr ||
		   0)
			print("\nstats\n");

		if(ostats.ncvtreg)
			print("	%4d cvtreg\n", ostats.ncvtreg);
		if(ostats.nspill)
			print("	%4d spill\n", ostats.nspill);
		if(ostats.nreload)
			print("	%4d reload\n", ostats.nreload);
		if(ostats.ndelmov)
			print("	%4d delmov\n", ostats.ndelmov);
		if(ostats.nvar)
			print("	%4d var\n", ostats.nvar);
		if(ostats.naddr)
			print("	%4d addr\n", ostats.naddr);

		memset(&ostats, 0, sizeof(ostats));
	}
}

static void
walkvardef(Node *n, Reg *r, int active)
{
	Reg *r1, *r2;
	int bn;
	Var *v;
	
	for(r1=r; r1!=R; r1=(Reg*)r1->f.s1) {
		if(r1->f.active == active)
			break;
		r1->f.active = active;
		if(r1->f.prog->as == AVARKILL && r1->f.prog->to.node == n)
			break;
		for(v=n->opt; v!=nil; v=v->nextinnode) {
			bn = v - var;
			biset(&r1->act, bn);
		}
		if(r1->f.prog->as == ACALL)
			break;
	}

	for(r2=r; r2!=r1; r2=(Reg*)r2->f.s1)
		if(r2->f.s2 != nil)
			walkvardef(n, (Reg*)r2->f.s2, active);
}

/*
 * add mov b,rn
 * just after r
 */
void
addmove(Reg *r, int bn, int rn, int f)
{
	Prog *p, *p1;
	Adr *a;
	Var *v;

	p1 = mal(sizeof(*p1));
	clearp(p1);
	p1->pc = 9999;

	p = r->f.prog;
	p1->link = p->link;
	p->link = p1;
	p1->lineno = p->lineno;

	v = var + bn;

	a = &p1->to;
	a->offset = v->offset;
	a->etype = v->etype;
	a->type = TYPE_MEM;
	a->name = v->name;
	a->node = v->node;
	a->sym = linksym(v->node->sym);

	// need to clean this up with wptr and
	// some of the defaults
	p1->as = AMOVL;
	switch(simtype[(uchar)v->etype]) {
	default:
		fatal("unknown type %E", v->etype);
	case TINT8:
	case TUINT8:
	case TBOOL:
		p1->as = AMOVB;
		break;
	case TINT16:
	case TUINT16:
		p1->as = AMOVW;
		break;
	case TINT64:
	case TUINT64:
	case TPTR64:
		p1->as = AMOVQ;
		break;
	case TFLOAT32:
		p1->as = AMOVSS;
		break;
	case TFLOAT64:
		p1->as = AMOVSD;
		break;
	case TINT32:
	case TUINT32:
	case TPTR32:
		break;
	}

	p1->from.type = TYPE_REG;
	p1->from.reg = rn;
	p1->from.name = NAME_NONE;
	if(!f) {
		p1->from = *a;
		*a = zprog.from;
		a->type = TYPE_REG;
		a->reg = rn;
		if(v->etype == TUINT8)
			p1->as = AMOVB;
		if(v->etype == TUINT16)
			p1->as = AMOVW;
	}
	if(debug['R'] && debug['v'])
		print("%P ===add=== %P\n", p, p1);
	ostats.nspill++;
}

uint32
doregbits(int r)
{
	uint32 b;

	b = 0;
	if(r >= REG_AX && r <= REG_R15)
		b |= RtoB(r);
	else
	if(r >= REG_AL && r <= REG_R15B)
		b |= RtoB(r-REG_AL+REG_AX);
	else
	if(r >= REG_AH && r <= REG_BH)
		b |= RtoB(r-REG_AH+REG_AX);
	else
	if(r >= REG_X0 && r <= REG_X0+15)
		b |= FtoB(r);
	return b;
}

static int
overlap(int64 o1, int w1, int64 o2, int w2)
{
	int64 t1, t2;

	t1 = o1+w1;
	t2 = o2+w2;

	if(!(t1 > o2 && t2 > o1))
		return 0;

	return 1;
}

Bits
mkvar(Reg *r, Adr *a)
{
	Var *v;
	int i, n, et, z, flag;
	int64 w;
	uint32 regu;
	int64 o;
	Bits bit;
	Node *node;

	/*
	 * mark registers used
	 */
	if(a->type == TYPE_NONE)
		goto none;

	if(r != R)
		r->use1.b[0] |= doregbits(a->index);

	switch(a->type) {
	default:
		regu = doregbits(a->reg);
		if(regu == 0)
			goto none;
		bit = zbits;
		bit.b[0] = regu;
		return bit;

	case TYPE_ADDR:
		a->type = TYPE_MEM;
		bit = mkvar(r, a);
		setaddrs(bit);
		a->type = TYPE_ADDR;
		ostats.naddr++;
		goto none;

	case TYPE_MEM:
		switch(a->name) {
		default:
			goto none;
		case NAME_EXTERN:
		case NAME_STATIC:
		case NAME_PARAM:
		case NAME_AUTO:
			n = a->name;
			break;
		}
	}

	node = a->node;
	if(node == N || node->op != ONAME || node->orig == N)
		goto none;
	node = node->orig;
	if(node->orig != node)
		fatal("%D: bad node", a);
	if(node->sym == S || node->sym->name[0] == '.')
		goto none;
	et = a->etype;
	o = a->offset;
	w = a->width;
	if(w < 0)
		fatal("bad width %lld for %D", w, a);

	flag = 0;
	for(i=0; i<nvar; i++) {
		v = var+i;
		if(v->node == node && v->name == n) {
			if(v->offset == o)
			if(v->etype == et)
			if(v->width == w)
				return blsh(i);

			// if they overlaps, disable both
			if(overlap(v->offset, v->width, o, w)) {
//				print("disable overlap %s %d %d %d %d, %E != %E\n", s->name, v->offset, v->width, o, w, v->etype, et);
				v->addr = 1;
				flag = 1;
			}
		}
	}
	switch(et) {
	case 0:
	case TFUNC:
		goto none;
	}

	if(nvar >= NVAR) {
		if(debug['w'] > 1 && node != N)
			fatal("variable not optimized: %#N", node);
		
		// If we're not tracking a word in a variable, mark the rest as
		// having its address taken, so that we keep the whole thing
		// live at all calls. otherwise we might optimize away part of
		// a variable but not all of it.
		for(i=0; i<nvar; i++) {
			v = var+i;
			if(v->node == node)
				v->addr = 1;
		}
		goto none;
	}

	i = nvar;
	nvar++;
	v = var+i;
	v->offset = o;
	v->name = n;
	v->etype = et;
	v->width = w;
	v->addr = flag;		// funny punning
	v->node = node;
	
	// node->opt is the head of a linked list
	// of Vars within the given Node, so that
	// we can start at a Var and find all the other
	// Vars in the same Go variable.
	v->nextinnode = node->opt;
	node->opt = v;

	bit = blsh(i);
	if(n == NAME_EXTERN || n == NAME_STATIC)
		for(z=0; z<BITS; z++)
			externs.b[z] |= bit.b[z];
	if(n == NAME_PARAM)
		for(z=0; z<BITS; z++)
			params.b[z] |= bit.b[z];

	if(node->class == PPARAM)
		for(z=0; z<BITS; z++)
			ivar.b[z] |= bit.b[z];
	if(node->class == PPARAMOUT)
		for(z=0; z<BITS; z++)
			ovar.b[z] |= bit.b[z];

	// Treat values with their address taken as live at calls,
	// because the garbage collector's liveness analysis in ../gc/plive.c does.
	// These must be consistent or else we will elide stores and the garbage
	// collector will see uninitialized data.
	// The typical case where our own analysis is out of sync is when the
	// node appears to have its address taken but that code doesn't actually
	// get generated and therefore doesn't show up as an address being
	// taken when we analyze the instruction stream.
	// One instance of this case is when a closure uses the same name as
	// an outer variable for one of its own variables declared with :=.
	// The parser flags the outer variable as possibly shared, and therefore
	// sets addrtaken, even though it ends up not being actually shared.
	// If we were better about _ elision, _ = &x would suffice too.
	// The broader := in a closure problem is mentioned in a comment in
	// closure.c:/^typecheckclosure and dcl.c:/^oldname.
	if(node->addrtaken)
		v->addr = 1;

	// Disable registerization for globals, because:
	// (1) we might panic at any time and we want the recovery code
	// to see the latest values (issue 1304).
	// (2) we don't know what pointers might point at them and we want
	// loads via those pointers to see updated values and vice versa (issue 7995).
	//
	// Disable registerization for results if using defer, because the deferred func
	// might recover and return, causing the current values to be used.
	if(node->class == PEXTERN || (hasdefer && node->class == PPARAMOUT))
		v->addr = 1;

	if(debug['R'])
		print("bit=%2d et=%2E w=%lld+%lld %#N %D flag=%d\n", i, et, o, w, node, a, v->addr);
	ostats.nvar++;

	return bit;

none:
	return zbits;
}

void
prop(Reg *r, Bits ref, Bits cal)
{
	Reg *r1, *r2;
	int z, i, j;
	Var *v, *v1;

	for(r1 = r; r1 != R; r1 = (Reg*)r1->f.p1) {
		for(z=0; z<BITS; z++) {
			ref.b[z] |= r1->refahead.b[z];
			if(ref.b[z] != r1->refahead.b[z]) {
				r1->refahead.b[z] = ref.b[z];
				change++;
			}
			cal.b[z] |= r1->calahead.b[z];
			if(cal.b[z] != r1->calahead.b[z]) {
				r1->calahead.b[z] = cal.b[z];
				change++;
			}
		}
		switch(r1->f.prog->as) {
		case ACALL:
			if(noreturn(r1->f.prog))
				break;

			// Mark all input variables (ivar) as used, because that's what the
			// liveness bitmaps say. The liveness bitmaps say that so that a
			// panic will not show stale values in the parameter dump.
			// Mark variables with a recent VARDEF (r1->act) as used,
			// so that the optimizer flushes initializations to memory,
			// so that if a garbage collection happens during this CALL,
			// the collector will see initialized memory. Again this is to
			// match what the liveness bitmaps say.
			for(z=0; z<BITS; z++) {
				cal.b[z] |= ref.b[z] | externs.b[z] | ivar.b[z] | r1->act.b[z];
				ref.b[z] = 0;
			}
			
			// cal.b is the current approximation of what's live across the call.
			// Every bit in cal.b is a single stack word. For each such word,
			// find all the other tracked stack words in the same Go variable
			// (struct/slice/string/interface) and mark them live too.
			// This is necessary because the liveness analysis for the garbage
			// collector works at variable granularity, not at word granularity.
			// It is fundamental for slice/string/interface: the garbage collector
			// needs the whole value, not just some of the words, in order to
			// interpret the other bits correctly. Specifically, slice needs a consistent
			// ptr and cap, string needs a consistent ptr and len, and interface
			// needs a consistent type word and data word.
			for(z=0; z<BITS; z++) {
				if(cal.b[z] == 0)
					continue;
				for(i=0; i<64; i++) {
					if(z*64+i >= nvar || ((cal.b[z]>>i)&1) == 0)
						continue;
					v = var+z*64+i;
					if(v->node->opt == nil) // v represents fixed register, not Go variable
						continue;

					// v->node->opt is the head of a linked list of Vars
					// corresponding to tracked words from the Go variable v->node.
					// Walk the list and set all the bits.
					// For a large struct this could end up being quadratic:
					// after the first setting, the outer loop (for z, i) would see a 1 bit
					// for all of the remaining words in the struct, and for each such
					// word would go through and turn on all the bits again.
					// To avoid the quadratic behavior, we only turn on the bits if
					// v is the head of the list or if the head's bit is not yet turned on.
					// This will set the bits at most twice, keeping the overall loop linear.
					v1 = v->node->opt;
					j = v1 - var;
					if(v == v1 || !btest(&cal, j)) {
						for(; v1 != nil; v1 = v1->nextinnode) {
							j = v1 - var;
							biset(&cal, j);
						}
					}
				}
			}
			break;

		case ATEXT:
			for(z=0; z<BITS; z++) {
				cal.b[z] = 0;
				ref.b[z] = 0;
			}
			break;

		case ARET:
			for(z=0; z<BITS; z++) {
				cal.b[z] = externs.b[z] | ovar.b[z];
				ref.b[z] = 0;
			}
			break;
		}
		for(z=0; z<BITS; z++) {
			ref.b[z] = (ref.b[z] & ~r1->set.b[z]) |
				r1->use1.b[z] | r1->use2.b[z];
			cal.b[z] &= ~(r1->set.b[z] | r1->use1.b[z] | r1->use2.b[z]);
			r1->refbehind.b[z] = ref.b[z];
			r1->calbehind.b[z] = cal.b[z];
		}
		if(r1->f.active)
			break;
		r1->f.active = 1;
	}
	for(; r != r1; r = (Reg*)r->f.p1)
		for(r2 = (Reg*)r->f.p2; r2 != R; r2 = (Reg*)r2->f.p2link)
			prop(r2, r->refbehind, r->calbehind);
}

void
synch(Reg *r, Bits dif)
{
	Reg *r1;
	int z;

	for(r1 = r; r1 != R; r1 = (Reg*)r1->f.s1) {
		for(z=0; z<BITS; z++) {
			dif.b[z] = (dif.b[z] &
				~(~r1->refbehind.b[z] & r1->refahead.b[z])) |
					r1->set.b[z] | r1->regdiff.b[z];
			if(dif.b[z] != r1->regdiff.b[z]) {
				r1->regdiff.b[z] = dif.b[z];
				change++;
			}
		}
		if(r1->f.active)
			break;
		r1->f.active = 1;
		for(z=0; z<BITS; z++)
			dif.b[z] &= ~(~r1->calbehind.b[z] & r1->calahead.b[z]);
		if(r1->f.s2 != nil)
			synch((Reg*)r1->f.s2, dif);
	}
}

uint32
allreg(uint32 b, Rgn *r)
{
	Var *v;
	int i;

	v = var + r->varno;
	r->regno = 0;
	switch(v->etype) {

	default:
		fatal("unknown etype %d/%E", bitno(b), v->etype);
		break;

	case TINT8:
	case TUINT8:
	case TINT16:
	case TUINT16:
	case TINT32:
	case TUINT32:
	case TINT64:
	case TUINT64:
	case TINT:
	case TUINT:
	case TUINTPTR:
	case TBOOL:
	case TPTR32:
	case TPTR64:
		i = BtoR(~b);
		if(i && r->cost > 0) {
			r->regno = i;
			return RtoB(i);
		}
		break;

	case TFLOAT32:
	case TFLOAT64:
		i = BtoF(~b);
		if(i && r->cost > 0) {
			r->regno = i;
			return FtoB(i);
		}
		break;
	}
	return 0;
}

void
paint1(Reg *r, int bn)
{
	Reg *r1;
	int z;
	uint64 bb;

	z = bn/64;
	bb = 1LL<<(bn%64);
	if(r->act.b[z] & bb)
		return;
	for(;;) {
		if(!(r->refbehind.b[z] & bb))
			break;
		r1 = (Reg*)r->f.p1;
		if(r1 == R)
			break;
		if(!(r1->refahead.b[z] & bb))
			break;
		if(r1->act.b[z] & bb)
			break;
		r = r1;
	}

	if(LOAD(r) & ~(r->set.b[z]&~(r->use1.b[z]|r->use2.b[z])) & bb) {
		change -= CLOAD * r->f.loop;
	}
	for(;;) {
		r->act.b[z] |= bb;

		if(r->f.prog->as != ANOP) { // don't give credit for NOPs
			if(r->use1.b[z] & bb)
				change += CREF * r->f.loop;
			if((r->use2.b[z]|r->set.b[z]) & bb)
				change += CREF * r->f.loop;
		}

		if(STORE(r) & r->regdiff.b[z] & bb) {
			change -= CLOAD * r->f.loop;
		}

		if(r->refbehind.b[z] & bb)
			for(r1 = (Reg*)r->f.p2; r1 != R; r1 = (Reg*)r1->f.p2link)
				if(r1->refahead.b[z] & bb)
					paint1(r1, bn);

		if(!(r->refahead.b[z] & bb))
			break;
		r1 = (Reg*)r->f.s2;
		if(r1 != R)
			if(r1->refbehind.b[z] & bb)
				paint1(r1, bn);
		r = (Reg*)r->f.s1;
		if(r == R)
			break;
		if(r->act.b[z] & bb)
			break;
		if(!(r->refbehind.b[z] & bb))
			break;
	}
}

uint32
paint2(Reg *r, int bn, int depth)
{
	Reg *r1;
	int z;
	uint64 bb, vreg;

	z = bn/64;
	bb = 1LL << (bn%64);
	vreg = regbits;
	if(!(r->act.b[z] & bb))
		return vreg;
	for(;;) {
		if(!(r->refbehind.b[z] & bb))
			break;
		r1 = (Reg*)r->f.p1;
		if(r1 == R)
			break;
		if(!(r1->refahead.b[z] & bb))
			break;
		if(!(r1->act.b[z] & bb))
			break;
		r = r1;
	}
	for(;;) {
		if(debug['R'] && debug['v'])
			print("  paint2 %d %P\n", depth, r->f.prog);

		r->act.b[z] &= ~bb;

		vreg |= r->regu;

		if(r->refbehind.b[z] & bb)
			for(r1 = (Reg*)r->f.p2; r1 != R; r1 = (Reg*)r1->f.p2link)
				if(r1->refahead.b[z] & bb)
					vreg |= paint2(r1, bn, depth+1);

		if(!(r->refahead.b[z] & bb))
			break;
		r1 = (Reg*)r->f.s2;
		if(r1 != R)
			if(r1->refbehind.b[z] & bb)
				vreg |= paint2(r1, bn, depth+1);
		r = (Reg*)r->f.s1;
		if(r == R)
			break;
		if(!(r->act.b[z] & bb))
			break;
		if(!(r->refbehind.b[z] & bb))
			break;
	}

	return vreg;
}

void
paint3(Reg *r, int bn, uint32 rb, int rn)
{
	Reg *r1;
	Prog *p;
	int z;
	uint64 bb;

	z = bn/64;
	bb = 1LL << (bn%64);
	if(r->act.b[z] & bb)
		return;
	for(;;) {
		if(!(r->refbehind.b[z] & bb))
			break;
		r1 = (Reg*)r->f.p1;
		if(r1 == R)
			break;
		if(!(r1->refahead.b[z] & bb))
			break;
		if(r1->act.b[z] & bb)
			break;
		r = r1;
	}

	if(LOAD(r) & ~(r->set.b[z] & ~(r->use1.b[z]|r->use2.b[z])) & bb)
		addmove(r, bn, rn, 0);
	for(;;) {
		r->act.b[z] |= bb;
		p = r->f.prog;

		if(r->use1.b[z] & bb) {
			if(debug['R'] && debug['v'])
				print("%P", p);
			addreg(&p->from, rn);
			if(debug['R'] && debug['v'])
				print(" ===change== %P\n", p);
		}
		if((r->use2.b[z]|r->set.b[z]) & bb) {
			if(debug['R'] && debug['v'])
				print("%P", p);
			addreg(&p->to, rn);
			if(debug['R'] && debug['v'])
				print(" ===change== %P\n", p);
		}

		if(STORE(r) & r->regdiff.b[z] & bb)
			addmove(r, bn, rn, 1);
		r->regu |= rb;

		if(r->refbehind.b[z] & bb)
			for(r1 = (Reg*)r->f.p2; r1 != R; r1 = (Reg*)r1->f.p2link)
				if(r1->refahead.b[z] & bb)
					paint3(r1, bn, rb, rn);

		if(!(r->refahead.b[z] & bb))
			break;
		r1 = (Reg*)r->f.s2;
		if(r1 != R)
			if(r1->refbehind.b[z] & bb)
				paint3(r1, bn, rb, rn);
		r = (Reg*)r->f.s1;
		if(r == R)
			break;
		if(r->act.b[z] & bb)
			break;
		if(!(r->refbehind.b[z] & bb))
			break;
	}
}

void
addreg(Adr *a, int rn)
{
	a->sym = nil;
	a->node = nil;
	a->offset = 0;
	a->type = TYPE_REG;
	a->reg = rn;
	a->name = 0;

	ostats.ncvtreg++;
}

uint32
RtoB(int r)
{

	if(r < REG_AX || r > REG_R15)
		return 0;
	return 1L << (r-REG_AX);
}

int
BtoR(uint32 b)
{
	b &= 0xffffL;
	if(nacl)
		b &= ~((1<<(REG_BP-REG_AX)) | (1<<(REG_R15-REG_AX)));
	else if(framepointer_enabled)
		// BP is part of the calling convention if framepointer_enabled.
		b &= ~(1<<(REG_BP-REG_AX));
	if(b == 0)
		return 0;
	return bitno(b) + REG_AX;
}

/*
 *	bit	reg
 *	16	X0
 *	...
 *	31	X15
 */
uint32
FtoB(int f)
{
	if(f < REG_X0 || f > REG_X15)
		return 0;
	return 1L << (f - REG_X0 + 16);
}

int
BtoF(uint32 b)
{

	b &= 0xFFFF0000L;
	if(b == 0)
		return 0;
	return bitno(b) - 16 + REG_X0;
}

void
dumpone(Flow *f, int isreg)
{
	int z;
	Bits bit;
	Reg *r;

	print("%d:%P", f->loop, f->prog);
	if(isreg) {	
		r = (Reg*)f;
		for(z=0; z<BITS; z++)
			bit.b[z] =
				r->set.b[z] |
				r->use1.b[z] |
				r->use2.b[z] |
				r->refbehind.b[z] |
				r->refahead.b[z] |
				r->calbehind.b[z] |
				r->calahead.b[z] |
				r->regdiff.b[z] |
				r->act.b[z] |
					0;
		if(bany(&bit)) {
			print("\t");
			if(bany(&r->set))
				print(" s:%Q", r->set);
			if(bany(&r->use1))
				print(" u1:%Q", r->use1);
			if(bany(&r->use2))
				print(" u2:%Q", r->use2);
			if(bany(&r->refbehind))
				print(" rb:%Q ", r->refbehind);
			if(bany(&r->refahead))
				print(" ra:%Q ", r->refahead);
			if(bany(&r->calbehind))
				print(" cb:%Q ", r->calbehind);
			if(bany(&r->calahead))
				print(" ca:%Q ", r->calahead);
			if(bany(&r->regdiff))
				print(" d:%Q ", r->regdiff);
			if(bany(&r->act))
				print(" a:%Q ", r->act);
		}
	}
	print("\n");
}

void
dumpit(char *str, Flow *r0, int isreg)
{
	Flow *r, *r1;

	print("\n%s\n", str);
	for(r = r0; r != nil; r = r->link) {
		dumpone(r, isreg);
		r1 = r->p2;
		if(r1 != nil) {
			print("	pred:");
			for(; r1 != nil; r1 = r1->p2link)
				print(" %.4ud", (int)r1->prog->pc);
			print("\n");
		}
		// Print successors if it's not just the next one
		if(r->s1 != r->link || r->s2 != nil) {
			print("	succ:");
			if(r->s1 != nil)
				print(" %.4ud", (int)r->s1->prog->pc);
			if(r->s2 != nil)
				print(" %.4ud", (int)r->s2->prog->pc);
			print("\n");
		}
	}
}
