// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reflect implements run-time reflection, allowing a program to
// manipulate objects with arbitrary types. The typical use is to take a value
// with static type interface{} and extract its dynamic type information by
// calling TypeOf, which returns a Type.
//
// A call to ValueOf returns a Value representing the run-time data.
// Zero takes a Type and returns a Value representing a zero value
// for that type.
//
// See "The Laws of Reflection" for an introduction to reflection in Go:
// https://golang.org/doc/articles/laws_of_reflection.html
package reflect

import (
	"runtime"
	"strconv"
	"sync"
	"unsafe"
)

// Type is the representation of a Go type.
//
// Not all methods apply to all kinds of types. Restrictions,
// if any, are noted in the documentation for each method.
// Use the Kind method to find out the kind of type before
// calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run-time panic.
type Type interface {
	// Methods applicable to all types.

	// Align returns the alignment in bytes of a value of
	// this type when allocated in memory.
	Align() int

	// FieldAlign returns the alignment in bytes of a value of
	// this type when used as a field in a struct.
	FieldAlign() int

	// Method returns the i'th method in the type's method set.
	// It panics if i is not in the range [0, NumMethod()).
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	Method(int) Method

	// MethodByName returns the method with that name in the type's
	// method set and a boolean indicating if the method was found.
	//
	// For a non-interface type T or *T, the returned Method's Type and Func
	// fields describe a function whose first argument is the receiver.
	//
	// For an interface type, the returned Method's Type field gives the
	// method signature, without a receiver, and the Func field is nil.
	MethodByName(string) (Method, bool)

	// NumMethod returns the number of methods in the type's method set.
	NumMethod() int

	// Name returns the type's name within its package.
	// It returns an empty string for unnamed types.
	Name() string

	// PkgPath returns a named type's package path, that is, the import path
	// that uniquely identifies the package, such as "encoding/base64".
	// If the type was predeclared (string, error) or unnamed (*T, struct{}, []int),
	// the package path will be the empty string.
	PkgPath() string

	// Size returns the number of bytes needed to store
	// a value of the given type; it is analogous to unsafe.Sizeof.
	Size() uintptr

	// String returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., base64 instead of "encoding/base64") and is not
	// guaranteed to be unique among types. To test for equality,
	// compare the Types directly.
	String() string

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	AssignableTo(u Type) bool

	// ConvertibleTo reports whether a value of the type is convertible to type u.
	ConvertibleTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	Comparable() bool

	// Methods applicable only to some types, depending on Kind.
	// The methods allowed for each kind are:
	//
	//	Int*, Uint*, Float*, Complex*: Bits
	//	Array: Elem, Len
	//	Chan: ChanDir, Elem
	//	Func: In, NumIn, Out, NumOut, IsVariadic.
	//	Map: Key, Elem
	//	Ptr: Elem
	//	Slice: Elem
	//	Struct: Field, FieldByIndex, FieldByName, FieldByNameFunc, NumField

	// Bits returns the size of the type in bits.
	// It panics if the type's Kind is not one of the
	// sized or unsized Int, Uint, Float, or Complex kinds.
	Bits() int

	// ChanDir returns a channel type's direction.
	// It panics if the type's Kind is not Chan.
	ChanDir() ChanDir

	// IsVariadic reports whether a function type's final input parameter
	// is a "..." parameter. If so, t.In(t.NumIn() - 1) returns the parameter's
	// implicit actual type []T.
	//
	// For concreteness, if t represents func(x int, y ... float64), then
	//
	//	t.NumIn() == 2
	//	t.In(0) is the reflect.Type for "int"
	//	t.In(1) is the reflect.Type for "[]float64"
	//	t.IsVariadic() == true
	//
	// IsVariadic panics if the type's Kind is not Func.
	IsVariadic() bool

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Ptr, or Slice.
	Elem() Type

	// Field returns a struct type's i'th field.
	// It panics if the type's Kind is not Struct.
	// It panics if i is not in the range [0, NumField()).
	Field(i int) StructField

	// FieldByIndex returns the nested field corresponding
	// to the index sequence. It is equivalent to calling Field
	// successively for each index i.
	// It panics if the type's Kind is not Struct.
	FieldByIndex(index []int) StructField

	// FieldByName returns the struct field with the given name
	// and a boolean indicating if the field was found.
	FieldByName(name string) (StructField, bool)

	// FieldByNameFunc returns the first struct field with a name
	// that satisfies the match function and a boolean indicating if
	// the field was found.
	FieldByNameFunc(match func(string) bool) (StructField, bool)

	// In returns the type of a function type's i'th input parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumIn()).
	In(i int) Type

	// Key returns a map type's key type.
	// It panics if the type's Kind is not Map.
	Key() Type

	// Len returns an array type's length.
	// It panics if the type's Kind is not Array.
	Len() int

	// NumField returns a struct type's field count.
	// It panics if the type's Kind is not Struct.
	NumField() int

	// NumIn returns a function type's input parameter count.
	// It panics if the type's Kind is not Func.
	NumIn() int

	// NumOut returns a function type's output parameter count.
	// It panics if the type's Kind is not Func.
	NumOut() int

	// Out returns the type of a function type's i'th output parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumOut()).
	Out(i int) Type

	common() *rtype
	uncommon() *uncommonType
}

// BUG(rsc): FieldByName and related functions consider struct field names to be equal
// if the names are equal, even if they are unexported names originating
// in different packages. The practical effect of this is that the result of
// t.FieldByName("x") is not well defined if the struct type t contains
// multiple fields named x (embedded from different packages).
// FieldByName may return one of the fields named x or may report that there are none.
// See golang.org/issue/4876 for more details.

/*
 * These data structures are known to the compiler (../../cmd/internal/gc/reflect.go).
 * A few are known to ../runtime/type.go to convey to debuggers.
 * They are also known to ../runtime/type.go.
 */

// A Kind represents the specific kind of type that a Type represents.
// The zero Kind is not a valid kind.
type Kind uint

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Ptr
	Slice
	String
	Struct
	UnsafePointer
)

// tflag is used by an rtype to signal what extra type information is
// available in the memory directly following the rtype value.
//
// tflag values must be kept in sync with copies in:
//	cmd/compile/internal/gc/reflect.go
//	cmd/link/internal/ld/decodesym.go
//	runtime/type.go
type tflag uint8

const (
	// tflagUncommon means that there is a pointer, *uncommonType,
	// just beyond the outer type structure.
	//
	// For example, if t.Kind() == Struct and t.tflag&tflagUncommon != 0,
	// then t has uncommonType data and it can be accessed as:
	//
	//	type tUncommon struct {
	//		structType
	//		u uncommonType
	//	}
	//	u := &(*tUncommon)(unsafe.Pointer(t)).u
	tflagUncommon tflag = 1 << 0

	// tflagExtraStar means the name in the str field has an
	// extraneous '*' prefix. This is because for most types T in
	// a program, the type *T also exists and reusing the str data
	// saves binary size.
	tflagExtraStar tflag = 1 << 1
)

// rtype is the common implementation of most values.
// It is embedded in other, public struct types, but always
// with a unique tag like `reflect:"array"` or `reflect:"ptr"`
// so that code cannot convert from, say, *arrayType to *ptrType.
type rtype struct {
	size       uintptr
	ptrdata    uintptr
	hash       uint32   // hash of type; avoids computation in hash tables
	tflag      tflag    // extra type information flags
	align      uint8    // alignment of variable with this type
	fieldAlign uint8    // alignment of struct field with this type
	kind       uint8    // enumeration for C
	alg        *typeAlg // algorithm table
	gcdata     *byte    // garbage collection data
	str        nameOff  // string form
	_          int32    // unused; keeps rtype always a multiple of ptrSize
}

// a copy of runtime.typeAlg
type typeAlg struct {
	// function for hashing objects of this type
	// (ptr to object, seed) -> hash
	hash func(unsafe.Pointer, uintptr) uintptr
	// function for comparing objects of this type
	// (ptr to object A, ptr to object B) -> ==?
	equal func(unsafe.Pointer, unsafe.Pointer) bool
}

// Method on non-interface type
type method struct {
	name nameOff // name of method
	mtyp typeOff // method type (without receiver)
	ifn  textOff // fn used in interface call (one-word receiver)
	tfn  textOff // fn used for normal method call
}

// uncommonType is present only for types with names or methods
// (if T is a named type, the uncommonTypes for T and *T have methods).
// Using a pointer to this struct reduces the overall size required
// to describe an unnamed type with no methods.
type uncommonType struct {
	pkgPath nameOff // import path; empty for built-in types like int, string
	mcount  uint16  // number of methods
	moff    uint16  // offset from this uncommontype to [mcount]method
}

// ChanDir represents a channel type's direction.
type ChanDir int

const (
	RecvDir ChanDir             = 1 << iota // <-chan
	SendDir                                 // chan<-
	BothDir = RecvDir | SendDir             // chan
)

// arrayType represents a fixed array type.
type arrayType struct {
	rtype `reflect:"array"`
	elem  *rtype // array element type
	slice *rtype // slice type
	len   uintptr
}

// chanType represents a channel type.
type chanType struct {
	rtype `reflect:"chan"`
	elem  *rtype  // channel element type
	dir   uintptr // channel direction (ChanDir)
}

// funcType represents a function type.
//
// A *rtype for each in and out parameter is stored in an array that
// directly follows the funcType (and possibly its uncommonType). So
// a function type with one method, one input, and one output is:
//
//	struct {
//		funcType
//		uncommonType
//		[2]*rtype    // [0] is in, [1] is out
//	}
type funcType struct {
	rtype    `reflect:"func"`
	inCount  uint16
	outCount uint16 // top bit is set if last input parameter is ...
}

// imethod represents a method on an interface type
type imethod struct {
	name nameOff // name of method
	typ  typeOff // .(*FuncType) underneath
}

// interfaceType represents an interface type.
type interfaceType struct {
	rtype   `reflect:"interface"`
	pkgPath name      // import path
	methods []imethod // sorted by hash
}

// mapType represents a map type.
type mapType struct {
	rtype         `reflect:"map"`
	key           *rtype // map key type
	elem          *rtype // map element (value) type
	bucket        *rtype // internal bucket structure
	hmap          *rtype // internal map header
	keysize       uint8  // size of key slot
	indirectkey   uint8  // store ptr to key instead of key itself
	valuesize     uint8  // size of value slot
	indirectvalue uint8  // store ptr to value instead of value itself
	bucketsize    uint16 // size of bucket
	reflexivekey  bool   // true if k==k for all keys
	needkeyupdate bool   // true if we need to update key on an overwrite
}

// ptrType represents a pointer type.
type ptrType struct {
	rtype `reflect:"ptr"`
	elem  *rtype // pointer element (pointed at) type
}

// sliceType represents a slice type.
type sliceType struct {
	rtype `reflect:"slice"`
	elem  *rtype // slice element type
}

// Struct field
type structField struct {
	name   name    // name is empty for embedded fields
	typ    *rtype  // type of field
	offset uintptr // byte offset of field within struct
}

// structType represents a struct type.
type structType struct {
	rtype   `reflect:"struct"`
	pkgPath name
	fields  []structField // sorted by offset
}

// name is an encoded type name with optional extra data.
//
// The first byte is a bit field containing:
//
//	1<<0 the name is exported
//	1<<1 tag data follows the name
//	1<<2 pkgPath nameOff follows the name and tag
//
// The next two bytes are the data length:
//
//	 l := uint16(data[1])<<8 | uint16(data[2])
//
// Bytes [3:3+l] are the string data.
//
// If tag data follows then bytes 3+l and 3+l+1 are the tag length,
// with the data following.
//
// If the import path follows, then 4 bytes at the end of
// the data form a nameOff. The import path is only set for concrete
// methods that are defined in a different package than their type.
//
// If a name starts with "*", then the exported bit represents
// whether the pointed to type is exported.
type name struct {
	bytes *byte
}

func (n name) data(off int) *byte {
	return (*byte)(add(unsafe.Pointer(n.bytes), uintptr(off)))
}

func (n name) isExported() bool {
	return (*n.bytes)&(1<<0) != 0
}

func (n name) nameLen() int {
	return int(uint16(*n.data(1))<<8 | uint16(*n.data(2)))
}

func (n name) tagLen() int {
	if *n.data(0)&(1<<1) == 0 {
		return 0
	}
	off := 3 + n.nameLen()
	return int(uint16(*n.data(off))<<8 | uint16(*n.data(off + 1)))
}

func (n name) name() (s string) {
	if n.bytes == nil {
		return ""
	}
	nl := n.nameLen()
	if nl == 0 {
		return ""
	}
	hdr := (*stringHeader)(unsafe.Pointer(&s))
	hdr.Data = unsafe.Pointer(n.data(3))
	hdr.Len = nl
	return s
}

func (n name) tag() (s string) {
	tl := n.tagLen()
	if tl == 0 {
		return ""
	}
	nl := n.nameLen()
	hdr := (*stringHeader)(unsafe.Pointer(&s))
	hdr.Data = unsafe.Pointer(n.data(3 + nl + 2))
	hdr.Len = tl
	return s
}

func (n name) pkgPath() string {
	if n.bytes == nil || *n.data(0)&(1<<2) == 0 {
		return ""
	}
	off := 3 + n.nameLen()
	if tl := n.tagLen(); tl > 0 {
		off += 2 + tl
	}
	var nameOff int32
	copy((*[4]byte)(unsafe.Pointer(&nameOff))[:], (*[4]byte)(unsafe.Pointer(n.data(off)))[:])
	pkgPathName := name{(*byte)(resolveTypeOff(unsafe.Pointer(n.bytes), nameOff))}
	return pkgPathName.name()
}

// round n up to a multiple of a.  a must be a power of 2.
func round(n, a uintptr) uintptr {
	return (n + a - 1) &^ (a - 1)
}

func newName(n, tag, pkgPath string, exported bool) name {
	if len(n) > 1<<16-1 {
		panic("reflect.nameFrom: name too long: " + n)
	}
	if len(tag) > 1<<16-1 {
		panic("reflect.nameFrom: tag too long: " + tag)
	}

	var bits byte
	l := 1 + 2 + len(n)
	if exported {
		bits |= 1 << 0
	}
	if len(tag) > 0 {
		l += 2 + len(tag)
		bits |= 1 << 1
	}
	if pkgPath != "" {
		bits |= 1 << 2
	}

	b := make([]byte, l)
	b[0] = bits
	b[1] = uint8(len(n) >> 8)
	b[2] = uint8(len(n))
	copy(b[3:], n)
	if len(tag) > 0 {
		tb := b[3+len(n):]
		tb[0] = uint8(len(tag) >> 8)
		tb[1] = uint8(len(tag))
		copy(tb[2:], tag)
	}

	if pkgPath != "" {
		panic("reflect: creating a name with a package path is not supported")
	}

	return name{bytes: &b[0]}
}

/*
 * The compiler knows the exact layout of all the data structures above.
 * The compiler does not know about the data structures and methods below.
 */

// Method represents a single method.
type Method struct {
	// Name is the method name.
	// PkgPath is the package path that qualifies a lower case (unexported)
	// method name. It is empty for upper case (exported) method names.
	// The combination of PkgPath and Name uniquely identifies a method
	// in a method set.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	Name    string
	PkgPath string

	Type  Type  // method type
	Func  Value // func with receiver as first argument
	Index int   // index for Type.Method
}

const (
	kindDirectIface = 1 << 5
	kindGCProg      = 1 << 6 // Type.gc points to GC program
	kindNoPointers  = 1 << 7
	kindMask        = (1 << 5) - 1
)

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "kind" + strconv.Itoa(int(k))
}

var kindNames = []string{
	Invalid:       "invalid",
	Bool:          "bool",
	Int:           "int",
	Int8:          "int8",
	Int16:         "int16",
	Int32:         "int32",
	Int64:         "int64",
	Uint:          "uint",
	Uint8:         "uint8",
	Uint16:        "uint16",
	Uint32:        "uint32",
	Uint64:        "uint64",
	Uintptr:       "uintptr",
	Float32:       "float32",
	Float64:       "float64",
	Complex64:     "complex64",
	Complex128:    "complex128",
	Array:         "array",
	Chan:          "chan",
	Func:          "func",
	Interface:     "interface",
	Map:           "map",
	Ptr:           "ptr",
	Slice:         "slice",
	String:        "string",
	Struct:        "struct",
	UnsafePointer: "unsafe.Pointer",
}

func (t *uncommonType) methods() []method {
	return (*[1 << 16]method)(add(unsafe.Pointer(t), uintptr(t.moff)))[:t.mcount:t.mcount]
}

// resolveNameOff resolves a name offset from a base pointer.
// The (*rtype).nameOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
func resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer

// resolveTypeOff resolves an *rtype offset from a base type.
// The (*rtype).typeOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
func resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// resolveTextOff resolves an function pointer offset from a base type.
// The (*rtype).textOff method is a convenience wrapper for this function.
// Implemented in the runtime package.
func resolveTextOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

// addReflectOff adds a pointer to the reflection lookup map in the runtime.
// It returns a new ID that can be used as a typeOff or textOff, and will
// be resolved correctly. Implemented in the runtime package.
func addReflectOff(ptr unsafe.Pointer) int32

// resolveReflectType adds a name to the reflection lookup map in the runtime.
// It returns a new nameOff that can be used to refer to the pointer.
func resolveReflectName(n name) nameOff {
	return nameOff(addReflectOff(unsafe.Pointer(n.bytes)))
}

// resolveReflectType adds a *rtype to the reflection lookup map in the runtime.
// It returns a new typeOff that can be used to refer to the pointer.
func resolveReflectType(t *rtype) typeOff {
	return typeOff(addReflectOff(unsafe.Pointer(t)))
}

// resolveReflectText adds a function pointer to the reflection lookup map in
// the runtime. It returns a new textOff that can be used to refer to the
// pointer.
func resolveReflectText(ptr unsafe.Pointer) textOff {
	return textOff(addReflectOff(ptr))
}

type nameOff int32 // offset to a name
type typeOff int32 // offset to an *rtype
type textOff int32 // offset from top of text section

func (t *rtype) nameOff(off nameOff) name {
	if off == 0 {
		return name{}
	}
	return name{(*byte)(resolveNameOff(unsafe.Pointer(t), int32(off)))}
}

func (t *rtype) typeOff(off typeOff) *rtype {
	if off == 0 {
		return nil
	}
	return (*rtype)(resolveTypeOff(unsafe.Pointer(t), int32(off)))
}

func (t *rtype) textOff(off textOff) unsafe.Pointer {
	return resolveTextOff(unsafe.Pointer(t), int32(off))
}

func (t *rtype) uncommon() *uncommonType {
	if t.tflag&tflagUncommon == 0 {
		return nil
	}
	switch t.Kind() {
	case Struct:
		return &(*structTypeUncommon)(unsafe.Pointer(t)).u
	case Ptr:
		type u struct {
			ptrType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Func:
		type u struct {
			funcType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Slice:
		type u struct {
			sliceType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Array:
		type u struct {
			arrayType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Chan:
		type u struct {
			chanType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Map:
		type u struct {
			mapType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	case Interface:
		type u struct {
			interfaceType
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	default:
		type u struct {
			rtype
			u uncommonType
		}
		return &(*u)(unsafe.Pointer(t)).u
	}
}

func (t *rtype) String() string {
	s := t.nameOff(t.str).name()
	if t.tflag&tflagExtraStar != 0 {
		return s[1:]
	}
	return s
}

func (t *rtype) Size() uintptr { return t.size }

func (t *rtype) Bits() int {
	if t == nil {
		panic("reflect: Bits of nil Type")
	}
	k := t.Kind()
	if k < Int || k > Complex128 {
		panic("reflect: Bits of non-arithmetic Type " + t.String())
	}
	return int(t.size) * 8
}

func (t *rtype) Align() int { return int(t.align) }

func (t *rtype) FieldAlign() int { return int(t.fieldAlign) }

func (t *rtype) Kind() Kind { return Kind(t.kind & kindMask) }

func (t *rtype) pointers() bool { return t.kind&kindNoPointers == 0 }

func (t *rtype) common() *rtype { return t }

var methodCache struct {
	sync.RWMutex
	m map[*rtype][]method
}

func (t *rtype) exportedMethods() []method {
	methodCache.RLock()
	methods, found := methodCache.m[t]
	methodCache.RUnlock()

	if found {
		return methods
	}

	ut := t.uncommon()
	if ut == nil {
		return nil
	}
	allm := ut.methods()
	allExported := true
	for _, m := range allm {
		name := t.nameOff(m.name)
		if !name.isExported() {
			allExported = false
			break
		}
	}
	if allExported {
		methods = allm
	} else {
		methods = make([]method, 0, len(allm))
		for _, m := range allm {
			name := t.nameOff(m.name)
			if name.isExported() {
				methods = append(methods, m)
			}
		}
		methods = methods[:len(methods):len(methods)]
	}

	methodCache.Lock()
	if methodCache.m == nil {
		methodCache.m = make(map[*rtype][]method)
	}
	methodCache.m[t] = methods
	methodCache.Unlock()

	return methods
}

func (t *rtype) NumMethod() int {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.NumMethod()
	}
	return len(t.exportedMethods())
}

func (t *rtype) Method(i int) (m Method) {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.Method(i)
	}
	methods := t.exportedMethods()
	if i < 0 || i >= len(methods) {
		panic("reflect: Method index out of range")
	}
	p := methods[i]
	pname := t.nameOff(p.name)
	m.Name = pname.name()
	fl := flag(Func)
	mtyp := t.typeOff(p.mtyp)
	ft := (*funcType)(unsafe.Pointer(mtyp))
	in := make([]Type, 0, 1+len(ft.in()))
	in = append(in, t)
	for _, arg := range ft.in() {
		in = append(in, arg)
	}
	out := make([]Type, 0, len(ft.out()))
	for _, ret := range ft.out() {
		out = append(out, ret)
	}
	mt := FuncOf(in, out, ft.IsVariadic())
	m.Type = mt
	tfn := t.textOff(p.tfn)
	fn := unsafe.Pointer(&tfn)
	m.Func = Value{mt.(*rtype), fn, fl}

	m.Index = i
	return m
}

func (t *rtype) MethodByName(name string) (m Method, ok bool) {
	if t.Kind() == Interface {
		tt := (*interfaceType)(unsafe.Pointer(t))
		return tt.MethodByName(name)
	}
	ut := t.uncommon()
	if ut == nil {
		return Method{}, false
	}
	utmethods := ut.methods()
	for i := 0; i < int(ut.mcount); i++ {
		p := utmethods[i]
		pname := t.nameOff(p.name)
		if pname.isExported() && pname.name() == name {
			return t.Method(i), true
		}
	}
	return Method{}, false
}

func (t *rtype) PkgPath() string {
	ut := t.uncommon()
	if ut == nil {
		return ""
	}
	return t.nameOff(ut.pkgPath).name()
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func (t *rtype) Name() string {
	s := t.String()
	if hasPrefix(s, "map[") {
		return ""
	}
	if hasPrefix(s, "struct {") {
		return ""
	}
	if hasPrefix(s, "chan ") {
		return ""
	}
	if hasPrefix(s, "chan<-") {
		return ""
	}
	if hasPrefix(s, "func(") {
		return ""
	}
	if hasPrefix(s, "interface {") {
		return ""
	}
	switch s[0] {
	case '[', '*', '<':
		return ""
	}
	i := len(s) - 1
	for i >= 0 {
		if s[i] == '.' {
			break
		}
		i--
	}
	return s[i+1:]
}

func (t *rtype) ChanDir() ChanDir {
	if t.Kind() != Chan {
		panic("reflect: ChanDir of non-chan type")
	}
	tt := (*chanType)(unsafe.Pointer(t))
	return ChanDir(tt.dir)
}

func (t *rtype) IsVariadic() bool {
	if t.Kind() != Func {
		panic("reflect: IsVariadic of non-func type")
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return tt.outCount&(1<<15) != 0
}

func (t *rtype) Elem() Type {
	switch t.Kind() {
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Chan:
		tt := (*chanType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Map:
		tt := (*mapType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Ptr:
		tt := (*ptrType)(unsafe.Pointer(t))
		return toType(tt.elem)
	case Slice:
		tt := (*sliceType)(unsafe.Pointer(t))
		return toType(tt.elem)
	}
	panic("reflect: Elem of invalid type")
}

func (t *rtype) Field(i int) StructField {
	if t.Kind() != Struct {
		panic("reflect: Field of non-struct type")
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.Field(i)
}

func (t *rtype) FieldByIndex(index []int) StructField {
	if t.Kind() != Struct {
		panic("reflect: FieldByIndex of non-struct type")
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByIndex(index)
}

func (t *rtype) FieldByName(name string) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByName of non-struct type")
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByName(name)
}

func (t *rtype) FieldByNameFunc(match func(string) bool) (StructField, bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByNameFunc of non-struct type")
	}
	tt := (*structType)(unsafe.Pointer(t))
	return tt.FieldByNameFunc(match)
}

func (t *rtype) In(i int) Type {
	if t.Kind() != Func {
		panic("reflect: In of non-func type")
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return toType(tt.in()[i])
}

func (t *rtype) Key() Type {
	if t.Kind() != Map {
		panic("reflect: Key of non-map type")
	}
	tt := (*mapType)(unsafe.Pointer(t))
	return toType(tt.key)
}

func (t *rtype) Len() int {
	if t.Kind() != Array {
		panic("reflect: Len of non-array type")
	}
	tt := (*arrayType)(unsafe.Pointer(t))
	return int(tt.len)
}

func (t *rtype) NumField() int {
	if t.Kind() != Struct {
		panic("reflect: NumField of non-struct type")
	}
	tt := (*structType)(unsafe.Pointer(t))
	return len(tt.fields)
}

func (t *rtype) NumIn() int {
	if t.Kind() != Func {
		panic("reflect: NumIn of non-func type")
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return int(tt.inCount)
}

func (t *rtype) NumOut() int {
	if t.Kind() != Func {
		panic("reflect: NumOut of non-func type")
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return len(tt.out())
}

func (t *rtype) Out(i int) Type {
	if t.Kind() != Func {
		panic("reflect: Out of non-func type")
	}
	tt := (*funcType)(unsafe.Pointer(t))
	return toType(tt.out()[i])
}

func (t *funcType) in() []*rtype {
	uadd := unsafe.Sizeof(*t)
	if t.tflag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(uncommonType{})
	}
	return (*[1 << 20]*rtype)(add(unsafe.Pointer(t), uadd))[:t.inCount]
}

func (t *funcType) out() []*rtype {
	uadd := unsafe.Sizeof(*t)
	if t.tflag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(uncommonType{})
	}
	outCount := t.outCount & (1<<15 - 1)
	return (*[1 << 20]*rtype)(add(unsafe.Pointer(t), uadd))[t.inCount : t.inCount+outCount]
}

func add(p unsafe.Pointer, x uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

func (d ChanDir) String() string {
	switch d {
	case SendDir:
		return "chan<-"
	case RecvDir:
		return "<-chan"
	case BothDir:
		return "chan"
	}
	return "ChanDir" + strconv.Itoa(int(d))
}

// Method returns the i'th method in the type's method set.
func (t *interfaceType) Method(i int) (m Method) {
	if i < 0 || i >= len(t.methods) {
		return
	}
	p := &t.methods[i]
	pname := t.nameOff(p.name)
	m.Name = pname.name()
	if !pname.isExported() {
		m.PkgPath = pname.pkgPath()
		if m.PkgPath == "" {
			m.PkgPath = t.pkgPath.name()
		}
	}
	m.Type = toType(t.typeOff(p.typ))
	m.Index = i
	return
}

// NumMethod returns the number of interface methods in the type's method set.
func (t *interfaceType) NumMethod() int { return len(t.methods) }

// MethodByName method with the given name in the type's method set.
func (t *interfaceType) MethodByName(name string) (m Method, ok bool) {
	if t == nil {
		return
	}
	var p *imethod
	for i := range t.methods {
		p = &t.methods[i]
		if t.nameOff(p.name).name() == name {
			return t.Method(i), true
		}
	}
	return
}

// A StructField describes a single field in a struct.
type StructField struct {
	// Name is the field name.
	Name string
	// PkgPath is the package path that qualifies a lower case (unexported)
	// field name. It is empty for upper case (exported) field names.
	// See https://golang.org/ref/spec#Uniqueness_of_identifiers
	PkgPath string

	Type      Type      // field type
	Tag       StructTag // field tag string
	Offset    uintptr   // offset within struct, in bytes
	Index     []int     // index sequence for Type.FieldByIndex
	Anonymous bool      // is an embedded field
}

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of
// optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control
// characters other than space (U+0020 ' '), quote (U+0022 '"'),
// and colon (U+003A ':').  Each value is quoted using U+0022 '"'
// characters and Go string literal syntax.
type StructTag string

// Get returns the value associated with key in the tag string.
// If there is no such key in the tag, Get returns the empty string.
// If the tag does not have the conventional format, the value
// returned by Get is unspecified. To determine whether a tag is
// explicitly set to the empty string, use Lookup.
func (tag StructTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value associated with key in the tag string.
// If the key is present in the tag the value (which may be empty)
// is returned. Otherwise the returned value will be the empty string.
// The ok return value reports whether the value was explicitly set in
// the tag string. If the tag does not have the conventional format,
// the value returned by Lookup is unspecified.
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	// When modifying this code, also update the validateStructTag code
	// in golang.org/x/tools/cmd/vet/structtag.go.

	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		// Strictly speaking, control chars include the range [0x7f, 0x9f], not just
		// [0x00, 0x1f], but in practice, we ignore the multi-byte control characters
		// as it is simpler to inspect the tag's bytes than the tag's runes.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]

		if key == name {
			value, err := strconv.Unquote(qvalue)
			if err != nil {
				break
			}
			return value, true
		}
	}
	return "", false
}

// Field returns the i'th struct field.
func (t *structType) Field(i int) (f StructField) {
	if i < 0 || i >= len(t.fields) {
		panic("reflect: Field index out of bounds")
	}
	p := &t.fields[i]
	f.Type = toType(p.typ)
	if name := p.name.name(); name != "" {
		f.Name = name
	} else {
		t := f.Type
		if t.Kind() == Ptr {
			t = t.Elem()
		}
		f.Name = t.Name()
		f.Anonymous = true
	}
	if !p.name.isExported() {
		// Fields never have an import path in their name.
		f.PkgPath = t.pkgPath.name()
	}
	if tag := p.name.tag(); tag != "" {
		f.Tag = StructTag(tag)
	}
	f.Offset = p.offset

	// NOTE(rsc): This is the only allocation in the interface
	// presented by a reflect.Type. It would be nice to avoid,
	// at least in the common cases, but we need to make sure
	// that misbehaving clients of reflect cannot affect other
	// uses of reflect. One possibility is CL 5371098, but we
	// postponed that ugliness until there is a demonstrated
	// need for the performance. This is issue 2320.
	f.Index = []int{i}
	return
}

// TODO(gri): Should there be an error/bool indicator if the index
//            is wrong for FieldByIndex?

// FieldByIndex returns the nested field corresponding to index.
func (t *structType) FieldByIndex(index []int) (f StructField) {
	f.Type = toType(&t.rtype)
	for i, x := range index {
		if i > 0 {
			ft := f.Type
			if ft.Kind() == Ptr && ft.Elem().Kind() == Struct {
				ft = ft.Elem()
			}
			f.Type = ft
		}
		f = f.Type.Field(x)
	}
	return
}

// A fieldScan represents an item on the fieldByNameFunc scan work list.
type fieldScan struct {
	typ   *structType
	index []int
}

// FieldByNameFunc returns the struct field with a name that satisfies the
// match function and a boolean to indicate if the field was found.
func (t *structType) FieldByNameFunc(match func(string) bool) (result StructField, ok bool) {
	// This uses the same condition that the Go language does: there must be a unique instance
	// of the match at a given depth level. If there are multiple instances of a match at the
	// same depth, they annihilate each other and inhibit any possible match at a lower level.
	// The algorithm is breadth first search, one depth level at a time.

	// The current and next slices are work queues:
	// current lists the fields to visit on this depth level,
	// and next lists the fields on the next lower level.
	current := []fieldScan{}
	next := []fieldScan{{typ: t}}

	// nextCount records the number of times an embedded type has been
	// encountered and considered for queueing in the 'next' slice.
	// We only queue the first one, but we increment the count on each.
	// If a struct type T can be reached more than once at a given depth level,
	// then it annihilates itself and need not be considered at all when we
	// process that next depth level.
	var nextCount map[*structType]int

	// visited records the structs that have been considered already.
	// Embedded pointer fields can create cycles in the graph of
	// reachable embedded types; visited avoids following those cycles.
	// It also avoids duplicated effort: if we didn't find the field in an
	// embedded type T at level 2, we won't find it in one at level 4 either.
	visited := map[*structType]bool{}

	for len(next) > 0 {
		current, next = next, current[:0]
		count := nextCount
		nextCount = nil

		// Process all the fields at this depth, now listed in 'current'.
		// The loop queues embedded fields found in 'next', for processing during the next
		// iteration. The multiplicity of the 'current' field counts is recorded
		// in 'count'; the multiplicity of the 'next' field counts is recorded in 'nextCount'.
		for _, scan := range current {
			t := scan.typ
			if visited[t] {
				// We've looked through this type before, at a higher level.
				// That higher level would shadow the lower level we're now at,
				// so this one can't be useful to us. Ignore it.
				continue
			}
			visited[t] = true
			for i := range t.fields {
				f := &t.fields[i]
				// Find name and type for field f.
				var fname string
				var ntyp *rtype
				if name := f.name.name(); name != "" {
					fname = name
				} else {
					// Anonymous field of type T or *T.
					// Name taken from type.
					ntyp = f.typ
					if ntyp.Kind() == Ptr {
						ntyp = ntyp.Elem().common()
					}
					fname = ntyp.Name()
				}

				// Does it match?
				if match(fname) {
					// Potential match
					if count[t] > 1 || ok {
						// Name appeared multiple times at this level: annihilate.
						return StructField{}, false
					}
					result = t.Field(i)
					result.Index = nil
					result.Index = append(result.Index, scan.index...)
					result.Index = append(result.Index, i)
					ok = true
					continue
				}

				// Queue embedded struct fields for processing with next level,
				// but only if we haven't seen a match yet at this level and only
				// if the embedded types haven't already been queued.
				if ok || ntyp == nil || ntyp.Kind() != Struct {
					continue
				}
				styp := (*structType)(unsafe.Pointer(ntyp))
				if nextCount[styp] > 0 {
					nextCount[styp] = 2 // exact multiple doesn't matter
					continue
				}
				if nextCount == nil {
					nextCount = map[*structType]int{}
				}
				nextCount[styp] = 1
				if count[t] > 1 {
					nextCount[styp] = 2 // exact multiple doesn't matter
				}
				var index []int
				index = append(index, scan.index...)
				index = append(index, i)
				next = append(next, fieldScan{styp, index})
			}
		}
		if ok {
			break
		}
	}
	return
}

// FieldByName returns the struct field with the given name
// and a boolean to indicate if the field was found.
func (t *structType) FieldByName(name string) (f StructField, present bool) {
	// Quick check for top-level name, or struct without anonymous fields.
	hasAnon := false
	if name != "" {
		for i := range t.fields {
			tf := &t.fields[i]
			tfname := tf.name.name()
			if tfname == "" {
				hasAnon = true
				continue
			}
			if tfname == name {
				return t.Field(i), true
			}
		}
	}
	if !hasAnon {
		return
	}
	return t.FieldByNameFunc(func(s string) bool { return s == name })
}

// TypeOf returns the reflection Type that represents the dynamic type of i.
// If i is a nil interface value, TypeOf returns nil.
func TypeOf(i interface{}) Type {
	eface := *(*emptyInterface)(unsafe.Pointer(&i))
	return toType(eface.typ)
}

// ptrMap is the cache for PtrTo.
var ptrMap struct {
	sync.RWMutex
	m map[*rtype]*ptrType
}

// PtrTo returns the pointer type with element t.
// For example, if t represents type Foo, PtrTo(t) represents *Foo.
func PtrTo(t Type) Type {
	return t.(*rtype).ptrTo()
}

func (t *rtype) ptrTo() *rtype {
	// Check the cache.
	ptrMap.RLock()
	if m := ptrMap.m; m != nil {
		if p := m[t]; p != nil {
			ptrMap.RUnlock()
			return &p.rtype
		}
	}
	ptrMap.RUnlock()

	ptrMap.Lock()
	if ptrMap.m == nil {
		ptrMap.m = make(map[*rtype]*ptrType)
	}
	p := ptrMap.m[t]
	if p != nil {
		// some other goroutine won the race and created it
		ptrMap.Unlock()
		return &p.rtype
	}

	// Look in known types.
	s := "*" + t.String()
	for _, tt := range typesByString(s) {
		p = (*ptrType)(unsafe.Pointer(tt))
		if p.elem == t {
			ptrMap.m[t] = p
			ptrMap.Unlock()
			return &p.rtype
		}
	}

	// Create a new ptrType starting with the description
	// of an *unsafe.Pointer.
	p = new(ptrType)
	var iptr interface{} = (*unsafe.Pointer)(nil)
	prototype := *(**ptrType)(unsafe.Pointer(&iptr))
	*p = *prototype

	p.str = resolveReflectName(newName(s, "", "", false))

	// For the type structures linked into the binary, the
	// compiler provides a good hash of the string.
	// Create a good hash for the new string by using
	// the FNV-1 hash's mixing function to combine the
	// old hash and the new "*".
	p.hash = fnv1(t.hash, '*')

	p.elem = t

	ptrMap.m[t] = p
	ptrMap.Unlock()
	return &p.rtype
}

// fnv1 incorporates the list of bytes into the hash x using the FNV-1 hash function.
func fnv1(x uint32, list ...byte) uint32 {
	for _, b := range list {
		x = x*16777619 ^ uint32(b)
	}
	return x
}

func (t *rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return implements(u.(*rtype), t)
}

func (t *rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	uu := u.(*rtype)
	return directlyAssignable(uu, t) || implements(uu, t)
}

func (t *rtype) ConvertibleTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.ConvertibleTo")
	}
	uu := u.(*rtype)
	return convertOp(uu, t) != nil
}

func (t *rtype) Comparable() bool {
	return t.alg != nil && t.alg.equal != nil
}

// implements reports whether the type V implements the interface type T.
func implements(T, V *rtype) bool {
	if T.Kind() != Interface {
		return false
	}
	t := (*interfaceType)(unsafe.Pointer(T))
	if len(t.methods) == 0 {
		return true
	}

	// The same algorithm applies in both cases, but the
	// method tables for an interface type and a concrete type
	// are different, so the code is duplicated.
	// In both cases the algorithm is a linear scan over the two
	// lists - T's methods and V's methods - simultaneously.
	// Since method tables are stored in a unique sorted order
	// (alphabetical, with no duplicate method names), the scan
	// through V's methods must hit a match for each of T's
	// methods along the way, or else V does not implement T.
	// This lets us run the scan in overall linear time instead of
	// the quadratic time  a naive search would require.
	// See also ../runtime/iface.go.
	if V.Kind() == Interface {
		v := (*interfaceType)(unsafe.Pointer(V))
		i := 0
		for j := 0; j < len(v.methods); j++ {
			tm := &t.methods[i]
			vm := &v.methods[j]
			if V.nameOff(vm.name).name() == t.nameOff(tm.name).name() && V.typeOff(vm.typ) == t.typeOff(tm.typ) {
				if i++; i >= len(t.methods) {
					return true
				}
			}
		}
		return false
	}

	v := V.uncommon()
	if v == nil {
		return false
	}
	i := 0
	vmethods := v.methods()
	for j := 0; j < int(v.mcount); j++ {
		tm := &t.methods[i]
		vm := vmethods[j]
		if V.nameOff(vm.name).name() == t.nameOff(tm.name).name() && V.typeOff(vm.mtyp) == t.typeOff(tm.typ) {
			if i++; i >= len(t.methods) {
				return true
			}
		}
	}
	return false
}

// directlyAssignable reports whether a value x of type V can be directly
// assigned (using memmove) to a value of type T.
// https://golang.org/doc/go_spec.html#Assignability
// Ignoring the interface rules (implemented elsewhere)
// and the ideal constant rules (no ideal constants at run time).
func directlyAssignable(T, V *rtype) bool {
	// x's type V is identical to T?
	if T == V {
		return true
	}

	// Otherwise at least one of T and V must be unnamed
	// and they must have the same kind.
	if T.Name() != "" && V.Name() != "" || T.Kind() != V.Kind() {
		return false
	}

	// x's type T and V must  have identical underlying types.
	return haveIdenticalUnderlyingType(T, V)
}

func haveIdenticalUnderlyingType(T, V *rtype) bool {
	if T == V {
		return true
	}

	kind := T.Kind()
	if kind != V.Kind() {
		return false
	}

	// Non-composite types of equal kind have same underlying type
	// (the predefined instance of the type).
	if Bool <= kind && kind <= Complex128 || kind == String || kind == UnsafePointer {
		return true
	}

	// Composite types.
	switch kind {
	case Array:
		return T.Elem() == V.Elem() && T.Len() == V.Len()

	case Chan:
		// Special case:
		// x is a bidirectional channel value, T is a channel type,
		// and x's type V and T have identical element types.
		if V.ChanDir() == BothDir && T.Elem() == V.Elem() {
			return true
		}

		// Otherwise continue test for identical underlying type.
		return V.ChanDir() == T.ChanDir() && T.Elem() == V.Elem()

	case Func:
		t := (*funcType)(unsafe.Pointer(T))
		v := (*funcType)(unsafe.Pointer(V))
		if t.outCount != v.outCount || t.inCount != v.inCount {
			return false
		}
		for i := 0; i < t.NumIn(); i++ {
			if t.In(i) != v.In(i) {
				return false
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if t.Out(i) != v.Out(i) {
				return false
			}
		}
		return true

	case Interface:
		t := (*interfaceType)(unsafe.Pointer(T))
		v := (*interfaceType)(unsafe.Pointer(V))
		if len(t.methods) == 0 && len(v.methods) == 0 {
			return true
		}
		// Might have the same methods but still
		// need a run time conversion.
		return false

	case Map:
		return T.Key() == V.Key() && T.Elem() == V.Elem()

	case Ptr, Slice:
		return T.Elem() == V.Elem()

	case Struct:
		t := (*structType)(unsafe.Pointer(T))
		v := (*structType)(unsafe.Pointer(V))
		if len(t.fields) != len(v.fields) {
			return false
		}
		for i := range t.fields {
			tf := &t.fields[i]
			vf := &v.fields[i]
			if tf.name.name() != vf.name.name() {
				return false
			}
			if tf.typ != vf.typ {
				return false
			}
			if tf.name.tag() != vf.name.tag() {
				return false
			}
			if tf.offset != vf.offset {
				return false
			}
		}
		return true
	}

	return false
}

// typelinks is implemented in package runtime.
// It returns a slice of the sections in each module,
// and a slice of *rtype offsets in each module.
//
// The types in each module are sorted by string. That is, the first
// two linked types of the first module are:
//
//	d0 := sections[0]
//	t1 := (*rtype)(add(d0, offset[0][0]))
//	t2 := (*rtype)(add(d0, offset[0][1]))
//
// and
//
//	t1.String() < t2.String()
//
// Note that strings are not unique identifiers for types:
// there can be more than one with a given string.
// Only types we might want to look up are included:
// pointers, channels, maps, slices, and arrays.
func typelinks() (sections []unsafe.Pointer, offset [][]int32)

func rtypeOff(section unsafe.Pointer, off int32) *rtype {
	return (*rtype)(add(section, uintptr(off)))
}

// typesByString returns the subslice of typelinks() whose elements have
// the given string representation.
// It may be empty (no known types with that string) or may have
// multiple elements (multiple types with that string).
func typesByString(s string) []*rtype {
	sections, offset := typelinks()
	var ret []*rtype

	for offsI, offs := range offset {
		section := sections[offsI]

		// We are looking for the first index i where the string becomes >= s.
		// This is a copy of sort.Search, with f(h) replaced by (*typ[h].String() >= s).
		i, j := 0, len(offs)
		for i < j {
			h := i + (j-i)/2 // avoid overflow when computing h
			// i ≤ h < j
			if !(rtypeOff(section, offs[h]).String() >= s) {
				i = h + 1 // preserves f(i-1) == false
			} else {
				j = h // preserves f(j) == true
			}
		}
		// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.

		// Having found the first, linear scan forward to find the last.
		// We could do a second binary search, but the caller is going
		// to do a linear scan anyway.
		for j := i; j < len(offs); j++ {
			typ := rtypeOff(section, offs[j])
			if typ.String() != s {
				break
			}
			ret = append(ret, typ)
		}
	}
	return ret
}

// The lookupCache caches ArrayOf, ChanOf, MapOf and SliceOf lookups.
var lookupCache struct {
	sync.RWMutex
	m map[cacheKey]*rtype
}

// A cacheKey is the key for use in the lookupCache.
// Four values describe any of the types we are looking for:
// type kind, one or two subtypes, and an extra integer.
type cacheKey struct {
	kind  Kind
	t1    *rtype
	t2    *rtype
	extra uintptr
}

// cacheGet looks for a type under the key k in the lookupCache.
// If it finds one, it returns that type.
// If not, it returns nil with the cache locked.
// The caller is expected to use cachePut to unlock the cache.
func cacheGet(k cacheKey) Type {
	lookupCache.RLock()
	t := lookupCache.m[k]
	lookupCache.RUnlock()
	if t != nil {
		return t
	}

	lookupCache.Lock()
	t = lookupCache.m[k]
	if t != nil {
		lookupCache.Unlock()
		return t
	}

	if lookupCache.m == nil {
		lookupCache.m = make(map[cacheKey]*rtype)
	}

	return nil
}

// cachePut stores the given type in the cache, unlocks the cache,
// and returns the type. It is expected that the cache is locked
// because cacheGet returned nil.
func cachePut(k cacheKey, t *rtype) Type {
	lookupCache.m[k] = t
	lookupCache.Unlock()
	return t
}

// The funcLookupCache caches FuncOf lookups.
// FuncOf does not share the common lookupCache since cacheKey is not
// sufficient to represent functions unambiguously.
var funcLookupCache struct {
	sync.RWMutex
	m map[uint32][]*rtype // keyed by hash calculated in FuncOf
}

// ChanOf returns the channel type with the given direction and element type.
// For example, if t represents int, ChanOf(RecvDir, t) represents <-chan int.
//
// The gc runtime imposes a limit of 64 kB on channel element types.
// If t's size is equal to or exceeds this limit, ChanOf panics.
func ChanOf(dir ChanDir, t Type) Type {
	typ := t.(*rtype)

	// Look in cache.
	ckey := cacheKey{Chan, typ, nil, uintptr(dir)}
	if ch := cacheGet(ckey); ch != nil {
		return ch
	}

	// This restriction is imposed by the gc compiler and the runtime.
	if typ.size >= 1<<16 {
		lookupCache.Unlock()
		panic("reflect.ChanOf: element size too large")
	}

	// Look in known types.
	// TODO: Precedence when constructing string.
	var s string
	switch dir {
	default:
		lookupCache.Unlock()
		panic("reflect.ChanOf: invalid dir")
	case SendDir:
		s = "chan<- " + typ.String()
	case RecvDir:
		s = "<-chan " + typ.String()
	case BothDir:
		s = "chan " + typ.String()
	}
	for _, tt := range typesByString(s) {
		ch := (*chanType)(unsafe.Pointer(tt))
		if ch.elem == typ && ch.dir == uintptr(dir) {
			return cachePut(ckey, tt)
		}
	}

	// Make a channel type.
	var ichan interface{} = (chan unsafe.Pointer)(nil)
	prototype := *(**chanType)(unsafe.Pointer(&ichan))
	ch := new(chanType)
	*ch = *prototype
	ch.dir = uintptr(dir)
	ch.str = resolveReflectName(newName(s, "", "", false))
	ch.hash = fnv1(typ.hash, 'c', byte(dir))
	ch.elem = typ

	return cachePut(ckey, &ch.rtype)
}

func ismapkey(*rtype) bool // implemented in runtime

// MapOf returns the map type with the given key and element types.
// For example, if k represents int and e represents string,
// MapOf(k, e) represents map[int]string.
//
// If the key type is not a valid map key type (that is, if it does
// not implement Go's == operator), MapOf panics.
func MapOf(key, elem Type) Type {
	ktyp := key.(*rtype)
	etyp := elem.(*rtype)

	if !ismapkey(ktyp) {
		panic("reflect.MapOf: invalid key type " + ktyp.String())
	}

	// Look in cache.
	ckey := cacheKey{Map, ktyp, etyp, 0}
	if mt := cacheGet(ckey); mt != nil {
		return mt
	}

	// Look in known types.
	s := "map[" + ktyp.String() + "]" + etyp.String()
	for _, tt := range typesByString(s) {
		mt := (*mapType)(unsafe.Pointer(tt))
		if mt.key == ktyp && mt.elem == etyp {
			return cachePut(ckey, tt)
		}
	}

	// Make a map type.
	var imap interface{} = (map[unsafe.Pointer]unsafe.Pointer)(nil)
	mt := new(mapType)
	*mt = **(**mapType)(unsafe.Pointer(&imap))
	mt.str = resolveReflectName(newName(s, "", "", false))
	mt.hash = fnv1(etyp.hash, 'm', byte(ktyp.hash>>24), byte(ktyp.hash>>16), byte(ktyp.hash>>8), byte(ktyp.hash))
	mt.key = ktyp
	mt.elem = etyp
	mt.bucket = bucketOf(ktyp, etyp)
	if ktyp.size > maxKeySize {
		mt.keysize = uint8(ptrSize)
		mt.indirectkey = 1
	} else {
		mt.keysize = uint8(ktyp.size)
		mt.indirectkey = 0
	}
	if etyp.size > maxValSize {
		mt.valuesize = uint8(ptrSize)
		mt.indirectvalue = 1
	} else {
		mt.valuesize = uint8(etyp.size)
		mt.indirectvalue = 0
	}
	mt.bucketsize = uint16(mt.bucket.size)
	mt.reflexivekey = isReflexive(ktyp)
	mt.needkeyupdate = needKeyUpdate(ktyp)

	return cachePut(ckey, &mt.rtype)
}

type funcTypeFixed4 struct {
	funcType
	args [4]*rtype
}
type funcTypeFixed8 struct {
	funcType
	args [8]*rtype
}
type funcTypeFixed16 struct {
	funcType
	args [16]*rtype
}
type funcTypeFixed32 struct {
	funcType
	args [32]*rtype
}
type funcTypeFixed64 struct {
	funcType
	args [64]*rtype
}
type funcTypeFixed128 struct {
	funcType
	args [128]*rtype
}

// FuncOf returns the function type with the given argument and result types.
// For example if k represents int and e represents string,
// FuncOf([]Type{k}, []Type{e}, false) represents func(int) string.
//
// The variadic argument controls whether the function is variadic. FuncOf
// panics if the in[len(in)-1] does not represent a slice and variadic is
// true.
func FuncOf(in, out []Type, variadic bool) Type {
	if variadic && (len(in) == 0 || in[len(in)-1].Kind() != Slice) {
		panic("reflect.FuncOf: last arg of variadic func must be slice")
	}

	// Make a func type.
	var ifunc interface{} = (func())(nil)
	prototype := *(**funcType)(unsafe.Pointer(&ifunc))
	n := len(in) + len(out)

	var ft *funcType
	var args []*rtype
	switch {
	case n <= 4:
		fixed := new(funcTypeFixed4)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 8:
		fixed := new(funcTypeFixed8)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 16:
		fixed := new(funcTypeFixed16)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 32:
		fixed := new(funcTypeFixed32)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 64:
		fixed := new(funcTypeFixed64)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	case n <= 128:
		fixed := new(funcTypeFixed128)
		args = fixed.args[:0:len(fixed.args)]
		ft = &fixed.funcType
	default:
		panic("reflect.FuncOf: too many arguments")
	}
	*ft = *prototype

	// Build a hash and minimally populate ft.
	var hash uint32
	for _, in := range in {
		t := in.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.hash>>24), byte(t.hash>>16), byte(t.hash>>8), byte(t.hash))
	}
	if variadic {
		hash = fnv1(hash, 'v')
	}
	hash = fnv1(hash, '.')
	for _, out := range out {
		t := out.(*rtype)
		args = append(args, t)
		hash = fnv1(hash, byte(t.hash>>24), byte(t.hash>>16), byte(t.hash>>8), byte(t.hash))
	}
	if len(args) > 50 {
		panic("reflect.FuncOf does not support more than 50 arguments")
	}
	ft.tflag = 0
	ft.hash = hash
	ft.inCount = uint16(len(in))
	ft.outCount = uint16(len(out))
	if variadic {
		ft.outCount |= 1 << 15
	}

	// Look in cache.
	funcLookupCache.RLock()
	for _, t := range funcLookupCache.m[hash] {
		if haveIdenticalUnderlyingType(&ft.rtype, t) {
			funcLookupCache.RUnlock()
			return t
		}
	}
	funcLookupCache.RUnlock()

	// Not in cache, lock and retry.
	funcLookupCache.Lock()
	defer funcLookupCache.Unlock()
	if funcLookupCache.m == nil {
		funcLookupCache.m = make(map[uint32][]*rtype)
	}
	for _, t := range funcLookupCache.m[hash] {
		if haveIdenticalUnderlyingType(&ft.rtype, t) {
			return t
		}
	}

	// Look in known types for the same string representation.
	str := funcStr(ft)
	for _, tt := range typesByString(str) {
		if haveIdenticalUnderlyingType(&ft.rtype, tt) {
			funcLookupCache.m[hash] = append(funcLookupCache.m[hash], tt)
			return tt
		}
	}

	// Populate the remaining fields of ft and store in cache.
	ft.str = resolveReflectName(newName(str, "", "", false))
	funcLookupCache.m[hash] = append(funcLookupCache.m[hash], &ft.rtype)

	return &ft.rtype
}

// funcStr builds a string representation of a funcType.
func funcStr(ft *funcType) string {
	repr := make([]byte, 0, 64)
	repr = append(repr, "func("...)
	for i, t := range ft.in() {
		if i > 0 {
			repr = append(repr, ", "...)
		}
		if ft.IsVariadic() && i == int(ft.inCount)-1 {
			repr = append(repr, "..."...)
			repr = append(repr, (*sliceType)(unsafe.Pointer(t)).elem.String()...)
		} else {
			repr = append(repr, t.String()...)
		}
	}
	repr = append(repr, ')')
	out := ft.out()
	if len(out) == 1 {
		repr = append(repr, ' ')
	} else if len(out) > 1 {
		repr = append(repr, " ("...)
	}
	for i, t := range out {
		if i > 0 {
			repr = append(repr, ", "...)
		}
		repr = append(repr, t.String()...)
	}
	if len(out) > 1 {
		repr = append(repr, ')')
	}
	return string(repr)
}

// isReflexive reports whether the == operation on the type is reflexive.
// That is, x == x for all values x of type t.
func isReflexive(t *rtype) bool {
	switch t.Kind() {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Ptr, String, UnsafePointer:
		return true
	case Float32, Float64, Complex64, Complex128, Interface:
		return false
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return isReflexive(tt.elem)
	case Struct:
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.fields {
			if !isReflexive(f.typ) {
				return false
			}
		}
		return true
	default:
		// Func, Map, Slice, Invalid
		panic("isReflexive called on non-key type " + t.String())
	}
}

// needKeyUpdate reports whether map overwrites require the key to be copied.
func needKeyUpdate(t *rtype) bool {
	switch t.Kind() {
	case Bool, Int, Int8, Int16, Int32, Int64, Uint, Uint8, Uint16, Uint32, Uint64, Uintptr, Chan, Ptr, UnsafePointer:
		return false
	case Float32, Float64, Complex64, Complex128, Interface, String:
		// Float keys can be updated from +0 to -0.
		// String keys can be updated to use a smaller backing store.
		// Interfaces might have floats of strings in them.
		return true
	case Array:
		tt := (*arrayType)(unsafe.Pointer(t))
		return needKeyUpdate(tt.elem)
	case Struct:
		tt := (*structType)(unsafe.Pointer(t))
		for _, f := range tt.fields {
			if needKeyUpdate(f.typ) {
				return true
			}
		}
		return false
	default:
		// Func, Map, Slice, Invalid
		panic("needKeyUpdate called on non-key type " + t.String())
	}
}

// Make sure these routines stay in sync with ../../runtime/hashmap.go!
// These types exist only for GC, so we only fill out GC relevant info.
// Currently, that's just size and the GC program. We also fill in string
// for possible debugging use.
const (
	bucketSize uintptr = 8
	maxKeySize uintptr = 128
	maxValSize uintptr = 128
)

func bucketOf(ktyp, etyp *rtype) *rtype {
	// See comment on hmap.overflow in ../runtime/hashmap.go.
	var kind uint8
	if ktyp.kind&kindNoPointers != 0 && etyp.kind&kindNoPointers != 0 &&
		ktyp.size <= maxKeySize && etyp.size <= maxValSize {
		kind = kindNoPointers
	}

	if ktyp.size > maxKeySize {
		ktyp = PtrTo(ktyp).(*rtype)
	}
	if etyp.size > maxValSize {
		etyp = PtrTo(etyp).(*rtype)
	}

	// Prepare GC data if any.
	// A bucket is at most bucketSize*(1+maxKeySize+maxValSize)+2*ptrSize bytes,
	// or 2072 bytes, or 259 pointer-size words, or 33 bytes of pointer bitmap.
	// Normally the enforced limit on pointer maps is 16 bytes,
	// but larger ones are acceptable, 33 bytes isn't too too big,
	// and it's easier to generate a pointer bitmap than a GC program.
	// Note that since the key and value are known to be <= 128 bytes,
	// they're guaranteed to have bitmaps instead of GC programs.
	var gcdata *byte
	var ptrdata uintptr
	var overflowPad uintptr

	// On NaCl, pad if needed to make overflow end at the proper struct alignment.
	// On other systems, align > ptrSize is not possible.
	if runtime.GOARCH == "amd64p32" && (ktyp.align > ptrSize || etyp.align > ptrSize) {
		overflowPad = ptrSize
	}
	size := bucketSize*(1+ktyp.size+etyp.size) + overflowPad + ptrSize
	if size&uintptr(ktyp.align-1) != 0 || size&uintptr(etyp.align-1) != 0 {
		panic("reflect: bad size computation in MapOf")
	}

	if kind != kindNoPointers {
		nptr := (bucketSize*(1+ktyp.size+etyp.size) + ptrSize) / ptrSize
		mask := make([]byte, (nptr+7)/8)
		base := bucketSize / ptrSize

		if ktyp.kind&kindNoPointers == 0 {
			if ktyp.kind&kindGCProg != 0 {
				panic("reflect: unexpected GC program in MapOf")
			}
			kmask := (*[16]byte)(unsafe.Pointer(ktyp.gcdata))
			for i := uintptr(0); i < ktyp.size/ptrSize; i++ {
				if (kmask[i/8]>>(i%8))&1 != 0 {
					for j := uintptr(0); j < bucketSize; j++ {
						word := base + j*ktyp.size/ptrSize + i
						mask[word/8] |= 1 << (word % 8)
					}
				}
			}
		}
		base += bucketSize * ktyp.size / ptrSize

		if etyp.kind&kindNoPointers == 0 {
			if etyp.kind&kindGCProg != 0 {
				panic("reflect: unexpected GC program in MapOf")
			}
			emask := (*[16]byte)(unsafe.Pointer(etyp.gcdata))
			for i := uintptr(0); i < etyp.size/ptrSize; i++ {
				if (emask[i/8]>>(i%8))&1 != 0 {
					for j := uintptr(0); j < bucketSize; j++ {
						word := base + j*etyp.size/ptrSize + i
						mask[word/8] |= 1 << (word % 8)
					}
				}
			}
		}
		base += bucketSize * etyp.size / ptrSize
		base += overflowPad / ptrSize

		word := base
		mask[word/8] |= 1 << (word % 8)
		gcdata = &mask[0]
		ptrdata = (word + 1) * ptrSize

		// overflow word must be last
		if ptrdata != size {
			panic("reflect: bad layout computation in MapOf")
		}
	}

	b := new(rtype)
	b.align = ptrSize
	if overflowPad > 0 {
		b.align = 8
	}
	b.size = size
	b.ptrdata = ptrdata
	b.kind = kind
	b.gcdata = gcdata
	s := "bucket(" + ktyp.String() + "," + etyp.String() + ")"
	b.str = resolveReflectName(newName(s, "", "", false))
	return b
}

// SliceOf returns the slice type with element type t.
// For example, if t represents int, SliceOf(t) represents []int.
func SliceOf(t Type) Type {
	typ := t.(*rtype)

	// Look in cache.
	ckey := cacheKey{Slice, typ, nil, 0}
	if slice := cacheGet(ckey); slice != nil {
		return slice
	}

	// Look in known types.
	s := "[]" + typ.String()
	for _, tt := range typesByString(s) {
		slice := (*sliceType)(unsafe.Pointer(tt))
		if slice.elem == typ {
			return cachePut(ckey, tt)
		}
	}

	// Make a slice type.
	var islice interface{} = ([]unsafe.Pointer)(nil)
	prototype := *(**sliceType)(unsafe.Pointer(&islice))
	slice := new(sliceType)
	*slice = *prototype
	slice.tflag = 0
	slice.str = resolveReflectName(newName(s, "", "", false))
	slice.hash = fnv1(typ.hash, '[')
	slice.elem = typ

	return cachePut(ckey, &slice.rtype)
}

// The structLookupCache caches StructOf lookups.
// StructOf does not share the common lookupCache since we need to pin
// the memory associated with *structTypeFixedN.
var structLookupCache struct {
	sync.RWMutex
	m map[uint32][]interface {
		common() *rtype
	} // keyed by hash calculated in StructOf
}

type structTypeUncommon struct {
	structType
	u uncommonType
}

// A *rtype representing a struct is followed directly in memory by an
// array of method objects representing the methods attached to the
// struct. To get the same layout for a run time generated type, we
// need an array directly following the uncommonType memory. The types
// structTypeFixed4, ...structTypeFixedN are used to do this.
//
// A similar strategy is used for funcTypeFixed4, ...funcTypeFixedN.

// TODO(crawshaw): as these structTypeFixedN and funcTypeFixedN structs
// have no methods, they could be defined at runtime using the StructOf
// function.

type structTypeFixed4 struct {
	structType
	u uncommonType
	m [4]method
}

type structTypeFixed8 struct {
	structType
	u uncommonType
	m [8]method
}

type structTypeFixed16 struct {
	structType
	u uncommonType
	m [16]method
}

type structTypeFixed32 struct {
	structType
	u uncommonType
	m [32]method
}

// StructOf returns the struct type containing fields.
// The Offset and Index fields are ignored and computed as they would be
// by the compiler.
//
// StructOf currently does not generate wrapper methods for embedded fields.
// This limitation may be lifted in a future version.
func StructOf(fields []StructField) Type {
	var (
		hash       = fnv1(0, []byte("struct {")...)
		size       uintptr
		typalign   uint8
		comparable = true
		hashable   = true
		methods    []method

		fs   = make([]structField, len(fields))
		repr = make([]byte, 0, 64)
		fset = map[string]struct{}{} // fields' names

		hasPtr    = false // records whether at least one struct-field is a pointer
		hasGCProg = false // records whether a struct-field type has a GCProg
	)

	repr = append(repr, "struct {"...)
	for i, field := range fields {
		if field.Type == nil {
			panic("reflect.StructOf: field " + strconv.Itoa(i) + " has no type")
		}
		f := runtimeStructField(field)
		ft := f.typ
		if ft.kind&kindGCProg != 0 {
			hasGCProg = true
		}
		if ft.pointers() {
			hasPtr = true
		}

		name := ""
		// Update string and hash
		if f.name.nameLen() > 0 {
			hash = fnv1(hash, []byte(f.name.name())...)
			repr = append(repr, (" " + f.name.name())...)
			name = f.name.name()
		} else {
			// Embedded field
			if f.typ.Kind() == Ptr {
				// Embedded ** and *interface{} are illegal
				elem := ft.Elem()
				if k := elem.Kind(); k == Ptr || k == Interface {
					panic("reflect.StructOf: illegal anonymous field type " + ft.String())
				}
				name = elem.String()
			} else {
				name = ft.String()
			}
			// TODO(sbinet) check for syntactically impossible type names?

			switch f.typ.Kind() {
			case Interface:
				ift := (*interfaceType)(unsafe.Pointer(ft))
				for im, m := range ift.methods {
					if ift.nameOff(m.name).pkgPath() != "" {
						// TODO(sbinet)
						panic("reflect: embedded interface with unexported method(s) not implemented")
					}

					var (
						mtyp    = ift.typeOff(m.typ)
						ifield  = i
						imethod = im
						ifn     Value
						tfn     Value
					)

					if ft.kind&kindDirectIface != 0 {
						tfn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
						ifn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
					} else {
						tfn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = in[0]
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
						ifn = MakeFunc(mtyp, func(in []Value) []Value {
							var args []Value
							var recv = Indirect(in[0])
							if len(in) > 1 {
								args = in[1:]
							}
							return recv.Field(ifield).Method(imethod).Call(args)
						})
					}

					methods = append(methods, method{
						name: resolveReflectName(ift.nameOff(m.name)),
						mtyp: resolveReflectType(mtyp),
						ifn:  resolveReflectText(unsafe.Pointer(&ifn)),
						tfn:  resolveReflectText(unsafe.Pointer(&tfn)),
					})
				}
			case Ptr:
				ptr := (*ptrType)(unsafe.Pointer(ft))
				if unt := ptr.uncommon(); unt != nil {
					for _, m := range unt.methods() {
						mname := ptr.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet)
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ptr.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ptr.textOff(m.ifn)),
							tfn:  resolveReflectText(ptr.textOff(m.tfn)),
						})
					}
				}
				if unt := ptr.elem.uncommon(); unt != nil {
					for _, m := range unt.methods() {
						mname := ptr.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet)
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ptr.elem.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ptr.elem.textOff(m.ifn)),
							tfn:  resolveReflectText(ptr.elem.textOff(m.tfn)),
						})
					}
				}
			default:
				if unt := ft.uncommon(); unt != nil {
					for _, m := range unt.methods() {
						mname := ft.nameOff(m.name)
						if mname.pkgPath() != "" {
							// TODO(sbinet)
							panic("reflect: embedded interface with unexported method(s) not implemented")
						}
						methods = append(methods, method{
							name: resolveReflectName(mname),
							mtyp: resolveReflectType(ft.typeOff(m.mtyp)),
							ifn:  resolveReflectText(ft.textOff(m.ifn)),
							tfn:  resolveReflectText(ft.textOff(m.tfn)),
						})

					}
				}
			}
		}
		if _, dup := fset[name]; dup {
			panic("reflect.StructOf: duplicate field " + name)
		}
		fset[name] = struct{}{}

		hash = fnv1(hash, byte(ft.hash>>24), byte(ft.hash>>16), byte(ft.hash>>8), byte(ft.hash))

		repr = append(repr, (" " + ft.String())...)
		if f.name.tagLen() > 0 {
			hash = fnv1(hash, []byte(f.name.tag())...)
			repr = append(repr, (" " + strconv.Quote(f.name.tag()))...)
		}
		if i < len(fields)-1 {
			repr = append(repr, ';')
		}

		comparable = comparable && (ft.alg.equal != nil)
		hashable = hashable && (ft.alg.hash != nil)

		f.offset = align(size, uintptr(ft.align))
		if ft.align > typalign {
			typalign = ft.align
		}
		size = f.offset + ft.size

		fs[i] = f
	}

	var typ *structType
	var ut *uncommonType
	var typPin interface {
		common() *rtype
	} // structTypeFixedN

	switch {
	case len(methods) == 0:
		t := new(structTypeUncommon)
		typ = &t.structType
		ut = &t.u
		typPin = t
	case len(methods) <= 4:
		t := new(structTypeFixed4)
		typ = &t.structType
		ut = &t.u
		copy(t.m[:], methods)
		typPin = t
	case len(methods) <= 8:
		t := new(structTypeFixed8)
		typ = &t.structType
		ut = &t.u
		copy(t.m[:], methods)
		typPin = t
	case len(methods) <= 16:
		t := new(structTypeFixed16)
		typ = &t.structType
		ut = &t.u
		copy(t.m[:], methods)
		typPin = t
	case len(methods) <= 32:
		t := new(structTypeFixed32)
		typ = &t.structType
		ut = &t.u
		copy(t.m[:], methods)
		typPin = t
	default:
		panic("reflect.StructOf: too many methods")
	}
	ut.mcount = uint16(len(methods))
	ut.moff = uint16(unsafe.Sizeof(uncommonType{}))

	if len(fs) > 0 {
		repr = append(repr, ' ')
	}
	repr = append(repr, '}')
	hash = fnv1(hash, '}')
	str := string(repr)

	// Round the size up to be a multiple of the alignment.
	size = align(size, uintptr(typalign))

	// Make the struct type.
	var istruct interface{} = struct{}{}
	prototype := *(**structType)(unsafe.Pointer(&istruct))
	*typ = *prototype
	typ.fields = fs

	// Look in cache
	structLookupCache.RLock()
	for _, st := range structLookupCache.m[hash] {
		t := st.common()
		if haveIdenticalUnderlyingType(&typ.rtype, t) {
			structLookupCache.RUnlock()
			return t
		}
	}
	structLookupCache.RUnlock()

	// not in cache, lock and retry
	structLookupCache.Lock()
	defer structLookupCache.Unlock()
	if structLookupCache.m == nil {
		structLookupCache.m = make(map[uint32][]interface {
			common() *rtype
		})
	}
	for _, st := range structLookupCache.m[hash] {
		t := st.common()
		if haveIdenticalUnderlyingType(&typ.rtype, t) {
			return t
		}
	}

	// Look in known types.
	for _, t := range typesByString(str) {
		if haveIdenticalUnderlyingType(&typ.rtype, t) {
			// even if 't' wasn't a structType with methods, we should be ok
			// as the 'u uncommonType' field won't be accessed except when
			// tflag&tflagUncommon is set.
			structLookupCache.m[hash] = append(structLookupCache.m[hash], t)
			return t
		}
	}

	typ.str = resolveReflectName(newName(str, "", "", false))
	typ.tflag = 0
	typ.hash = hash
	typ.size = size
	typ.align = typalign
	typ.fieldAlign = typalign
	if len(methods) > 0 {
		typ.tflag |= tflagUncommon
	}
	if !hasPtr {
		typ.kind |= kindNoPointers
	} else {
		typ.kind &^= kindNoPointers
	}

	if hasGCProg {
		lastPtrField := 0
		for i, ft := range fs {
			if ft.typ.pointers() {
				lastPtrField = i
			}
		}
		prog := []byte{0, 0, 0, 0} // will be length of prog
		for i, ft := range fs {
			if i > lastPtrField {
				// gcprog should not include anything for any field after
				// the last field that contains pointer data
				break
			}
			// FIXME(sbinet) handle padding, fields smaller than a word
			elemGC := (*[1 << 30]byte)(unsafe.Pointer(ft.typ.gcdata))[:]
			elemPtrs := ft.typ.ptrdata / ptrSize
			switch {
			case ft.typ.kind&kindGCProg == 0 && ft.typ.ptrdata != 0:
				// Element is small with pointer mask; use as literal bits.
				mask := elemGC
				// Emit 120-bit chunks of full bytes (max is 127 but we avoid using partial bytes).
				var n uintptr
				for n := elemPtrs; n > 120; n -= 120 {
					prog = append(prog, 120)
					prog = append(prog, mask[:15]...)
					mask = mask[15:]
				}
				prog = append(prog, byte(n))
				prog = append(prog, mask[:(n+7)/8]...)
			case ft.typ.kind&kindGCProg != 0:
				// Element has GC program; emit one element.
				elemProg := elemGC[4 : 4+*(*uint32)(unsafe.Pointer(&elemGC[0]))-1]
				prog = append(prog, elemProg...)
			}
			// Pad from ptrdata to size.
			elemWords := ft.typ.size / ptrSize
			if elemPtrs < elemWords {
				// Emit literal 0 bit, then repeat as needed.
				prog = append(prog, 0x01, 0x00)
				if elemPtrs+1 < elemWords {
					prog = append(prog, 0x81)
					prog = appendVarint(prog, elemWords-elemPtrs-1)
				}
			}
		}
		*(*uint32)(unsafe.Pointer(&prog[0])) = uint32(len(prog) - 4)
		typ.kind |= kindGCProg
		typ.gcdata = &prog[0]
	} else {
		typ.kind &^= kindGCProg
		bv := new(bitVector)
		addTypeBits(bv, 0, typ.common())
		if len(bv.data) > 0 {
			typ.gcdata = &bv.data[0]
		}
	}
	typ.ptrdata = typeptrdata(typ.common())
	typ.alg = new(typeAlg)
	if hashable {
		typ.alg.hash = func(p unsafe.Pointer, seed uintptr) uintptr {
			o := seed
			for _, ft := range typ.fields {
				pi := unsafe.Pointer(uintptr(p) + ft.offset)
				o = ft.typ.alg.hash(pi, o)
			}
			return o
		}
	}

	if comparable {
		typ.alg.equal = func(p, q unsafe.Pointer) bool {
			for _, ft := range typ.fields {
				pi := unsafe.Pointer(uintptr(p) + ft.offset)
				qi := unsafe.Pointer(uintptr(q) + ft.offset)
				if !ft.typ.alg.equal(pi, qi) {
					return false
				}
			}
			return true
		}
	}

	switch {
	case len(fs) == 1 && !ifaceIndir(fs[0].typ):
		// structs of 1 direct iface type can be direct
		typ.kind |= kindDirectIface
	default:
		typ.kind &^= kindDirectIface
	}

	structLookupCache.m[hash] = append(structLookupCache.m[hash], typPin)
	return &typ.rtype
}

func runtimeStructField(field StructField) structField {
	exported := field.PkgPath == ""
	if field.Name == "" {
		t := field.Type.(*rtype)
		if t.Kind() == Ptr {
			t = t.Elem().(*rtype)
		}
		exported = t.nameOff(t.str).isExported()
	} else if exported {
		b0 := field.Name[0]
		if ('a' <= b0 && b0 <= 'z') || b0 == '_' {
			panic("reflect.StructOf: field \"" + field.Name + "\" is unexported but has no PkgPath")
		}
	}

	_ = resolveReflectType(field.Type.common())
	return structField{
		name:   newName(field.Name, string(field.Tag), field.PkgPath, exported),
		typ:    field.Type.common(),
		offset: 0,
	}
}

// typeptrdata returns the length in bytes of the prefix of t
// containing pointer data. Anything after this offset is scalar data.
// keep in sync with ../cmd/compile/internal/gc/reflect.go
func typeptrdata(t *rtype) uintptr {
	if !t.pointers() {
		return 0
	}
	switch t.Kind() {
	case Struct:
		st := (*structType)(unsafe.Pointer(t))
		// find the last field that has pointers.
		field := 0
		for i := range st.fields {
			ft := st.fields[i].typ
			if ft.pointers() {
				field = i
			}
		}
		f := st.fields[field]
		return f.offset + f.typ.ptrdata

	default:
		panic("reflect.typeptrdata: unexpected type, " + t.String())
	}
}

// See cmd/compile/internal/gc/reflect.go for derivation of constant.
const maxPtrmaskBytes = 2048

// ArrayOf returns the array type with the given count and element type.
// For example, if t represents int, ArrayOf(5, t) represents [5]int.
//
// If the resulting type would be larger than the available address space,
// ArrayOf panics.
func ArrayOf(count int, elem Type) Type {
	typ := elem.(*rtype)
	// call SliceOf here as it calls cacheGet/cachePut.
	// ArrayOf also calls cacheGet/cachePut and thus may modify the state of
	// the lookupCache mutex.
	slice := SliceOf(elem)

	// Look in cache.
	ckey := cacheKey{Array, typ, nil, uintptr(count)}
	if array := cacheGet(ckey); array != nil {
		return array
	}

	// Look in known types.
	s := "[" + strconv.Itoa(count) + "]" + typ.String()
	for _, tt := range typesByString(s) {
		array := (*arrayType)(unsafe.Pointer(tt))
		if array.elem == typ {
			return cachePut(ckey, tt)
		}
	}

	// Make an array type.
	var iarray interface{} = [1]unsafe.Pointer{}
	prototype := *(**arrayType)(unsafe.Pointer(&iarray))
	array := new(arrayType)
	*array = *prototype
	array.str = resolveReflectName(newName(s, "", "", false))
	array.hash = fnv1(typ.hash, '[')
	for n := uint32(count); n > 0; n >>= 8 {
		array.hash = fnv1(array.hash, byte(n))
	}
	array.hash = fnv1(array.hash, ']')
	array.elem = typ
	max := ^uintptr(0) / typ.size
	if uintptr(count) > max {
		panic("reflect.ArrayOf: array size would exceed virtual address space")
	}
	array.size = typ.size * uintptr(count)
	if count > 0 && typ.ptrdata != 0 {
		array.ptrdata = typ.size*uintptr(count-1) + typ.ptrdata
	}
	array.align = typ.align
	array.fieldAlign = typ.fieldAlign
	array.len = uintptr(count)
	array.slice = slice.(*rtype)

	array.kind &^= kindNoPointers
	switch {
	case typ.kind&kindNoPointers != 0 || array.size == 0:
		// No pointers.
		array.kind |= kindNoPointers
		array.gcdata = nil
		array.ptrdata = 0

	case count == 1:
		// In memory, 1-element array looks just like the element.
		array.kind |= typ.kind & kindGCProg
		array.gcdata = typ.gcdata
		array.ptrdata = typ.ptrdata

	case typ.kind&kindGCProg == 0 && array.size <= maxPtrmaskBytes*8*ptrSize:
		// Element is small with pointer mask; array is still small.
		// Create direct pointer mask by turning each 1 bit in elem
		// into count 1 bits in larger mask.
		mask := make([]byte, (array.ptrdata/ptrSize+7)/8)
		elemMask := (*[1 << 30]byte)(unsafe.Pointer(typ.gcdata))[:]
		elemWords := typ.size / ptrSize
		for j := uintptr(0); j < typ.ptrdata/ptrSize; j++ {
			if (elemMask[j/8]>>(j%8))&1 != 0 {
				for i := uintptr(0); i < array.len; i++ {
					k := i*elemWords + j
					mask[k/8] |= 1 << (k % 8)
				}
			}
		}
		array.gcdata = &mask[0]

	default:
		// Create program that emits one element
		// and then repeats to make the array.
		prog := []byte{0, 0, 0, 0} // will be length of prog
		elemGC := (*[1 << 30]byte)(unsafe.Pointer(typ.gcdata))[:]
		elemPtrs := typ.ptrdata / ptrSize
		if typ.kind&kindGCProg == 0 {
			// Element is small with pointer mask; use as literal bits.
			mask := elemGC
			// Emit 120-bit chunks of full bytes (max is 127 but we avoid using partial bytes).
			var n uintptr
			for n = elemPtrs; n > 120; n -= 120 {
				prog = append(prog, 120)
				prog = append(prog, mask[:15]...)
				mask = mask[15:]
			}
			prog = append(prog, byte(n))
			prog = append(prog, mask[:(n+7)/8]...)
		} else {
			// Element has GC program; emit one element.
			elemProg := elemGC[4 : 4+*(*uint32)(unsafe.Pointer(&elemGC[0]))-1]
			prog = append(prog, elemProg...)
		}
		// Pad from ptrdata to size.
		elemWords := typ.size / ptrSize
		if elemPtrs < elemWords {
			// Emit literal 0 bit, then repeat as needed.
			prog = append(prog, 0x01, 0x00)
			if elemPtrs+1 < elemWords {
				prog = append(prog, 0x81)
				prog = appendVarint(prog, elemWords-elemPtrs-1)
			}
		}
		// Repeat count-1 times.
		if elemWords < 0x80 {
			prog = append(prog, byte(elemWords|0x80))
		} else {
			prog = append(prog, 0x80)
			prog = appendVarint(prog, elemWords)
		}
		prog = appendVarint(prog, uintptr(count)-1)
		prog = append(prog, 0)
		*(*uint32)(unsafe.Pointer(&prog[0])) = uint32(len(prog) - 4)
		array.kind |= kindGCProg
		array.gcdata = &prog[0]
		array.ptrdata = array.size // overestimate but ok; must match program
	}

	etyp := typ.common()
	esize := etyp.Size()
	ealg := etyp.alg

	array.alg = new(typeAlg)
	if ealg.equal != nil {
		eequal := ealg.equal
		array.alg.equal = func(p, q unsafe.Pointer) bool {
			for i := 0; i < count; i++ {
				pi := arrayAt(p, i, esize)
				qi := arrayAt(q, i, esize)
				if !eequal(pi, qi) {
					return false
				}

			}
			return true
		}
	}
	if ealg.hash != nil {
		ehash := ealg.hash
		array.alg.hash = func(ptr unsafe.Pointer, seed uintptr) uintptr {
			o := seed
			for i := 0; i < count; i++ {
				o = ehash(arrayAt(ptr, i, esize), o)
			}
			return o
		}
	}

	switch {
	case count == 1 && !ifaceIndir(typ):
		// array of 1 direct iface type can be direct
		array.kind |= kindDirectIface
	default:
		array.kind &^= kindDirectIface
	}

	return cachePut(ckey, &array.rtype)
}

func appendVarint(x []byte, v uintptr) []byte {
	for ; v >= 0x80; v >>= 7 {
		x = append(x, byte(v|0x80))
	}
	x = append(x, byte(v))
	return x
}

// toType converts from a *rtype to a Type that can be returned
// to the client of package reflect. In gc, the only concern is that
// a nil *rtype must be replaced by a nil Type, but in gccgo this
// function takes care of ensuring that multiple *rtype for the same
// type are coalesced into a single Type.
func toType(t *rtype) Type {
	if t == nil {
		return nil
	}
	return t
}

type layoutKey struct {
	t    *rtype // function signature
	rcvr *rtype // receiver type, or nil if none
}

type layoutType struct {
	t         *rtype
	argSize   uintptr // size of arguments
	retOffset uintptr // offset of return values.
	stack     *bitVector
	framePool *sync.Pool
}

var layoutCache struct {
	sync.RWMutex
	m map[layoutKey]layoutType
}

// funcLayout computes a struct type representing the layout of the
// function arguments and return values for the function type t.
// If rcvr != nil, rcvr specifies the type of the receiver.
// The returned type exists only for GC, so we only fill out GC relevant info.
// Currently, that's just size and the GC program. We also fill in
// the name for possible debugging use.
func funcLayout(t *rtype, rcvr *rtype) (frametype *rtype, argSize, retOffset uintptr, stk *bitVector, framePool *sync.Pool) {
	if t.Kind() != Func {
		panic("reflect: funcLayout of non-func type")
	}
	if rcvr != nil && rcvr.Kind() == Interface {
		panic("reflect: funcLayout with interface receiver " + rcvr.String())
	}
	k := layoutKey{t, rcvr}
	layoutCache.RLock()
	if x := layoutCache.m[k]; x.t != nil {
		layoutCache.RUnlock()
		return x.t, x.argSize, x.retOffset, x.stack, x.framePool
	}
	layoutCache.RUnlock()
	layoutCache.Lock()
	if x := layoutCache.m[k]; x.t != nil {
		layoutCache.Unlock()
		return x.t, x.argSize, x.retOffset, x.stack, x.framePool
	}

	tt := (*funcType)(unsafe.Pointer(t))

	// compute gc program & stack bitmap for arguments
	ptrmap := new(bitVector)
	var offset uintptr
	if rcvr != nil {
		// Reflect uses the "interface" calling convention for
		// methods, where receivers take one word of argument
		// space no matter how big they actually are.
		if ifaceIndir(rcvr) || rcvr.pointers() {
			ptrmap.append(1)
		}
		offset += ptrSize
	}
	for _, arg := range tt.in() {
		offset += -offset & uintptr(arg.align-1)
		addTypeBits(ptrmap, offset, arg)
		offset += arg.size
	}
	argN := ptrmap.n
	argSize = offset
	if runtime.GOARCH == "amd64p32" {
		offset += -offset & (8 - 1)
	}
	offset += -offset & (ptrSize - 1)
	retOffset = offset
	for _, res := range tt.out() {
		offset += -offset & uintptr(res.align-1)
		addTypeBits(ptrmap, offset, res)
		offset += res.size
	}
	offset += -offset & (ptrSize - 1)

	// build dummy rtype holding gc program
	x := new(rtype)
	x.align = ptrSize
	if runtime.GOARCH == "amd64p32" {
		x.align = 8
	}
	x.size = offset
	x.ptrdata = uintptr(ptrmap.n) * ptrSize
	if ptrmap.n > 0 {
		x.gcdata = &ptrmap.data[0]
	} else {
		x.kind |= kindNoPointers
	}
	ptrmap.n = argN

	var s string
	if rcvr != nil {
		s = "methodargs(" + rcvr.String() + ")(" + t.String() + ")"
	} else {
		s = "funcargs(" + t.String() + ")"
	}
	x.str = resolveReflectName(newName(s, "", "", false))

	// cache result for future callers
	if layoutCache.m == nil {
		layoutCache.m = make(map[layoutKey]layoutType)
	}
	framePool = &sync.Pool{New: func() interface{} {
		return unsafe_New(x)
	}}
	layoutCache.m[k] = layoutType{
		t:         x,
		argSize:   argSize,
		retOffset: retOffset,
		stack:     ptrmap,
		framePool: framePool,
	}
	layoutCache.Unlock()
	return x, argSize, retOffset, ptrmap, framePool
}

// ifaceIndir reports whether t is stored indirectly in an interface value.
func ifaceIndir(t *rtype) bool {
	return t.kind&kindDirectIface == 0
}

// Layout matches runtime.BitVector (well enough).
type bitVector struct {
	n    uint32 // number of bits
	data []byte
}

// append a bit to the bitmap.
func (bv *bitVector) append(bit uint8) {
	if bv.n%8 == 0 {
		bv.data = append(bv.data, 0)
	}
	bv.data[bv.n/8] |= bit << (bv.n % 8)
	bv.n++
}

func addTypeBits(bv *bitVector, offset uintptr, t *rtype) {
	if t.kind&kindNoPointers != 0 {
		return
	}

	switch Kind(t.kind & kindMask) {
	case Chan, Func, Map, Ptr, Slice, String, UnsafePointer:
		// 1 pointer at start of representation
		for bv.n < uint32(offset/uintptr(ptrSize)) {
			bv.append(0)
		}
		bv.append(1)

	case Interface:
		// 2 pointers
		for bv.n < uint32(offset/uintptr(ptrSize)) {
			bv.append(0)
		}
		bv.append(1)
		bv.append(1)

	case Array:
		// repeat inner type
		tt := (*arrayType)(unsafe.Pointer(t))
		for i := 0; i < int(tt.len); i++ {
			addTypeBits(bv, offset+uintptr(i)*tt.elem.size, tt.elem)
		}

	case Struct:
		// apply fields
		tt := (*structType)(unsafe.Pointer(t))
		for i := range tt.fields {
			f := &tt.fields[i]
			addTypeBits(bv, offset+f.offset, f.typ)
		}
	}
}
