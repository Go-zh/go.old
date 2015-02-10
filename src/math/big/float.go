// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements multi-precision floating-point numbers.
// Like in the GNU MPFR library (http://www.mpfr.org/), operands
// can be of mixed precision. Unlike MPFR, the rounding mode is
// not specified with each operation, but with each operand. The
// rounding mode of the result operand determines the rounding
// mode of an operation. This is a from-scratch implementation.

// CAUTION: WORK IN PROGRESS - USE AT YOUR OWN RISK.

package big

import (
	"fmt"
	"math"
)

const debugFloat = true // enable for debugging

// A Float represents a multi-precision floating point number of the form
//
//   sign * mantissa * 2**exponent
//
// with 0.5 <= mantissa < 1.0, and MinExp <= exponent <= MaxExp (with the
// exception of 0 and Inf which have a 0 mantissa and special exponents).
//
// Each Float value also has a precision, rounding mode, and accuracy.
//
// The precision is the number of mantissa bits used to represent the
// value. The rounding mode specifies how a result should be rounded
// to fit into the mantissa bits, and accuracy describes the rounding
// error with respect to the exact result.
//
// All operations, including setters, that specify a *Float for the result,
// usually via the receiver, round their result to the result's precision
// and according to its rounding mode, unless specified otherwise. If the
// result precision is 0 (see below), it is set to the precision of the
// argument with the largest precision value before any rounding takes
// place.
// TODO(gri) should the rounding mode also be copied in this case?
//
// By setting the desired precision to 24 or 53 and using ToNearestEven
// rounding, Float operations produce the same results as the corresponding
// float32 or float64 IEEE-754 arithmetic for normalized operands (no NaNs
// or denormalized numbers). Additionally, positive and negative zeros and
// infinities are fully supported.
//
// The zero (uninitialized) value for a Float is ready to use and
// represents the number +0.0 of 0 bit precision.
//
type Float struct {
	mode RoundingMode
	acc  Accuracy
	neg  bool
	mant nat
	exp  int32
	prec uint // TODO(gri) make this a 32bit field
}

// Internal representation details: The mantissa bits x.mant of a Float x
// are stored in the shortest nat slice long enough to hold x.prec bits.
// Unless x is a zero or an infinity, x.mant is normalized such that the
// msb of x.mant == 1. Thus, if the precision is not a multiple of the
// the Word size _W, x.mant[0] contains trailing zero bits. Zero and Inf
// values have an empty mantissa and a 0 or infExp exponent, respectively.

// NewFloat returns a new Float with value x rounded
// to prec bits according to the given rounding mode.
// If prec == 0, the result has value 0.0 independent
// of the value of x.
// BUG(gri) For prec == 0 and x == Inf, the result
// should be Inf as well.
// TODO(gri) rethink this signature.
func NewFloat(x float64, prec uint, mode RoundingMode) *Float {
	var z Float
	if prec > 0 {
		// TODO(gri) should make this more efficient
		z.SetFloat64(x)
		return z.Round(&z, prec, mode)
	}
	z.mode = mode // TODO(gri) don't do this twice for prec > 0
	return &z
}

const (
	MaxExp = math.MaxInt32 // largest supported exponent magnitude
	infExp = -MaxExp - 1   // exponent for Inf values
)

// NewInf returns a new infinite Float value with value +Inf (sign >= 0),
// or -Inf (sign < 0).
func NewInf(sign int) *Float {
	return &Float{neg: sign < 0, exp: infExp}
}

// Accuracy describes the rounding error produced by the most recent
// operation that generated a Float value, relative to the exact value:
//
//  -1: below exact value
//   0: exact value
//  +1: above exact value
//
type Accuracy int8

// Constants describing the Accuracy of a Float.
const (
	Below Accuracy = -1
	Exact Accuracy = 0
	Above Accuracy = +1
)

func (a Accuracy) String() string {
	switch {
	case a < 0:
		return "below"
	default:
		return "exact"
	case a > 0:
		return "above"
	}
}

// RoundingMode determines how a Float value is rounded to the
// desired precision. Rounding may change the Float value; the
// rounding error is described by the Float's Accuracy.
type RoundingMode uint8

// The following rounding modes are supported.
const (
	ToNearestEven RoundingMode = iota // == IEEE 754-2008 roundTiesToEven
	ToNearestAway                     // == IEEE 754-2008 roundTiesToAway
	ToZero                            // == IEEE 754-2008 roundTowardZero
	AwayFromZero                      // no IEEE 754-2008 equivalent
	ToNegativeInf                     // == IEEE 754-2008 roundTowardNegative
	ToPositiveInf                     // == IEEE 754-2008 roundTowardPositive
)

func (mode RoundingMode) String() string {
	switch mode {
	case ToNearestEven:
		return "ToNearestEven"
	case ToNearestAway:
		return "ToNearestAway"
	case ToZero:
		return "ToZero"
	case AwayFromZero:
		return "AwayFromZero"
	case ToNegativeInf:
		return "ToNegativeInf"
	case ToPositiveInf:
		return "ToPositiveInf"
	}
	panic("unreachable")
}

// Precision returns the mantissa precision of x in bits.
// The precision may be 0 for |x| == 0 or |x| == Inf.
func (x *Float) Precision() uint {
	return uint(x.prec)
}

// Accuracy returns the accuracy of x produced by the most recent operation.
func (x *Float) Accuracy() Accuracy {
	return x.acc
}

// Mode returns the rounding mode of x.
func (x *Float) Mode() RoundingMode {
	return x.mode
}

// IsInf reports whether x is an infinity, according to sign.
// If sign > 0, IsInf reports whether x is positive infinity.
// If sign < 0, IsInf reports whether x is negative infinity.
// If sign == 0, IsInf reports whether x is either infinity.
func (x *Float) IsInf(sign int) bool {
	return x.exp == infExp && (sign == 0 || x.neg == (sign < 0))
}

// setExp sets the exponent for z.
// If the exponent's magnitude is too large, z becomes +/-Inf.
func (z *Float) setExp(e int64) {
	if -MaxExp <= e && e <= MaxExp {
		z.exp = int32(e)
		return
	}
	// Inf
	z.mant = z.mant[:0]
	z.exp = infExp
}

// debugging support
func (x *Float) validate() {
	const msb = 1 << (_W - 1)
	m := len(x.mant)
	if m == 0 {
		// 0.0 or Inf
		if x.exp != 0 && x.exp != infExp {
			panic(fmt.Sprintf("empty matissa with invalid exponent %d", x.exp))
		}
		return
	}
	if x.mant[m-1]&msb == 0 {
		panic(fmt.Sprintf("msb not set in last word %#x of %s", x.mant[m-1], x.Format('p', 0)))
	}
	if x.prec <= 0 {
		panic(fmt.Sprintf("invalid precision %d", x.prec))
	}
}

// round rounds z according to z.mode to z.prec bits and sets z.acc accordingly.
// sbit must be 0 or 1 and summarizes any "sticky bit" information one might
// have before calling round. z's mantissa must be normalized (with the msb set)
// or empty.
func (z *Float) round(sbit uint) {
	z.acc = Exact

	// handle zero and Inf
	m := uint(len(z.mant)) // mantissa length in words for current precision
	if m == 0 {
		if z.exp != infExp {
			z.exp = 0
		}
		return
	}
	// z.prec > 0

	if debugFloat {
		z.validate()
	}

	bits := m * _W // available mantissa bits
	if bits == z.prec {
		// mantissa fits Exactly => nothing to do
		return
	}

	n := (z.prec + (_W - 1)) / _W // mantissa length in words for desired precision
	if bits < z.prec {
		// mantissa too small => extend
		if m < n {
			// slice too short => extend slice
			if int(n) <= cap(z.mant) {
				// reuse existing slice
				z.mant = z.mant[:n]
				copy(z.mant[n-m:], z.mant[:m])
				z.mant[:n-m].clear()
			} else {
				// n > cap(z.mant) => allocate new slice
				const e = 4 // extra capacity (see nat.make)
				new := make(nat, n, n+e)
				copy(new[n-m:], z.mant)
			}
		}
		return
	}

	// Rounding is based on two bits: the rounding bit (rbit) and the
	// sticky bit (sbit). The rbit is the bit immediately before the
	// mantissa bits (the "0.5"). The sbit is set if any of the bits
	// before the rbit are set (the "0.25", "0.125", etc.):
	//
	//   rbit  sbit  => "fractional part"
	//
	//   0     0        == 0
	//   0     1        >  0  , < 0.5
	//   1     0        == 0.5
	//   1     1        >  0.5, < 1.0

	// bits > z.prec: mantissa too large => round
	r := bits - z.prec - 1 // rounding bit position; r >= 0
	rbit := z.mant.bit(r)  // rounding bit
	if sbit == 0 {
		sbit = z.mant.sticky(r)
	}
	if debugFloat && sbit&^1 != 0 {
		panic(fmt.Sprintf("invalid sbit %#x", sbit))
	}

	// convert ToXInf rounding modes
	mode := z.mode
	switch mode {
	case ToNegativeInf:
		mode = ToZero
		if z.neg {
			mode = AwayFromZero
		}
	case ToPositiveInf:
		mode = AwayFromZero
		if z.neg {
			mode = ToZero
		}
	}

	// cut off extra words
	if m > n {
		copy(z.mant, z.mant[m-n:]) // move n last words to front
		z.mant = z.mant[:n]
	}

	// determine number of trailing zero bits t
	t := n*_W - z.prec // 0 <= t < _W
	lsb := Word(1) << t

	// make rounding decision
	// TODO(gri) This can be simplified (see roundBits in float_test.go).
	switch mode {
	case ToZero:
		// nothing to do
	case ToNearestEven, ToNearestAway:
		if rbit == 0 {
			// rounding bits == 0b0x
			mode = ToZero
		} else if sbit == 1 {
			// rounding bits == 0b11
			mode = AwayFromZero
		}
	case AwayFromZero:
		if rbit|sbit == 0 {
			mode = ToZero
		}
	default:
		// ToXInf modes have been converted to ToZero or AwayFromZero
		panic("unreachable")
	}

	// round and determine accuracy
	switch mode {
	case ToZero:
		if rbit|sbit != 0 {
			z.acc = Below
		}

	case ToNearestEven, ToNearestAway:
		if debugFloat && rbit != 1 {
			panic("internal error in rounding")
		}
		if mode == ToNearestEven && sbit == 0 && z.mant[0]&lsb == 0 {
			z.acc = Below
			break
		}
		// mode == ToNearestAway || sbit == 1 || z.mant[0]&lsb != 0
		fallthrough

	case AwayFromZero:
		// add 1 to mantissa
		if addVW(z.mant, z.mant, lsb) != 0 {
			// overflow => shift mantissa right by 1 and add msb
			shrVU(z.mant, z.mant, 1)
			z.mant[n-1] |= 1 << (_W - 1)
			// adjust exponent
			z.exp++
		}
		z.acc = Above
	}

	// zero out trailing bits in least-significant word
	z.mant[0] &^= lsb - 1

	// update accuracy
	if z.neg {
		z.acc = -z.acc
	}

	if debugFloat {
		z.validate()
	}

	return
}

// Round sets z to the value of x rounded according to mode to prec bits and returns z.
// TODO(gri) rethink this signature.
// TODO(gri) adjust this to match precision semantics.
func (z *Float) Round(x *Float, prec uint, mode RoundingMode) *Float {
	z.Set(x)
	z.prec = prec
	z.mode = mode
	z.round(0)
	return z
}

// nlz returns the number of leading zero bits in x.
func nlz(x Word) uint {
	return _W - uint(bitLen(x))
}

func nlz64(x uint64) uint {
	// TODO(gri) this can be done more nicely
	if _W == 32 {
		if x>>32 == 0 {
			return 32 + nlz(Word(x))
		}
		return nlz(Word(x >> 32))
	}
	if _W == 64 {
		return nlz(Word(x))
	}
	panic("unreachable")
}

// SetUint64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 64 (and rounding will have
// no effect).
func (z *Float) SetUint64(x uint64) *Float {
	if z.prec == 0 {
		z.prec = 64
	}
	z.acc = Exact
	z.neg = false
	if x == 0 {
		z.mant = z.mant[:0]
		z.exp = 0
		return z
	}
	// x != 0
	s := nlz64(x)
	z.mant = z.mant.setUint64(x << s)
	z.exp = int32(64 - s) // always fits
	if z.prec < 64 {
		z.round(0)
	}
	return z
}

// SetInt64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 64 (and rounding will have
// no effect).
func (z *Float) SetInt64(x int64) *Float {
	u := x
	if u < 0 {
		u = -u
	}
	z.SetUint64(uint64(u))
	z.neg = x < 0
	return z
}

// SetFloat64 sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to 53 (and rounding will have
// no effect).
// If x is denormalized or NaN, the result is unspecified.
// TODO(gri) should return nil in those cases
func (z *Float) SetFloat64(x float64) *Float {
	if z.prec == 0 {
		z.prec = 53
	}
	z.acc = Exact
	z.neg = math.Signbit(x) // handle -0 correctly
	if math.IsInf(x, 0) {
		z.mant = z.mant[:0]
		z.exp = infExp
		return z
	}
	if x == 0 {
		z.mant = z.mant[:0]
		z.exp = 0
		return z
	}
	// x != 0
	fmant, exp := math.Frexp(x) // get normalized mantissa
	z.mant = z.mant.setUint64(1<<63 | math.Float64bits(fmant)<<11)
	z.exp = int32(exp) // always fits
	if z.prec < 53 {
		z.round(0)
	}
	return z
}

// fnorm normalizes mantissa m by shifting it to the left
// such that the msb of the most-significant word (msw) is 1.
// It returns the shift amount. It assumes that len(m) != 0.
func fnorm(m nat) uint {
	if debugFloat && (len(m) == 0 || m[len(m)-1] == 0) {
		panic("msw of mantissa is 0")
	}
	s := nlz(m[len(m)-1])
	if s > 0 {
		c := shlVU(m, m, s)
		if debugFloat && c != 0 {
			panic("nlz or shlVU incorrect")
		}
	}
	return s
}

// SetInt sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to x.BitLen() (and rounding will have
// no effect).
func (z *Float) SetInt(x *Int) *Float {
	// TODO(gri) can be more efficient if z.prec > 0
	// but small compared to the size of x, or if there
	// are many trailing 0's.
	bits := uint(x.BitLen())
	if z.prec == 0 {
		z.prec = bits
	}
	z.acc = Exact
	z.neg = x.neg
	if len(x.abs) == 0 {
		z.mant = z.mant[:0]
		z.exp = 0
		return z
	}
	// x != 0
	z.mant = z.mant.set(x.abs)
	fnorm(z.mant)
	z.setExp(int64(bits))
	if z.prec < bits {
		z.round(0)
	}
	return z
}

// SetRat sets z to the (possibly rounded) value of x and returns z.
// If z's precision is 0, it is changed to the larger of a.BitLen()
// and b.BitLen(), where a and b are the numerator and denominator
// of x, respectively (x = a/b).
func (z *Float) SetRat(x *Rat) *Float {
	// TODO(gri) can be more efficient if x is an integer
	var a, b Float
	a.SetInt(x.Num())
	b.SetInt(x.Denom())
	if z.prec == 0 {
		z.prec = umax(a.prec, b.prec)
	}
	return z.Quo(&a, &b)
}

// Set sets z to x, with the same precision as x, and returns z.
// TODO(gri) adjust this to match precision semantics.
func (z *Float) Set(x *Float) *Float {
	if z != x {
		z.neg = x.neg
		z.exp = x.exp
		z.mant = z.mant.set(x.mant)
		z.prec = x.prec
	}
	return z
}

func high64(x nat) uint64 {
	i := len(x)
	if i == 0 {
		return 0
	}
	// i > 0
	v := uint64(x[i-1])
	if _W == 32 {
		v <<= 32
		if i > 1 {
			v |= uint64(x[i-2])
		}
	}
	return v
}

// TODO(gri) FIX THIS (Inf, rounding mode, errors, accuracy, etc.)
func (x *Float) Uint64() uint64 {
	m := high64(x.mant)
	s := x.exp
	if s >= 0 {
		return m >> (64 - uint(s))
	}
	return 0 // imprecise
}

// TODO(gri) FIX THIS (inf, rounding mode, errors, etc.)
func (x *Float) Int64() int64 {
	v := int64(x.Uint64())
	if x.neg {
		return -v
	}
	return v
}

// Float64 returns the closest float64 value of x
// by rounding to nearest with 53 bits precision.
// TODO(gri) implement/document error scenarios.
func (x *Float) Float64() (float64, Accuracy) {
	// x == +/-Inf
	if x.exp == infExp {
		var sign int
		if x.neg {
			sign = -1
		}
		return math.Inf(sign), Exact
	}
	// x == 0
	if len(x.mant) == 0 {
		return 0, Exact
	}
	// x != 0
	r := new(Float).Round(x, 53, ToNearestEven)
	var s uint64
	if r.neg {
		s = 1 << 63
	}
	e := uint64(1022+r.exp) & 0x7ff // TODO(gri) check for overflow
	m := high64(r.mant) >> 11 & (1<<52 - 1)
	return math.Float64frombits(s | e<<52 | m), r.acc
}

func (x *Float) Int() *Int {
	if len(x.mant) == 0 {
		return new(Int)
	}
	panic("unimplemented")
}

func (x *Float) Rat() *Rat {
	panic("unimplemented")
}

func (x *Float) IsInt() bool {
	if len(x.mant) == 0 {
		return true
	}
	if x.exp <= 0 {
		return false
	}
	if uint(x.exp) >= x.prec {
		return true
	}
	panic("unimplemented")
}

// Abs sets z to |x| (the absolute value of x) and returns z.
// TODO(gri) adjust this to match precision semantics.
func (z *Float) Abs(x *Float) *Float {
	z.Set(x)
	z.neg = false
	return z
}

// Neg sets z to x with its sign negated, and returns z.
// TODO(gri) adjust this to match precision semantics.
func (z *Float) Neg(x *Float) *Float {
	z.Set(x)
	z.neg = !z.neg
	return z
}

// z = x + y, ignoring signs of x and y.
// x and y must not be 0 or an Inf.
func (z *Float) uadd(x, y *Float) {
	// Note: This implementation requires 2 shifts most of the
	// time. It is also inefficient if exponents or precisions
	// differ by wide margins. The following article describes
	// an efficient (but much more complicated) implementation
	// compatible with the internal representation used here:
	//
	// Vincent Lefèvre: "The Generic Multiple-Precision Floating-
	// Point Addition With Exact Rounding (as in the MPFR Library)"
	// http://www.vinc17.net/research/papers/rnc6.pdf

	if debugFloat && (len(x.mant) == 0 || len(y.mant) == 0) {
		panic("uadd called with 0 argument")
	}

	// compute exponents ex, ey for mantissa with "binary point"
	// on the right (mantissa.0) - use int64 to avoid overflow
	ex := int64(x.exp) - int64(len(x.mant))*_W
	ey := int64(y.exp) - int64(len(y.mant))*_W

	// TODO(gri) having a combined add-and-shift primitive
	//           could make this code significantly faster
	switch {
	case ex < ey:
		t := z.mant.shl(y.mant, uint(ey-ex))
		z.mant = t.add(x.mant, t)
	default:
		// ex == ey, no shift needed
		z.mant = z.mant.add(x.mant, y.mant)
	case ex > ey:
		t := z.mant.shl(x.mant, uint(ex-ey))
		z.mant = t.add(t, y.mant)
		ex = ey
	}
	// len(z.mant) > 0

	z.setExp(ex + int64(len(z.mant))*_W - int64(fnorm(z.mant)))
	z.round(0)
}

// z = x - y for x >= y, ignoring signs of x and y.
// x and y must not be 0 or an Inf.
func (z *Float) usub(x, y *Float) {
	// This code is symmetric to uadd.
	// We have not factored the common code out because
	// eventually uadd (and usub) should be optimized
	// by special-casing, and the code will diverge.

	if debugFloat && (len(x.mant) == 0 || len(y.mant) == 0) {
		panic("usub called with 0 argument")
	}

	ex := int64(x.exp) - int64(len(x.mant))*_W
	ey := int64(y.exp) - int64(len(y.mant))*_W

	switch {
	case ex < ey:
		t := z.mant.shl(y.mant, uint(ey-ex))
		z.mant = t.sub(x.mant, t)
	default:
		// ex == ey, no shift needed
		z.mant = z.mant.sub(x.mant, y.mant)
	case ex > ey:
		t := z.mant.shl(x.mant, uint(ex-ey))
		z.mant = t.sub(t, y.mant)
		ex = ey
	}

	// operands may have cancelled each other out
	if len(z.mant) == 0 {
		z.acc = Exact
		z.setExp(0)
		return
	}
	// len(z.mant) > 0

	z.setExp(ex + int64(len(z.mant))*_W - int64(fnorm(z.mant)))
	z.round(0)
}

// z = x * y, ignoring signs of x and y.
// x and y must not be 0 or an Inf.
func (z *Float) umul(x, y *Float) {
	if debugFloat && (len(x.mant) == 0 || len(y.mant) == 0) {
		panic("umul called with 0 argument")
	}

	// Note: This is doing too much work if the precision
	// of z is less than the sum of the precisions of x
	// and y which is often the case (e.g., if all floats
	// have the same precision).
	// TODO(gri) Optimize this for the common case.

	e := int64(x.exp) + int64(y.exp)
	z.mant = z.mant.mul(x.mant, y.mant)

	// normalize mantissa
	z.setExp(e - int64(fnorm(z.mant)))
	z.round(0)
}

// z = x / y, ignoring signs of x and y.
// x and y must not be 0 or an Inf.
func (z *Float) uquo(x, y *Float) {
	if debugFloat && (len(x.mant) == 0 || len(y.mant) == 0) {
		panic("uquo called with 0 argument")
	}

	// mantissa length in words for desired result precision + 1
	// (at least one extra bit so we get the rounding bit after
	// the division)
	n := int(z.prec/_W) + 1

	// compute adjusted x.mant such that we get enough result precision
	xadj := x.mant
	if d := n - len(x.mant) + len(y.mant); d > 0 {
		// d extra words needed => add d "0 digits" to x
		xadj = make(nat, len(x.mant)+d)
		copy(xadj[d:], x.mant)
	}
	// TODO(gri): If we have too many digits (d < 0), we should be able
	// to shorten x for faster division. But we must be extra careful
	// with rounding in that case.

	// divide
	var r nat
	z.mant, r = z.mant.div(nil, xadj, y.mant)

	// determine exponent
	e := int64(x.exp) - int64(y.exp) - int64(len(xadj)-len(y.mant)-len(z.mant))*_W

	// normalize mantissa
	z.setExp(e - int64(fnorm(z.mant)))

	// The result is long enough to include (at least) the rounding bit.
	// If there's a non-zero remainder, the corresponding fractional part
	// (if it were computed), would have a non-zero sticky bit (if it were
	// zero, it couldn't have a non-zero remainder).
	var sbit uint
	if len(r) > 0 {
		sbit = 1
	}
	z.round(sbit)
}

// ucmp returns -1, 0, or 1, depending on whether x < y, x == y, or x > y,
// while ignoring the signs of x and y. x and y must not be 0 or an Inf.
func (x *Float) ucmp(y *Float) int {
	if debugFloat && (len(x.mant) == 0 || len(y.mant) == 0) {
		panic("ucmp called with 0 argument")
	}

	switch {
	case x.exp < y.exp:
		return -1
	case x.exp > y.exp:
		return 1
	}
	// x.exp == y.exp

	// compare mantissas
	i := len(x.mant)
	j := len(y.mant)
	for i > 0 || j > 0 {
		var xm, ym Word
		if i > 0 {
			i--
			xm = x.mant[i]
		}
		if j > 0 {
			j--
			ym = y.mant[j]
		}
		switch {
		case xm < ym:
			return -1
		case xm > ym:
			return 1
		}
	}

	return 0
}

// Handling of sign bit as defined by IEEE 754-2008,
// section 6.3 (note that there are no NaN Floats):
//
// When neither the inputs nor result are NaN, the sign of a product or
// quotient is the exclusive OR of the operands’ signs; the sign of a sum,
// or of a difference x−y regarded as a sum x+(−y), differs from at most
// one of the addends’ signs; and the sign of the result of conversions,
// the quantize operation, the roundToIntegral operations, and the
// roundToIntegralExact (see 5.3.1) is the sign of the first or only operand.
// These rules shall apply even when operands or results are zero or infinite.
//
// When the sum of two operands with opposite signs (or the difference of
// two operands with like signs) is exactly zero, the sign of that sum (or
// difference) shall be +0 in all rounding-direction attributes except
// roundTowardNegative; under that attribute, the sign of an exact zero
// sum (or difference) shall be −0. However, x+x = x−(−x) retains the same
// sign as x even when x is zero.

// Add sets z to the rounded sum x+y and returns z.
// If z's precision is 0, it is changed to the larger
// of x's or y's precision before the operation.
// Rounding is performed according to z's precision
// and rounding mode; and z's accuracy reports the
// result error relative to the exact (not rounded)
// result.
func (z *Float) Add(x, y *Float) *Float {
	if z.prec == 0 {
		z.prec = umax(x.prec, y.prec)
	}

	// TODO(gri) what about -0?
	if len(y.mant) == 0 {
		// TODO(gri) handle Inf
		return z.Round(x, z.prec, z.mode)
	}
	if len(x.mant) == 0 {
		// TODO(gri) handle Inf
		return z.Round(y, z.prec, z.mode)
	}

	// x, y != 0
	neg := x.neg
	if x.neg == y.neg {
		// x + y == x + y
		// (-x) + (-y) == -(x + y)
		z.uadd(x, y)
	} else {
		// x + (-y) == x - y == -(y - x)
		// (-x) + y == y - x == -(x - y)
		if x.ucmp(y) >= 0 {
			z.usub(x, y)
		} else {
			neg = !neg
			z.usub(y, x)
		}
	}
	z.neg = neg
	return z
}

// Sub sets z to the rounded difference x-y and returns z.
// Precision, rounding, and accuracy reporting are as for Add.
func (z *Float) Sub(x, y *Float) *Float {
	if z.prec == 0 {
		z.prec = umax(x.prec, y.prec)
	}

	// TODO(gri) what about -0?
	if len(y.mant) == 0 {
		// TODO(gri) handle Inf
		return z.Round(x, z.prec, z.mode)
	}
	if len(x.mant) == 0 {
		prec := z.prec
		mode := z.mode
		z.Neg(y)
		return z.Round(z, prec, mode)
	}

	// x, y != 0
	neg := x.neg
	if x.neg != y.neg {
		// x - (-y) == x + y
		// (-x) - y == -(x + y)
		z.uadd(x, y)
	} else {
		// x - y == x - y == -(y - x)
		// (-x) - (-y) == y - x == -(x - y)
		if x.ucmp(y) >= 0 {
			z.usub(x, y)
		} else {
			neg = !neg
			z.usub(y, x)
		}
	}
	z.neg = neg
	return z
}

// Mul sets z to the rounded product x*y and returns z.
// Precision, rounding, and accuracy reporting are as for Add.
func (z *Float) Mul(x, y *Float) *Float {
	if z.prec == 0 {
		z.prec = umax(x.prec, y.prec)
	}

	// TODO(gri) handle Inf

	// TODO(gri) what about -0?
	if len(x.mant) == 0 || len(y.mant) == 0 {
		z.neg = false
		z.mant = z.mant[:0]
		z.exp = 0
		z.acc = Exact
		return z
	}

	// x, y != 0
	z.umul(x, y)
	z.neg = x.neg != y.neg
	return z
}

// Quo sets z to the rounded quotient x/y and returns z.
// If y == 0, a division-by-zero run-time panic occurs. TODO(gri) this should become Inf
// Precision, rounding, and accuracy reporting are as for Add.
func (z *Float) Quo(x, y *Float) *Float {
	if z.prec == 0 {
		z.prec = umax(x.prec, y.prec)
	}

	// TODO(gri) handle Inf

	// TODO(gri) check that this is correct
	z.neg = x.neg != y.neg

	if len(y.mant) == 0 {
		z.setExp(infExp)
		return z
	}

	if len(x.mant) == 0 {
		z.mant = z.mant[:0]
		z.exp = 0
		z.acc = Exact
		return z
	}

	// x, y != 0
	z.uquo(x, y)
	return z
}

// Lsh sets z to the rounded x * (1<<s) and returns z.
// If z's precision is 0, it is changed to x's precision.
// Rounding is performed according to z's precision
// and rounding mode; and z's accuracy reports the
// result error relative to the exact (not rounded)
// result.
func (z *Float) Lsh(x *Float, s uint, mode RoundingMode) *Float {
	if z.prec == 0 {
		z.prec = x.prec
	}

	// TODO(gri) handle Inf

	z.Round(x, z.prec, mode)
	z.setExp(int64(z.exp) + int64(s))
	return z
}

// Rsh sets z to the rounded x / (1<<s) and returns z.
// Precision, rounding, and accuracy reporting are as for Lsh.
func (z *Float) Rsh(x *Float, s uint, mode RoundingMode) *Float {
	if z.prec == 0 {
		z.prec = x.prec
	}

	// TODO(gri) handle Inf

	z.Round(x, z.prec, mode)
	z.setExp(int64(z.exp) - int64(s))
	return z
}

// Cmp compares x and y and returns:
//
//   -1 if x <  y
//    0 if x == y (incl. -0 == 0)
//   +1 if x >  y
//
func (x *Float) Cmp(y *Float) int {
	// TODO(gri) handle Inf

	// special cases
	switch {
	case len(x.mant) == 0:
		// 0 cmp y == -sign(y)
		return -y.Sign()
	case len(y.mant) == 0:
		// x cmp 0 == sign(x)
		return x.Sign()
	}
	// x != 0 && y != 0

	// x cmp y == x cmp y
	// x cmp (-y) == 1
	// (-x) cmp y == -1
	// (-x) cmp (-y) == -(x cmp y)
	switch {
	case x.neg == y.neg:
		r := x.ucmp(y)
		if x.neg {
			r = -r
		}
		return r
	case x.neg:
		return -1
	default:
		return 1
	}
	return 0
}

// Sign returns:
//
//	-1 if x <  0
//	 0 if x == 0 (incl. x == -0) // TODO(gri) is this correct?
//	+1 if x >  0
//
func (x *Float) Sign() int {
	if len(x.mant) == 0 {
		return 0
	}
	if x.neg {
		return -1
	}
	return 1
}

func umax(x, y uint) uint {
	if x > y {
		return x
	}
	return y
}
