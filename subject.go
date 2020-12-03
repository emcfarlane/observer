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
	frame *frame
	Value interface{}
	index uint8
}

// Next returns the next view or blocks until a new value is set.
func (v View) Next() View {
	i := v.index + 1
	f := v.frame

	if i != 64 {
		if f.has(i) {
			return f.index(i)
		}

		// Wait for the next view.
		f.sub.mu.Lock()
		for !f.has(i) {
			f.sub.cond.Wait()
		}
		f.sub.mu.Unlock()
		return f.index(i)
	}

	if next := f.load(); next != nil {
		return next.index(0)
	}

	// Wait for the next frame.
	f.sub.mu.Lock()
	next := f.load()
	for ; next == nil; next = f.load() {
		f.sub.cond.Wait()
	}
	f.sub.mu.Unlock()
	return next.index(0)
}

// Len returns the current length.
func (v View) Len() int {
	i := -int(v.index)
	for f := v.frame; f != nil; f = f.load() {
		j := f.last() + 1
		i += int(j)
		if j != 64 {
			break // skip load
		}
	}
	return i
}

type frame struct {
	views [64]interface{}
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

func (f *frame) set(i uint8, val interface{}) View {
	f.views[i] = val
	atomic.AddUint64(&f.mask, 1<<i)
	return View{frame: f, Value: val, index: i}
}

func (f *frame) latest() View {
	i := f.last()
	return f.index(i)
}

func (f *frame) index(i uint8) View {
	val := f.views[i]
	return View{frame: f, Value: val, index: i}
}

// A Subject controls broadcasting events to multiple viewers.
type Subject struct {
	mu    sync.Mutex
	cond  sync.Cond
	frame unsafe.Pointer
}

func (s *Subject) load() *frame {
	return (*frame)(atomic.LoadPointer(&s.frame))
}

// View returns the latest value for the subject.
// Blocks if Set has not been called.
func (s *Subject) View() View {
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
	return f.index(0)
}

// Set the latest view to val and notify waiting viewers.
func (s *Subject) Set(val interface{}) (v View) {
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

	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
	return
}
