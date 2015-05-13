// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ld

import "cmd/internal/obj"

// Decoding the type.* symbols.	 This has to be in sync with
// ../../runtime/type.go, or more specifically, with what
// ../gc/reflect.c stuffs in these.

func decode_reloc(s *LSym, off int32) *Reloc {
	for i := 0; i < len(s.R); i++ {
		if s.R[i].Off == off {
			return &s.R[i:][0]
		}
	}
	return nil
}

func decode_reloc_sym(s *LSym, off int32) *LSym {
	r := decode_reloc(s, off)
	if r == nil {
		return nil
	}
	return r.Sym
}

func decode_inuxi(p []byte, sz int) uint64 {
	switch sz {
	case 2:
		return uint64(Ctxt.Arch.ByteOrder.Uint16(p))
	case 4:
		return uint64(Ctxt.Arch.ByteOrder.Uint32(p))
	case 8:
		return Ctxt.Arch.ByteOrder.Uint64(p)
	default:
		Exitf("dwarf: decode inuxi %d", sz)
		panic("unreachable")
	}
}

// commonsize returns the size of the common prefix for all type
// structures (runtime._type).
func commonsize() int {
	return 9*Thearch.Ptrsize + 8
}

// Type.commonType.kind
func decodetype_kind(s *LSym) uint8 {
	return uint8(s.P[2*Thearch.Ptrsize+7] & obj.KindMask) //  0x13 / 0x1f
}

// Type.commonType.kind
func decodetype_noptr(s *LSym) uint8 {
	return uint8(s.P[2*Thearch.Ptrsize+7] & obj.KindNoPointers) //  0x13 / 0x1f
}

// Type.commonType.kind
func decodetype_usegcprog(s *LSym) uint8 {
	return uint8(s.P[2*Thearch.Ptrsize+7] & obj.KindGCProg) //  0x13 / 0x1f
}

// Type.commonType.size
func decodetype_size(s *LSym) int64 {
	return int64(decode_inuxi(s.P, Thearch.Ptrsize)) // 0x8 / 0x10
}

// Type.commonType.ptrdata
func decodetype_ptrdata(s *LSym) int64 {
	return int64(decode_inuxi(s.P[Thearch.Ptrsize:], Thearch.Ptrsize)) // 0x8 / 0x10
}

// Type.commonType.gc
func decodetype_gcprog(s *LSym) *LSym {
	if s.Type == obj.SDYNIMPORT {
		// The gcprog for "type.$name" is calle "type..gcprog.$name".
		x := "type..gcprog." + s.Name[5:]
		return Linklookup(Ctxt, x, 0)
	}
	return decode_reloc_sym(s, 2*int32(Thearch.Ptrsize)+8+2*int32(Thearch.Ptrsize))
}

func decodetype_gcprog_shlib(s *LSym) uint64 {
	return decode_inuxi(s.P[2*int32(Thearch.Ptrsize)+8+1*int32(Thearch.Ptrsize):], Thearch.Ptrsize)
}

func decodetype_gcmask(s *LSym) []byte {
	if s.Type == obj.SDYNIMPORT {
		// ldshlibsyms makes special efforts to read the value
		// of gcmask for types defined in that shared library.
		return s.gcmask
	}
	mask := decode_reloc_sym(s, 2*int32(Thearch.Ptrsize)+8+1*int32(Thearch.Ptrsize))
	return mask.P
}

// Type.ArrayType.elem and Type.SliceType.Elem
func decodetype_arrayelem(s *LSym) *LSym {
	return decode_reloc_sym(s, int32(commonsize())) // 0x1c / 0x30
}

func decodetype_arraylen(s *LSym) int64 {
	return int64(decode_inuxi(s.P[commonsize()+2*Thearch.Ptrsize:], Thearch.Ptrsize))
}

// Type.PtrType.elem
func decodetype_ptrelem(s *LSym) *LSym {
	return decode_reloc_sym(s, int32(commonsize())) // 0x1c / 0x30
}

// Type.MapType.key, elem
func decodetype_mapkey(s *LSym) *LSym {
	return decode_reloc_sym(s, int32(commonsize())) // 0x1c / 0x30
}

func decodetype_mapvalue(s *LSym) *LSym {
	return decode_reloc_sym(s, int32(commonsize())+int32(Thearch.Ptrsize)) // 0x20 / 0x38
}

// Type.ChanType.elem
func decodetype_chanelem(s *LSym) *LSym {
	return decode_reloc_sym(s, int32(commonsize())) // 0x1c / 0x30
}

// Type.FuncType.dotdotdot
func decodetype_funcdotdotdot(s *LSym) int {
	return int(s.P[commonsize()])
}

// Type.FuncType.in.length
func decodetype_funcincount(s *LSym) int {
	return int(decode_inuxi(s.P[commonsize()+2*Thearch.Ptrsize:], Thearch.Intsize))
}

func decodetype_funcoutcount(s *LSym) int {
	return int(decode_inuxi(s.P[commonsize()+3*Thearch.Ptrsize+2*Thearch.Intsize:], Thearch.Intsize))
}

func decodetype_funcintype(s *LSym, i int) *LSym {
	r := decode_reloc(s, int32(commonsize())+int32(Thearch.Ptrsize))
	if r == nil {
		return nil
	}
	return decode_reloc_sym(r.Sym, int32(r.Add+int64(int32(i)*int32(Thearch.Ptrsize))))
}

func decodetype_funcouttype(s *LSym, i int) *LSym {
	r := decode_reloc(s, int32(commonsize())+2*int32(Thearch.Ptrsize)+2*int32(Thearch.Intsize))
	if r == nil {
		return nil
	}
	return decode_reloc_sym(r.Sym, int32(r.Add+int64(int32(i)*int32(Thearch.Ptrsize))))
}

// Type.StructType.fields.Slice::length
func decodetype_structfieldcount(s *LSym) int {
	return int(decode_inuxi(s.P[commonsize()+Thearch.Ptrsize:], Thearch.Intsize))
}

func structfieldsize() int {
	return 5 * Thearch.Ptrsize
}

// Type.StructType.fields[]-> name, typ and offset.
func decodetype_structfieldname(s *LSym, i int) string {
	// go.string."foo"  0x28 / 0x40
	s = decode_reloc_sym(s, int32(commonsize())+int32(Thearch.Ptrsize)+2*int32(Thearch.Intsize)+int32(i)*int32(structfieldsize()))

	if s == nil { // embedded structs have a nil name.
		return ""
	}
	r := decode_reloc(s, 0) // s has a pointer to the string data at offset 0
	if r == nil {           // shouldn't happen.
		return ""
	}
	return cstring(r.Sym.P[r.Add:])
}

func decodetype_structfieldtype(s *LSym, i int) *LSym {
	return decode_reloc_sym(s, int32(commonsize())+int32(Thearch.Ptrsize)+2*int32(Thearch.Intsize)+int32(i)*int32(structfieldsize())+2*int32(Thearch.Ptrsize))
}

func decodetype_structfieldoffs(s *LSym, i int) int64 {
	return int64(decode_inuxi(s.P[commonsize()+Thearch.Ptrsize+2*Thearch.Intsize+i*structfieldsize()+4*Thearch.Ptrsize:], Thearch.Intsize))
}

// InterfaceType.methods.length
func decodetype_ifacemethodcount(s *LSym) int64 {
	return int64(decode_inuxi(s.P[commonsize()+Thearch.Ptrsize:], Thearch.Intsize))
}
