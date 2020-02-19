package observer

import (
	"math/bits"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

// View is a value seen by the observer.
type View struct {
	Value interface{}
	index uint8
}

func (v *View) frame() *frame {
	// TODO: justify this...
	return (*frame)(unsafe.Pointer(
		uintptr(unsafe.Pointer(v)) - uintptr(v.index)*unsafe.Sizeof(*v)),
	)
}

// Next returns the next view or blocks until a new value is set.
func (v *View) Next() *View {
	i := v.index + 1
	f := v.frame()

	if i != 64 {
		if f.has(i) {
			return &f.views[i]
		}

		// Wait for the next view.
		f.sub.mu.Lock()
		for !f.has(i) {
			f.sub.cond.Wait()
		}
		f.sub.mu.Unlock()
		return &f.views[i]
	}

	if next := f.load(); next != nil {
		return &next.views[0]
	}

	// Wait for the next frame.
	f.sub.mu.Lock()
	next := f.load()
	for ; next == nil; next = f.load() {
		f.sub.cond.Wait()
	}
	f.sub.mu.Unlock()
	return &next.views[0]
}

func (v *View) Range(fn func(val interface{}) bool) *View {
	f := v.frame()
	for i, j := v.index, f.last(); ; i, j = 0, f.last() {
		var next *frame
		if j == 63 {
			next = f.load()
		}

		// Ensure all values are set until latest.
		want := ^uint64(0) >> (63 - j)
		for atomic.LoadUint64(&f.mask)&want != want {
			// Spin lock.
			runtime.Gosched()
		}

		for ; i <= j; i++ {
			if !fn(f.views[i].Value) {
				return &f.views[i]
			}
		}

		if next == nil {
			return &f.views[j]
		}
		f = next
	}
}

type frame struct {
	views [64]View
	mask  uint64
	sub   *Subject
	next  unsafe.Pointer
	count uint32
}

func (f *frame) load() *frame {
	return (*frame)(atomic.LoadPointer(&f.next))
}

func (f *frame) has(i uint8) bool {
	return atomic.LoadUint64(&f.mask)&(1<<i) != 0
}

func (f *frame) last() uint8 {
	return uint8(bits.Len64(atomic.LoadUint64(&f.mask)) - 1)
}

func (f *frame) set(i uint8, val interface{}) *View {
	f.views[i].Value = val
	f.views[i].index = i
	atomic.AddUint64(&f.mask, 1<<i)
	return &f.views[i]
}

func (f *frame) latest() *View {
	return &f.views[f.last()]
}

// A Subject controls broadcasting events to multiple viewers.
type Subject struct {
	mu    sync.Mutex
	cond  sync.Cond
	frame unsafe.Pointer

	deadlock bool // Skip lock on broadcast, can deadlock.
}

func (s *Subject) load() *frame {
	return (*frame)(atomic.LoadPointer(&s.frame))
}

// View returns the latest value for the subject.
// Blocks if Set has not been called.
func (s *Subject) View() *View {
	f := s.load()
	if f != nil {
		return f.latest()
	}

	// init
	s.mu.Lock()
	if s.cond.L == nil {
		s.cond.L = &s.mu
	}
	for f = s.load(); f == nil; f = s.load() {
		s.cond.Wait()
	}
	s.mu.Unlock()
	return &f.views[0]
}

// Set the latest view to val and notify waiting viewers.
func (s *Subject) Set(val interface{}) (v *View) {
	f := s.load()
	if f == nil {
		s.mu.Lock()
		if s.cond.L == nil {
			s.cond.L = &s.mu
		}
		if f = s.load(); f == nil {
			f = &frame{sub: s}
			v = f.set(0, val)

			atomic.StorePointer(&s.frame, unsafe.Pointer(f))
			s.cond.Broadcast()
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}

	i := atomic.AddUint32(&f.count, 1)
	for ; i > 64; i = atomic.AddUint32(&f.count, 1) {
		// Spin lock.
		next := f.load()
		for ; next == nil; next = f.load() {
			runtime.Gosched()
		}
		f = next
	}

	if i == 64 {
		next := &frame{sub: s}
		v = next.set(0, val)
		atomic.StorePointer(&f.next, unsafe.Pointer(next))
		atomic.StorePointer(&s.frame, unsafe.Pointer(next))
	} else {
		v = f.set(uint8(i), val)
	}

	if s.deadlock {
		s.cond.Broadcast()
		return
	}
	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
	return
}
