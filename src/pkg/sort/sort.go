// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sort provides primitives for sorting slices and user-defined
// collections.

// sort 包为切片及用户定义的集合的排序操作提供了原语.
package sort

import "math"

// A type, typically a collection, that satisfies sort.Interface can be
// sorted by the routines in this package.  The methods require that the
// elements of the collection be enumerated by an integer index.

// 任何实现了 sort.Interface 的类型（一般为集合），均可使用该包中的方法进行排序。
// 这些方法要求集合内列出元素的索引为整数。
type Interface interface {
	// Len is the number of elements in the collection.
	// Len 为集合内元素的总数
	Len() int
	// Less returns whether the element with index i should sort
	// before the element with index j.
	// Less 返回索引为 i 的元素是否应排在索引为 j 的元素之前。
	Less(i, j int) bool
	// Swap swaps the elements with indexes i and j.
	// Swap 交换索引为 i 和 j 的元素
	Swap(i, j int)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Insertion sort

// 插入排序
func insertionSort(data Interface, a, b int) {
	for i := a + 1; i < b; i++ {
		for j := i; j > a && data.Less(j, j-1); j-- {
			data.Swap(j, j-1)
		}
	}
}

// siftDown implements the heap property on data[lo, hi).
// first is an offset into the array where the root of the heap lies.

// siftDown 为 data[lo, hi) 实现堆的性质。
// first 为堆的根节点在数组中的偏移量。
func siftDown(data Interface, lo, hi, first int) {
	root := lo
	for {
		child := 2*root + 1
		if child >= hi {
			break
		}
		if child+1 < hi && data.Less(first+child, first+child+1) {
			child++
		}
		if !data.Less(first+root, first+child) {
			return
		}
		data.Swap(first+root, first+child)
		root = child
	}
}

func heapSort(data Interface, a, b int) {
	first := a
	lo := 0
	hi := b - a

	// Build heap with greatest element at top.
	// 以最大元素为顶建堆。
	for i := (hi - 1) / 2; i >= 0; i-- {
		siftDown(data, i, hi, first)
	}

	// Pop elements, largest first, into end of data.
	// 弹出元素，从大到小的顺序，从后向前依次追加到数组 data。
	for i := hi - 1; i >= 0; i-- {
		data.Swap(first, first+i)
		siftDown(data, lo, i, first)
	}
}

// Quicksort, following Bentley and McIlroy,
// ``Engineering a Sort Function,'' SP&E November 1993.
// 快速排序，实现参考 Bentley and McIlroy,
// ``Engineering a Sort Function,'' SP&E November 1993.

// medianOfThree moves the median of the three values data[a], data[b], data[c] into data[a].

// medianOfThree 将 data[a]、data[b] 和 data[c] 三个值的中值交换到 data[a]。
func medianOfThree(data Interface, a, b, c int) {
	m0 := b
	m1 := a
	m2 := c
	// bubble sort on 3 elements
	// 对3个元素进行冒泡排序
	if data.Less(m1, m0) {
		data.Swap(m1, m0)
	}
	if data.Less(m2, m1) {
		data.Swap(m2, m1)
	}
	if data.Less(m1, m0) {
		data.Swap(m1, m0)
	}
	// now data[m0] <= data[m1] <= data[m2]
	// 现在 data[m0] <= data[m1] <= data[m2]
}

func swapRange(data Interface, a, b, n int) {
	for i := 0; i < n; i++ {
		data.Swap(a+i, b+i)
	}
}

func doPivot(data Interface, lo, hi int) (midlo, midhi int) {
	m := lo + (hi-lo)/2 // Written like this to avoid integer overflow. // 这样写避免整数溢出。
	if hi-lo > 40 {
		// Tukey's ``Ninther,'' median of three medians of three.
		// Tukey的“Ninther”算法，分别求三组值的中值，再求这三个值的中值。
		s := (hi - lo) / 8
		medianOfThree(data, lo, lo+s, lo+2*s)
		medianOfThree(data, m, m-s, m+s)
		medianOfThree(data, hi-1, hi-1-s, hi-1-2*s)
	}
	medianOfThree(data, lo, m, hi-1)

	// Invariants are:
	//	data[lo] = pivot (set up by ChoosePivot)
	//	data[lo <= i < a] = pivot
	//	data[a <= i < b] < pivot
	//	data[b <= i < c] is unexamined
	//	data[c <= i < d] > pivot
	//	data[d <= i < hi] = pivot
	//
	// Once b meets c, can swap the "= pivot" sections
	// into the middle of the slice.

	// 算法不变式为：
	//	data[lo] = pivot (由 ChoosePivot 决定)
	//	data[lo <= i < a] = pivot
	//	data[a <= i < b] < pivot
	//	data[b <= i < c] 未经检查
	//	data[c <= i < d] > pivot
	//	data[d <= i < hi] = pivot
	//
	// 一旦 b 与 c 相遇，就将“= pivot”的这部分交换到切片中间。
	pivot := lo
	a, b, c, d := lo+1, lo+1, hi, hi
	for b < c {
		if data.Less(b, pivot) { // data[b] < pivot
			b++
			continue
		}
		if !data.Less(pivot, b) { // data[b] = pivot
			data.Swap(a, b)
			a++
			b++
			continue
		}
		if data.Less(pivot, c-1) { // data[c-1] > pivot
			c--
			continue
		}
		if !data.Less(c-1, pivot) { // data[c-1] = pivot
			data.Swap(c-1, d-1)
			c--
			d--
			continue
		}
		// data[b] > pivot; data[c-1] < pivot
		data.Swap(b, c-1)
		b++
		c--
	}

	n := min(b-a, a-lo)
	swapRange(data, lo, b-n, n)

	n = min(hi-d, d-c)
	swapRange(data, c, hi-n, n)

	return lo + b - a, hi - (d - c)
}

func quickSort(data Interface, a, b, maxDepth int) {
	for b-a > 7 {
		if maxDepth == 0 {
			heapSort(data, a, b)
			return
		}
		maxDepth--
		mlo, mhi := doPivot(data, a, b)
		// Avoiding recursion on the larger subproblem guarantees
		// a stack depth of at most lg(b-a).
		// 避免大量子问题的递归，以保证栈的深度在 lg(b-a) 以内。
		if mlo-a < b-mhi {
			quickSort(data, a, mlo, maxDepth)
			a = mhi // i.e., quickSort(data, mhi, b) // 例如 quickSort(data, mhi, b)
		} else {
			quickSort(data, mhi, b, maxDepth)
			b = mlo // i.e., quickSort(data, a, mlo) // 例如 quickSort(data, a, mlo)
		}
	}
	if b-a > 1 {
		insertionSort(data, a, b)
	}
}

// Sort sorts data.
// It makes one call to data.Len to determine n, and O(n*log(n)) calls to
// data.Less and data.Swap. The sort is not guaranteed to be stable.

// Sort 对 data 进行排序。
// 它调用一次 data.Len 来决定排序的长度 n，调用 data.Less 和 data.Swap 的开销为
// O(n*log(n))。此排序为不稳定排序。
func Sort(data Interface) {
	// Switch to heapsort if depth of 2*ceil(lg(n+1)) is reached.
	// 若深度达到 2*ceil(lg(n+1)) 就切换到堆排序算法。
	n := data.Len()
	maxDepth := 0
	for i := n; i > 0; i >>= 1 {
		maxDepth++
	}
	maxDepth *= 2
	quickSort(data, 0, n, maxDepth)
}

type reverse struct {
	// This embedded Interface permits Reverse to use the methods of
	// another Interface implementation.
	Interface
}

// Less returns the opposite of the embedded implementation's Less method.
func (r reverse) Less(i, j int) bool {
	return r.Interface.Less(j, i)
}

// Reverse returns the reverse order for data.
func Reverse(data Interface) Interface {
	return &reverse{data}
}

// IsSorted reports whether data is sorted.

// IsSorted 返回数据是否已经排序。
func IsSorted(data Interface) bool {
	n := data.Len()
	for i := n - 1; i > 0; i-- {
		if data.Less(i, i-1) {
			return false
		}
	}
	return true
}

// Convenience types for common cases
// 针对一般情况的便捷性类型

// IntSlice attaches the methods of Interface to []int, sorting in increasing order.

// IntSlice 针对 []int 实现接口的方法，以升序排列。
type IntSlice []int

func (p IntSlice) Len() int           { return len(p) }
func (p IntSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p IntSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.

// Sort 为便捷性方法
func (p IntSlice) Sort() { Sort(p) }

// Float64Slice attaches the methods of Interface to []float64, sorting in increasing order.

// Float64Slice 针对 []float6 实现接口的方法，以升序排列。
type Float64Slice []float64

func (p Float64Slice) Len() int           { return len(p) }
func (p Float64Slice) Less(i, j int) bool { return p[i] < p[j] || math.IsNaN(p[i]) && !math.IsNaN(p[j]) }
func (p Float64Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.

// Sort 为便捷性方法
func (p Float64Slice) Sort() { Sort(p) }

// StringSlice attaches the methods of Interface to []string, sorting in increasing order.

// StringSlice 针对  []string 实现接口的方法，以升序排列。
type StringSlice []string

func (p StringSlice) Len() int           { return len(p) }
func (p StringSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p StringSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// Sort is a convenience method.

// Sort 为便捷性方法
func (p StringSlice) Sort() { Sort(p) }

// Convenience wrappers for common cases
// 针对一般情况的便捷性封装

// Ints sorts a slice of ints in increasing order.

//Ints 以升序排列 int 切片。
func Ints(a []int) { Sort(IntSlice(a)) }

// Float64s sorts a slice of float64s in increasing order.

// Float64s 以升序排列 float64 切片
func Float64s(a []float64) { Sort(Float64Slice(a)) }

// Strings sorts a slice of strings in increasing order.

// Strings 以升序排列 string 切片。
func Strings(a []string) { Sort(StringSlice(a)) }

// IntsAreSorted tests whether a slice of ints is sorted in increasing order.

// IntsAreSorted 判断 int 切片是否已经按升序排列。
func IntsAreSorted(a []int) bool { return IsSorted(IntSlice(a)) }

// Float64sAreSorted tests whether a slice of float64s is sorted in increasing order.

// Float64sAreSorted 判断 float64 切片是否已经按升序排列。
func Float64sAreSorted(a []float64) bool { return IsSorted(Float64Slice(a)) }

// StringsAreSorted tests whether a slice of strings is sorted in increasing order.

// StringsAreSorted 判断 string 切片是否已经按升序排列。
func StringsAreSorted(a []string) bool { return IsSorted(StringSlice(a)) }
