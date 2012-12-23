// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sort_test

import (
	"fmt"
	"sort"
)

// Reverse embeds a sort.Interface value and implements a reverse sort over
// that value.

// Reverse 嵌入了一个 sort.Interface 值，并实现了该值的逆序排序。
type Reverse struct {
	// This embedded Interface permits Reverse to use the methods of
	// another Interface implementation.
	// 此嵌入式接口允许 Reverse 使用其他 Interface 实现的方法。
	sort.Interface
}

// Less returns the opposite of the embedded implementation's Less method.

// Less 返回的结果与嵌入式实现中的 Less 方法相反。
func (r Reverse) Less(i, j int) bool {
	return r.Interface.Less(j, i)
}

func ExampleInterface_reverse() {
	s := []int{5, 2, 6, 3, 1, 4} // unsorted // 未排序
	sort.Sort(Reverse{sort.IntSlice(s)})
	fmt.Println(s)
	// Output: [6 5 4 3 2 1]
}
