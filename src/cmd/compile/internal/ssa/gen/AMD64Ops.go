// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import "strings"

// Notes:
//  - Integer types live in the low portion of registers. Upper portions are junk.
//  - Boolean types use the low-order byte of a register. 0=false, 1=true.
//    Upper bytes are junk.
//  - Floating-point types live in the low natural slot of an sse2 register.
//    Unused portions are junk.
//  - We do not use AH,BH,CH,DH registers.
//  - When doing sub-register operations, we try to write the whole
//    destination register to avoid a partial-register write.
//  - Unused portions of AuxInt (or the Val portion of ValAndOff) are
//    filled by sign-extending the used portion.  Users of AuxInt which interpret
//    AuxInt as unsigned (e.g. shifts) must be careful.

// Suffixes encode the bit width of various instructions.
// Q (quad word) = 64 bit
// L (long word) = 32 bit
// W (word)      = 16 bit
// B (byte)      = 8 bit

// copied from ../../amd64/reg.go
var regNamesAMD64 = []string{
	"AX",
	"CX",
	"DX",
	"BX",
	"SP",
	"BP",
	"SI",
	"DI",
	"R8",
	"R9",
	"R10",
	"R11",
	"R12",
	"R13",
	"R14",
	"R15",
	"X0",
	"X1",
	"X2",
	"X3",
	"X4",
	"X5",
	"X6",
	"X7",
	"X8",
	"X9",
	"X10",
	"X11",
	"X12",
	"X13",
	"X14",
	"X15",

	// pseudo-registers
	"SB",
	"FLAGS",
}

func init() {
	// Make map from reg names to reg integers.
	if len(regNamesAMD64) > 64 {
		panic("too many registers")
	}
	num := map[string]int{}
	for i, name := range regNamesAMD64 {
		num[name] = i
	}
	buildReg := func(s string) regMask {
		m := regMask(0)
		for _, r := range strings.Split(s, " ") {
			if n, ok := num[r]; ok {
				m |= regMask(1) << uint(n)
				continue
			}
			panic("register " + r + " not found")
		}
		return m
	}

	// Common individual register masks
	var (
		ax         = buildReg("AX")
		cx         = buildReg("CX")
		dx         = buildReg("DX")
		x15        = buildReg("X15")
		gp         = buildReg("AX CX DX BX BP SI DI R8 R9 R10 R11 R12 R13 R14 R15")
		fp         = buildReg("X0 X1 X2 X3 X4 X5 X6 X7 X8 X9 X10 X11 X12 X13 X14 X15")
		gpsp       = gp | buildReg("SP")
		gpspsb     = gpsp | buildReg("SB")
		flags      = buildReg("FLAGS")
		callerSave = gp | fp | flags
	)
	// Common slices of register masks
	var (
		gponly    = []regMask{gp}
		fponly    = []regMask{fp}
		flagsonly = []regMask{flags}
	)

	// Common regInfo
	var (
		gp01      = regInfo{inputs: []regMask{}, outputs: gponly}
		gp11      = regInfo{inputs: []regMask{gp}, outputs: gponly, clobbers: flags}
		gp11sp    = regInfo{inputs: []regMask{gpsp}, outputs: gponly, clobbers: flags}
		gp11nf    = regInfo{inputs: []regMask{gpsp}, outputs: gponly} // nf: no flags clobbered
		gp11sb    = regInfo{inputs: []regMask{gpspsb}, outputs: gponly}
		gp21      = regInfo{inputs: []regMask{gp, gp}, outputs: gponly, clobbers: flags}
		gp21sp    = regInfo{inputs: []regMask{gpsp, gp}, outputs: gponly, clobbers: flags}
		gp21sb    = regInfo{inputs: []regMask{gpspsb, gpsp}, outputs: gponly}
		gp21shift = regInfo{inputs: []regMask{gp, cx}, outputs: []regMask{gp}, clobbers: flags}
		gp11div   = regInfo{inputs: []regMask{ax, gpsp &^ dx}, outputs: []regMask{ax},
			clobbers: dx | flags}
		gp11hmul = regInfo{inputs: []regMask{ax, gpsp}, outputs: []regMask{dx},
			clobbers: ax | flags}
		gp11mod = regInfo{inputs: []regMask{ax, gpsp &^ dx}, outputs: []regMask{dx},
			clobbers: ax | flags}

		gp2flags = regInfo{inputs: []regMask{gpsp, gpsp}, outputs: flagsonly}
		gp1flags = regInfo{inputs: []regMask{gpsp}, outputs: flagsonly}
		flagsgp  = regInfo{inputs: flagsonly, outputs: gponly}

		// for CMOVconst -- uses AX to hold constant temporary.
		gp1flagsgp = regInfo{inputs: []regMask{gp &^ ax, flags}, clobbers: ax | flags, outputs: []regMask{gp &^ ax}}

		readflags = regInfo{inputs: flagsonly, outputs: gponly}
		flagsgpax = regInfo{inputs: flagsonly, clobbers: ax | flags, outputs: []regMask{gp &^ ax}}

		gpload    = regInfo{inputs: []regMask{gpspsb, 0}, outputs: gponly}
		gploadidx = regInfo{inputs: []regMask{gpspsb, gpsp, 0}, outputs: gponly}

		gpstore         = regInfo{inputs: []regMask{gpspsb, gpsp, 0}}
		gpstoreconst    = regInfo{inputs: []regMask{gpspsb, 0}}
		gpstoreidx      = regInfo{inputs: []regMask{gpspsb, gpsp, gpsp, 0}}
		gpstoreconstidx = regInfo{inputs: []regMask{gpspsb, gpsp, 0}}

		fp01    = regInfo{inputs: []regMask{}, outputs: fponly}
		fp21    = regInfo{inputs: []regMask{fp, fp}, outputs: fponly}
		fp21x15 = regInfo{inputs: []regMask{fp &^ x15, fp &^ x15},
			clobbers: x15, outputs: []regMask{fp &^ x15}}
		fpgp     = regInfo{inputs: fponly, outputs: gponly}
		gpfp     = regInfo{inputs: gponly, outputs: fponly}
		fp11     = regInfo{inputs: fponly, outputs: fponly}
		fp2flags = regInfo{inputs: []regMask{fp, fp}, outputs: flagsonly}

		fpload    = regInfo{inputs: []regMask{gpspsb, 0}, outputs: fponly}
		fploadidx = regInfo{inputs: []regMask{gpspsb, gpsp, 0}, outputs: fponly}

		fpstore    = regInfo{inputs: []regMask{gpspsb, fp, 0}}
		fpstoreidx = regInfo{inputs: []regMask{gpspsb, gpsp, fp, 0}}
	)

	var AMD64ops = []opData{
		// fp ops
		{name: "ADDSS", argLength: 2, reg: fp21, asm: "ADDSS", commutative: true, resultInArg0: true}, // fp32 add
		{name: "ADDSD", argLength: 2, reg: fp21, asm: "ADDSD", commutative: true, resultInArg0: true}, // fp64 add
		{name: "SUBSS", argLength: 2, reg: fp21x15, asm: "SUBSS", resultInArg0: true},                 // fp32 sub
		{name: "SUBSD", argLength: 2, reg: fp21x15, asm: "SUBSD", resultInArg0: true},                 // fp64 sub
		{name: "MULSS", argLength: 2, reg: fp21, asm: "MULSS", commutative: true, resultInArg0: true}, // fp32 mul
		{name: "MULSD", argLength: 2, reg: fp21, asm: "MULSD", commutative: true, resultInArg0: true}, // fp64 mul
		{name: "DIVSS", argLength: 2, reg: fp21x15, asm: "DIVSS", resultInArg0: true},                 // fp32 div
		{name: "DIVSD", argLength: 2, reg: fp21x15, asm: "DIVSD", resultInArg0: true},                 // fp64 div

		{name: "MOVSSload", argLength: 2, reg: fpload, asm: "MOVSS", aux: "SymOff"},            // fp32 load
		{name: "MOVSDload", argLength: 2, reg: fpload, asm: "MOVSD", aux: "SymOff"},            // fp64 load
		{name: "MOVSSconst", reg: fp01, asm: "MOVSS", aux: "Float32", rematerializeable: true}, // fp32 constant
		{name: "MOVSDconst", reg: fp01, asm: "MOVSD", aux: "Float64", rematerializeable: true}, // fp64 constant
		{name: "MOVSSloadidx1", argLength: 3, reg: fploadidx, asm: "MOVSS", aux: "SymOff"},     // fp32 load indexed by i
		{name: "MOVSSloadidx4", argLength: 3, reg: fploadidx, asm: "MOVSS", aux: "SymOff"},     // fp32 load indexed by 4*i
		{name: "MOVSDloadidx1", argLength: 3, reg: fploadidx, asm: "MOVSD", aux: "SymOff"},     // fp64 load indexed by i
		{name: "MOVSDloadidx8", argLength: 3, reg: fploadidx, asm: "MOVSD", aux: "SymOff"},     // fp64 load indexed by 8*i

		{name: "MOVSSstore", argLength: 3, reg: fpstore, asm: "MOVSS", aux: "SymOff"},        // fp32 store
		{name: "MOVSDstore", argLength: 3, reg: fpstore, asm: "MOVSD", aux: "SymOff"},        // fp64 store
		{name: "MOVSSstoreidx1", argLength: 4, reg: fpstoreidx, asm: "MOVSS", aux: "SymOff"}, // fp32 indexed by i store
		{name: "MOVSSstoreidx4", argLength: 4, reg: fpstoreidx, asm: "MOVSS", aux: "SymOff"}, // fp32 indexed by 4i store
		{name: "MOVSDstoreidx1", argLength: 4, reg: fpstoreidx, asm: "MOVSD", aux: "SymOff"}, // fp64 indexed by i store
		{name: "MOVSDstoreidx8", argLength: 4, reg: fpstoreidx, asm: "MOVSD", aux: "SymOff"}, // fp64 indexed by 8i store

		// binary ops
		{name: "ADDQ", argLength: 2, reg: gp21sp, asm: "ADDQ", commutative: true},                // arg0 + arg1
		{name: "ADDL", argLength: 2, reg: gp21sp, asm: "ADDL", commutative: true},                // arg0 + arg1
		{name: "ADDQconst", argLength: 1, reg: gp11sp, asm: "ADDQ", aux: "Int64", typ: "UInt64"}, // arg0 + auxint
		{name: "ADDLconst", argLength: 1, reg: gp11sp, asm: "ADDL", aux: "Int32"},                // arg0 + auxint

		{name: "SUBQ", argLength: 2, reg: gp21, asm: "SUBQ", resultInArg0: true},                    // arg0 - arg1
		{name: "SUBL", argLength: 2, reg: gp21, asm: "SUBL", resultInArg0: true},                    // arg0 - arg1
		{name: "SUBQconst", argLength: 1, reg: gp11, asm: "SUBQ", aux: "Int64", resultInArg0: true}, // arg0 - auxint
		{name: "SUBLconst", argLength: 1, reg: gp11, asm: "SUBL", aux: "Int32", resultInArg0: true}, // arg0 - auxint

		{name: "MULQ", argLength: 2, reg: gp21, asm: "IMULQ", commutative: true, resultInArg0: true}, // arg0 * arg1
		{name: "MULL", argLength: 2, reg: gp21, asm: "IMULL", commutative: true, resultInArg0: true}, // arg0 * arg1
		{name: "MULQconst", argLength: 1, reg: gp11, asm: "IMULQ", aux: "Int64", resultInArg0: true}, // arg0 * auxint
		{name: "MULLconst", argLength: 1, reg: gp11, asm: "IMULL", aux: "Int32", resultInArg0: true}, // arg0 * auxint

		{name: "HMULQ", argLength: 2, reg: gp11hmul, asm: "IMULQ"}, // (arg0 * arg1) >> width
		{name: "HMULL", argLength: 2, reg: gp11hmul, asm: "IMULL"}, // (arg0 * arg1) >> width
		{name: "HMULW", argLength: 2, reg: gp11hmul, asm: "IMULW"}, // (arg0 * arg1) >> width
		{name: "HMULB", argLength: 2, reg: gp11hmul, asm: "IMULB"}, // (arg0 * arg1) >> width
		{name: "HMULQU", argLength: 2, reg: gp11hmul, asm: "MULQ"}, // (arg0 * arg1) >> width
		{name: "HMULLU", argLength: 2, reg: gp11hmul, asm: "MULL"}, // (arg0 * arg1) >> width
		{name: "HMULWU", argLength: 2, reg: gp11hmul, asm: "MULW"}, // (arg0 * arg1) >> width
		{name: "HMULBU", argLength: 2, reg: gp11hmul, asm: "MULB"}, // (arg0 * arg1) >> width

		{name: "AVGQU", argLength: 2, reg: gp21, commutative: true, resultInArg0: true}, // (arg0 + arg1) / 2 as unsigned, all 64 result bits

		{name: "DIVQ", argLength: 2, reg: gp11div, asm: "IDIVQ"}, // arg0 / arg1
		{name: "DIVL", argLength: 2, reg: gp11div, asm: "IDIVL"}, // arg0 / arg1
		{name: "DIVW", argLength: 2, reg: gp11div, asm: "IDIVW"}, // arg0 / arg1
		{name: "DIVQU", argLength: 2, reg: gp11div, asm: "DIVQ"}, // arg0 / arg1
		{name: "DIVLU", argLength: 2, reg: gp11div, asm: "DIVL"}, // arg0 / arg1
		{name: "DIVWU", argLength: 2, reg: gp11div, asm: "DIVW"}, // arg0 / arg1

		{name: "MODQ", argLength: 2, reg: gp11mod, asm: "IDIVQ"}, // arg0 % arg1
		{name: "MODL", argLength: 2, reg: gp11mod, asm: "IDIVL"}, // arg0 % arg1
		{name: "MODW", argLength: 2, reg: gp11mod, asm: "IDIVW"}, // arg0 % arg1
		{name: "MODQU", argLength: 2, reg: gp11mod, asm: "DIVQ"}, // arg0 % arg1
		{name: "MODLU", argLength: 2, reg: gp11mod, asm: "DIVL"}, // arg0 % arg1
		{name: "MODWU", argLength: 2, reg: gp11mod, asm: "DIVW"}, // arg0 % arg1

		{name: "ANDQ", argLength: 2, reg: gp21, asm: "ANDQ", commutative: true, resultInArg0: true}, // arg0 & arg1
		{name: "ANDL", argLength: 2, reg: gp21, asm: "ANDL", commutative: true, resultInArg0: true}, // arg0 & arg1
		{name: "ANDQconst", argLength: 1, reg: gp11, asm: "ANDQ", aux: "Int64", resultInArg0: true}, // arg0 & auxint
		{name: "ANDLconst", argLength: 1, reg: gp11, asm: "ANDL", aux: "Int32", resultInArg0: true}, // arg0 & auxint

		{name: "ORQ", argLength: 2, reg: gp21, asm: "ORQ", commutative: true, resultInArg0: true}, // arg0 | arg1
		{name: "ORL", argLength: 2, reg: gp21, asm: "ORL", commutative: true, resultInArg0: true}, // arg0 | arg1
		{name: "ORQconst", argLength: 1, reg: gp11, asm: "ORQ", aux: "Int64", resultInArg0: true}, // arg0 | auxint
		{name: "ORLconst", argLength: 1, reg: gp11, asm: "ORL", aux: "Int32", resultInArg0: true}, // arg0 | auxint

		{name: "XORQ", argLength: 2, reg: gp21, asm: "XORQ", commutative: true, resultInArg0: true}, // arg0 ^ arg1
		{name: "XORL", argLength: 2, reg: gp21, asm: "XORL", commutative: true, resultInArg0: true}, // arg0 ^ arg1
		{name: "XORQconst", argLength: 1, reg: gp11, asm: "XORQ", aux: "Int64", resultInArg0: true}, // arg0 ^ auxint
		{name: "XORLconst", argLength: 1, reg: gp11, asm: "XORL", aux: "Int32", resultInArg0: true}, // arg0 ^ auxint

		{name: "CMPQ", argLength: 2, reg: gp2flags, asm: "CMPQ", typ: "Flags"},                    // arg0 compare to arg1
		{name: "CMPL", argLength: 2, reg: gp2flags, asm: "CMPL", typ: "Flags"},                    // arg0 compare to arg1
		{name: "CMPW", argLength: 2, reg: gp2flags, asm: "CMPW", typ: "Flags"},                    // arg0 compare to arg1
		{name: "CMPB", argLength: 2, reg: gp2flags, asm: "CMPB", typ: "Flags"},                    // arg0 compare to arg1
		{name: "CMPQconst", argLength: 1, reg: gp1flags, asm: "CMPQ", typ: "Flags", aux: "Int64"}, // arg0 compare to auxint
		{name: "CMPLconst", argLength: 1, reg: gp1flags, asm: "CMPL", typ: "Flags", aux: "Int32"}, // arg0 compare to auxint
		{name: "CMPWconst", argLength: 1, reg: gp1flags, asm: "CMPW", typ: "Flags", aux: "Int16"}, // arg0 compare to auxint
		{name: "CMPBconst", argLength: 1, reg: gp1flags, asm: "CMPB", typ: "Flags", aux: "Int8"},  // arg0 compare to auxint

		{name: "UCOMISS", argLength: 2, reg: fp2flags, asm: "UCOMISS", typ: "Flags"}, // arg0 compare to arg1, f32
		{name: "UCOMISD", argLength: 2, reg: fp2flags, asm: "UCOMISD", typ: "Flags"}, // arg0 compare to arg1, f64

		{name: "TESTQ", argLength: 2, reg: gp2flags, asm: "TESTQ", typ: "Flags"},                    // (arg0 & arg1) compare to 0
		{name: "TESTL", argLength: 2, reg: gp2flags, asm: "TESTL", typ: "Flags"},                    // (arg0 & arg1) compare to 0
		{name: "TESTW", argLength: 2, reg: gp2flags, asm: "TESTW", typ: "Flags"},                    // (arg0 & arg1) compare to 0
		{name: "TESTB", argLength: 2, reg: gp2flags, asm: "TESTB", typ: "Flags"},                    // (arg0 & arg1) compare to 0
		{name: "TESTQconst", argLength: 1, reg: gp1flags, asm: "TESTQ", typ: "Flags", aux: "Int64"}, // (arg0 & auxint) compare to 0
		{name: "TESTLconst", argLength: 1, reg: gp1flags, asm: "TESTL", typ: "Flags", aux: "Int32"}, // (arg0 & auxint) compare to 0
		{name: "TESTWconst", argLength: 1, reg: gp1flags, asm: "TESTW", typ: "Flags", aux: "Int16"}, // (arg0 & auxint) compare to 0
		{name: "TESTBconst", argLength: 1, reg: gp1flags, asm: "TESTB", typ: "Flags", aux: "Int8"},  // (arg0 & auxint) compare to 0

		{name: "SHLQ", argLength: 2, reg: gp21shift, asm: "SHLQ", resultInArg0: true},               // arg0 << arg1, shift amount is mod 64
		{name: "SHLL", argLength: 2, reg: gp21shift, asm: "SHLL", resultInArg0: true},               // arg0 << arg1, shift amount is mod 32
		{name: "SHLQconst", argLength: 1, reg: gp11, asm: "SHLQ", aux: "Int64", resultInArg0: true}, // arg0 << auxint, shift amount 0-63
		{name: "SHLLconst", argLength: 1, reg: gp11, asm: "SHLL", aux: "Int32", resultInArg0: true}, // arg0 << auxint, shift amount 0-31
		// Note: x86 is weird, the 16 and 8 byte shifts still use all 5 bits of shift amount!

		{name: "SHRQ", argLength: 2, reg: gp21shift, asm: "SHRQ", resultInArg0: true},               // unsigned arg0 >> arg1, shift amount is mod 64
		{name: "SHRL", argLength: 2, reg: gp21shift, asm: "SHRL", resultInArg0: true},               // unsigned arg0 >> arg1, shift amount is mod 32
		{name: "SHRW", argLength: 2, reg: gp21shift, asm: "SHRW", resultInArg0: true},               // unsigned arg0 >> arg1, shift amount is mod 32
		{name: "SHRB", argLength: 2, reg: gp21shift, asm: "SHRB", resultInArg0: true},               // unsigned arg0 >> arg1, shift amount is mod 32
		{name: "SHRQconst", argLength: 1, reg: gp11, asm: "SHRQ", aux: "Int64", resultInArg0: true}, // unsigned arg0 >> auxint, shift amount 0-63
		{name: "SHRLconst", argLength: 1, reg: gp11, asm: "SHRL", aux: "Int32", resultInArg0: true}, // unsigned arg0 >> auxint, shift amount 0-31
		{name: "SHRWconst", argLength: 1, reg: gp11, asm: "SHRW", aux: "Int16", resultInArg0: true}, // unsigned arg0 >> auxint, shift amount 0-31
		{name: "SHRBconst", argLength: 1, reg: gp11, asm: "SHRB", aux: "Int8", resultInArg0: true},  // unsigned arg0 >> auxint, shift amount 0-31

		{name: "SARQ", argLength: 2, reg: gp21shift, asm: "SARQ", resultInArg0: true},               // signed arg0 >> arg1, shift amount is mod 64
		{name: "SARL", argLength: 2, reg: gp21shift, asm: "SARL", resultInArg0: true},               // signed arg0 >> arg1, shift amount is mod 32
		{name: "SARW", argLength: 2, reg: gp21shift, asm: "SARW", resultInArg0: true},               // signed arg0 >> arg1, shift amount is mod 32
		{name: "SARB", argLength: 2, reg: gp21shift, asm: "SARB", resultInArg0: true},               // signed arg0 >> arg1, shift amount is mod 32
		{name: "SARQconst", argLength: 1, reg: gp11, asm: "SARQ", aux: "Int64", resultInArg0: true}, // signed arg0 >> auxint, shift amount 0-63
		{name: "SARLconst", argLength: 1, reg: gp11, asm: "SARL", aux: "Int32", resultInArg0: true}, // signed arg0 >> auxint, shift amount 0-31
		{name: "SARWconst", argLength: 1, reg: gp11, asm: "SARW", aux: "Int16", resultInArg0: true}, // signed arg0 >> auxint, shift amount 0-31
		{name: "SARBconst", argLength: 1, reg: gp11, asm: "SARB", aux: "Int8", resultInArg0: true},  // signed arg0 >> auxint, shift amount 0-31

		{name: "ROLQconst", argLength: 1, reg: gp11, asm: "ROLQ", aux: "Int64", resultInArg0: true}, // arg0 rotate left auxint, rotate amount 0-63
		{name: "ROLLconst", argLength: 1, reg: gp11, asm: "ROLL", aux: "Int32", resultInArg0: true}, // arg0 rotate left auxint, rotate amount 0-31
		{name: "ROLWconst", argLength: 1, reg: gp11, asm: "ROLW", aux: "Int16", resultInArg0: true}, // arg0 rotate left auxint, rotate amount 0-15
		{name: "ROLBconst", argLength: 1, reg: gp11, asm: "ROLB", aux: "Int8", resultInArg0: true},  // arg0 rotate left auxint, rotate amount 0-7

		// unary ops
		{name: "NEGQ", argLength: 1, reg: gp11, asm: "NEGQ", resultInArg0: true}, // -arg0
		{name: "NEGL", argLength: 1, reg: gp11, asm: "NEGL", resultInArg0: true}, // -arg0

		{name: "NOTQ", argLength: 1, reg: gp11, asm: "NOTQ", resultInArg0: true}, // ^arg0
		{name: "NOTL", argLength: 1, reg: gp11, asm: "NOTL", resultInArg0: true}, // ^arg0

		{name: "BSFQ", argLength: 1, reg: gp11, asm: "BSFQ"}, // arg0 # of low-order zeroes ; undef if zero
		{name: "BSFL", argLength: 1, reg: gp11, asm: "BSFL"}, // arg0 # of low-order zeroes ; undef if zero
		{name: "BSFW", argLength: 1, reg: gp11, asm: "BSFW"}, // arg0 # of low-order zeroes ; undef if zero

		{name: "BSRQ", argLength: 1, reg: gp11, asm: "BSRQ"}, // arg0 # of high-order zeroes ; undef if zero
		{name: "BSRL", argLength: 1, reg: gp11, asm: "BSRL"}, // arg0 # of high-order zeroes ; undef if zero
		{name: "BSRW", argLength: 1, reg: gp11, asm: "BSRW"}, // arg0 # of high-order zeroes ; undef if zero

		// Note ASM for ops moves whole register
		{name: "CMOVQEQconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVQEQ", typ: "UInt64", aux: "Int64", resultInArg0: true}, // replace arg0 w/ constant if Z set
		{name: "CMOVLEQconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVLEQ", typ: "UInt32", aux: "Int32", resultInArg0: true}, // replace arg0 w/ constant if Z set
		{name: "CMOVWEQconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVLEQ", typ: "UInt16", aux: "Int16", resultInArg0: true}, // replace arg0 w/ constant if Z set
		{name: "CMOVQNEconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVQNE", typ: "UInt64", aux: "Int64", resultInArg0: true}, // replace arg0 w/ constant if Z not set
		{name: "CMOVLNEconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVLNE", typ: "UInt32", aux: "Int32", resultInArg0: true}, // replace arg0 w/ constant if Z not set
		{name: "CMOVWNEconst", argLength: 2, reg: gp1flagsgp, asm: "CMOVLNE", typ: "UInt16", aux: "Int16", resultInArg0: true}, // replace arg0 w/ constant if Z not set

		{name: "BSWAPQ", argLength: 1, reg: gp11, asm: "BSWAPQ", resultInArg0: true}, // arg0 swap bytes
		{name: "BSWAPL", argLength: 1, reg: gp11, asm: "BSWAPL", resultInArg0: true}, // arg0 swap bytes

		{name: "SQRTSD", argLength: 1, reg: fp11, asm: "SQRTSD"}, // sqrt(arg0)

		{name: "SBBQcarrymask", argLength: 1, reg: flagsgp, asm: "SBBQ"}, // (int64)(-1) if carry is set, 0 if carry is clear.
		{name: "SBBLcarrymask", argLength: 1, reg: flagsgp, asm: "SBBL"}, // (int32)(-1) if carry is set, 0 if carry is clear.
		// Note: SBBW and SBBB are subsumed by SBBL

		{name: "SETEQ", argLength: 1, reg: readflags, asm: "SETEQ"}, // extract == condition from arg0
		{name: "SETNE", argLength: 1, reg: readflags, asm: "SETNE"}, // extract != condition from arg0
		{name: "SETL", argLength: 1, reg: readflags, asm: "SETLT"},  // extract signed < condition from arg0
		{name: "SETLE", argLength: 1, reg: readflags, asm: "SETLE"}, // extract signed <= condition from arg0
		{name: "SETG", argLength: 1, reg: readflags, asm: "SETGT"},  // extract signed > condition from arg0
		{name: "SETGE", argLength: 1, reg: readflags, asm: "SETGE"}, // extract signed >= condition from arg0
		{name: "SETB", argLength: 1, reg: readflags, asm: "SETCS"},  // extract unsigned < condition from arg0
		{name: "SETBE", argLength: 1, reg: readflags, asm: "SETLS"}, // extract unsigned <= condition from arg0
		{name: "SETA", argLength: 1, reg: readflags, asm: "SETHI"},  // extract unsigned > condition from arg0
		{name: "SETAE", argLength: 1, reg: readflags, asm: "SETCC"}, // extract unsigned >= condition from arg0
		// Need different opcodes for floating point conditions because
		// any comparison involving a NaN is always FALSE and thus
		// the patterns for inverting conditions cannot be used.
		{name: "SETEQF", argLength: 1, reg: flagsgpax, asm: "SETEQ"}, // extract == condition from arg0
		{name: "SETNEF", argLength: 1, reg: flagsgpax, asm: "SETNE"}, // extract != condition from arg0
		{name: "SETORD", argLength: 1, reg: flagsgp, asm: "SETPC"},   // extract "ordered" (No Nan present) condition from arg0
		{name: "SETNAN", argLength: 1, reg: flagsgp, asm: "SETPS"},   // extract "unordered" (Nan present) condition from arg0

		{name: "SETGF", argLength: 1, reg: flagsgp, asm: "SETHI"},  // extract floating > condition from arg0
		{name: "SETGEF", argLength: 1, reg: flagsgp, asm: "SETCC"}, // extract floating >= condition from arg0

		{name: "MOVBQSX", argLength: 1, reg: gp11nf, asm: "MOVBQSX"}, // sign extend arg0 from int8 to int64
		{name: "MOVBQZX", argLength: 1, reg: gp11nf, asm: "MOVBQZX"}, // zero extend arg0 from int8 to int64
		{name: "MOVWQSX", argLength: 1, reg: gp11nf, asm: "MOVWQSX"}, // sign extend arg0 from int16 to int64
		{name: "MOVWQZX", argLength: 1, reg: gp11nf, asm: "MOVWQZX"}, // zero extend arg0 from int16 to int64
		{name: "MOVLQSX", argLength: 1, reg: gp11nf, asm: "MOVLQSX"}, // sign extend arg0 from int32 to int64
		{name: "MOVLQZX", argLength: 1, reg: gp11nf, asm: "MOVLQZX"}, // zero extend arg0 from int32 to int64

		{name: "MOVLconst", reg: gp01, asm: "MOVL", typ: "UInt32", aux: "Int32", rematerializeable: true}, // 32 low bits of auxint
		{name: "MOVQconst", reg: gp01, asm: "MOVQ", typ: "UInt64", aux: "Int64", rematerializeable: true}, // auxint

		{name: "CVTTSD2SL", argLength: 1, reg: fpgp, asm: "CVTTSD2SL"}, // convert float64 to int32
		{name: "CVTTSD2SQ", argLength: 1, reg: fpgp, asm: "CVTTSD2SQ"}, // convert float64 to int64
		{name: "CVTTSS2SL", argLength: 1, reg: fpgp, asm: "CVTTSS2SL"}, // convert float32 to int32
		{name: "CVTTSS2SQ", argLength: 1, reg: fpgp, asm: "CVTTSS2SQ"}, // convert float32 to int64
		{name: "CVTSL2SS", argLength: 1, reg: gpfp, asm: "CVTSL2SS"},   // convert int32 to float32
		{name: "CVTSL2SD", argLength: 1, reg: gpfp, asm: "CVTSL2SD"},   // convert int32 to float64
		{name: "CVTSQ2SS", argLength: 1, reg: gpfp, asm: "CVTSQ2SS"},   // convert int64 to float32
		{name: "CVTSQ2SD", argLength: 1, reg: gpfp, asm: "CVTSQ2SD"},   // convert int64 to float64
		{name: "CVTSD2SS", argLength: 1, reg: fp11, asm: "CVTSD2SS"},   // convert float64 to float32
		{name: "CVTSS2SD", argLength: 1, reg: fp11, asm: "CVTSS2SD"},   // convert float32 to float64

		{name: "PXOR", argLength: 2, reg: fp21, asm: "PXOR", commutative: true, resultInArg0: true}, // exclusive or, applied to X regs for float negation.

		{name: "LEAQ", argLength: 1, reg: gp11sb, aux: "SymOff", rematerializeable: true}, // arg0 + auxint + offset encoded in aux
		{name: "LEAQ1", argLength: 2, reg: gp21sb, aux: "SymOff"},                         // arg0 + arg1 + auxint + aux
		{name: "LEAQ2", argLength: 2, reg: gp21sb, aux: "SymOff"},                         // arg0 + 2*arg1 + auxint + aux
		{name: "LEAQ4", argLength: 2, reg: gp21sb, aux: "SymOff"},                         // arg0 + 4*arg1 + auxint + aux
		{name: "LEAQ8", argLength: 2, reg: gp21sb, aux: "SymOff"},                         // arg0 + 8*arg1 + auxint + aux
		// Note: LEAQ{1,2,4,8} must not have OpSB as either argument.

		// auxint+aux == add auxint and the offset of the symbol in aux (if any) to the effective address
		{name: "MOVBload", argLength: 2, reg: gpload, asm: "MOVBLZX", aux: "SymOff", typ: "UInt8"},  // load byte from arg0+auxint+aux. arg1=mem.  Zero extend.
		{name: "MOVBQSXload", argLength: 2, reg: gpload, asm: "MOVBQSX", aux: "SymOff"},             // ditto, sign extend to int64
		{name: "MOVWload", argLength: 2, reg: gpload, asm: "MOVWLZX", aux: "SymOff", typ: "UInt16"}, // load 2 bytes from arg0+auxint+aux. arg1=mem.  Zero extend.
		{name: "MOVWQSXload", argLength: 2, reg: gpload, asm: "MOVWQSX", aux: "SymOff"},             // ditto, sign extend to int64
		{name: "MOVLload", argLength: 2, reg: gpload, asm: "MOVL", aux: "SymOff", typ: "UInt32"},    // load 4 bytes from arg0+auxint+aux. arg1=mem.  Zero extend.
		{name: "MOVLQSXload", argLength: 2, reg: gpload, asm: "MOVLQSX", aux: "SymOff"},             // ditto, sign extend to int64
		{name: "MOVQload", argLength: 2, reg: gpload, asm: "MOVQ", aux: "SymOff", typ: "UInt64"},    // load 8 bytes from arg0+auxint+aux. arg1=mem
		{name: "MOVBstore", argLength: 3, reg: gpstore, asm: "MOVB", aux: "SymOff", typ: "Mem"},     // store byte in arg1 to arg0+auxint+aux. arg2=mem
		{name: "MOVWstore", argLength: 3, reg: gpstore, asm: "MOVW", aux: "SymOff", typ: "Mem"},     // store 2 bytes in arg1 to arg0+auxint+aux. arg2=mem
		{name: "MOVLstore", argLength: 3, reg: gpstore, asm: "MOVL", aux: "SymOff", typ: "Mem"},     // store 4 bytes in arg1 to arg0+auxint+aux. arg2=mem
		{name: "MOVQstore", argLength: 3, reg: gpstore, asm: "MOVQ", aux: "SymOff", typ: "Mem"},     // store 8 bytes in arg1 to arg0+auxint+aux. arg2=mem
		{name: "MOVOload", argLength: 2, reg: fpload, asm: "MOVUPS", aux: "SymOff", typ: "Int128"},  // load 16 bytes from arg0+auxint+aux. arg1=mem
		{name: "MOVOstore", argLength: 3, reg: fpstore, asm: "MOVUPS", aux: "SymOff", typ: "Mem"},   // store 16 bytes in arg1 to arg0+auxint+aux. arg2=mem

		// indexed loads/stores
		{name: "MOVBloadidx1", argLength: 3, reg: gploadidx, asm: "MOVBLZX", aux: "SymOff"}, // load a byte from arg0+arg1+auxint+aux. arg2=mem
		{name: "MOVWloadidx1", argLength: 3, reg: gploadidx, asm: "MOVWLZX", aux: "SymOff"}, // load 2 bytes from arg0+arg1+auxint+aux. arg2=mem
		{name: "MOVWloadidx2", argLength: 3, reg: gploadidx, asm: "MOVWLZX", aux: "SymOff"}, // load 2 bytes from arg0+2*arg1+auxint+aux. arg2=mem
		{name: "MOVLloadidx1", argLength: 3, reg: gploadidx, asm: "MOVL", aux: "SymOff"},    // load 4 bytes from arg0+arg1+auxint+aux. arg2=mem
		{name: "MOVLloadidx4", argLength: 3, reg: gploadidx, asm: "MOVL", aux: "SymOff"},    // load 4 bytes from arg0+4*arg1+auxint+aux. arg2=mem
		{name: "MOVQloadidx1", argLength: 3, reg: gploadidx, asm: "MOVQ", aux: "SymOff"},    // load 8 bytes from arg0+arg1+auxint+aux. arg2=mem
		{name: "MOVQloadidx8", argLength: 3, reg: gploadidx, asm: "MOVQ", aux: "SymOff"},    // load 8 bytes from arg0+8*arg1+auxint+aux. arg2=mem
		// TODO: sign-extending indexed loads
		{name: "MOVBstoreidx1", argLength: 4, reg: gpstoreidx, asm: "MOVB", aux: "SymOff"}, // store byte in arg2 to arg0+arg1+auxint+aux. arg3=mem
		{name: "MOVWstoreidx1", argLength: 4, reg: gpstoreidx, asm: "MOVW", aux: "SymOff"}, // store 2 bytes in arg2 to arg0+arg1+auxint+aux. arg3=mem
		{name: "MOVWstoreidx2", argLength: 4, reg: gpstoreidx, asm: "MOVW", aux: "SymOff"}, // store 2 bytes in arg2 to arg0+2*arg1+auxint+aux. arg3=mem
		{name: "MOVLstoreidx1", argLength: 4, reg: gpstoreidx, asm: "MOVL", aux: "SymOff"}, // store 4 bytes in arg2 to arg0+arg1+auxint+aux. arg3=mem
		{name: "MOVLstoreidx4", argLength: 4, reg: gpstoreidx, asm: "MOVL", aux: "SymOff"}, // store 4 bytes in arg2 to arg0+4*arg1+auxint+aux. arg3=mem
		{name: "MOVQstoreidx1", argLength: 4, reg: gpstoreidx, asm: "MOVQ", aux: "SymOff"}, // store 8 bytes in arg2 to arg0+arg1+auxint+aux. arg3=mem
		{name: "MOVQstoreidx8", argLength: 4, reg: gpstoreidx, asm: "MOVQ", aux: "SymOff"}, // store 8 bytes in arg2 to arg0+8*arg1+auxint+aux. arg3=mem
		// TODO: add size-mismatched indexed loads, like MOVBstoreidx4.

		// For storeconst ops, the AuxInt field encodes both
		// the value to store and an address offset of the store.
		// Cast AuxInt to a ValAndOff to extract Val and Off fields.
		{name: "MOVBstoreconst", argLength: 2, reg: gpstoreconst, asm: "MOVB", aux: "SymValAndOff", typ: "Mem"}, // store low byte of ValAndOff(AuxInt).Val() to arg0+ValAndOff(AuxInt).Off()+aux.  arg1=mem
		{name: "MOVWstoreconst", argLength: 2, reg: gpstoreconst, asm: "MOVW", aux: "SymValAndOff", typ: "Mem"}, // store low 2 bytes of ...
		{name: "MOVLstoreconst", argLength: 2, reg: gpstoreconst, asm: "MOVL", aux: "SymValAndOff", typ: "Mem"}, // store low 4 bytes of ...
		{name: "MOVQstoreconst", argLength: 2, reg: gpstoreconst, asm: "MOVQ", aux: "SymValAndOff", typ: "Mem"}, // store 8 bytes of ...

		{name: "MOVBstoreconstidx1", argLength: 3, reg: gpstoreconstidx, asm: "MOVB", aux: "SymValAndOff", typ: "Mem"}, // store low byte of ValAndOff(AuxInt).Val() to arg0+1*arg1+ValAndOff(AuxInt).Off()+aux.  arg2=mem
		{name: "MOVWstoreconstidx1", argLength: 3, reg: gpstoreconstidx, asm: "MOVW", aux: "SymValAndOff", typ: "Mem"}, // store low 2 bytes of ... arg1 ...
		{name: "MOVWstoreconstidx2", argLength: 3, reg: gpstoreconstidx, asm: "MOVW", aux: "SymValAndOff", typ: "Mem"}, // store low 2 bytes of ... 2*arg1 ...
		{name: "MOVLstoreconstidx1", argLength: 3, reg: gpstoreconstidx, asm: "MOVL", aux: "SymValAndOff", typ: "Mem"}, // store low 4 bytes of ... arg1 ...
		{name: "MOVLstoreconstidx4", argLength: 3, reg: gpstoreconstidx, asm: "MOVL", aux: "SymValAndOff", typ: "Mem"}, // store low 4 bytes of ... 4*arg1 ...
		{name: "MOVQstoreconstidx1", argLength: 3, reg: gpstoreconstidx, asm: "MOVQ", aux: "SymValAndOff", typ: "Mem"}, // store 8 bytes of ... arg1 ...
		{name: "MOVQstoreconstidx8", argLength: 3, reg: gpstoreconstidx, asm: "MOVQ", aux: "SymValAndOff", typ: "Mem"}, // store 8 bytes of ... 8*arg1 ...

		// arg0 = (duff-adjusted) pointer to start of memory to zero
		// arg1 = value to store (will always be zero)
		// arg2 = mem
		// auxint = offset into duffzero code to start executing
		// returns mem
		{
			name:      "DUFFZERO",
			aux:       "Int64",
			argLength: 3,
			reg: regInfo{
				inputs:   []regMask{buildReg("DI"), buildReg("X0")},
				clobbers: buildReg("DI FLAGS"),
			},
		},
		{name: "MOVOconst", reg: regInfo{nil, 0, []regMask{fp}}, typ: "Int128", aux: "Int128", rematerializeable: true},

		// arg0 = address of memory to zero
		// arg1 = # of 8-byte words to zero
		// arg2 = value to store (will always be zero)
		// arg3 = mem
		// returns mem
		{
			name:      "REPSTOSQ",
			argLength: 4,
			reg: regInfo{
				inputs:   []regMask{buildReg("DI"), buildReg("CX"), buildReg("AX")},
				clobbers: buildReg("DI CX"),
			},
		},

		{name: "CALLstatic", argLength: 1, reg: regInfo{clobbers: callerSave}, aux: "SymOff"},                                // call static function aux.(*gc.Sym).  arg0=mem, auxint=argsize, returns mem
		{name: "CALLclosure", argLength: 3, reg: regInfo{[]regMask{gpsp, buildReg("DX"), 0}, callerSave, nil}, aux: "Int64"}, // call function via closure.  arg0=codeptr, arg1=closure, arg2=mem, auxint=argsize, returns mem
		{name: "CALLdefer", argLength: 1, reg: regInfo{clobbers: callerSave}, aux: "Int64"},                                  // call deferproc.  arg0=mem, auxint=argsize, returns mem
		{name: "CALLgo", argLength: 1, reg: regInfo{clobbers: callerSave}, aux: "Int64"},                                     // call newproc.  arg0=mem, auxint=argsize, returns mem
		{name: "CALLinter", argLength: 2, reg: regInfo{inputs: []regMask{gp}, clobbers: callerSave}, aux: "Int64"},           // call fn by pointer.  arg0=codeptr, arg1=mem, auxint=argsize, returns mem

		// arg0 = destination pointer
		// arg1 = source pointer
		// arg2 = mem
		// auxint = offset from duffcopy symbol to call
		// returns memory
		{
			name:      "DUFFCOPY",
			aux:       "Int64",
			argLength: 3,
			reg: regInfo{
				inputs:   []regMask{buildReg("DI"), buildReg("SI")},
				clobbers: buildReg("DI SI X0 FLAGS"), // uses X0 as a temporary
			},
		},

		// arg0 = destination pointer
		// arg1 = source pointer
		// arg2 = # of 8-byte words to copy
		// arg3 = mem
		// returns memory
		{
			name:      "REPMOVSQ",
			argLength: 4,
			reg: regInfo{
				inputs:   []regMask{buildReg("DI"), buildReg("SI"), buildReg("CX")},
				clobbers: buildReg("DI SI CX"),
			},
		},

		// (InvertFlags (CMPQ a b)) == (CMPQ b a)
		// So if we want (SETL (CMPQ a b)) but we can't do that because a is a constant,
		// then we do (SETL (InvertFlags (CMPQ b a))) instead.
		// Rewrites will convert this to (SETG (CMPQ b a)).
		// InvertFlags is a pseudo-op which can't appear in assembly output.
		{name: "InvertFlags", argLength: 1}, // reverse direction of arg0

		// Pseudo-ops
		{name: "LoweredGetG", argLength: 1, reg: gp01}, // arg0=mem
		// Scheduler ensures LoweredGetClosurePtr occurs only in entry block,
		// and sorts it to the very beginning of the block to prevent other
		// use of DX (the closure pointer)
		{name: "LoweredGetClosurePtr", reg: regInfo{outputs: []regMask{buildReg("DX")}}},
		//arg0=ptr,arg1=mem, returns void.  Faults if ptr is nil.
		{name: "LoweredNilCheck", argLength: 2, reg: regInfo{inputs: []regMask{gpsp}, clobbers: flags}},

		// MOVQconvert converts between pointers and integers.
		// We have a special op for this so as to not confuse GC
		// (particularly stack maps).  It takes a memory arg so it
		// gets correctly ordered with respect to GC safepoints.
		// arg0=ptr/int arg1=mem, output=int/ptr
		{name: "MOVQconvert", argLength: 2, reg: gp11nf, asm: "MOVQ"},

		// Constant flag values. For any comparison, there are 5 possible
		// outcomes: the three from the signed total order (<,==,>) and the
		// three from the unsigned total order. The == cases overlap.
		// Note: there's a sixth "unordered" outcome for floating-point
		// comparisons, but we don't use such a beast yet.
		// These ops are for temporary use by rewrite rules. They
		// cannot appear in the generated assembly.
		{name: "FlagEQ"},     // equal
		{name: "FlagLT_ULT"}, // signed < and unsigned <
		{name: "FlagLT_UGT"}, // signed < and unsigned >
		{name: "FlagGT_UGT"}, // signed > and unsigned <
		{name: "FlagGT_ULT"}, // signed > and unsigned >
	}

	var AMD64blocks = []blockData{
		{name: "EQ"},
		{name: "NE"},
		{name: "LT"},
		{name: "LE"},
		{name: "GT"},
		{name: "GE"},
		{name: "ULT"},
		{name: "ULE"},
		{name: "UGT"},
		{name: "UGE"},
		{name: "EQF"},
		{name: "NEF"},
		{name: "ORD"}, // FP, ordered comparison (parity zero)
		{name: "NAN"}, // FP, unordered comparison (parity one)
	}

	archs = append(archs, arch{
		name:     "AMD64",
		pkg:      "cmd/internal/obj/x86",
		genfile:  "../../amd64/ssa.go",
		ops:      AMD64ops,
		blocks:   AMD64blocks,
		regnames: regNamesAMD64,
	})
}
