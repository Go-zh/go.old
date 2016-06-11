// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gc

import (
	"cmd/compile/internal/big"
	"fmt"
)

// implements integer arithmetic

// Mpint represents an integer constant.
type Mpint struct {
	Val  big.Int
	Ovf  bool // set if Val overflowed compiler limit (sticky)
	Rune bool // set if syntax indicates default type rune
}

func (a *Mpint) SetOverflow() {
	a.Val.SetUint64(1) // avoid spurious div-zero errors
	a.Ovf = true
}

func (a *Mpint) checkOverflow(extra int) bool {
	// We don't need to be precise here, any reasonable upper limit would do.
	// For now, use existing limit so we pass all the tests unchanged.
	if a.Val.BitLen()+extra > Mpprec {
		a.SetOverflow()
	}
	return a.Ovf
}

func (a *Mpint) Set(b *Mpint) {
	a.Val.Set(&b.Val)
}

func (a *Mpint) SetFloat(b *Mpflt) int {
	// avoid converting huge floating-point numbers to integers
	// (2*Mpprec is large enough to permit all tests to pass)
	if b.Val.MantExp(nil) > 2*Mpprec {
		return -1
	}

	if _, acc := b.Val.Int(&a.Val); acc == big.Exact {
		return 0
	}

	const delta = 16 // a reasonably small number of bits > 0
	var t big.Float
	t.SetPrec(Mpprec - delta)

	// try rounding down a little
	t.SetMode(big.ToZero)
	t.Set(&b.Val)
	if _, acc := t.Int(&a.Val); acc == big.Exact {
		return 0
	}

	// try rounding up a little
	t.SetMode(big.AwayFromZero)
	t.Set(&b.Val)
	if _, acc := t.Int(&a.Val); acc == big.Exact {
		return 0
	}

	return -1
}

func (a *Mpint) Add(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpaddfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Add(&a.Val, &b.Val)

	if a.checkOverflow(0) {
		Yyerror("constant addition overflow")
	}
}

func (a *Mpint) Sub(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpsubfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Sub(&a.Val, &b.Val)

	if a.checkOverflow(0) {
		Yyerror("constant subtraction overflow")
	}
}

func (a *Mpint) Mul(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpmulfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Mul(&a.Val, &b.Val)

	if a.checkOverflow(0) {
		Yyerror("constant multiplication overflow")
	}
}

func (a *Mpint) Quo(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpdivfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Quo(&a.Val, &b.Val)

	if a.checkOverflow(0) {
		// can only happen for div-0 which should be checked elsewhere
		Yyerror("constant division overflow")
	}
}

func (a *Mpint) Rem(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpmodfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Rem(&a.Val, &b.Val)

	if a.checkOverflow(0) {
		// should never happen
		Yyerror("constant modulo overflow")
	}
}

func (a *Mpint) Or(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mporfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Or(&a.Val, &b.Val)
}

func (a *Mpint) And(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpandfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.And(&a.Val, &b.Val)
}

func (a *Mpint) AndNot(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpandnotfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.AndNot(&a.Val, &b.Val)
}

func (a *Mpint) Xor(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mpxorfixfix")
		}
		a.SetOverflow()
		return
	}

	a.Val.Xor(&a.Val, &b.Val)
}

func (a *Mpint) Lsh(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mplshfixfix")
		}
		a.SetOverflow()
		return
	}

	s := b.Int64()
	if s < 0 || s >= Mpprec {
		msg := "shift count too large"
		if s < 0 {
			msg = "invalid negative shift count"
		}
		Yyerror("%s: %d", msg, s)
		a.SetInt64(0)
		return
	}

	if a.checkOverflow(int(s)) {
		Yyerror("constant shift overflow")
		return
	}
	a.Val.Lsh(&a.Val, uint(s))
}

func (a *Mpint) Rsh(b *Mpint) {
	if a.Ovf || b.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("ovf in mprshfixfix")
		}
		a.SetOverflow()
		return
	}

	s := b.Int64()
	if s < 0 {
		Yyerror("invalid negative shift count: %d", s)
		if a.Val.Sign() < 0 {
			a.SetInt64(-1)
		} else {
			a.SetInt64(0)
		}
		return
	}

	a.Val.Rsh(&a.Val, uint(s))
}

func (a *Mpint) Cmp(b *Mpint) int {
	return a.Val.Cmp(&b.Val)
}

func (a *Mpint) CmpInt64(c int64) int {
	if c == 0 {
		return a.Val.Sign() // common case shortcut
	}
	return a.Val.Cmp(big.NewInt(c))
}

func (a *Mpint) Neg() {
	a.Val.Neg(&a.Val)
}

func (a *Mpint) Int64() int64 {
	if a.Ovf {
		if nsavederrors+nerrors == 0 {
			Yyerror("constant overflow")
		}
		return 0
	}

	return a.Val.Int64()
}

func (a *Mpint) SetInt64(c int64) {
	a.Val.SetInt64(c)
}

func (a *Mpint) SetString(as string) {
	_, ok := a.Val.SetString(as, 0)
	if !ok {
		// required syntax is [+-][0[x]]d*
		// At the moment we lose precise error cause;
		// the old code distinguished between:
		// - malformed hex constant
		// - malformed octal constant
		// - malformed decimal constant
		// TODO(gri) use different conversion function
		Yyerror("malformed integer constant: %s", as)
		a.Val.SetUint64(0)
		return
	}
	if a.checkOverflow(0) {
		Yyerror("constant too large: %s", as)
	}
}

func (x *Mpint) String() string {
	return bconv(x, 0)
}

func bconv(xval *Mpint, flag FmtFlag) string {
	if flag&FmtSharp != 0 {
		return fmt.Sprintf("%#x", &xval.Val)
	}
	return xval.Val.String()
}
