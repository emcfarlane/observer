package observer

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

// View is a value seen by the observer.
type View struct {
	value interface{}
	sub   *Subject
	next  unsafe.Pointer
}

func (v *View) load() *View {
	return (*View)(atomic.LoadPointer(&v.next))
}

// Next returns the next view or blocks until a new value is set.
func (v *View) Next() *View {
	next := v.load()
	if next != nil {
		return next
	}

	// Slow-path.
	v.sub.mu.Lock()
	for next = v.load(); next == nil; next = v.load() {
		v.sub.cond.Wait()
	}
	v.sub.mu.Unlock()
	return next
}

// Value of the current view.
func (v *View) Value() interface{} { return v.value }

// A Subject controls broadcasting events to multiple viewers.
type Subject struct {
	mu   sync.Mutex
	cond *sync.Cond
	view unsafe.Pointer
}

func (s *Subject) load() *View {
	return (*View)(atomic.LoadPointer(&s.view))
}

// View returns the latest value for the subject.
// Blocks if Set has not been called.
func (s *Subject) View() *View {
	v := s.load()
	if v != nil {
		return v
	}

	// Slow-path.
	s.mu.Lock()
	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}
	for v = s.load(); v == nil; v = s.load() {
		s.cond.Wait()
	}
	s.mu.Unlock()
	return v
}

// Set the latest view to val and notify waiting viewers.
func (s *Subject) Set(val interface{}) *View {
	v := &View{value: val, sub: s}
	vOld := s.load()
	if vOld == nil {
		s.mu.Lock()
		if s.cond == nil {
			s.cond = sync.NewCond(&s.mu)
		}
		if vOld = s.load(); vOld == nil {
			atomic.StorePointer(&s.view, unsafe.Pointer(v))
			s.cond.Broadcast()
			s.mu.Unlock()
			return v
		}
		s.mu.Unlock()
	}

	for !atomic.CompareAndSwapPointer(&s.view, unsafe.Pointer(vOld), unsafe.Pointer(v)) {
		vOld = s.load()
	}
	atomic.StorePointer(&vOld.next, unsafe.Pointer(v))

	s.mu.Lock()
	s.cond.Broadcast()
	s.mu.Unlock()
	return v
}
