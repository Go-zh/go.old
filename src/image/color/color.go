// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package color implements a basic color library.

// color 包实现了基本的颜色库。
package color

// Color can convert itself to alpha-premultiplied 16-bits per channel RGBA.
// The conversion may be lossy.

// Color可以将它自己转化成每个RGBA通道都预乘透明度。
// 这种转化可能是有损的。
type Color interface {
	// RGBA returns the alpha-premultiplied red, green, blue and alpha values
	// for the color. Each value ranges within [0, 0xffff], but is represented
	// by a uint32 so that multiplying by a blend factor up to 0xffff will not
	// overflow.
	//
	// An alpha-premultiplied color component c has been scaled by alpha (a),
	// so has valid values 0 <= c <= a.
	RGBA() (r, g, b, a uint32)
}

// RGBA represents a traditional 32-bit alpha-premultiplied color, having 8
// bits for each of red, green, blue and alpha.
//
// An alpha-premultiplied color component C has been scaled by alpha (A), so
// has valid values 0 <= C <= A.

// RGBA 表示一般的 32 位预乘透明度的颜色，其中红，绿，蓝和透明度各占 8 位数值。
//
// 一个预乘透明度的颜色成分 C 由透明度 alpha (A) 所调整，因此有效值为 0 <= C <= A。
type RGBA struct {
	R, G, B, A uint8
}

func (c RGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	g = uint32(c.G)
	g |= g << 8
	b = uint32(c.B)
	b |= b << 8
	a = uint32(c.A)
	a |= a << 8
	return
}

// RGBA64 represents a 64-bit alpha-premultiplied color, having 16 bits for
// each of red, green, blue and alpha.
//
// An alpha-premultiplied color component C has been scaled by alpha (A), so
// has valid values 0 <= C <= A.

// RGBA64 表示一般的 64 位预乘透明度的颜色，其中红，绿，蓝和透明度各占 16 位数值。
//
// 一个预乘透明度的颜色成分 C 由透明度 alpha (A) 所调整，因此有效值为 0 <= C <= A。
type RGBA64 struct {
	R, G, B, A uint16
}

func (c RGBA64) RGBA() (r, g, b, a uint32) {
	return uint32(c.R), uint32(c.G), uint32(c.B), uint32(c.A)
}

// NRGBA represents a non-alpha-premultiplied 32-bit color.

// NRGBA代表一个没有32位透明度加乘的颜色。
type NRGBA struct {
	R, G, B, A uint8
}

func (c NRGBA) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	r *= uint32(c.A)
	r /= 0xff
	g = uint32(c.G)
	g |= g << 8
	g *= uint32(c.A)
	g /= 0xff
	b = uint32(c.B)
	b |= b << 8
	b *= uint32(c.A)
	b /= 0xff
	a = uint32(c.A)
	a |= a << 8
	return
}

// NRGBA64 represents a non-alpha-premultiplied 64-bit color,
// having 16 bits for each of red, green, blue and alpha.

// NRGBA64代表无透明度加乘的64-bit的颜色，
// 它的每个红，绿，蓝，和透明度都是个16bit的数值。
type NRGBA64 struct {
	R, G, B, A uint16
}

func (c NRGBA64) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r *= uint32(c.A)
	r /= 0xffff
	g = uint32(c.G)
	g *= uint32(c.A)
	g /= 0xffff
	b = uint32(c.B)
	b *= uint32(c.A)
	b /= 0xffff
	a = uint32(c.A)
	return
}

// Alpha represents an 8-bit alpha color.

// Alpha代表一个8-bit的透明度。
type Alpha struct {
	A uint8
}

func (c Alpha) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	a |= a << 8
	return a, a, a, a
}

// Alpha16 represents a 16-bit alpha color.

// Alpha16代表一个16位的透明度。
type Alpha16 struct {
	A uint16
}

func (c Alpha16) RGBA() (r, g, b, a uint32) {
	a = uint32(c.A)
	return a, a, a, a
}

// Gray represents an 8-bit grayscale color.

// Gray代表一个8-bit的灰度。
type Gray struct {
	Y uint8
}

func (c Gray) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	y |= y << 8
	return y, y, y, 0xffff
}

// Gray16 represents a 16-bit grayscale color.

// Gray16代表了一个16-bit的灰度。
type Gray16 struct {
	Y uint16
}

func (c Gray16) RGBA() (r, g, b, a uint32) {
	y := uint32(c.Y)
	return y, y, y, 0xffff
}

// Model can convert any Color to one from its own color model. The conversion
// may be lossy.

// Model可以在它自己的颜色模型中将一种颜色转化到另一种。
// 这种转换可能是有损的。
type Model interface {
	Convert(c Color) Color
}

// ModelFunc returns a Model that invokes f to implement the conversion.

// ModelFunc返回一个Model，它可以调用f来实现转换。
func ModelFunc(f func(Color) Color) Model {
	// Note: using *modelFunc as the implementation
	// means that callers can still use comparisons
	// like m == RGBAModel.  This is not possible if
	// we use the func value directly, because funcs
	// are no longer comparable.
	return &modelFunc{f}
}

type modelFunc struct {
	f func(Color) Color
}

func (m *modelFunc) Convert(c Color) Color {
	return m.f(c)
}

// Models for the standard color types.

// 基本的颜色模型。
var (
	RGBAModel    Model = ModelFunc(rgbaModel)
	RGBA64Model  Model = ModelFunc(rgba64Model)
	NRGBAModel   Model = ModelFunc(nrgbaModel)
	NRGBA64Model Model = ModelFunc(nrgba64Model)
	AlphaModel   Model = ModelFunc(alphaModel)
	Alpha16Model Model = ModelFunc(alpha16Model)
	GrayModel    Model = ModelFunc(grayModel)
	Gray16Model  Model = ModelFunc(gray16Model)
)

func rgbaModel(c Color) Color {
	if _, ok := c.(RGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func rgba64Model(c Color) Color {
	if _, ok := c.(RGBA64); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	return RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func nrgbaModel(c Color) Color {
	if _, ok := c.(NRGBA); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
	}
	if a == 0 {
		return NRGBA{0, 0, 0, 0}
	}
	// Since Color.RGBA returns a alpha-premultiplied color, we should have r <= a && g <= a && b <= a.
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func nrgba64Model(c Color) Color {
	if _, ok := c.(NRGBA64); ok {
		return c
	}
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return NRGBA64{uint16(r), uint16(g), uint16(b), 0xffff}
	}
	if a == 0 {
		return NRGBA64{0, 0, 0, 0}
	}
	// Since Color.RGBA returns a alpha-premultiplied color, we should have r <= a && g <= a && b <= a.
	r = (r * 0xffff) / a
	g = (g * 0xffff) / a
	b = (b * 0xffff) / a
	return NRGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func alphaModel(c Color) Color {
	if _, ok := c.(Alpha); ok {
		return c
	}
	_, _, _, a := c.RGBA()
	return Alpha{uint8(a >> 8)}
}

func alpha16Model(c Color) Color {
	if _, ok := c.(Alpha16); ok {
		return c
	}
	_, _, _, a := c.RGBA()
	return Alpha16{uint16(a)}
}

func grayModel(c Color) Color {
	if _, ok := c.(Gray); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b + 500) / 1000
	return Gray{uint8(y >> 8)}
}

func gray16Model(c Color) Color {
	if _, ok := c.(Gray16); ok {
		return c
	}
	r, g, b, _ := c.RGBA()
	y := (299*r + 587*g + 114*b + 500) / 1000
	return Gray16{uint16(y)}
}

// Palette is a palette of colors.

// Palette是颜色的调色板。
type Palette []Color

// Convert returns the palette color closest to c in Euclidean R,G,B space.

// Convert在Euclidean R,G,B空间中找到最接近c的调色板。
func (p Palette) Convert(c Color) Color {
	if len(p) == 0 {
		return nil
	}
	return p[p.Index(c)]
}

// Index returns the index of the palette color closest to c in Euclidean
// R,G,B,A space.

// Index 在欧几里得 R,G,B,A 色彩空间中找到最接近 c 的调色板对应的索引。
func (p Palette) Index(c Color) int {
	// A batch version of this computation is in image/draw/draw.go.

	cr, cg, cb, ca := c.RGBA()
	ret, bestSum := 0, uint32(1<<32-1)
	for i, v := range p {
		vr, vg, vb, va := v.RGBA()
		sum := sqDiff(cr, vr) + sqDiff(cg, vg) + sqDiff(cb, vb) + sqDiff(ca, va)
		if sum < bestSum {
			if sum == 0 {
				return i
			}
			ret, bestSum = i, sum
		}
	}
	return ret
}

// sqDiff returns the squared-difference of x and y, shifted by 2 so that
// adding four of those won't overflow a uint32.
//
// x and y are both assumed to be in the range [0, 0xffff].

// sqDiff 返回 x 与 y 之差的平方，右移 2 位使其加上四个 uint32 也不会溢出。
//
// 假设 x 和 y 均在区间 [0, 0xffff] 上。
func sqDiff(x, y uint32) uint32 {
	var d uint32
	if x > y {
		d = x - y
	} else {
		d = y - x
	}
	return (d * d) >> 2
}

// Standard colors.

// 标准颜色。
var (
	Black       = Gray16{0}
	White       = Gray16{0xffff}
	Transparent = Alpha16{0}
	Opaque      = Alpha16{0xffff}
)
