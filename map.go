package observer

import (
	"runtime"
	"sync"
	"sync/atomic"
	_ "unsafe"
)

type entry struct {
	key, val interface{} // TODO: []byte?
	del      bool
}

//go:linkname goEFaceHash runtime.efaceHash
func goEFaceHash(i interface{}, seed uintptr) uintptr

type store struct {
	sync.RWMutex
	values map[interface{}]interface{}
	view   *View
}

func (s *store) flush(to *View) {
	// Lock waits for all readers to leave.
	s.Lock()
	defer s.Unlock()

	for s.view != to {
		v := s.view.Next()

		e := v.Value.(entry)
		if e.del {
			delete(s.values, e.key)
		} else {
			s.values[e.key] = e.val
		}

		s.view = v
	}
}

type Map struct {
	read atomic.Value

	writeFlag spin
	write     *store
	queue     Subject
}

func (m *Map) Get(key interface{}) (val interface{}, ok bool) {
	read, ok := m.read.Load().(*store)
	if !ok {
		// Failed to load store, happens on init.
		return nil, false
	}

	read.RLock()
	defer read.RUnlock()

	val, ok = read.values[key]

	// Check if we have items in the queue.
	l := read.view.Len()
	if l == 1 {
		return val, ok
	}

	hasWrite := m.writeFlag.GetLock()

	// Ensure we havn't been switched out during the read.
	if hasWrite && m.write == read {
		m.writeFlag.Unlock()
		hasWrite = false
	}

	view := read.view
	for i := 0; i < l; i++ {
		view = view.Next()

		e := view.Value.(entry)
		if e.key != key {
			continue
		}

		if e.del {
			val, ok = nil, false
		} else {
			val, ok = e.val, true
		}
	}

	if hasWrite {
		m.write.flush(view)
		m.read.Store(m.write)
		m.write = read
		m.writeFlag.Unlock()
	}
	return val, ok
}

func (m *Map) set(key, val interface{}, del bool) {
	//hash := goEFaceHash(key, 0)
	e := entry{key: key, val: val, del: del}

	if !m.writeFlag.GetLock() {
		// Init condition, spin on read map being created.
		for read := m.read.Load(); read != nil; read = m.read.Load() {
			runtime.Gosched()
		}

		m.queue.Set(e)
		return
	}
	defer m.writeFlag.Unlock()

	view := m.queue.Set(e)

	// Init condition, create read & write maps for the first value.
	if m.write == nil {
		m.write = &store{
			values: make(map[interface{}]interface{}),
			view:   view,
		}
		read := &store{
			values: make(map[interface{}]interface{}),
			view:   view,
		}
		if !del {
			m.write.values[key] = val
			read.values[key] = val
		}
		m.read.Store(read)
		return
	}

	// Write up until the view.
	m.write.flush(view)

	read := m.read.Load().(*store)
	m.read.Store(m.write)
	m.write = read
	return
}

func (m *Map) Set(key, val interface{}) { m.set(key, val, false) }
func (m *Map) Del(key interface{})      { m.set(key, nil, true) }
