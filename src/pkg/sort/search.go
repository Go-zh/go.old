// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file implements binary search.
// 该文件实现二分查找

package sort

// Search uses binary search to find and return the smallest index i
// in [0, n) at which f(i) is true, assuming that on the range [0, n),
// f(i) == true implies f(i+1) == true.  That is, Search requires that
// f is false for some (possibly empty) prefix of the input range [0, n)
// and then true for the (possibly empty) remainder; Search returns
// the first true index.  If there is no such index, Search returns n.
// (Note that the "not found" return value is not -1 as in, for instance,
// strings.Index).
// Search calls f(i) only for i in the range [0, n).
//
// A common use of Search is to find the index i for a value x in
// a sorted, indexable data structure such as an array or slice.
// In this case, the argument f, typically a closure, captures the value
// to be searched for, and how the data structure is indexed and
// ordered.
//
// For instance, given a slice data sorted in ascending order,
// the call Search(len(data), func(i int) bool { return data[i] >= 23 })
// returns the smallest index i such that data[i] >= 23.  If the caller
// wants to find whether 23 is in the slice, it must test data[i] == 23
// separately.
//
// Searching data sorted in descending order would use the <=
// operator instead of the >= operator.
//
// To complete the example above, the following code tries to find the value
// x in an integer slice data sorted in ascending order:
//
//	x := 23
//	i := sort.Search(len(data), func(i int) bool { return data[i] >= x })
//	if i < len(data) && data[i] == x {
//		// x is present at data[i]
//	} else {
//		// x is not present in data,
//		// but i is the index where it would be inserted.
//	}
//
// As a more whimsical example, this program guesses your number:
//
//	func GuessingGame() {
//		var s string
//		fmt.Printf("Pick an integer from 0 to 100.\n")
//		answer := sort.Search(100, func(i int) bool {
//			fmt.Printf("Is your number <= %d? ", i)
//			fmt.Scanf("%s", &s)
//			return s != "" && s[0] == 'y'
//		})
//		fmt.Printf("Your number is %d.\n", answer)
//	}
//

// Search 使用二分查找，找出并返回
// 满足在范围[0,n)内，f取值为真的最小索引i，如果在范围[0, n)上，
// f(i) == true 成立则 f(i+1) == true 也成立. 就是说，Search 要求
// f 函数的取值 对于当i在范围 [0, n)中的前某一部分序列（或不存在）取值为false时
// 对范围剩下的一部分取值为true; Search返回
// 第一个取值为真的索引. 如果该索引不存在，Search 返回 n
// (注意，“没找到”返回不是-1，这与strings.Index中的行为
// 不相同)
// Search 反复调用f(i),i将在范围[0,n)内取值
//
// Search常见的用法是在一个可通过索引访问并排好序的数组或切片中
// 寻找某值x的索引i，
// 在这种情况下，参数f，或者说是一个闭包，调用需要传入被搜索的值，以及被索引和
// 排序的数据
//
// 举个例子, 给定一个升序排列的切片数据,
// 调用 Search(len(data), func(i int) bool { return data[i] >= 23 })
// 返回满足 data[i] >= 23 的最小索引i. 如果调用者
// 想知道23是否在切片中, 必须另外通过data[i] == 23判断
//
// 搜索降序排列的数据需要用 <= 操作符，
// 而不是>=操作符
//
// 补全上面的例子, 下面的代码试图在升序排列的切片中找出整形变量x的索引
//
//	x := 23
//	i := sort.Search(len(data), func(i int) bool { return data[i] >= x })
//	if i < len(data) && data[i] == x {
//		// x 在 data[i] 中 
//	} else {
//		// x 不在data中
//		// 但i可以作为x的插入索引
//	}
//
// 一个有趣的例子，猜你想要的数字
//
//	func GuessingGame() {
//		var s string
//		fmt.Printf("Pick an integer from 0 to 100.\n")
//		answer := sort.Search(100, func(i int) bool {
//			fmt.Printf("Is your number <= %d? ", i)
//			fmt.Scanf("%s", &s)
//			return s != "" && s[0] == 'y'
//		})
//		fmt.Printf("Your number is %d.\n", answer)
//	}
//
func Search(n int, f func(int) bool) int {
	// Define f(-1) == false and f(n) == true.
	// Invariant: f(i-1) == false, f(j) == true.
	i, j := 0, n
	for i < j {
		h := i + (j-i)/2 // avoid overflow when computing h // 避免计算h的时候溢出
		// i ≤ h < j
		if !f(h) {
			i = h + 1 // preserves f(i-1) == false
		} else {
			j = h // preserves f(j) == true
		}
	}
	// i == j, f(i-1) == false, and f(j) (= f(i)) == true  =>  answer is i.
	return i
}

// Convenience wrappers for common cases.

// SearchInts searches for x in a sorted slice of ints and returns the index
// as specified by Search. The return value is the index to insert x if x is
// not present (it could be len(a)).
// The slice must be sorted in ascending order.
//
// 常见案例的便捷调用封装

// SearchInts 在ints切片中搜索x并返回索引
// 如Search函数所述. 返回可以插入x值的索引位置，如果x
// 不存在，返回数组a的长度
// 切片必须以升序排列
//
func SearchInts(a []int, x int) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchFloat64s 在float64s切片中搜索x并返回索引
// 如Search函数所述. 返回可以插入x值的索引位置，如果x
// 不存在，返回数组a的长度
// 切片必须以升序排列
//
func SearchFloat64s(a []float64, x float64) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// SearchFloat64s 在strings切片中搜索x并返回索引
// 如Search函数所述. 返回可以插入x值的索引位置，如果x
// 不存在，返回数组a的长度
// 切片必须以升序排列
//
func SearchStrings(a []string, x string) int {
	return Search(len(a), func(i int) bool { return a[i] >= x })
}

// Search 返回以调用者和x为参数调用SearchInts后的结果
func (p IntSlice) Search(x int) int { return SearchInts(p, x) }

// Search 返回以调用者和x为参数调用SearchFloat64s后的结果
func (p Float64Slice) Search(x float64) int { return SearchFloat64s(p, x) }

// Search 返回以调用者和x为参数调用SearchStrings后的结果
func (p StringSlice) Search(x string) int { return SearchStrings(p, x) }
