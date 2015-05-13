// Inferno utils/6l/pass.c
// http://code.google.com/p/inferno-os/source/browse/utils/6l/pass.c
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

package x86

import (
	"cmd/internal/obj"
	"encoding/binary"
	"fmt"
	"log"
	"math"
)

func canuse1insntls(ctxt *obj.Link) bool {
	if ctxt.Arch.Regsize == 4 {
		switch ctxt.Headtype {
		case obj.Hlinux,
			obj.Hnacl,
			obj.Hplan9,
			obj.Hwindows:
			return false
		}

		return true
	}

	switch ctxt.Headtype {
	case obj.Hplan9,
		obj.Hwindows:
		return false
	case obj.Hlinux:
		return ctxt.Flag_shared == 0
	}

	return true
}

func progedit(ctxt *obj.Link, p *obj.Prog) {
	// Maintain information about code generation mode.
	if ctxt.Mode == 0 {
		ctxt.Mode = ctxt.Arch.Regsize * 8
	}
	p.Mode = int8(ctxt.Mode)

	switch p.As {
	case AMODE:
		if p.From.Type == obj.TYPE_CONST || (p.From.Type == obj.TYPE_MEM && p.From.Reg == REG_NONE) {
			switch int(p.From.Offset) {
			case 16, 32, 64:
				ctxt.Mode = int(p.From.Offset)
			}
		}
		obj.Nopout(p)
	}

	// Thread-local storage references use the TLS pseudo-register.
	// As a register, TLS refers to the thread-local storage base, and it
	// can only be loaded into another register:
	//
	//         MOVQ TLS, AX
	//
	// An offset from the thread-local storage base is written off(reg)(TLS*1).
	// Semantically it is off(reg), but the (TLS*1) annotation marks this as
	// indexing from the loaded TLS base. This emits a relocation so that
	// if the linker needs to adjust the offset, it can. For example:
	//
	//         MOVQ TLS, AX
	//         MOVQ 0(AX)(TLS*1), CX // load g into CX
	//
	// On systems that support direct access to the TLS memory, this
	// pair of instructions can be reduced to a direct TLS memory reference:
	//
	//         MOVQ 0(TLS), CX // load g into CX
	//
	// The 2-instruction and 1-instruction forms correspond to the two code
	// sequences for loading a TLS variable in the local exec model given in "ELF
	// Handling For Thread-Local Storage".
	//
	// We apply this rewrite on systems that support the 1-instruction form.
	// The decision is made using only the operating system and the -shared flag,
	// not the link mode. If some link modes on a particular operating system
	// require the 2-instruction form, then all builds for that operating system
	// will use the 2-instruction form, so that the link mode decision can be
	// delayed to link time.
	//
	// In this way, all supported systems use identical instructions to
	// access TLS, and they are rewritten appropriately first here in
	// liblink and then finally using relocations in the linker.
	//
	// When -shared is passed, we leave the code in the 2-instruction form but
	// assemble (and relocate) them in different ways to generate the initial
	// exec code sequence. It's a bit of a fluke that this is possible without
	// rewriting the instructions more comprehensively, and it only does because
	// we only support a single TLS variable (g).

	if canuse1insntls(ctxt) {
		// Reduce 2-instruction sequence to 1-instruction sequence.
		// Sequences like
		//	MOVQ TLS, BX
		//	... off(BX)(TLS*1) ...
		// become
		//	NOP
		//	... off(TLS) ...
		//
		// TODO(rsc): Remove the Hsolaris special case. It exists only to
		// guarantee we are producing byte-identical binaries as before this code.
		// But it should be unnecessary.
		if (p.As == AMOVQ || p.As == AMOVL) && p.From.Type == obj.TYPE_REG && p.From.Reg == REG_TLS && p.To.Type == obj.TYPE_REG && REG_AX <= p.To.Reg && p.To.Reg <= REG_R15 && ctxt.Headtype != obj.Hsolaris {
			obj.Nopout(p)
		}
		if p.From.Type == obj.TYPE_MEM && p.From.Index == REG_TLS && REG_AX <= p.From.Reg && p.From.Reg <= REG_R15 {
			p.From.Reg = REG_TLS
			p.From.Scale = 0
			p.From.Index = REG_NONE
		}

		if p.To.Type == obj.TYPE_MEM && p.To.Index == REG_TLS && REG_AX <= p.To.Reg && p.To.Reg <= REG_R15 {
			p.To.Reg = REG_TLS
			p.To.Scale = 0
			p.To.Index = REG_NONE
		}
	} else {
		// load_g_cx, below, always inserts the 1-instruction sequence. Rewrite it
		// as the 2-instruction sequence if necessary.
		//	MOVQ 0(TLS), BX
		// becomes
		//	MOVQ TLS, BX
		//	MOVQ 0(BX)(TLS*1), BX
		if (p.As == AMOVQ || p.As == AMOVL) && p.From.Type == obj.TYPE_MEM && p.From.Reg == REG_TLS && p.To.Type == obj.TYPE_REG && REG_AX <= p.To.Reg && p.To.Reg <= REG_R15 {
			q := obj.Appendp(ctxt, p)
			q.As = p.As
			q.From = p.From
			q.From.Type = obj.TYPE_MEM
			q.From.Reg = p.To.Reg
			q.From.Index = REG_TLS
			q.From.Scale = 2 // TODO: use 1
			q.To = p.To
			p.From.Type = obj.TYPE_REG
			p.From.Reg = REG_TLS
			p.From.Index = REG_NONE
			p.From.Offset = 0
		}
	}

	// TODO: Remove.
	if ctxt.Headtype == obj.Hwindows && p.Mode == 64 || ctxt.Headtype == obj.Hplan9 {
		if p.From.Scale == 1 && p.From.Index == REG_TLS {
			p.From.Scale = 2
		}
		if p.To.Scale == 1 && p.To.Index == REG_TLS {
			p.To.Scale = 2
		}
	}

	// Rewrite 0 to $0 in 3rd argment to CMPPS etc.
	// That's what the tables expect.
	switch p.As {
	case ACMPPD, ACMPPS, ACMPSD, ACMPSS:
		if p.To.Type == obj.TYPE_MEM && p.To.Name == obj.NAME_NONE && p.To.Reg == REG_NONE && p.To.Index == REG_NONE && p.To.Sym == nil {
			p.To.Type = obj.TYPE_CONST
		}
	}

	// Rewrite CALL/JMP/RET to symbol as TYPE_BRANCH.
	switch p.As {
	case obj.ACALL, obj.AJMP, obj.ARET:
		if p.To.Type == obj.TYPE_MEM && (p.To.Name == obj.NAME_EXTERN || p.To.Name == obj.NAME_STATIC) && p.To.Sym != nil {
			p.To.Type = obj.TYPE_BRANCH
		}
	}

	// Rewrite MOVL/MOVQ $XXX(FP/SP) as LEAL/LEAQ.
	if p.From.Type == obj.TYPE_ADDR && (ctxt.Arch.Thechar == '6' || p.From.Name != obj.NAME_EXTERN && p.From.Name != obj.NAME_STATIC) {
		switch p.As {
		case AMOVL:
			p.As = ALEAL
			p.From.Type = obj.TYPE_MEM
		case AMOVQ:
			p.As = ALEAQ
			p.From.Type = obj.TYPE_MEM
		}
	}

	if ctxt.Headtype == obj.Hnacl && p.Mode == 64 {
		nacladdr(ctxt, p, &p.From3)
		nacladdr(ctxt, p, &p.From)
		nacladdr(ctxt, p, &p.To)
	}

	// Rewrite float constants to values stored in memory.
	switch p.As {
	// Convert AMOVSS $(0), Xx to AXORPS Xx, Xx
	case AMOVSS:
		if p.From.Type == obj.TYPE_FCONST {
			if p.From.Val.(float64) == 0 {
				if p.To.Type == obj.TYPE_REG && REG_X0 <= p.To.Reg && p.To.Reg <= REG_X15 {
					p.As = AXORPS
					p.From = p.To
					break
				}
			}
		}
		fallthrough

	case AFMOVF,
		AFADDF,
		AFSUBF,
		AFSUBRF,
		AFMULF,
		AFDIVF,
		AFDIVRF,
		AFCOMF,
		AFCOMFP,
		AADDSS,
		ASUBSS,
		AMULSS,
		ADIVSS,
		ACOMISS,
		AUCOMISS:
		if p.From.Type == obj.TYPE_FCONST {
			f32 := float32(p.From.Val.(float64))
			i32 := math.Float32bits(f32)
			literal := fmt.Sprintf("$f32.%08x", i32)
			s := obj.Linklookup(ctxt, literal, 0)
			p.From.Type = obj.TYPE_MEM
			p.From.Name = obj.NAME_EXTERN
			p.From.Sym = s
			p.From.Sym.Local = true
			p.From.Offset = 0
		}

	case AMOVSD:
		// Convert AMOVSD $(0), Xx to AXORPS Xx, Xx
		if p.From.Type == obj.TYPE_FCONST {
			if p.From.Val.(float64) == 0 {
				if p.To.Type == obj.TYPE_REG && REG_X0 <= p.To.Reg && p.To.Reg <= REG_X15 {
					p.As = AXORPS
					p.From = p.To
					break
				}
			}
		}
		fallthrough

	case AFMOVD,
		AFADDD,
		AFSUBD,
		AFSUBRD,
		AFMULD,
		AFDIVD,
		AFDIVRD,
		AFCOMD,
		AFCOMDP,
		AADDSD,
		ASUBSD,
		AMULSD,
		ADIVSD,
		ACOMISD,
		AUCOMISD:
		if p.From.Type == obj.TYPE_FCONST {
			i64 := math.Float64bits(p.From.Val.(float64))
			literal := fmt.Sprintf("$f64.%016x", i64)
			s := obj.Linklookup(ctxt, literal, 0)
			p.From.Type = obj.TYPE_MEM
			p.From.Name = obj.NAME_EXTERN
			p.From.Sym = s
			p.From.Sym.Local = true
			p.From.Offset = 0
		}
	}

	if ctxt.Flag_dynlink && (p.As == obj.ADUFFCOPY || p.As == obj.ADUFFZERO) {
		var sym *obj.LSym
		if p.As == obj.ADUFFZERO {
			sym = obj.Linklookup(ctxt, "runtime.duffzero", 0)
		} else {
			sym = obj.Linklookup(ctxt, "runtime.duffcopy", 0)
		}
		offset := p.To.Offset
		p.As = AMOVQ
		p.From.Type = obj.TYPE_MEM
		p.From.Name = obj.NAME_GOTREF
		p.From.Sym = sym
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_R15
		p.To.Offset = 0
		p.To.Sym = nil
		p1 := obj.Appendp(ctxt, p)
		p1.As = AADDQ
		p1.From.Type = obj.TYPE_CONST
		p1.From.Offset = offset
		p1.To.Type = obj.TYPE_REG
		p1.To.Reg = REG_R15
		p2 := obj.Appendp(ctxt, p1)
		p2.As = obj.ACALL
		p2.To.Type = obj.TYPE_REG
		p2.To.Reg = REG_R15
	}

	if ctxt.Flag_dynlink {
		if p.As == ALEAQ && p.From.Type == obj.TYPE_MEM && p.From.Name == obj.NAME_EXTERN && !p.From.Sym.Local {
			p.As = AMOVQ
			p.From.Type = obj.TYPE_ADDR
		}
		if p.From.Type == obj.TYPE_ADDR && p.From.Name == obj.NAME_EXTERN && !p.From.Sym.Local {
			if p.As != AMOVQ {
				ctxt.Diag("do not know how to handle TYPE_ADDR in %v with -dynlink", p)
			}
			if p.To.Type != obj.TYPE_REG {
				ctxt.Diag("do not know how to handle LEAQ-type insn to non-register in %v with -dynlink", p)
			}
			p.From.Type = obj.TYPE_MEM
			p.From.Name = obj.NAME_GOTREF
			if p.From.Offset != 0 {
				q := obj.Appendp(ctxt, p)
				q.As = AADDQ
				q.From.Type = obj.TYPE_CONST
				q.From.Offset = p.From.Offset
				q.To = p.To
				p.From.Offset = 0
			}
		}
		if p.From3.Name == obj.NAME_EXTERN {
			ctxt.Diag("don't know how to handle %v with -dynlink", p)
		}
		if p.To2.Name == obj.NAME_EXTERN {
			ctxt.Diag("don't know how to handle %v with -dynlink", p)
		}
		var source *obj.Addr
		if p.From.Name == obj.NAME_EXTERN && !p.From.Sym.Local {
			if p.To.Name == obj.NAME_EXTERN && !p.To.Sym.Local {
				ctxt.Diag("cannot handle NAME_EXTERN on both sides in %v with -dynlink", p)
			}
			source = &p.From
		} else if p.To.Name == obj.NAME_EXTERN && !p.To.Sym.Local {
			source = &p.To
		} else {
			return
		}
		if p.As == obj.ATEXT || p.As == obj.AFUNCDATA || p.As == obj.ACALL || p.As == obj.ARET || p.As == obj.AJMP {
			return
		}
		if source.Type != obj.TYPE_MEM {
			ctxt.Diag("don't know how to handle %v with -dynlink", p)
		}
		p1 := obj.Appendp(ctxt, p)
		p2 := obj.Appendp(ctxt, p1)

		p1.As = AMOVQ
		p1.From.Type = obj.TYPE_MEM
		p1.From.Sym = source.Sym
		p1.From.Name = obj.NAME_GOTREF
		p1.To.Type = obj.TYPE_REG
		p1.To.Reg = REG_R15

		p2.As = p.As
		p2.From = p.From
		p2.To = p.To
		if p.From.Name == obj.NAME_EXTERN {
			p2.From.Reg = REG_R15
			p2.From.Name = obj.NAME_NONE
			p2.From.Sym = nil
		} else if p.To.Name == obj.NAME_EXTERN {
			p2.To.Reg = REG_R15
			p2.To.Name = obj.NAME_NONE
			p2.To.Sym = nil
		} else {
			return
		}
		l := p.Link
		l2 := p2.Link
		*p = *p1
		*p1 = *p2
		p.Link = l
		p1.Link = l2
	}
}

func nacladdr(ctxt *obj.Link, p *obj.Prog, a *obj.Addr) {
	if p.As == ALEAL || p.As == ALEAQ {
		return
	}

	if a.Reg == REG_BP {
		ctxt.Diag("invalid address: %v", p)
		return
	}

	if a.Reg == REG_TLS {
		a.Reg = REG_BP
	}
	if a.Type == obj.TYPE_MEM && a.Name == obj.NAME_NONE {
		switch a.Reg {
		// all ok
		case REG_BP, REG_SP, REG_R15:
			break

		default:
			if a.Index != REG_NONE {
				ctxt.Diag("invalid address %v", p)
			}
			a.Index = a.Reg
			if a.Index != REG_NONE {
				a.Scale = 1
			}
			a.Reg = REG_R15
		}
	}
}

func preprocess(ctxt *obj.Link, cursym *obj.LSym) {
	if ctxt.Tlsg == nil {
		ctxt.Tlsg = obj.Linklookup(ctxt, "runtime.tlsg", 0)
	}

	if ctxt.Headtype == obj.Hplan9 && ctxt.Plan9privates == nil {
		ctxt.Plan9privates = obj.Linklookup(ctxt, "_privates", 0)
	}

	ctxt.Cursym = cursym

	if cursym.Text == nil || cursym.Text.Link == nil {
		return
	}

	p := cursym.Text
	autoffset := int32(p.To.Offset)
	if autoffset < 0 {
		autoffset = 0
	}

	var bpsize int
	if p.Mode == 64 && obj.Framepointer_enabled != 0 && autoffset > 0 {
		// Make room for to save a base pointer.  If autoffset == 0,
		// this might do something special like a tail jump to
		// another function, so in that case we omit this.
		bpsize = ctxt.Arch.Ptrsize

		autoffset += int32(bpsize)
		p.To.Offset += int64(bpsize)
	} else {
		bpsize = 0
	}

	textarg := int64(p.To.Val.(int32))
	cursym.Args = int32(textarg)
	cursym.Locals = int32(p.To.Offset)

	// TODO(rsc): Remove.
	if p.Mode == 32 && cursym.Locals < 0 {
		cursym.Locals = 0
	}

	// TODO(rsc): Remove 'p.Mode == 64 &&'.
	if p.Mode == 64 && autoffset < obj.StackSmall && p.From3.Offset&obj.NOSPLIT == 0 {
		for q := p; q != nil; q = q.Link {
			if q.As == obj.ACALL {
				goto noleaf
			}
			if (q.As == obj.ADUFFCOPY || q.As == obj.ADUFFZERO) && autoffset >= obj.StackSmall-8 {
				goto noleaf
			}
		}

		p.From3.Offset |= obj.NOSPLIT
	noleaf:
	}

	if p.From3.Offset&obj.NOSPLIT == 0 || (p.From3.Offset&obj.WRAPPER != 0) {
		p = obj.Appendp(ctxt, p)
		p = load_g_cx(ctxt, p) // load g into CX
	}

	var q *obj.Prog
	if cursym.Text.From3.Offset&obj.NOSPLIT == 0 {
		p = stacksplit(ctxt, p, autoffset, int32(textarg), &q) // emit split check
	}

	if autoffset != 0 {
		if autoffset%int32(ctxt.Arch.Regsize) != 0 {
			ctxt.Diag("unaligned stack size %d", autoffset)
		}
		p = obj.Appendp(ctxt, p)
		p.As = AADJSP
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = int64(autoffset)
		p.Spadj = autoffset
	} else {
		// zero-byte stack adjustment.
		// Insert a fake non-zero adjustment so that stkcheck can
		// recognize the end of the stack-splitting prolog.
		p = obj.Appendp(ctxt, p)

		p.As = obj.ANOP
		p.Spadj = int32(-ctxt.Arch.Ptrsize)
		p = obj.Appendp(ctxt, p)
		p.As = obj.ANOP
		p.Spadj = int32(ctxt.Arch.Ptrsize)
	}

	if q != nil {
		q.Pcond = p
	}
	deltasp := autoffset

	if bpsize > 0 {
		// Save caller's BP
		p = obj.Appendp(ctxt, p)

		p.As = AMOVQ
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_BP
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = REG_SP
		p.To.Scale = 1
		p.To.Offset = int64(autoffset) - int64(bpsize)

		// Move current frame to BP
		p = obj.Appendp(ctxt, p)

		p.As = ALEAQ
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_SP
		p.From.Scale = 1
		p.From.Offset = int64(autoffset) - int64(bpsize)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_BP
	}

	if cursym.Text.From3.Offset&obj.WRAPPER != 0 {
		// if(g->panic != nil && g->panic->argp == FP) g->panic->argp = bottom-of-frame
		//
		//	MOVQ g_panic(CX), BX
		//	TESTQ BX, BX
		//	JEQ end
		//	LEAQ (autoffset+8)(SP), DI
		//	CMPQ panic_argp(BX), DI
		//	JNE end
		//	MOVQ SP, panic_argp(BX)
		// end:
		//	NOP
		//
		// The NOP is needed to give the jumps somewhere to land.
		// It is a liblink NOP, not an x86 NOP: it encodes to 0 instruction bytes.

		p = obj.Appendp(ctxt, p)

		p.As = AMOVQ
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_CX
		p.From.Offset = 4 * int64(ctxt.Arch.Ptrsize) // G.panic
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_BX
		if ctxt.Headtype == obj.Hnacl && p.Mode == 64 {
			p.As = AMOVL
			p.From.Type = obj.TYPE_MEM
			p.From.Reg = REG_R15
			p.From.Scale = 1
			p.From.Index = REG_CX
		}
		if p.Mode == 32 {
			p.As = AMOVL
		}

		p = obj.Appendp(ctxt, p)
		p.As = ATESTQ
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_BX
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_BX
		if ctxt.Headtype == obj.Hnacl || p.Mode == 32 {
			p.As = ATESTL
		}

		p = obj.Appendp(ctxt, p)
		p.As = AJEQ
		p.To.Type = obj.TYPE_BRANCH
		p1 := p

		p = obj.Appendp(ctxt, p)
		p.As = ALEAQ
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_SP
		p.From.Offset = int64(autoffset) + int64(ctxt.Arch.Regsize)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_DI
		if ctxt.Headtype == obj.Hnacl || p.Mode == 32 {
			p.As = ALEAL
		}

		p = obj.Appendp(ctxt, p)
		p.As = ACMPQ
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_BX
		p.From.Offset = 0 // Panic.argp
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_DI
		if ctxt.Headtype == obj.Hnacl && p.Mode == 64 {
			p.As = ACMPL
			p.From.Type = obj.TYPE_MEM
			p.From.Reg = REG_R15
			p.From.Scale = 1
			p.From.Index = REG_BX
		}
		if p.Mode == 32 {
			p.As = ACMPL
		}

		p = obj.Appendp(ctxt, p)
		p.As = AJNE
		p.To.Type = obj.TYPE_BRANCH
		p2 := p

		p = obj.Appendp(ctxt, p)
		p.As = AMOVQ
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_SP
		p.To.Type = obj.TYPE_MEM
		p.To.Reg = REG_BX
		p.To.Offset = 0 // Panic.argp
		if ctxt.Headtype == obj.Hnacl && p.Mode == 64 {
			p.As = AMOVL
			p.To.Type = obj.TYPE_MEM
			p.To.Reg = REG_R15
			p.To.Scale = 1
			p.To.Index = REG_BX
		}
		if p.Mode == 32 {
			p.As = AMOVL
		}

		p = obj.Appendp(ctxt, p)
		p.As = obj.ANOP
		p1.Pcond = p
		p2.Pcond = p
	}

	if ctxt.Debugzerostack != 0 && autoffset != 0 && cursym.Text.From3.Offset&obj.NOSPLIT == 0 {
		// 6l -Z means zero the stack frame on entry.
		// This slows down function calls but can help avoid
		// false positives in garbage collection.
		p = obj.Appendp(ctxt, p)

		p.As = AMOVQ
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_SP
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_DI
		if p.Mode == 32 {
			p.As = AMOVL
		}

		p = obj.Appendp(ctxt, p)
		p.As = AMOVQ
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = int64(autoffset) / int64(ctxt.Arch.Regsize)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_CX
		if p.Mode == 32 {
			p.As = AMOVL
		}

		p = obj.Appendp(ctxt, p)
		p.As = AMOVQ
		p.From.Type = obj.TYPE_CONST
		p.From.Offset = 0
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_AX
		if p.Mode == 32 {
			p.As = AMOVL
		}

		p = obj.Appendp(ctxt, p)
		p.As = AREP

		p = obj.Appendp(ctxt, p)
		p.As = ASTOSQ
		if p.Mode == 32 {
			p.As = ASTOSL
		}
	}

	var a int
	var pcsize int
	for ; p != nil; p = p.Link {
		pcsize = int(p.Mode) / 8
		a = int(p.From.Name)
		if a == obj.NAME_AUTO {
			p.From.Offset += int64(deltasp) - int64(bpsize)
		}
		if a == obj.NAME_PARAM {
			p.From.Offset += int64(deltasp) + int64(pcsize)
		}
		a = int(p.From3.Name)
		if a == obj.NAME_AUTO {
			p.From3.Offset += int64(deltasp) - int64(bpsize)
		}
		if a == obj.NAME_PARAM {
			p.From3.Offset += int64(deltasp) + int64(pcsize)
		}
		a = int(p.To.Name)
		if a == obj.NAME_AUTO {
			p.To.Offset += int64(deltasp) - int64(bpsize)
		}
		if a == obj.NAME_PARAM {
			p.To.Offset += int64(deltasp) + int64(pcsize)
		}

		switch p.As {
		default:
			continue

		case APUSHL, APUSHFL:
			deltasp += 4
			p.Spadj = 4
			continue

		case APUSHQ, APUSHFQ:
			deltasp += 8
			p.Spadj = 8
			continue

		case APUSHW, APUSHFW:
			deltasp += 2
			p.Spadj = 2
			continue

		case APOPL, APOPFL:
			deltasp -= 4
			p.Spadj = -4
			continue

		case APOPQ, APOPFQ:
			deltasp -= 8
			p.Spadj = -8
			continue

		case APOPW, APOPFW:
			deltasp -= 2
			p.Spadj = -2
			continue

		case obj.ARET:
			break
		}

		if autoffset != deltasp {
			ctxt.Diag("unbalanced PUSH/POP")
		}

		if autoffset != 0 {
			if bpsize > 0 {
				// Restore caller's BP
				p.As = AMOVQ

				p.From.Type = obj.TYPE_MEM
				p.From.Reg = REG_SP
				p.From.Scale = 1
				p.From.Offset = int64(autoffset) - int64(bpsize)
				p.To.Type = obj.TYPE_REG
				p.To.Reg = REG_BP
				p = obj.Appendp(ctxt, p)
			}

			p.As = AADJSP
			p.From.Type = obj.TYPE_CONST
			p.From.Offset = int64(-autoffset)
			p.Spadj = -autoffset
			p = obj.Appendp(ctxt, p)
			p.As = obj.ARET

			// If there are instructions following
			// this ARET, they come from a branch
			// with the same stackframe, so undo
			// the cleanup.
			p.Spadj = +autoffset
		}

		if p.To.Sym != nil { // retjmp
			p.As = obj.AJMP
		}
	}
}

func indir_cx(ctxt *obj.Link, p *obj.Prog, a *obj.Addr) {
	if ctxt.Headtype == obj.Hnacl && p.Mode == 64 {
		a.Type = obj.TYPE_MEM
		a.Reg = REG_R15
		a.Index = REG_CX
		a.Scale = 1
		return
	}

	a.Type = obj.TYPE_MEM
	a.Reg = REG_CX
}

// Append code to p to load g into cx.
// Overwrites p with the first instruction (no first appendp).
// Overwriting p is unusual but it lets use this in both the
// prologue (caller must call appendp first) and in the epilogue.
// Returns last new instruction.
func load_g_cx(ctxt *obj.Link, p *obj.Prog) *obj.Prog {
	p.As = AMOVQ
	if ctxt.Arch.Ptrsize == 4 {
		p.As = AMOVL
	}
	p.From.Type = obj.TYPE_MEM
	p.From.Reg = REG_TLS
	p.From.Offset = 0
	p.To.Type = obj.TYPE_REG
	p.To.Reg = REG_CX

	next := p.Link
	progedit(ctxt, p)
	for p.Link != next {
		p = p.Link
	}

	if p.From.Index == REG_TLS {
		p.From.Scale = 2
	}

	return p
}

// Append code to p to check for stack split.
// Appends to (does not overwrite) p.
// Assumes g is in CX.
// Returns last new instruction.
// On return, *jmpok is the instruction that should jump
// to the stack frame allocation if no split is needed.
func stacksplit(ctxt *obj.Link, p *obj.Prog, framesize int32, textarg int32, jmpok **obj.Prog) *obj.Prog {
	cmp := ACMPQ
	lea := ALEAQ
	mov := AMOVQ
	sub := ASUBQ

	if ctxt.Headtype == obj.Hnacl || p.Mode == 32 {
		cmp = ACMPL
		lea = ALEAL
		mov = AMOVL
		sub = ASUBL
	}

	var q1 *obj.Prog
	if framesize <= obj.StackSmall {
		// small stack: SP <= stackguard
		//	CMPQ SP, stackguard
		p = obj.Appendp(ctxt, p)

		p.As = int16(cmp)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_SP
		indir_cx(ctxt, p, &p.To)
		p.To.Offset = 2 * int64(ctxt.Arch.Ptrsize) // G.stackguard0
		if ctxt.Cursym.Cfunc != 0 {
			p.To.Offset = 3 * int64(ctxt.Arch.Ptrsize) // G.stackguard1
		}
	} else if framesize <= obj.StackBig {
		// large stack: SP-framesize <= stackguard-StackSmall
		//	LEAQ -xxx(SP), AX
		//	CMPQ AX, stackguard
		p = obj.Appendp(ctxt, p)

		p.As = int16(lea)
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_SP
		p.From.Offset = -(int64(framesize) - obj.StackSmall)
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_AX

		p = obj.Appendp(ctxt, p)
		p.As = int16(cmp)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_AX
		indir_cx(ctxt, p, &p.To)
		p.To.Offset = 2 * int64(ctxt.Arch.Ptrsize) // G.stackguard0
		if ctxt.Cursym.Cfunc != 0 {
			p.To.Offset = 3 * int64(ctxt.Arch.Ptrsize) // G.stackguard1
		}
	} else {
		// Such a large stack we need to protect against wraparound.
		// If SP is close to zero:
		//	SP-stackguard+StackGuard <= framesize + (StackGuard-StackSmall)
		// The +StackGuard on both sides is required to keep the left side positive:
		// SP is allowed to be slightly below stackguard. See stack.h.
		//
		// Preemption sets stackguard to StackPreempt, a very large value.
		// That breaks the math above, so we have to check for that explicitly.
		//	MOVQ	stackguard, CX
		//	CMPQ	CX, $StackPreempt
		//	JEQ	label-of-call-to-morestack
		//	LEAQ	StackGuard(SP), AX
		//	SUBQ	CX, AX
		//	CMPQ	AX, $(framesize+(StackGuard-StackSmall))

		p = obj.Appendp(ctxt, p)

		p.As = int16(mov)
		indir_cx(ctxt, p, &p.From)
		p.From.Offset = 2 * int64(ctxt.Arch.Ptrsize) // G.stackguard0
		if ctxt.Cursym.Cfunc != 0 {
			p.From.Offset = 3 * int64(ctxt.Arch.Ptrsize) // G.stackguard1
		}
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_SI

		p = obj.Appendp(ctxt, p)
		p.As = int16(cmp)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_SI
		p.To.Type = obj.TYPE_CONST
		p.To.Offset = obj.StackPreempt
		if p.Mode == 32 {
			p.To.Offset = int64(uint32(obj.StackPreempt & (1<<32 - 1)))
		}

		p = obj.Appendp(ctxt, p)
		p.As = AJEQ
		p.To.Type = obj.TYPE_BRANCH
		q1 = p

		p = obj.Appendp(ctxt, p)
		p.As = int16(lea)
		p.From.Type = obj.TYPE_MEM
		p.From.Reg = REG_SP
		p.From.Offset = obj.StackGuard
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_AX

		p = obj.Appendp(ctxt, p)
		p.As = int16(sub)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_SI
		p.To.Type = obj.TYPE_REG
		p.To.Reg = REG_AX

		p = obj.Appendp(ctxt, p)
		p.As = int16(cmp)
		p.From.Type = obj.TYPE_REG
		p.From.Reg = REG_AX
		p.To.Type = obj.TYPE_CONST
		p.To.Offset = int64(framesize) + (obj.StackGuard - obj.StackSmall)
	}

	// common
	p = obj.Appendp(ctxt, p)

	p.As = AJHI
	p.To.Type = obj.TYPE_BRANCH
	q := p

	p = obj.Appendp(ctxt, p)
	p.As = obj.ACALL
	p.To.Type = obj.TYPE_BRANCH
	if ctxt.Cursym.Cfunc != 0 {
		p.To.Sym = obj.Linklookup(ctxt, "runtime.morestackc", 0)
	} else if ctxt.Cursym.Text.From3.Offset&obj.NEEDCTXT == 0 {
		p.To.Sym = obj.Linklookup(ctxt, "runtime.morestack_noctxt", 0)
	} else {
		p.To.Sym = obj.Linklookup(ctxt, "runtime.morestack", 0)
	}

	p = obj.Appendp(ctxt, p)
	p.As = obj.AJMP
	p.To.Type = obj.TYPE_BRANCH
	p.Pcond = ctxt.Cursym.Text.Link

	if q != nil {
		q.Pcond = p.Link
	}
	if q1 != nil {
		q1.Pcond = q.Link
	}

	*jmpok = q
	return p
}

func follow(ctxt *obj.Link, s *obj.LSym) {
	ctxt.Cursym = s

	firstp := ctxt.NewProg()
	lastp := firstp
	xfol(ctxt, s.Text, &lastp)
	lastp.Link = nil
	s.Text = firstp.Link
}

func nofollow(a int) bool {
	switch a {
	case obj.AJMP,
		obj.ARET,
		AIRETL,
		AIRETQ,
		AIRETW,
		ARETFL,
		ARETFQ,
		ARETFW,
		obj.AUNDEF:
		return true
	}

	return false
}

func pushpop(a int) bool {
	switch a {
	case APUSHL,
		APUSHFL,
		APUSHQ,
		APUSHFQ,
		APUSHW,
		APUSHFW,
		APOPL,
		APOPFL,
		APOPQ,
		APOPFQ,
		APOPW,
		APOPFW:
		return true
	}

	return false
}

func relinv(a int16) int16 {
	switch a {
	case AJEQ:
		return AJNE
	case AJNE:
		return AJEQ
	case AJLE:
		return AJGT
	case AJLS:
		return AJHI
	case AJLT:
		return AJGE
	case AJMI:
		return AJPL
	case AJGE:
		return AJLT
	case AJPL:
		return AJMI
	case AJGT:
		return AJLE
	case AJHI:
		return AJLS
	case AJCS:
		return AJCC
	case AJCC:
		return AJCS
	case AJPS:
		return AJPC
	case AJPC:
		return AJPS
	case AJOS:
		return AJOC
	case AJOC:
		return AJOS
	}

	log.Fatalf("unknown relation: %s", obj.Aconv(int(a)))
	return 0
}

func xfol(ctxt *obj.Link, p *obj.Prog, last **obj.Prog) {
	var q *obj.Prog
	var i int
	var a int

loop:
	if p == nil {
		return
	}
	if p.As == obj.AJMP {
		q = p.Pcond
		if q != nil && q.As != obj.ATEXT {
			/* mark instruction as done and continue layout at target of jump */
			p.Mark = 1

			p = q
			if p.Mark == 0 {
				goto loop
			}
		}
	}

	if p.Mark != 0 {
		/*
		 * p goes here, but already used it elsewhere.
		 * copy up to 4 instructions or else branch to other copy.
		 */
		i = 0
		q = p
		for ; i < 4; i, q = i+1, q.Link {
			if q == nil {
				break
			}
			if q == *last {
				break
			}
			a = int(q.As)
			if a == obj.ANOP {
				i--
				continue
			}

			if nofollow(a) || pushpop(a) {
				break // NOTE(rsc): arm does goto copy
			}
			if q.Pcond == nil || q.Pcond.Mark != 0 {
				continue
			}
			if a == obj.ACALL || a == ALOOP {
				continue
			}
			for {
				if p.As == obj.ANOP {
					p = p.Link
					continue
				}

				q = obj.Copyp(ctxt, p)
				p = p.Link
				q.Mark = 1
				(*last).Link = q
				*last = q
				if int(q.As) != a || q.Pcond == nil || q.Pcond.Mark != 0 {
					continue
				}

				q.As = relinv(q.As)
				p = q.Pcond
				q.Pcond = q.Link
				q.Link = p
				xfol(ctxt, q.Link, last)
				p = q.Link
				if p.Mark != 0 {
					return
				}
				goto loop
				/* */
			}
		}
		q = ctxt.NewProg()
		q.As = obj.AJMP
		q.Lineno = p.Lineno
		q.To.Type = obj.TYPE_BRANCH
		q.To.Offset = p.Pc
		q.Pcond = p
		p = q
	}

	/* emit p */
	p.Mark = 1

	(*last).Link = p
	*last = p
	a = int(p.As)

	/* continue loop with what comes after p */
	if nofollow(a) {
		return
	}
	if p.Pcond != nil && a != obj.ACALL {
		/*
		 * some kind of conditional branch.
		 * recurse to follow one path.
		 * continue loop on the other.
		 */
		q = obj.Brchain(ctxt, p.Pcond)
		if q != nil {
			p.Pcond = q
		}
		q = obj.Brchain(ctxt, p.Link)
		if q != nil {
			p.Link = q
		}
		if p.From.Type == obj.TYPE_CONST {
			if p.From.Offset == 1 {
				/*
				 * expect conditional jump to be taken.
				 * rewrite so that's the fall-through case.
				 */
				p.As = relinv(int16(a))

				q = p.Link
				p.Link = p.Pcond
				p.Pcond = q
			}
		} else {
			q = p.Link
			if q.Mark != 0 {
				if a != ALOOP {
					p.As = relinv(int16(a))
					p.Link = p.Pcond
					p.Pcond = q
				}
			}
		}

		xfol(ctxt, p.Link, last)
		if p.Pcond.Mark != 0 {
			return
		}
		p = p.Pcond
		goto loop
	}

	p = p.Link
	goto loop
}

var unaryDst = map[int]bool{
	ABSWAPL:    true,
	ABSWAPQ:    true,
	ACMPXCHG8B: true,
	ADECB:      true,
	ADECL:      true,
	ADECQ:      true,
	ADECW:      true,
	AINCB:      true,
	AINCL:      true,
	AINCQ:      true,
	AINCW:      true,
	ANEGB:      true,
	ANEGL:      true,
	ANEGQ:      true,
	ANEGW:      true,
	ANOTB:      true,
	ANOTL:      true,
	ANOTQ:      true,
	ANOTW:      true,
	APOPL:      true,
	APOPQ:      true,
	APOPW:      true,
	ASETCC:     true,
	ASETCS:     true,
	ASETEQ:     true,
	ASETGE:     true,
	ASETGT:     true,
	ASETHI:     true,
	ASETLE:     true,
	ASETLS:     true,
	ASETLT:     true,
	ASETMI:     true,
	ASETNE:     true,
	ASETOC:     true,
	ASETOS:     true,
	ASETPC:     true,
	ASETPL:     true,
	ASETPS:     true,
	AFFREE:     true,
	AFLDENV:    true,
	AFSAVE:     true,
	AFSTCW:     true,
	AFSTENV:    true,
	AFSTSW:     true,
	AFXSAVE:    true,
	AFXSAVE64:  true,
	ASTMXCSR:   true,
}

var Linkamd64 = obj.LinkArch{
	ByteOrder:  binary.LittleEndian,
	Name:       "amd64",
	Thechar:    '6',
	Preprocess: preprocess,
	Assemble:   span6,
	Follow:     follow,
	Progedit:   progedit,
	UnaryDst:   unaryDst,
	Minlc:      1,
	Ptrsize:    8,
	Regsize:    8,
}

var Linkamd64p32 = obj.LinkArch{
	ByteOrder:  binary.LittleEndian,
	Name:       "amd64p32",
	Thechar:    '6',
	Preprocess: preprocess,
	Assemble:   span6,
	Follow:     follow,
	Progedit:   progedit,
	UnaryDst:   unaryDst,
	Minlc:      1,
	Ptrsize:    4,
	Regsize:    8,
}

var Link386 = obj.LinkArch{
	ByteOrder:  binary.LittleEndian,
	Name:       "386",
	Thechar:    '8',
	Preprocess: preprocess,
	Assemble:   span6,
	Follow:     follow,
	Progedit:   progedit,
	UnaryDst:   unaryDst,
	Minlc:      1,
	Ptrsize:    4,
	Regsize:    4,
}
