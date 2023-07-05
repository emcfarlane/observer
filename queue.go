package observer

import (
	"fmt"
	"math/bits"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

type queue[T any] struct {
	values  [64]T
	mask    uint64
	channel *Channel[T]
	next    unsafe.Pointer
	head    uint32
	tail    uint32
	closed  bool // locked by channel.mu
}

func (q *queue[T]) load() *queue[T] {
	return (*queue[T])(atomic.LoadPointer(&q.next))
}

func (q *queue[T]) has(i uint8) bool {
	return atomic.LoadUint64(&q.mask)&(1<<i) != 0
}

func (q *queue[T]) index(i uint8) T {
	val := q.values[i]
	return val
}

func (q *queue[T]) last() uint8 {
	return uint8(bits.Len64(atomic.LoadUint64(&q.mask)) - 1)
}

func (q *queue[T]) set(i uint8, val T) {
	q.values[i] = val
	atomic.AddUint64(&q.mask, 1<<i)
}
func (q *queue[T]) assertWriteable() {
	if q.closed {
		q.channel.mu.Unlock()
		panic("enqueue on closed *observer.Channel")
	}
}

type Channel[T any] struct {
	mu   sync.Mutex
	cond sync.Cond
	head unsafe.Pointer // *queue read head
	tail unsafe.Pointer // *queue write tail
}

func (c *Channel[T]) loadHead() *queue[T] {
	return (*queue[T])(atomic.LoadPointer(&c.head))
}
func (c *Channel[T]) loadTail() *queue[T] {
	return (*queue[T])(atomic.LoadPointer(&c.tail))
}

func (c *Channel[T]) Enqueue(val T) {
	q := c.loadTail()
	if q == nil {
		c.mu.Lock()
		if c.cond.L == nil {
			c.cond.L = &c.mu
		}
		if q = c.loadTail(); q == nil {
			q = &queue[T]{channel: c}
			q.set(0, val)

			atomic.StorePointer(&c.tail, unsafe.Pointer(q))
			atomic.StorePointer(&c.head, unsafe.Pointer(q))
			c.cond.Broadcast()
			c.mu.Unlock()
			return
		}
		q.assertWriteable()
		c.mu.Unlock()
	}

	i := atomic.AddUint32(&q.tail, 1)
	for ; i > 64; i = atomic.AddUint32(&q.tail, 1) {
		// Spin lock.
		next := q.load()
		for ; next == nil; next = q.load() {
			runtime.Gosched()
		}
		q = next
	}

	if i == 64 {
		next := &queue[T]{channel: c}
		next.set(0, val)
		atomic.StorePointer(&q.next, unsafe.Pointer(next))
		atomic.StorePointer(&c.tail, unsafe.Pointer(next))
	} else {
		q.set(uint8(i), val)
	}

	c.mu.Lock()
	q.assertWriteable()
	c.cond.Broadcast()
	c.mu.Unlock()
}

func (c *Channel[T]) Close() {
	c.mu.Lock()
	q := c.loadTail()
	if q == nil {
		if c.cond.L == nil {
			c.cond.L = &c.mu
		}
		if q = c.loadTail(); q == nil {
			q = &queue[T]{channel: c}
			atomic.StorePointer(&c.tail, unsafe.Pointer(q))
			atomic.StorePointer(&c.head, unsafe.Pointer(q))
		}
	}
	q.closed = true
	c.cond.Broadcast()
	c.mu.Unlock()
}

func (c *Channel[T]) Dequeue() (val T, ok bool) {
	q := c.loadHead()
	if q == nil {
		c.mu.Lock()
		if c.cond.L == nil {
			c.cond.L = &c.mu
		}
		q = c.loadHead()
		for ; q == nil; q = c.loadHead() {
			c.cond.Wait()
		}
		c.mu.Unlock()
	}

	i := atomic.AddUint32(&q.head, 1)
	for ; i >= 64; i = atomic.AddUint32(&q.head, 1) {

		c.mu.Lock()
		next := q.load()
		for ; next == nil; next = q.load() {
			if q.closed {
				c.mu.Unlock()
				return
			}
			c.cond.Wait()
		}
		q = next
		c.mu.Unlock()
	}

	j := uint8(i)
	if q.has(j) {
		return q.index(j), true
	}

	// Wait for next value.
	c.mu.Lock()
	for !q.has(j) {
		// TODO: fix race on has write and close.
		// must check j > tail.
		if q.closed {
			c.mu.Unlock()
			return
		}
		c.cond.Wait()
	}
	c.mu.Unlock()
	return q.index(j), true
}

//func (q *Channel[T]) DequeueBlock() (T, bool) {}
//func (q *Channel[T]) DequeueNonBlock() (T, bool) {}

//func (q *Fifo[T]) Peek() (T, bool) {}

//
// select {
// case one, ok := <- ch1:
// 	...
// case two, ok := <- ch2:
// 	...
// default:
// 	...
// }
//
// if one, ok := ch1.DequeueNonBlock(); ok {
//  	...
// } else if two, ok := ch2.DequeueNonBlock(); ok {
// 	...
// } else {
// 	...
// }

//func (q *Channel[T]) Select(f func(T, bool)) Pollable
//func Select(pollable ...Pollable)
// observer.Select(
// 	ch1.Select(func(one int, ok bool) {
// 		...
// 	}),
// 	ch2.Select(func(two int, ok bool) {
// 		...
// 	}),
// )

// Len returns the number of items in the queue.
func (c *Channel[T]) Len() int {
	t := c.loadTail()
	if t == nil {
		return 0
	}
	i := int(atomic.LoadUint32(&t.head))
	if i > 64 {
		i = 64 //
	}
	i = -i
	for ; t != nil; t = t.load() {
		j := t.last()
		i += int(j)
		if j != 63 {
			break // skip last load
		}
	}
	if i < 0 {
		return 0 // head > tail
	}
	return i
}
