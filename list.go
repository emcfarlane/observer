package observer

import (
	"sync/atomic"
	"unsafe"
)

type Element struct {
	next, prev unsafe.Pointer
	list       *List
	Value      interface{}
}

func (e *Element) isNil() bool {
	return e == nil || e.list == nil || e == &e.list.root
}

func (e *Element) Next() *Element {
	n := (*Element)(atomic.LoadPointer(&e.next))
	if n.isNil() {
		return nil
	}
	return n
}

func (e *Element) Prev() *Element {
	p := (*Element)(atomic.LoadPointer(&e.prev))
	if p.isNil() {
		return nil
	}
	return p
}

// WARN: dragons
type List struct {
	root Element
	len  int32
}

func (l *List) Len() int {
	return int(atomic.LoadInt32(&l.len))
}

func (l *List) PushFront(v interface{}) *Element {
	e := &Element{
		list:  l,
		Value: v,
	}

	for {
		lp := atomic.LoadPointer(&l.root.next)
		before, after := &l.root, l.root.Next()
		// Lazy init
		if after == nil {
			after = &l.root
		}
		e.next, e.prev = unsafe.Pointer(after), unsafe.Pointer(before)

		if atomic.CompareAndSwapPointer(&before.next, lp, unsafe.Pointer(e)) {
			atomic.SwapPointer(&after.prev, unsafe.Pointer(e))
			break
		}
	}
	atomic.AddInt32(&l.len, 1)
	return e
}

func (l *List) Remove(e *Element) {
	for {
		before, after := e.Prev(), e.Next()
		if before == nil {
			before = &l.root
		}
		if after == nil {
			after = &l.root
		}

		lp := atomic.LoadPointer(&e.next)
		if atomic.CompareAndSwapPointer(&before.next, unsafe.Pointer(e), lp) {
			atomic.SwapPointer(&after.prev, unsafe.Pointer(before))
			break
		}
	}
	atomic.AddInt32(&l.len, -1)
}

func (l *List) Front() *Element {
	if l.Len() == 0 {
		return nil
	}
	return l.root.Next()
}

func (l *List) Back() *Element {
	if l.Len() == 0 {
		return nil
	}
	return l.root.Prev()
}
