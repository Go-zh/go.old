// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package hmac implements the Keyed-Hash Message Authentication Code (HMAC) as
	defined in U.S. Federal Information Processing Standards Publication 198.
	An HMAC is a cryptographic hash that uses a key to sign a message.
	The receiver verifies the hash by recomputing it using the same key.
	
	Receivers should be careful to use Equal to compare MACs in order to avoid
	timing side-channels:
	
		// CheckMAC returns true if messageMAC is a valid HMAC tag for message.
		func CheckMAC(message, messageMAC, key []byte) bool {
			mac := hmac.New(sha256.New, key)
			mac.Write(message)
			expectedMAC := mac.Sum(nil)
			return hmac.Equal(messageMAC, expectedMAC)
		}
*/

/*
hmac 包实现了基于密钥的哈希消息认证码(HMAC)，其在 198 号美国联邦信息处理标准中定义。
HMAC 是使用密钥对一段消息进行签名的密码学意义的哈希值。接收者使用同样的密钥重新计算
一遍来验证该哈希值。

为了避免时间相关的旁道攻击，接收者需要非常小心地使用 Equal 方法来比较 MAC 值:

	// CheckMAC 在 messageMAC 是 message 的正确的 HAMC 标记时返回 true。
	func CheckMAC(message, messageMAC, key []byte) bool {
		mac := hmac.New(sha256.New, key)
		mac.Write(message)
		expectedMAC := mac.Sum(nil)
		return hmac.Equal(messageMAC, expectedMAC)
	}
*/
package hmac

import (
	"crypto/subtle"
	"hash"
)

//	FIPS 198:
//	http://csrc.nist.gov/publications/fips/fips198/fips-198a.pdf

//	key is zero padded to the block size of the hash function
//	ipad = 0x36 byte repeated for key length
//	opad = 0x5c byte repeated for key length
//	hmac = H([key ^ opad] H([key ^ ipad] text))

// 198 号美国联邦信息处理标准：
// http://csrc.nist.gov/publications/fips/fips198/fips-198a.pdf

// key 会用 0 来填充以满足哈希函数的块大小
// ipad = 0x36 重复的字节，个数与 key 的长度相同
// opad = 0x5c 重复的字节，个数与 key 的长度相同
// hmac = H([key ^ opad] H([key ^ ipad] text))

type hmac struct {
	size         int
	blocksize    int
	key, tmp     []byte
	outer, inner hash.Hash
}

func (h *hmac) tmpPad(xor byte) {
	for i, k := range h.key {
		h.tmp[i] = xor ^ k
	}
	for i := len(h.key); i < h.blocksize; i++ {
		h.tmp[i] = xor
	}
}

func (h *hmac) Sum(in []byte) []byte {
	origLen := len(in)
	in = h.inner.Sum(in)
	h.tmpPad(0x5c)
	copy(h.tmp[h.blocksize:], in[origLen:])
	h.outer.Reset()
	h.outer.Write(h.tmp)
	return h.outer.Sum(in[:origLen])
}

func (h *hmac) Write(p []byte) (n int, err error) {
	return h.inner.Write(p)
}

func (h *hmac) Size() int { return h.size }

func (h *hmac) BlockSize() int { return h.blocksize }

func (h *hmac) Reset() {
	h.inner.Reset()
	h.tmpPad(0x36)
	h.inner.Write(h.tmp[:h.blocksize])
}

//	New returns a new HMAC hash using the given hash.Hash type and key.

// New 构造并返回一个使用指定的 hash.Hash 类型和 key 的 HMAC 哈希接口。
func New(h func() hash.Hash, key []byte) hash.Hash {
	hm := new(hmac)
	hm.outer = h()
	hm.inner = h()
	hm.size = hm.inner.Size()
	hm.blocksize = hm.inner.BlockSize()
	hm.tmp = make([]byte, hm.blocksize+hm.size)
	if len(key) > hm.blocksize {
		// If key is too big, hash it.
		hm.outer.Write(key)
		key = hm.outer.Sum(nil)
	}
	hm.key = make([]byte, len(key))
	copy(hm.key, key)
	hm.Reset()
	return hm
}

//	Equal compares two MACs for equality without leaking timing information.

// Equal 比较两个 MAC 是否相等，并且不会泄露CPU耗时信息。
func Equal(mac1, mac2 []byte) bool {
	// We don't have to be constant time if the lengths of the MACs are
	// different as that suggests that a completely different hash function
	// was used.
	return len(mac1) == len(mac2) && subtle.ConstantTimeCompare(mac1, mac2) == 1
}
