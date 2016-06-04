// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strconv

// ParseBool returns the boolean value represented by the string.
// It accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// Any other value returns an error.

// ParseBool 返回字符串表示的布尔值。
// 只接受 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False。
// 除此之外的值均返回一个错误。
func ParseBool(str string) (bool, error) {
	switch str {
	case "1", "t", "T", "true", "TRUE", "True":
		return true, nil
	case "0", "f", "F", "false", "FALSE", "False":
		return false, nil
	}
	return false, syntaxError("ParseBool", str)
}

// FormatBool returns "true" or "false" according to the value of b

// FormatBool 根据 b 的值返回 "true" 或是 "false"
func FormatBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// AppendBool appends "true" or "false", according to the value of b,
// to dst and returns the extended buffer.

// AppendBool 根据 b 的值向 dst 中追加 "true" 或是 "false"
// 返回扩充后的 buffer
func AppendBool(dst []byte, b bool) []byte {
	if b {
		return append(dst, "true"...)
	}
	return append(dst, "false"...)
}
