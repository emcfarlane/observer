package observer

import (
	"math/bits"
	"sync"
	"sync/atomic"
	"unsafe"
)

// View is a value seen by the observer.
type View struct {
	Value interface{}
	frame *frame
}

func (v *View) index() uint64 {
	a, b := uintptr(unsafe.Pointer(&v.frame.views[0])), uintptr(unsafe.Pointer(v))
	diff := b - a
	c := diff / unsafe.Sizeof(*v)
	return uint64(c)
}

// Next returns the next view or blocks until a new value is set.
func (v *View) Next() *View {
	i := v.index() + 1
	f := v.frame

	if i != 64 && f.has(i) {
		return &f.views[i]
	}

	// Slow-path.
	f.sub.mu.Lock()
	if i == 64 {
		// Wait for the next frame
		for next := f.load(); next == nil; next = f.load() {
			f.sub.cond.Wait()
		}
		i = 0
	} else {
		// Wait for the next view
		for !f.has(i) {
			f.sub.cond.Wait()
		}
	}
	f.sub.mu.Unlock()
	return &f.views[i]
}

func (v *View) Range(fn func(val interface{}) bool) *View {
	f := v.frame
	vNext := f.latest()
	i, j := v.index(), vNext.index()

	for ; i <= j; i++ {
		if !fn(f.views[i].Value) {
			return &f.views[i]
		}
	}
	if j == 64 {
		// TODO: non recursive
		if f = f.load(); f != nil {
			return f.views[0].Range(fn)
		}
	}

	return &f.views[j]
}

type frame struct {
	mask  uint64
	views [64]View
	sub   *Subject
	next  unsafe.Pointer
	count uint32
}

func (f *frame) load() *frame {
	return (*frame)(atomic.LoadPointer(&f.next))
}

func (f *frame) has(i uint64) bool {
	return atomic.LoadUint64(&f.mask)&(1<<i) != 0
}

func (f *frame) set(i uint64, val interface{}) *View {
	f.views[i].Value = val
	f.views[i].frame = f
	atomic.AddUint64(&f.mask, 1<<i)
	return &f.views[i]
}

func (f *frame) latest() *View {
	i := 63 - bits.LeadingZeros64(atomic.LoadUint64(&f.mask))
	return &f.views[i]
}

// A Subject controls broadcasting events to multiple viewers.
type Subject struct {
	mu    sync.Mutex
	cond  sync.Cond // TODO: split mask/frame cond?
	frame unsafe.Pointer

	deadlock bool
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

	// Slow-path.
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
		// Slow-path.
		f.sub.mu.Lock()
		for f = f.load(); f == nil; f = f.load() {
			f.sub.cond.Wait()
		}
		f.sub.mu.Unlock()
	}

	if i == 64 {
		fNew := &frame{sub: s}
		v = fNew.set(0, val)
		atomic.StorePointer(&f.next, unsafe.Pointer(fNew))
		atomic.StorePointer(&s.frame, unsafe.Pointer(fNew))
	} else {
		v = f.set(uint64(i), val)
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
