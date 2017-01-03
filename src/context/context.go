// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// context 包定义了 Context 类型，它提供了跨 API 和进程的请求间超时时间，取消信号，和域变量
// 的支持。
//
// 任何进入服务器的请求都应该创建一个 Context ，任何向服务器的请求调用也都应接受一个
// Context 。函数之前的调用都必须要传递 Context ，并且可以用 WithDeadline ，
// WithTimeout ，WithCancel 或 WithValue 来可选的创建一个带有新功能的 Context 副本。
//
// 任何使用 Context 的程序都应该遵循以下约定来约束接口，以保证不同包函数之间的顺利通信和静态
// 检查工具的正确执行：
//
// 不要将 Context 存入一个 struct ，而是通过函数的参数来传递它。Context 应该为函数的第一个
// 参数，名字通常叫 ctx ：
//
// 	func DoSomething(ctx context.Context, arg Arg) error {
// 		// ... 使用 ctx ...
// 	}
//
// 永远不要传递一个 nil Context ，如果你不确定使用哪个 Context ，请传递 context.TODO 。
//
// 仅将一个请求域中的数据保存在 Context 的 Values 上。而不是保存函数的可选参数。
//
// 同一个 Context 可能被传递至不同 go 程中的函数。在多个 go 程中使用同一个 Context 是安全
// 的。
//
// 更多有关在一个服务器中使用 Context 的代码示例请参阅
// https://blog.golang.org/context 。
package context

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// 一个 Context 提供了跨 API 和进程的请求间超时时间，取消信号，和域变量
// 的支持。
//
// 在多个不用的 go 间使用 Context 的方法是安全地。
type Context interface {
	// Deadline 返回该请求应该被结束的时间点（超时时间）。如果并没有设置任何的超时时间，
	// 那个返回 ok == false 。连续地调用 Deadline 将会返回相同的结果。
	Deadline() (deadline time.Time, ok bool)

	// Done 返回一个 channel ，该 channel 会在请求应当被结束时被关闭。如果这个 context
	// 永远不会被取消，那么 Done 可能会返回 nil 。连续地调用 Done 将会返回相同的结果。
	//
	// WithCancel 将会在 cancel 函数被调用时关闭 Done 返回的 channel 。
	// WithDeadline 将会在指定时间到达后关闭 Done 返回的 channel 。
	// WithTimeout 将会在超时时间过后关闭 Done 返回的 channel 。
	//
	// Done 主要被用在 select 声明中：
	//
	//  // Stream 使用 DoSomething 生成一些值，然后将它们传递至 out ，除非
	//  // DoSomething 返回一个错误或 ctx.Done 被关闭。
	//  func Stream(ctx context.Context, out chan<- Value) error {
	//  	for {
	//  		v, err := DoSomething(ctx)
	//  		if err != nil {
	//  			return err
	//  		}
	//  		select {
	//  		case <-ctx.Done():
	//  			return ctx.Err()
	//  		case out <- v:
	//  		}
	//  	}
	//  }
	//
	// 更多如何使用 Done 的例子请参阅：https://blog.golang.org/pipelines 。
	Done() <-chan struct{}

	// 在 Done 返回的 channel 被关闭后，Err 会返回一个非 nil 的错误。如果 context 被
	// 取消那么会返回 Canceled 。如果 context 超时那么会返回 DeadlineExceeded 。在
	// 在 Done 返回的 channel 被关闭后，连续调用 Err 会返回相同的结果。
	Err() error

	// Value 返回该 context 中指定 key 所关联的值，如果该 key 并没有所关联的值，那么
	// 返回 nil 。对相同的 key 调用 Value 将会返回相同的结果。
	//
	// 仅在跨 API 或进程请求的同一个请求域里使用 context Value ，而不是用它来传递函数的可选参
	// 数。
	//
	// 在 Context 中，一个 key 关联一个指定的值。函数可以通过 context.WithValue 和
	// Context.Value 来保存一个全局键/值对。key 可以是支持等号操作的任意值。包应该使用
	// 未导出的类型来定义 key ，用以避免潜在的冲突。
	//
	// 定义了 Context key 的包一定要为其值提供类型安全地访问器：
	//
	//  // user 包定义了一个 User 用以保存在 Context 中。
	//
	// 	import "context"
	//
	//  // User 是 Context 中保存的值的类型。
	// 	type User struct {...}
	//
	//  // key 是一个该包中定义的未导出类型。
	//  // 这避免和其他包的命名冲突。
	// 	type key int
	//
	//  // userKey 是 Context 中 user.User 值的 key 。它也是未导出的。
	//  // 客户端将会使用 user.NewContext and user.FromContext 而不是直接
	//  // 使用这个 key 。
	// 	var userKey key = 0
	//
	//  // NewContext 返回一个带有值 u 的新 Context 。
	// 	func NewContext(ctx context.Context, u *User) context.Context {
	// 		return context.WithValue(ctx, userKey, u)
	// 	}
	//
	//  // FromContext 返回 ctx 中可能存在的 User 值。
	// 	func FromContext(ctx context.Context) (*User, bool) {
	// 		u, ok := ctx.Value(userKey).(*User)
	// 		return u, ok
	// 	}
	Value(key interface{}) interface{}
}

// Canceled 是当 context 被取消时由 Context.Err 返回的错误。
var Canceled = errors.New("context canceled")

// DeadlineExceeded 是当 context 超时时由 Context.Err 返回的错误。
var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string { return "context deadline exceeded" }

func (deadlineExceededError) Timeout() bool { return true }

// An emptyCtx is never canceled, has no values, and has no deadline.  It is not
// struct{}, since vars of this type must have distinct addresses.
type emptyCtx int

func (*emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (*emptyCtx) Done() <-chan struct{} {
	return nil
}

func (*emptyCtx) Err() error {
	return nil
}

func (*emptyCtx) Value(key interface{}) interface{} {
	return nil
}

func (e *emptyCtx) String() string {
	switch e {
	case background:
		return "context.Background"
	case todo:
		return "context.TODO"
	}
	return "unknown empty Context"
}

var (
	background = new(emptyCtx)
	todo       = new(emptyCtx)
)

// Background 返回一个非 nil 的空 context 。它不包含任何值，不会被取消也不会超时。
// 它通常被用在 main 函数，初始化函数，测试或请求的顶层 Context 中。
func Background() Context {
	return background
}

// TODO 返回一个非 nil 的空 context 。当不知道该使用哪一个 Context 时，代码可以使用
// context.TODO 。TODO 可以被静态分析工具正确的识别，用以判断是否 Context 在代码中
// 是否被正确得传递。
func TODO() Context {
	return todo
}

// CancelFunc 指明一个操作需要被取消。
// CancelFunc 不会等待操作的结束。
// 在第一次调用之后，之后的 CancelFunc 调用什么都不会做。
type CancelFunc func()

// WithCancel 返回一个带有新 Done channel 的父 context 副本。返回的 ctx 的 Done
// channel 会在 cancel 函数被调用时关闭，或者在父 context 的 Done channel 关闭时关闭，
// 这取决于哪一种情况先发生。
//
// 取消这个 context 意味着需要释放它占用的资源，所以代码需要在这个 context 上的操作结束
// 后立刻调用 cancel 。
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	c := newCancelCtx(parent)
	propagateCancel(parent, &c)
	return &c, func() { c.cancel(true, Canceled) }
}

// newCancelCtx returns an initialized cancelCtx.
func newCancelCtx(parent Context) cancelCtx {
	return cancelCtx{
		Context: parent,
		done:    make(chan struct{}),
	}
}

// propagateCancel arranges for child to be canceled when parent is.
func propagateCancel(parent Context, child canceler) {
	if parent.Done() == nil {
		return // parent is never canceled
	}
	if p, ok := parentCancelCtx(parent); ok {
		p.mu.Lock()
		if p.err != nil {
			// parent has already been canceled
			child.cancel(false, p.err)
		} else {
			if p.children == nil {
				p.children = make(map[canceler]bool)
			}
			p.children[child] = true
		}
		p.mu.Unlock()
	} else {
		go func() {
			select {
			case <-parent.Done():
				child.cancel(false, parent.Err())
			case <-child.Done():
			}
		}()
	}
}

// parentCancelCtx follows a chain of parent references until it finds a
// *cancelCtx.  This function understands how each of the concrete types in this
// package represents its parent.
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	for {
		switch c := parent.(type) {
		case *cancelCtx:
			return c, true
		case *timerCtx:
			return &c.cancelCtx, true
		case *valueCtx:
			parent = c.Context
		default:
			return nil, false
		}
	}
}

// removeChild removes a context from its parent.
func removeChild(parent Context, child canceler) {
	p, ok := parentCancelCtx(parent)
	if !ok {
		return
	}
	p.mu.Lock()
	if p.children != nil {
		delete(p.children, child)
	}
	p.mu.Unlock()
}

// A canceler is a context type that can be canceled directly.  The
// implementations are *cancelCtx and *timerCtx.
type canceler interface {
	cancel(removeFromParent bool, err error)
	Done() <-chan struct{}
}

// A cancelCtx can be canceled.  When canceled, it also cancels any children
// that implement canceler.
type cancelCtx struct {
	Context

	done chan struct{} // closed by the first cancel call.

	mu       sync.Mutex
	children map[canceler]bool // set to nil by the first cancel call
	err      error             // set to non-nil by the first cancel call
}

func (c *cancelCtx) Done() <-chan struct{} {
	return c.done
}

func (c *cancelCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *cancelCtx) String() string {
	return fmt.Sprintf("%v.WithCancel", c.Context)
}

// cancel closes c.done, cancels each of c's children, and, if
// removeFromParent is true, removes c from its parent's children.
func (c *cancelCtx) cancel(removeFromParent bool, err error) {
	if err == nil {
		panic("context: internal error: missing cancel error")
	}
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return // already canceled
	}
	c.err = err
	close(c.done)
	for child := range c.children {
		// NOTE: acquiring the child's lock while holding parent's lock.
		child.cancel(false, err)
	}
	c.children = nil
	c.mu.Unlock()

	if removeFromParent {
		removeChild(c.Context, c)
	}
}

// WithDeadline 返回一个带有新超时时间点的父 context 副本。如果父 context 的超时时间点比
// d 要早，那么调用 WithDeadline(parent, d) 的返回值在语义上等于父 context 。返回的
// ctx 的 Done channel 会在超时时关闭，或者在父 context 的 Done
// channel 关闭时关闭，这取决于哪一种情况先发生。
//
// 取消这个 context 意味着需要释放它占用的资源，所以代码需要在这个 context 上的操作结束
// 后立刻调用 cancel 。
func WithDeadline(parent Context, deadline time.Time) (Context, CancelFunc) {
	if cur, ok := parent.Deadline(); ok && cur.Before(deadline) {
		// The current deadline is already sooner than the new one.
		return WithCancel(parent)
	}
	c := &timerCtx{
		cancelCtx: newCancelCtx(parent),
		deadline:  deadline,
	}
	propagateCancel(parent, c)
	d := deadline.Sub(time.Now())
	if d <= 0 {
		c.cancel(true, DeadlineExceeded) // deadline has already passed
		return c, func() { c.cancel(true, Canceled) }
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.timer = time.AfterFunc(d, func() {
			c.cancel(true, DeadlineExceeded)
		})
	}
	return c, func() { c.cancel(true, Canceled) }
}

// A timerCtx carries a timer and a deadline.  It embeds a cancelCtx to
// implement Done and Err.  It implements cancel by stopping its timer then
// delegating to cancelCtx.cancel.
type timerCtx struct {
	cancelCtx
	timer *time.Timer // Under cancelCtx.mu.

	deadline time.Time
}

func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
	return c.deadline, true
}

func (c *timerCtx) String() string {
	return fmt.Sprintf("%v.WithDeadline(%s [%s])", c.cancelCtx.Context, c.deadline, c.deadline.Sub(time.Now()))
}

func (c *timerCtx) cancel(removeFromParent bool, err error) {
	c.cancelCtx.cancel(false, err)
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		removeChild(c.cancelCtx.Context, c)
	}
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
}

// WithTimeout 返回 WithDeadline(parent, time.Now().Add(timeout)) 。
//
// 取消这个 context 意味着需要释放它占用的资源，所以代码需要在这个 context 上的操作结束
// 后立刻调用 cancel 。
//
// 	func slowOperationWithTimeout(ctx context.Context) (Result, error) {
// 		ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
// 		defer cancel()  // 如果 slowOperation 在超时前完成，释放资源。
// 		return slowOperation(ctx)
// 	}
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

// WithValue 返回一个 key 关联的值为 val 的父 context 副本。
//
// 仅在跨 API 或进程请求的同一个请求域里使用 context Value ，而不是用它来传递函数的可选参
// 数。
//
// 提供的 key 必须是可比较的。
func WithValue(parent Context, key, val interface{}) Context {
	if key == nil {
		panic("nil key")
	}
	if !reflect.TypeOf(key).Comparable() {
		panic("key is not comparable")
	}
	return &valueCtx{parent, key, val}
}

// A valueCtx carries a key-value pair.  It implements Value for that key and
// delegates all other calls to the embedded Context.
type valueCtx struct {
	Context
	key, val interface{}
}

func (c *valueCtx) String() string {
	return fmt.Sprintf("%v.WithValue(%#v, %#v)", c.Context, c.key, c.val)
}

func (c *valueCtx) Value(key interface{}) interface{} {
	if c.key == key {
		return c.val
	}
	return c.Context.Value(key)
}
