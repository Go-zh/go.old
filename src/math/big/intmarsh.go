// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements encoding/decoding of Ints.

// 本文件实现了 Int 的编解码。

package big

import "fmt"

// Gob codec version. Permits backward-compatible changes to the encoding.

// Gob 编解码器版本。允许对编码进行向前兼容的更改。
const intGobVersion byte = 1

// GobEncode implements the gob.GobEncoder interface.

// GobEncode 实现了 gob.GobEncoder 接口。
func (x *Int) GobEncode() ([]byte, error) {
	if x == nil {
		return nil, nil
	}
	buf := make([]byte, 1+len(x.abs)*_S) // extra byte for version and sign bit  // 版本和符号位的扩展字节
	i := x.abs.bytes(buf) - 1            // i >= 0
	b := intGobVersion << 1              // make space for sign bit // 为符号位留下空间
	if x.neg {
		b |= 1
	}
	buf[i] = b
	return buf[i:], nil
}

// GobDecode implements the gob.GobDecoder interface.

// GobDecode 实现了 gob.GobDecoder 接口。
func (z *Int) GobDecode(buf []byte) error {
	if len(buf) == 0 {
		// Other side sent a nil or default value.
		*z = Int{}
		return nil
	}
	b := buf[0]
	if b>>1 != intGobVersion {
		return fmt.Errorf("Int.GobDecode: encoding version %d not supported", b>>1)
	}
	z.neg = b&1 != 0
	z.abs = z.abs.setBytes(buf[1:])
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.

// MarshalText 实现了 encoding.TextMarshaler 接口。
func (x *Int) MarshalText() (text []byte, err error) {
	if x == nil {
		return []byte("<nil>"), nil
	}
	return x.abs.itoa(x.neg, 10), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.

// UnmarshalText 实现了 encoding.TextUnmarshaler 接口。
func (z *Int) UnmarshalText(text []byte) error {
	// TODO(gri): get rid of the []byte/string conversion
	if _, ok := z.SetString(string(text), 0); !ok {
		return fmt.Errorf("math/big: cannot unmarshal %q into a *big.Int", text)
	}
	return nil
}

// The JSON marshallers are only here for API backward compatibility
// (programs that explicitly look for these two methods). JSON works
// fine with the TextMarshaler only.

// 这里的 JSON 编组器仅用于 API 的向前兼容（即显式地查找这两个方法的程序）。
// JSON 只用 TextMarshaler 就能良好地工作。

// MarshalJSON implements the json.Marshaler interface.

// MarshalJSON 实现了 json.Marshaler 接口。
func (x *Int) MarshalJSON() ([]byte, error) {
	return x.MarshalText()
}

// UnmarshalJSON implements the json.Unmarshaler interface.

// UnmarshalJSON 实现了 json.Unmarshaler 接口。
func (z *Int) UnmarshalJSON(text []byte) error {
	return z.UnmarshalText(text)
}
