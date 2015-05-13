// Inferno utils/5l/asm.c
// http://code.google.com/p/inferno-os/source/browse/utils/5l/asm.c
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

package main

import (
	"cmd/internal/ld"
	"cmd/internal/obj"
	"encoding/binary"
	"fmt"
	"log"
)

func needlib(name string) int {
	if name[0] == '\x00' {
		return 0
	}

	/* reuse hash code in symbol table */
	p := fmt.Sprintf(".dynlib.%s", name)

	s := ld.Linklookup(ld.Ctxt, p, 0)

	if s.Type == 0 {
		s.Type = 100 // avoid SDATA, etc.
		return 1
	}

	return 0
}

func gentext() {
	var s *ld.LSym
	var stub *ld.LSym
	var pprevtextp **ld.LSym
	var r *ld.Reloc
	var n string
	var o1 uint32
	var i int

	// The ppc64 ABI PLT has similar concepts to other
	// architectures, but is laid out quite differently.  When we
	// see an R_PPC64_REL24 relocation to a dynamic symbol
	// (indicating that the call needs to go through the PLT), we
	// generate up to three stubs and reserve a PLT slot.
	//
	// 1) The call site will be bl x; nop (where the relocation
	//    applies to the bl).  We rewrite this to bl x_stub; ld
	//    r2,24(r1).  The ld is necessary because x_stub will save
	//    r2 (the TOC pointer) at 24(r1) (the "TOC save slot").
	//
	// 2) We reserve space for a pointer in the .plt section (once
	//    per referenced dynamic function).  .plt is a data
	//    section filled solely by the dynamic linker (more like
	//    .plt.got on other architectures).  Initially, the
	//    dynamic linker will fill each slot with a pointer to the
	//    corresponding x@plt entry point.
	//
	// 3) We generate the "call stub" x_stub (once per dynamic
	//    function/object file pair).  This saves the TOC in the
	//    TOC save slot, reads the function pointer from x's .plt
	//    slot and calls it like any other global entry point
	//    (including setting r12 to the function address).
	//
	// 4) We generate the "symbol resolver stub" x@plt (once per
	//    dynamic function).  This is solely a branch to the glink
	//    resolver stub.
	//
	// 5) We generate the glink resolver stub (only once).  This
	//    computes which symbol resolver stub we came through and
	//    invokes the dynamic resolver via a pointer provided by
	//    the dynamic linker.  This will patch up the .plt slot to
	//    point directly at the function so future calls go
	//    straight from the call stub to the real function, and
	//    then call the function.

	// NOTE: It's possible we could make ppc64 closer to other
	// architectures: ppc64's .plt is like .plt.got on other
	// platforms and ppc64's .glink is like .plt on other
	// platforms.

	// Find all R_PPC64_REL24 relocations that reference dynamic
	// imports.  Reserve PLT entries for these symbols and
	// generate call stubs.  The call stubs need to live in .text,
	// which is why we need to do this pass this early.
	//
	// This assumes "case 1" from the ABI, where the caller needs
	// us to save and restore the TOC pointer.
	pprevtextp = &ld.Ctxt.Textp

	for s = *pprevtextp; s != nil; pprevtextp, s = &s.Next, s.Next {
		for i = range s.R {
			r = &s.R[i]
			if r.Type != 256+ld.R_PPC64_REL24 || r.Sym.Type != obj.SDYNIMPORT {
				continue
			}

			// Reserve PLT entry and generate symbol
			// resolver
			addpltsym(ld.Ctxt, r.Sym)

			// Generate call stub
			n = fmt.Sprintf("%s.%s", s.Name, r.Sym.Name)

			stub = ld.Linklookup(ld.Ctxt, n, 0)
			stub.Reachable = stub.Reachable || s.Reachable
			if stub.Size == 0 {
				// Need outer to resolve .TOC.
				stub.Outer = s

				// Link in to textp before s (we could
				// do it after, but would have to skip
				// the subsymbols)
				*pprevtextp = stub

				stub.Next = s
				pprevtextp = &stub.Next

				gencallstub(1, stub, r.Sym)
			}

			// Update the relocation to use the call stub
			r.Sym = stub

			// Restore TOC after bl.  The compiler put a
			// nop here for us to overwrite.
			o1 = 0xe8410018 // ld r2,24(r1)
			ld.Ctxt.Arch.ByteOrder.PutUint32(s.P[r.Off+4:], o1)
		}
	}
}

// Construct a call stub in stub that calls symbol targ via its PLT
// entry.
func gencallstub(abicase int, stub *ld.LSym, targ *ld.LSym) {
	if abicase != 1 {
		// If we see R_PPC64_TOCSAVE or R_PPC64_REL24_NOTOC
		// relocations, we'll need to implement cases 2 and 3.
		log.Fatalf("gencallstub only implements case 1 calls")
	}

	plt := ld.Linklookup(ld.Ctxt, ".plt", 0)

	stub.Type = obj.STEXT

	// Save TOC pointer in TOC save slot
	ld.Adduint32(ld.Ctxt, stub, 0xf8410018) // std r2,24(r1)

	// Load the function pointer from the PLT.
	r := ld.Addrel(stub)

	r.Off = int32(stub.Size)
	r.Sym = plt
	r.Add = int64(targ.Plt)
	r.Siz = 2
	if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
		r.Off += int32(r.Siz)
	}
	r.Type = obj.R_POWER_TOC
	r.Variant = ld.RV_POWER_HA
	ld.Adduint32(ld.Ctxt, stub, 0x3d820000) // addis r12,r2,targ@plt@toc@ha
	r = ld.Addrel(stub)
	r.Off = int32(stub.Size)
	r.Sym = plt
	r.Add = int64(targ.Plt)
	r.Siz = 2
	if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
		r.Off += int32(r.Siz)
	}
	r.Type = obj.R_POWER_TOC
	r.Variant = ld.RV_POWER_LO
	ld.Adduint32(ld.Ctxt, stub, 0xe98c0000) // ld r12,targ@plt@toc@l(r12)

	// Jump to the loaded pointer
	ld.Adduint32(ld.Ctxt, stub, 0x7d8903a6) // mtctr r12
	ld.Adduint32(ld.Ctxt, stub, 0x4e800420) // bctr
}

func adddynrela(rel *ld.LSym, s *ld.LSym, r *ld.Reloc) {
	log.Fatalf("adddynrela not implemented")
}

func adddynrel(s *ld.LSym, r *ld.Reloc) {
	targ := r.Sym
	ld.Ctxt.Cursym = s

	switch r.Type {
	default:
		if r.Type >= 256 {
			ld.Diag("unexpected relocation type %d", r.Type)
			return
		}

		// Handle relocations found in ELF object files.
	case 256 + ld.R_PPC64_REL24:
		r.Type = obj.R_CALLPOWER

		// This is a local call, so the caller isn't setting
		// up r12 and r2 is the same for the caller and
		// callee.  Hence, we need to go to the local entry
		// point.  (If we don't do this, the callee will try
		// to use r12 to compute r2.)
		r.Add += int64(r.Sym.Localentry) * 4

		if targ.Type == obj.SDYNIMPORT {
			// Should have been handled in elfsetupplt
			ld.Diag("unexpected R_PPC64_REL24 for dyn import")
		}

		return

	case 256 + ld.R_PPC64_ADDR64:
		r.Type = obj.R_ADDR
		if targ.Type == obj.SDYNIMPORT {
			// These happen in .toc sections
			adddynsym(ld.Ctxt, targ)

			rela := ld.Linklookup(ld.Ctxt, ".rela", 0)
			ld.Addaddrplus(ld.Ctxt, rela, s, int64(r.Off))
			ld.Adduint64(ld.Ctxt, rela, ld.ELF64_R_INFO(uint32(targ.Dynid), ld.R_PPC64_ADDR64))
			ld.Adduint64(ld.Ctxt, rela, uint64(r.Add))
			r.Type = 256 // ignore during relocsym
		}

		return

	case 256 + ld.R_PPC64_TOC16:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_LO | ld.RV_CHECK_OVERFLOW
		return

	case 256 + ld.R_PPC64_TOC16_LO:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_LO
		return

	case 256 + ld.R_PPC64_TOC16_HA:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_HA | ld.RV_CHECK_OVERFLOW
		return

	case 256 + ld.R_PPC64_TOC16_HI:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_HI | ld.RV_CHECK_OVERFLOW
		return

	case 256 + ld.R_PPC64_TOC16_DS:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_DS | ld.RV_CHECK_OVERFLOW
		return

	case 256 + ld.R_PPC64_TOC16_LO_DS:
		r.Type = obj.R_POWER_TOC
		r.Variant = ld.RV_POWER_DS
		return

	case 256 + ld.R_PPC64_REL16_LO:
		r.Type = obj.R_PCREL
		r.Variant = ld.RV_POWER_LO
		r.Add += 2 // Compensate for relocation size of 2
		return

	case 256 + ld.R_PPC64_REL16_HI:
		r.Type = obj.R_PCREL
		r.Variant = ld.RV_POWER_HI | ld.RV_CHECK_OVERFLOW
		r.Add += 2
		return

	case 256 + ld.R_PPC64_REL16_HA:
		r.Type = obj.R_PCREL
		r.Variant = ld.RV_POWER_HA | ld.RV_CHECK_OVERFLOW
		r.Add += 2
		return
	}

	// Handle references to ELF symbols from our own object files.
	if targ.Type != obj.SDYNIMPORT {
		return
	}

	// TODO(austin): Translate our relocations to ELF

	ld.Diag("unsupported relocation for dynamic symbol %s (type=%d stype=%d)", targ.Name, r.Type, targ.Type)
}

func elfreloc1(r *ld.Reloc, sectoff int64) int {
	// TODO(minux)
	return -1
}

func elfsetupplt() {
	plt := ld.Linklookup(ld.Ctxt, ".plt", 0)
	if plt.Size == 0 {
		// The dynamic linker stores the address of the
		// dynamic resolver and the DSO identifier in the two
		// doublewords at the beginning of the .plt section
		// before the PLT array.  Reserve space for these.
		plt.Size = 16
	}
}

func machoreloc1(r *ld.Reloc, sectoff int64) int {
	return -1
}

// Return the value of .TOC. for symbol s
func symtoc(s *ld.LSym) int64 {
	var toc *ld.LSym

	if s.Outer != nil {
		toc = ld.Linkrlookup(ld.Ctxt, ".TOC.", int(s.Outer.Version))
	} else {
		toc = ld.Linkrlookup(ld.Ctxt, ".TOC.", int(s.Version))
	}

	if toc == nil {
		ld.Diag("TOC-relative relocation in object without .TOC.")
		return 0
	}

	return toc.Value
}

func archreloc(r *ld.Reloc, s *ld.LSym, val *int64) int {
	if ld.Linkmode == ld.LinkExternal {
		// TODO(minux): translate R_ADDRPOWER and R_CALLPOWER into standard ELF relocations.
		// R_ADDRPOWER corresponds to R_PPC_ADDR16_HA and R_PPC_ADDR16_LO.
		// R_CALLPOWER corresponds to R_PPC_REL24.
		return -1
	}

	switch r.Type {
	case obj.R_CONST:
		*val = r.Add
		return 0

	case obj.R_GOTOFF:
		*val = ld.Symaddr(r.Sym) + r.Add - ld.Symaddr(ld.Linklookup(ld.Ctxt, ".got", 0))
		return 0

	case obj.R_ADDRPOWER:
		// r->add is two ppc64 instructions holding an immediate 32-bit constant.
		// We want to add r->sym's address to that constant.
		// The encoding of the immediate x<<16 + y,
		// where x is the low 16 bits of the first instruction and y is the low 16
		// bits of the second. Both x and y are signed (int16, not uint16).
		o1 := uint32(r.Add >> 32)
		o2 := uint32(r.Add)
		t := ld.Symaddr(r.Sym)
		if t < 0 {
			ld.Ctxt.Diag("relocation for %s is too big (>=2G): %d", s.Name, ld.Symaddr(r.Sym))
		}

		t += int64((o1&0xffff)<<16 + uint32(int32(o2)<<16>>16))
		if t&0x8000 != 0 {
			t += 0x10000
		}
		o1 = o1&0xffff0000 | (uint32(t)>>16)&0xffff
		o2 = o2&0xffff0000 | uint32(t)&0xffff

		// when laid out, the instruction order must always be o1, o2.
		if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
			*val = int64(o1)<<32 | int64(o2)
		} else {
			*val = int64(o2)<<32 | int64(o1)
		}
		return 0

	case obj.R_CALLPOWER:
		// Bits 6 through 29 = (S + A - P) >> 2
		var o1 uint32
		if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
			o1 = ld.Be32(s.P[r.Off:])
		} else {
			o1 = ld.Le32(s.P[r.Off:])
		}

		t := ld.Symaddr(r.Sym) + r.Add - (s.Value + int64(r.Off))
		if t&3 != 0 {
			ld.Ctxt.Diag("relocation for %s+%d is not aligned: %d", r.Sym.Name, r.Off, t)
		}
		if int64(int32(t<<6)>>6) != t {
			// TODO(austin) This can happen if text > 32M.
			// Add a call trampoline to .text in that case.
			ld.Ctxt.Diag("relocation for %s+%d is too big: %d", r.Sym.Name, r.Off, t)
		}

		*val = int64(o1&0xfc000003 | uint32(t)&^0xfc000003)
		return 0

	case obj.R_POWER_TOC: // S + A - .TOC.
		*val = ld.Symaddr(r.Sym) + r.Add - symtoc(s)

		return 0
	}

	return -1
}

func archrelocvariant(r *ld.Reloc, s *ld.LSym, t int64) int64 {
	switch r.Variant & ld.RV_TYPE_MASK {
	default:
		ld.Diag("unexpected relocation variant %d", r.Variant)
		fallthrough

	case ld.RV_NONE:
		return t

	case ld.RV_POWER_LO:
		if r.Variant&ld.RV_CHECK_OVERFLOW != 0 {
			// Whether to check for signed or unsigned
			// overflow depends on the instruction
			var o1 uint32
			if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
				o1 = ld.Be32(s.P[r.Off-2:])
			} else {
				o1 = ld.Le32(s.P[r.Off:])
			}
			switch o1 >> 26 {
			case 24, // ori
				26, // xori
				28: // andi
				if t>>16 != 0 {
					goto overflow
				}

			default:
				if int64(int16(t)) != t {
					goto overflow
				}
			}
		}

		return int64(int16(t))

	case ld.RV_POWER_HA:
		t += 0x8000
		fallthrough

		// Fallthrough
	case ld.RV_POWER_HI:
		t >>= 16

		if r.Variant&ld.RV_CHECK_OVERFLOW != 0 {
			// Whether to check for signed or unsigned
			// overflow depends on the instruction
			var o1 uint32
			if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
				o1 = ld.Be32(s.P[r.Off-2:])
			} else {
				o1 = ld.Le32(s.P[r.Off:])
			}
			switch o1 >> 26 {
			case 25, // oris
				27, // xoris
				29: // andis
				if t>>16 != 0 {
					goto overflow
				}

			default:
				if int64(int16(t)) != t {
					goto overflow
				}
			}
		}

		return int64(int16(t))

	case ld.RV_POWER_DS:
		var o1 uint32
		if ld.Ctxt.Arch.ByteOrder == binary.BigEndian {
			o1 = uint32(ld.Be16(s.P[r.Off:]))
		} else {
			o1 = uint32(ld.Le16(s.P[r.Off:]))
		}
		if t&3 != 0 {
			ld.Diag("relocation for %s+%d is not aligned: %d", r.Sym.Name, r.Off, t)
		}
		if (r.Variant&ld.RV_CHECK_OVERFLOW != 0) && int64(int16(t)) != t {
			goto overflow
		}
		return int64(o1)&0x3 | int64(int16(t))
	}

overflow:
	ld.Diag("relocation for %s+%d is too big: %d", r.Sym.Name, r.Off, t)
	return t
}

func addpltsym(ctxt *ld.Link, s *ld.LSym) {
	if s.Plt >= 0 {
		return
	}

	adddynsym(ctxt, s)

	if ld.Iself {
		plt := ld.Linklookup(ctxt, ".plt", 0)
		rela := ld.Linklookup(ctxt, ".rela.plt", 0)
		if plt.Size == 0 {
			elfsetupplt()
		}

		// Create the glink resolver if necessary
		glink := ensureglinkresolver()

		// Write symbol resolver stub (just a branch to the
		// glink resolver stub)
		r := ld.Addrel(glink)

		r.Sym = glink
		r.Off = int32(glink.Size)
		r.Siz = 4
		r.Type = obj.R_CALLPOWER
		ld.Adduint32(ctxt, glink, 0x48000000) // b .glink

		// In the ppc64 ABI, the dynamic linker is responsible
		// for writing the entire PLT.  We just need to
		// reserve 8 bytes for each PLT entry and generate a
		// JMP_SLOT dynamic relocation for it.
		//
		// TODO(austin): ABI v1 is different
		s.Plt = int32(plt.Size)

		plt.Size += 8

		ld.Addaddrplus(ctxt, rela, plt, int64(s.Plt))
		ld.Adduint64(ctxt, rela, ld.ELF64_R_INFO(uint32(s.Dynid), ld.R_PPC64_JMP_SLOT))
		ld.Adduint64(ctxt, rela, 0)
	} else {
		ld.Diag("addpltsym: unsupported binary format")
	}
}

// Generate the glink resolver stub if necessary and return the .glink section
func ensureglinkresolver() *ld.LSym {
	glink := ld.Linklookup(ld.Ctxt, ".glink", 0)
	if glink.Size != 0 {
		return glink
	}

	// This is essentially the resolver from the ppc64 ELF ABI.
	// At entry, r12 holds the address of the symbol resolver stub
	// for the target routine and the argument registers hold the
	// arguments for the target routine.
	//
	// This stub is PIC, so first get the PC of label 1 into r11.
	// Other things will be relative to this.
	ld.Adduint32(ld.Ctxt, glink, 0x7c0802a6) // mflr r0
	ld.Adduint32(ld.Ctxt, glink, 0x429f0005) // bcl 20,31,1f
	ld.Adduint32(ld.Ctxt, glink, 0x7d6802a6) // 1: mflr r11
	ld.Adduint32(ld.Ctxt, glink, 0x7c0803a6) // mtlf r0

	// Compute the .plt array index from the entry point address.
	// Because this is PIC, everything is relative to label 1b (in
	// r11):
	//   r0 = ((r12 - r11) - (res_0 - r11)) / 4 = (r12 - res_0) / 4
	ld.Adduint32(ld.Ctxt, glink, 0x3800ffd0) // li r0,-(res_0-1b)=-48
	ld.Adduint32(ld.Ctxt, glink, 0x7c006214) // add r0,r0,r12
	ld.Adduint32(ld.Ctxt, glink, 0x7c0b0050) // sub r0,r0,r11
	ld.Adduint32(ld.Ctxt, glink, 0x7800f082) // srdi r0,r0,2

	// r11 = address of the first byte of the PLT
	r := ld.Addrel(glink)

	r.Off = int32(glink.Size)
	r.Sym = ld.Linklookup(ld.Ctxt, ".plt", 0)
	r.Siz = 8
	r.Type = obj.R_ADDRPOWER

	// addis r11,0,.plt@ha; addi r11,r11,.plt@l
	r.Add = 0x3d600000<<32 | 0x396b0000

	glink.Size += 8

	// Load r12 = dynamic resolver address and r11 = DSO
	// identifier from the first two doublewords of the PLT.
	ld.Adduint32(ld.Ctxt, glink, 0xe98b0000) // ld r12,0(r11)
	ld.Adduint32(ld.Ctxt, glink, 0xe96b0008) // ld r11,8(r11)

	// Jump to the dynamic resolver
	ld.Adduint32(ld.Ctxt, glink, 0x7d8903a6) // mtctr r12
	ld.Adduint32(ld.Ctxt, glink, 0x4e800420) // bctr

	// The symbol resolvers must immediately follow.
	//   res_0:

	// Add DT_PPC64_GLINK .dynamic entry, which points to 32 bytes
	// before the first symbol resolver stub.
	s := ld.Linklookup(ld.Ctxt, ".dynamic", 0)

	ld.Elfwritedynentsymplus(s, ld.DT_PPC64_GLINK, glink, glink.Size-32)

	return glink
}

func adddynsym(ctxt *ld.Link, s *ld.LSym) {
	if s.Dynid >= 0 {
		return
	}

	if ld.Iself {
		s.Dynid = int32(ld.Nelfsym)
		ld.Nelfsym++

		d := ld.Linklookup(ctxt, ".dynsym", 0)

		name := s.Extname
		ld.Adduint32(ctxt, d, uint32(ld.Addstring(ld.Linklookup(ctxt, ".dynstr", 0), name)))

		/* type */
		t := ld.STB_GLOBAL << 4

		if s.Cgoexport != 0 && s.Type&obj.SMASK == obj.STEXT {
			t |= ld.STT_FUNC
		} else {
			t |= ld.STT_OBJECT
		}
		ld.Adduint8(ctxt, d, uint8(t))

		/* reserved */
		ld.Adduint8(ctxt, d, 0)

		/* section where symbol is defined */
		if s.Type == obj.SDYNIMPORT {
			ld.Adduint16(ctxt, d, ld.SHN_UNDEF)
		} else {
			ld.Adduint16(ctxt, d, 1)
		}

		/* value */
		if s.Type == obj.SDYNIMPORT {
			ld.Adduint64(ctxt, d, 0)
		} else {
			ld.Addaddr(ctxt, d, s)
		}

		/* size of object */
		ld.Adduint64(ctxt, d, uint64(s.Size))
	} else {
		ld.Diag("adddynsym: unsupported binary format")
	}
}

func adddynlib(lib string) {
	if needlib(lib) == 0 {
		return
	}

	if ld.Iself {
		s := ld.Linklookup(ld.Ctxt, ".dynstr", 0)
		if s.Size == 0 {
			ld.Addstring(s, "")
		}
		ld.Elfwritedynent(ld.Linklookup(ld.Ctxt, ".dynamic", 0), ld.DT_NEEDED, uint64(ld.Addstring(s, lib)))
	} else {
		ld.Diag("adddynlib: unsupported binary format")
	}
}

func asmb() {
	if ld.Debug['v'] != 0 {
		fmt.Fprintf(&ld.Bso, "%5.2f asmb\n", obj.Cputime())
	}
	ld.Bso.Flush()

	if ld.Iself {
		ld.Asmbelfsetup()
	}

	sect := ld.Segtext.Sect
	ld.Cseek(int64(sect.Vaddr - ld.Segtext.Vaddr + ld.Segtext.Fileoff))
	ld.Codeblk(int64(sect.Vaddr), int64(sect.Length))
	for sect = sect.Next; sect != nil; sect = sect.Next {
		ld.Cseek(int64(sect.Vaddr - ld.Segtext.Vaddr + ld.Segtext.Fileoff))
		ld.Datblk(int64(sect.Vaddr), int64(sect.Length))
	}

	if ld.Segrodata.Filelen > 0 {
		if ld.Debug['v'] != 0 {
			fmt.Fprintf(&ld.Bso, "%5.2f rodatblk\n", obj.Cputime())
		}
		ld.Bso.Flush()

		ld.Cseek(int64(ld.Segrodata.Fileoff))
		ld.Datblk(int64(ld.Segrodata.Vaddr), int64(ld.Segrodata.Filelen))
	}

	if ld.Debug['v'] != 0 {
		fmt.Fprintf(&ld.Bso, "%5.2f datblk\n", obj.Cputime())
	}
	ld.Bso.Flush()

	ld.Cseek(int64(ld.Segdata.Fileoff))
	ld.Datblk(int64(ld.Segdata.Vaddr), int64(ld.Segdata.Filelen))

	/* output symbol table */
	ld.Symsize = 0

	ld.Lcsize = 0
	symo := uint32(0)
	if ld.Debug['s'] == 0 {
		// TODO: rationalize
		if ld.Debug['v'] != 0 {
			fmt.Fprintf(&ld.Bso, "%5.2f sym\n", obj.Cputime())
		}
		ld.Bso.Flush()
		switch ld.HEADTYPE {
		default:
			if ld.Iself {
				symo = uint32(ld.Segdata.Fileoff + ld.Segdata.Filelen)
				symo = uint32(ld.Rnd(int64(symo), int64(ld.INITRND)))
			}

		case obj.Hplan9:
			symo = uint32(ld.Segdata.Fileoff + ld.Segdata.Filelen)
		}

		ld.Cseek(int64(symo))
		switch ld.HEADTYPE {
		default:
			if ld.Iself {
				if ld.Debug['v'] != 0 {
					fmt.Fprintf(&ld.Bso, "%5.2f elfsym\n", obj.Cputime())
				}
				ld.Asmelfsym()
				ld.Cflush()
				ld.Cwrite(ld.Elfstrdat)

				if ld.Debug['v'] != 0 {
					fmt.Fprintf(&ld.Bso, "%5.2f dwarf\n", obj.Cputime())
				}
				ld.Dwarfemitdebugsections()

				if ld.Linkmode == ld.LinkExternal {
					ld.Elfemitreloc()
				}
			}

		case obj.Hplan9:
			ld.Asmplan9sym()
			ld.Cflush()

			sym := ld.Linklookup(ld.Ctxt, "pclntab", 0)
			if sym != nil {
				ld.Lcsize = int32(len(sym.P))
				for i := 0; int32(i) < ld.Lcsize; i++ {
					ld.Cput(uint8(sym.P[i]))
				}

				ld.Cflush()
			}
		}
	}

	ld.Ctxt.Cursym = nil
	if ld.Debug['v'] != 0 {
		fmt.Fprintf(&ld.Bso, "%5.2f header\n", obj.Cputime())
	}
	ld.Bso.Flush()
	ld.Cseek(0)
	switch ld.HEADTYPE {
	default:
	case obj.Hplan9: /* plan 9 */
		ld.Thearch.Lput(0x647)                      /* magic */
		ld.Thearch.Lput(uint32(ld.Segtext.Filelen)) /* sizes */
		ld.Thearch.Lput(uint32(ld.Segdata.Filelen))
		ld.Thearch.Lput(uint32(ld.Segdata.Length - ld.Segdata.Filelen))
		ld.Thearch.Lput(uint32(ld.Symsize))      /* nsyms */
		ld.Thearch.Lput(uint32(ld.Entryvalue())) /* va of entry */
		ld.Thearch.Lput(0)
		ld.Thearch.Lput(uint32(ld.Lcsize))

	case obj.Hlinux,
		obj.Hfreebsd,
		obj.Hnetbsd,
		obj.Hopenbsd,
		obj.Hnacl:
		ld.Asmbelf(int64(symo))
	}

	ld.Cflush()
	if ld.Debug['c'] != 0 {
		fmt.Printf("textsize=%d\n", ld.Segtext.Filelen)
		fmt.Printf("datsize=%d\n", ld.Segdata.Filelen)
		fmt.Printf("bsssize=%d\n", ld.Segdata.Length-ld.Segdata.Filelen)
		fmt.Printf("symsize=%d\n", ld.Symsize)
		fmt.Printf("lcsize=%d\n", ld.Lcsize)
		fmt.Printf("total=%d\n", ld.Segtext.Filelen+ld.Segdata.Length+uint64(ld.Symsize)+uint64(ld.Lcsize))
	}
}
