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
// The zero value for a Subject is a nil valued View.
type Subject struct {
	mu   sync.Mutex
	cond *sync.Cond
	view *View
}

func (s *Subject) setWithLock(val interface{}) *View {
	v := &View{value: val, sub: s}
	if s.cond == nil {
		s.cond = sync.NewCond(&s.mu)
	}
	if s.view != nil {
		atomic.StorePointer(&s.view.next, unsafe.Pointer(v))
	}
	s.view = v
	return v
}

// View returns the latest value for the subject.
func (s *Subject) View() *View {
	s.mu.Lock()
	v := s.view
	if v == nil {
		v = s.setWithLock(nil)
	}
	s.mu.Unlock()
	return v
}

// Set the latest view to val and notify waiting viewers.
func (s *Subject) Set(val interface{}) *View {
	s.mu.Lock()
	v := s.setWithLock(val)
	s.mu.Unlock()
	s.cond.Broadcast()
	return v
}
