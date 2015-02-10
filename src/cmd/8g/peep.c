// Derived from Inferno utils/6c/peep.c
// http://code.google.com/p/inferno-os/source/browse/utils/6c/peep.c
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

enum {
	REGEXT = 0,
};

static void	conprop(Flow *r);
static void	elimshortmov(Graph*);
static int	subprop(Flow*);
static int	copyprop(Graph*, Flow*);
static int	copy1(Adr*, Adr*, Flow*, int);
static int	copyas(Adr*, Adr*);
static int	copyau(Adr*, Adr*);
static int	copysub(Adr*, Adr*, Adr*, int);

static uint32	gactive;

// do we need the carry bit
static int
needc(Prog *p)
{
	ProgInfo info;

	while(p != P) {
		proginfo(&info, p);
		if(info.flags & UseCarry)
			return 1;
		if(info.flags & (SetCarry|KillCarry))
			return 0;
		p = p->link;
	}
	return 0;
}

static Flow*
rnops(Flow *r)
{
	Prog *p;
	Flow *r1;

	if(r != nil)
	for(;;) {
		p = r->prog;
		if(p->as != ANOP || p->from.type != TYPE_NONE || p->to.type != TYPE_NONE)
			break;
		r1 = uniqs(r);
		if(r1 == nil)
			break;
		r = r1;
	}
	return r;
}

void
peep(Prog *firstp)
{
	Flow *r, *r1;
	Graph *g;
	Prog *p, *p1;
	int t;

	g = flowstart(firstp, sizeof(Flow));
	if(g == nil)
		return;
	gactive = 0;

	// byte, word arithmetic elimination.
	elimshortmov(g);

	// constant propagation
	// find MOV $con,R followed by
	// another MOV $con,R without
	// setting R in the interim
	for(r=g->start; r!=nil; r=r->link) {
		p = r->prog;
		switch(p->as) {
		case ALEAL:
			if(regtyp(&p->to))
			if(p->from.sym != nil)
			if(p->from.index == REG_NONE)
				conprop(r);
			break;

		case AMOVB:
		case AMOVW:
		case AMOVL:
		case AMOVSS:
		case AMOVSD:
			if(regtyp(&p->to))
			if(p->from.type == TYPE_CONST || p->from.type == TYPE_FCONST)
				conprop(r);
			break;
		}
	}

loop1:
	if(debug['P'] && debug['v'])
		dumpit("loop1", g->start, 0);

	t = 0;
	for(r=g->start; r!=nil; r=r->link) {
		p = r->prog;
		switch(p->as) {
		case AMOVL:
		case AMOVSS:
		case AMOVSD:
			if(regtyp(&p->to))
			if(regtyp(&p->from)) {
				if(copyprop(g, r)) {
					excise(r);
					t++;
				} else
				if(subprop(r) && copyprop(g, r)) {
					excise(r);
					t++;
				}
			}
			break;

		case AMOVBLZX:
		case AMOVWLZX:
		case AMOVBLSX:
		case AMOVWLSX:
			if(regtyp(&p->to)) {
				r1 = rnops(uniqs(r));
				if(r1 != nil) {
					p1 = r1->prog;
					if(p->as == p1->as && p->to.type == p1->from.type && p->to.reg == p1->from.reg){
						p1->as = AMOVL;
						t++;
					}
				}
			}
			break;

		case AADDL:
		case AADDW:
			if(p->from.type != TYPE_CONST || needc(p->link))
				break;
			if(p->from.offset == -1){
				if(p->as == AADDL)
					p->as = ADECL;
				else
					p->as = ADECW;
				p->from = zprog.from;
				break;
			}
			if(p->from.offset == 1){
				if(p->as == AADDL)
					p->as = AINCL;
				else
					p->as = AINCW;
				p->from = zprog.from;
				break;
			}
			break;

		case ASUBL:
		case ASUBW:
			if(p->from.type != TYPE_CONST || needc(p->link))
				break;
			if(p->from.offset == -1) {
				if(p->as == ASUBL)
					p->as = AINCL;
				else
					p->as = AINCW;
				p->from = zprog.from;
				break;
			}
			if(p->from.offset == 1){
				if(p->as == ASUBL)
					p->as = ADECL;
				else
					p->as = ADECW;
				p->from = zprog.from;
				break;
			}
			break;
		}
	}
	if(t)
		goto loop1;

	// MOVSD removal.
	// We never use packed registers, so a MOVSD between registers
	// can be replaced by MOVAPD, which moves the pair of float64s
	// instead of just the lower one.  We only use the lower one, but
	// the processor can do better if we do moves using both.
	for(r=g->start; r!=nil; r=r->link) {
		p = r->prog;
		if(p->as == AMOVSD)
		if(regtyp(&p->from))
		if(regtyp(&p->to))
			p->as = AMOVAPD;
	}
	
	flowend(g);
}

void
excise(Flow *r)
{
	Prog *p;

	p = r->prog;
	if(debug['P'] && debug['v'])
		print("%P ===delete===\n", p);

	nopout(p);

	ostats.ndelmov++;
}

int
regtyp(Adr *a)
{
	return a->type == TYPE_REG && (REG_AX <= a->reg && a->reg <= REG_DI || REG_X0 <= a->reg && a->reg <= REG_X7);
}

// movb elimination.
// movb is simulated by the linker
// when a register other than ax, bx, cx, dx
// is used, so rewrite to other instructions
// when possible.  a movb into a register
// can smash the entire 64-bit register without
// causing any trouble.
static void
elimshortmov(Graph *g)
{
	Prog *p;
	Flow *r;

	for(r=g->start; r!=nil; r=r->link) {
		p = r->prog;
		if(regtyp(&p->to)) {
			switch(p->as) {
			case AINCB:
			case AINCW:
				p->as = AINCL;
				break;
			case ADECB:
			case ADECW:
				p->as = ADECL;
				break;
			case ANEGB:
			case ANEGW:
				p->as = ANEGL;
				break;
			case ANOTB:
			case ANOTW:
				p->as = ANOTL;
				break;
			}
			if(regtyp(&p->from) || p->from.type == TYPE_CONST) {
				// move or artihmetic into partial register.
				// from another register or constant can be movl.
				// we don't switch to 32-bit arithmetic if it can
				// change how the carry bit is set (and the carry bit is needed).
				switch(p->as) {
				case AMOVB:
				case AMOVW:
					p->as = AMOVL;
					break;
				case AADDB:
				case AADDW:
					if(!needc(p->link))
						p->as = AADDL;
					break;
				case ASUBB:
				case ASUBW:
					if(!needc(p->link))
						p->as = ASUBL;
					break;
				case AMULB:
				case AMULW:
					p->as = AMULL;
					break;
				case AIMULB:
				case AIMULW:
					p->as = AIMULL;
					break;
				case AANDB:
				case AANDW:
					p->as = AANDL;
					break;
				case AORB:
				case AORW:
					p->as = AORL;
					break;
				case AXORB:
				case AXORW:
					p->as = AXORL;
					break;
				case ASHLB:
				case ASHLW:
					p->as = ASHLL;
					break;
				}
			} else {
				// explicit zero extension
				switch(p->as) {
				case AMOVB:
					p->as = AMOVBLZX;
					break;
				case AMOVW:
					p->as = AMOVWLZX;
					break;
				}
			}
		}
	}
}

/*
 * the idea is to substitute
 * one register for another
 * from one MOV to another
 *	MOV	a, R0
 *	ADD	b, R0	/ no use of R1
 *	MOV	R0, R1
 * would be converted to
 *	MOV	a, R1
 *	ADD	b, R1
 *	MOV	R1, R0
 * hopefully, then the former or latter MOV
 * will be eliminated by copy propagation.
 */
static int
subprop(Flow *r0)
{
	Prog *p;
	Adr *v1, *v2;
	Flow *r;
	int t;
	ProgInfo info;

	p = r0->prog;
	v1 = &p->from;
	if(!regtyp(v1))
		return 0;
	v2 = &p->to;
	if(!regtyp(v2))
		return 0;
	for(r=uniqp(r0); r!=nil; r=uniqp(r)) {
		if(debug['P'] && debug['v'])
			print("\t? %P\n", r->prog);
		if(uniqs(r) == nil)
			break;
		p = r->prog;
		if(p->as == AVARDEF || p->as == AVARKILL)
			continue;
		proginfo(&info, p);
		if(info.flags & Call)
			return 0;

		if(info.reguse | info.regset)
			return 0;

		if((info.flags & Move) && (info.flags & (SizeL|SizeQ|SizeF|SizeD)) && p->to.type == v1->type && p->to.reg == v1->reg)
			goto gotit;

		if(copyau(&p->from, v2) || copyau(&p->to, v2))
			break;
		if(copysub(&p->from, v1, v2, 0) || copysub(&p->to, v1, v2, 0))
			break;
	}
	return 0;

gotit:
	copysub(&p->to, v1, v2, 1);
	if(debug['P']) {
		print("gotit: %D->%D\n%P", v1, v2, r->prog);
		if(p->from.type == v2->type && p->from.reg == v2->reg)
			print(" excise");
		print("\n");
	}
	for(r=uniqs(r); r!=r0; r=uniqs(r)) {
		p = r->prog;
		copysub(&p->from, v1, v2, 1);
		copysub(&p->to, v1, v2, 1);
		if(debug['P'])
			print("%P\n", r->prog);
	}
	t = v1->reg;
	v1->reg = v2->reg;
	v2->reg = t;
	if(debug['P'])
		print("%P last\n", r->prog);
	return 1;
}

/*
 * The idea is to remove redundant copies.
 *	v1->v2	F=0
 *	(use v2	s/v2/v1/)*
 *	set v1	F=1
 *	use v2	return fail
 *	-----------------
 *	v1->v2	F=0
 *	(use v2	s/v2/v1/)*
 *	set v1	F=1
 *	set v2	return success
 */
static int
copyprop(Graph *g, Flow *r0)
{
	Prog *p;
	Adr *v1, *v2;

	USED(g);
	p = r0->prog;
	v1 = &p->from;
	v2 = &p->to;
	if(copyas(v1, v2))
		return 1;
	gactive++;
	return copy1(v1, v2, r0->s1, 0);
}

static int
copy1(Adr *v1, Adr *v2, Flow *r, int f)
{
	int t;
	Prog *p;

	if(r->active == gactive) {
		if(debug['P'])
			print("act set; return 1\n");
		return 1;
	}
	r->active = gactive;
	if(debug['P'])
		print("copy %D->%D f=%d\n", v1, v2, f);
	for(; r != nil; r = r->s1) {
		p = r->prog;
		if(debug['P'])
			print("%P", p);
		if(!f && uniqp(r) == nil) {
			f = 1;
			if(debug['P'])
				print("; merge; f=%d", f);
		}
		t = copyu(p, v2, nil);
		switch(t) {
		case 2:	/* rar, can't split */
			if(debug['P'])
				print("; %D rar; return 0\n", v2);
			return 0;

		case 3:	/* set */
			if(debug['P'])
				print("; %D set; return 1\n", v2);
			return 1;

		case 1:	/* used, substitute */
		case 4:	/* use and set */
			if(f) {
				if(!debug['P'])
					return 0;
				if(t == 4)
					print("; %D used+set and f=%d; return 0\n", v2, f);
				else
					print("; %D used and f=%d; return 0\n", v2, f);
				return 0;
			}
			if(copyu(p, v2, v1)) {
				if(debug['P'])
					print("; sub fail; return 0\n");
				return 0;
			}
			if(debug['P'])
				print("; sub %D/%D", v2, v1);
			if(t == 4) {
				if(debug['P'])
					print("; %D used+set; return 1\n", v2);
				return 1;
			}
			break;
		}
		if(!f) {
			t = copyu(p, v1, nil);
			if(!f && (t == 2 || t == 3 || t == 4)) {
				f = 1;
				if(debug['P'])
					print("; %D set and !f; f=%d", v1, f);
			}
		}
		if(debug['P'])
			print("\n");
		if(r->s2)
			if(!copy1(v1, v2, r->s2, f))
				return 0;
	}
	return 1;
}

/*
 * return
 * 1 if v only used (and substitute),
 * 2 if read-alter-rewrite
 * 3 if set
 * 4 if set and used
 * 0 otherwise (not touched)
 */
int
copyu(Prog *p, Adr *v, Adr *s)
{
	ProgInfo info;

	switch(p->as) {
	case AJMP:
		if(s != nil) {
			if(copysub(&p->to, v, s, 1))
				return 1;
			return 0;
		}
		if(copyau(&p->to, v))
			return 1;
		return 0;

	case ARET:
		if(s != nil)
			return 1;
		return 3;

	case ACALL:
		if(REGEXT && v->type == TYPE_REG && v->reg <= REGEXT && v->reg > exregoffset)
			return 2;
		if(REGARG >= 0 && v->type == TYPE_REG && v->reg == (uchar)REGARG)
			return 2;
		if(v->type == p->from.type && v->reg == p->from.reg)
			return 2;

		if(s != nil) {
			if(copysub(&p->to, v, s, 1))
				return 1;
			return 0;
		}
		if(copyau(&p->to, v))
			return 4;
		return 3;

	case ATEXT:
		if(REGARG >= 0 && v->type == TYPE_REG && v->reg == (uchar)REGARG)
			return 3;
		return 0;
	}

	if(p->as == AVARDEF || p->as == AVARKILL)
		return 0;
	proginfo(&info, p);

	if((info.reguse|info.regset) & RtoB(v->reg))
		return 2;
		
	if(info.flags & LeftAddr)
		if(copyas(&p->from, v))
			return 2;

	if((info.flags & (RightRead|RightWrite)) == (RightRead|RightWrite))
		if(copyas(&p->to, v))
			return 2;
	
	if(info.flags & RightWrite) {
		if(copyas(&p->to, v)) {
			if(s != nil)
				return copysub(&p->from, v, s, 1);
			if(copyau(&p->from, v))
				return 4;
			return 3;
		}
	}
	
	if(info.flags & (LeftAddr|LeftRead|LeftWrite|RightAddr|RightRead|RightWrite)) {
		if(s != nil) {
			if(copysub(&p->from, v, s, 1))
				return 1;
			return copysub(&p->to, v, s, 1);
		}
		if(copyau(&p->from, v))
			return 1;
		if(copyau(&p->to, v))
			return 1;
	}

	return 0;
}

/*
 * direct reference,
 * could be set/use depending on
 * semantics
 */
static int
copyas(Adr *a, Adr *v)
{
	if(REG_AL <= a->reg && a->reg <= REG_BL)
		fatal("use of byte register");
	if(REG_AL <= v->reg && v->reg <= REG_BL)
		fatal("use of byte register");

	if(a->type != v->type || a->name != v->name || a->reg != v->reg)
		return 0;
	if(regtyp(v))
		return 1;
	if(v->type == TYPE_MEM && (v->name == NAME_AUTO || v->name == NAME_PARAM))
		if(v->offset == a->offset)
			return 1;
	return 0;
}

int
sameaddr(Addr *a, Addr *v)
{
	if(a->type != v->type || a->name != v->name || a->reg != v->reg)
		return 0;
	if(regtyp(v))
		return 1;
	if(v->type == TYPE_MEM && (v->name == NAME_AUTO || v->name == NAME_PARAM))
		if(v->offset == a->offset)
			return 1;
	return 0;
}

/*
 * either direct or indirect
 */
static int
copyau(Adr *a, Adr *v)
{

	if(copyas(a, v))
		return 1;
	if(regtyp(v)) {
		if(a->type == TYPE_MEM && a->reg == v->reg)
			return 1;
		if(a->index == v->reg)
			return 1;
	}
	return 0;
}

/*
 * substitute s for v in a
 * return failure to substitute
 */
static int
copysub(Adr *a, Adr *v, Adr *s, int f)
{
	int reg;

	if(copyas(a, v)) {
		reg = s->reg;
		if(reg >= REG_AX && reg <= REG_DI || reg >= REG_X0 && reg <= REG_X7) {
			if(f)
				a->reg = reg;
		}
		return 0;
	}
	if(regtyp(v)) {
		reg = v->reg;
		if(a->type == TYPE_MEM && a->reg == reg) {
			if((s->reg == REG_BP) && a->index != TYPE_NONE)
				return 1;	/* can't use BP-base with index */
			if(f)
				a->reg = s->reg;
//			return 0;
		}
		if(a->index == reg) {
			if(f)
				a->index = s->reg;
			return 0;
		}
		return 0;
	}
	return 0;
}

static void
conprop(Flow *r0)
{
	Flow *r;
	Prog *p, *p0;
	int t;
	Adr *v0;

	p0 = r0->prog;
	v0 = &p0->to;
	r = r0;

loop:
	r = uniqs(r);
	if(r == nil || r == r0)
		return;
	if(uniqp(r) == nil)
		return;

	p = r->prog;
	t = copyu(p, v0, nil);
	switch(t) {
	case 0:	// miss
	case 1:	// use
		goto loop;

	case 2:	// rar
	case 4:	// use and set
		break;

	case 3:	// set
		if(p->as == p0->as)
		if(p->from.type == p0->from.type)
		if(p->from.reg == p0->from.reg)
		if(p->from.node == p0->from.node)
		if(p->from.offset == p0->from.offset)
		if(p->from.scale == p0->from.scale)
		if(p->from.type == TYPE_FCONST && p->from.u.dval == p0->from.u.dval)
		if(p->from.index == p0->from.index) {
			excise(r);
			goto loop;
		}
		break;
	}
}

int
smallindir(Addr *a, Addr *reg)
{
	return regtyp(reg) &&
		a->type == TYPE_MEM && a->reg == reg->reg &&
		a->index == REG_NONE &&
		0 <= a->offset && a->offset < 4096;
}

int
stackaddr(Addr *a)
{
	return a->type == TYPE_REG && a->reg == REG_SP;
}
