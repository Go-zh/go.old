// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT _rt0_arm_linux(SB),NOSPLIT,$-4
	MOVW	(R13), R0	// argc
	MOVW	$4(R13), R1		// argv
	MOVW	$_rt0_arm_linux1(SB), R4
	B		(R4)

// When building with -buildmode=c-shared, this symbol is called when the shared
// library is loaded.
TEXT _rt0_arm_linux_lib(SB),NOSPLIT,$32
	// Preserve callee-save registers.  Raspberry Pi's dlopen(), for example,
	// actually cares that R11 is preserved.
	MOVW	R4, 12(R13)
	MOVW	R5, 16(R13)
	MOVW	R6, 20(R13)
	MOVW	R7, 24(R13)
	MOVW	R8, 28(R13)
	MOVW	R11, 32(R13)

	// Save argc/argv.
	MOVW	R0, _rt0_arm_linux_lib_argc<>(SB)
	MOVW	R1, _rt0_arm_linux_lib_argv<>(SB)

	// Create a new thread to do the runtime initialization.
	MOVW	_cgo_sys_thread_create(SB), R2
	CMP	$0, R2
	BEQ	nocgo
	MOVW	$_rt0_arm_linux_lib_go<>(SB), R0
	MOVW	$0, R1
	BL	(R2)
	B	rr
nocgo:
	MOVW	$0x800000, R0                     // stacksize = 8192KB
	MOVW	$_rt0_arm_linux_lib_go<>(SB), R1  // fn
	MOVW	R0, 4(R13)
	MOVW	R1, 8(R13)
	BL	runtime·newosproc0(SB)
rr:
	// Restore callee-save registers and return.
	MOVW	12(R13), R4
	MOVW	16(R13), R5
	MOVW	20(R13), R6
	MOVW	24(R13), R7
	MOVW	28(R13), R8
	MOVW	32(R13), R11
	RET

TEXT _rt0_arm_linux_lib_go<>(SB),NOSPLIT,$8
	MOVW	_rt0_arm_linux_lib_argc<>(SB), R0
	MOVW	_rt0_arm_linux_lib_argv<>(SB), R1
	MOVW	R0, 0(R13)
	MOVW	R1, 4(R13)
	B	runtime·rt0_go(SB)

DATA _rt0_arm_linux_lib_argc<>(SB)/4,$0
GLOBL _rt0_arm_linux_lib_argc<>(SB),NOPTR,$4
DATA _rt0_arm_linux_lib_argv<>(SB)/4,$0
GLOBL _rt0_arm_linux_lib_argv<>(SB),NOPTR,$4

TEXT _rt0_arm_linux1(SB),NOSPLIT,$-4
	// We first need to detect the kernel ABI, and warn the user
	// if the system only supports OABI
	// The strategy here is to call some EABI syscall to see if
	// SIGILL is received.
	// To catch SIGILL, we have to first setup sigaction, this is
	// a chicken-and-egg problem, because we can't do syscall if
	// we don't know the kernel ABI... Oh, not really, we can do
	// syscall in Thumb mode.

	// Save argc and argv
	MOVM.DB.W [R0-R1], (R13)

	// Thumb mode OABI check disabled because there are some
	// EABI systems that do not support Thumb execution.
	// We can run on them except for this check!

	// // set up sa_handler
	// MOVW	$bad_abi<>(SB), R0 // sa_handler
	// MOVW	$0, R1 // sa_flags
	// MOVW	$0, R2 // sa_restorer
	// MOVW	$0, R3 // sa_mask
	// MOVM.DB.W [R0-R3], (R13)
	// MOVW	$4, R0 // SIGILL
	// MOVW	R13, R1 // sa
	// SUB	$16, R13
	// MOVW	R13, R2 // old_sa
	// MOVW	$8, R3 // c
	// MOVW	$174, R7 // sys_sigaction
	// BL	oabi_syscall<>(SB)

	// do an EABI syscall
	MOVW	$20, R7 // sys_getpid
	SWI	$0 // this will trigger SIGILL on OABI systems
	
	// MOVW	$4, R0  // SIGILL
	// MOVW	R13, R1 // sa
	// MOVW	$0, R2 // old_sa
	// MOVW	$8, R3 // c
	// MOVW	$174, R7 // sys_sigaction
	// SWI	$0 // restore signal handler
	// ADD	$32, R13

	B	runtime·rt0_go(SB)

TEXT bad_abi<>(SB),NOSPLIT,$-4
	// give diagnosis and exit
	MOVW	$2, R0 // stderr
	MOVW	$bad_abi_msg(SB), R1 // data
	MOVW	$45, R2 // len
	MOVW	$4, R7 // sys_write
	BL	oabi_syscall<>(SB)
	MOVW	$1, R0
	MOVW	$1, R7 // sys_exit
	BL	oabi_syscall<>(SB)
	B  	0(PC)

DATA bad_abi_msg+0x00(SB)/8, $"This pro"
DATA bad_abi_msg+0x08(SB)/8, $"gram can"
DATA bad_abi_msg+0x10(SB)/8, $" only be"
DATA bad_abi_msg+0x18(SB)/8, $" run on "
DATA bad_abi_msg+0x20(SB)/8, $"EABI ker"
DATA bad_abi_msg+0x28(SB)/4, $"nels"
DATA bad_abi_msg+0x2c(SB)/1, $0xa
GLOBL bad_abi_msg(SB), RODATA, $45

TEXT oabi_syscall<>(SB),NOSPLIT,$-4
	ADD $1, R15, R4 // R15 is hardware PC
	WORD $0xe12fff14 //BX	(R4) // enter thumb mode
	// TODO(minux): only supports little-endian CPUs
	WORD $0x4770df01 // swi $1; bx lr

TEXT main(SB),NOSPLIT,$-4
	MOVW	$_rt0_arm_linux1(SB), R4
	B		(R4)

