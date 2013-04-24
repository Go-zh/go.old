package main

import "fmt"

// fib 返回一个函数，该函数返回连续的斐波纳契数。
func fib() func() int {
	a, b := 0, 1
	return func() int {
		a, b = b, a+b
		return a
	}
}

func main() {
	f := fib()
	// 函数调用按从左到右顺序求值。
	fmt.Println(f(), f(), f(), f(), f())
}
