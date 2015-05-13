// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Runtime type representation.

package runtime

import "unsafe"

// Needs to be in sync with ../cmd/internal/ld/decodesym.go:/^func.commonsize,
// ../cmd/internal/gc/reflect.go:/^func.dcommontype and
// ../reflect/type.go:/^type.rtype.
type _type struct {
	size       uintptr
	ptrdata    uintptr // size of memory prefix holding all pointers
	hash       uint32
	_unused    uint8
	align      uint8
	fieldalign uint8
	kind       uint8
	alg        *typeAlg
	// gc stores type info required for garbage collector.
	// If (kind&KindGCProg)==0, then gc[0] points at sparse GC bitmap
	// (no indirection), 4 bits per word.
	// If (kind&KindGCProg)!=0, then gc[1] points to a compiler-generated
	// read-only GC program; and gc[0] points to BSS space for sparse GC bitmap.
	// For huge types (>maxGCMask), runtime unrolls the program directly into
	// GC bitmap and gc[0] is not used. For moderately-sized types, runtime
	// unrolls the program into gc[0] space on first use. The first byte of gc[0]
	// (gc[0][0]) contains 'unroll' flag saying whether the program is already
	// unrolled into gc[0] or not.
	gc      [2]uintptr
	_string *string
	x       *uncommontype
	ptrto   *_type
	zero    *byte // ptr to the zero value for this type
}

type method struct {
	name    *string
	pkgpath *string
	mtyp    *_type
	typ     *_type
	ifn     unsafe.Pointer
	tfn     unsafe.Pointer
}

type uncommontype struct {
	name    *string
	pkgpath *string
	mhdr    []method
}

type imethod struct {
	name    *string
	pkgpath *string
	_type   *_type
}

type interfacetype struct {
	typ  _type
	mhdr []imethod
}

type maptype struct {
	typ           _type
	key           *_type
	elem          *_type
	bucket        *_type // internal type representing a hash bucket
	hmap          *_type // internal type representing a hmap
	keysize       uint8  // size of key slot
	indirectkey   bool   // store ptr to key instead of key itself
	valuesize     uint8  // size of value slot
	indirectvalue bool   // store ptr to value instead of value itself
	bucketsize    uint16 // size of bucket
	reflexivekey  bool   // true if k==k for all keys
}

type chantype struct {
	typ  _type
	elem *_type
	dir  uintptr
}

type slicetype struct {
	typ  _type
	elem *_type
}

type functype struct {
	typ       _type
	dotdotdot bool
	in        slice
	out       slice
}

type ptrtype struct {
	typ  _type
	elem *_type
}
