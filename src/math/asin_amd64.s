// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#include "textflag.h"

TEXT ·Asin(SB),NOSPLIT,$0
	JMP ·asin(SB)

TEXT ·Acos(SB),NOSPLIT,$0
	JMP ·acos(SB)
