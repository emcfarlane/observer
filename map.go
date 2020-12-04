package observer

import (
	"runtime"
	"sync/atomic"
)

type entry struct {
	key, val interface{}
	del      bool
}

type store struct {
	count  uint64
	values map[interface{}]interface{}
	view   View
}

func (s *store) flush() {
	n := s.view.Len()

	// Skip the first value, already flushed to map.
	for i := 1; i < n; i++ {
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

func (s *store) loadCount() uint64 { return atomic.LoadUint64(&s.count) }

const flagAorB uint64 = 1 << 63

type Map struct {
	init  uint32 // Required for setting up view
	count uint64
	a, b  store
	queue Subject

	writeFlag  spin
	writeCount uint64
}

func (m *Map) read(x uint64) *store {
	if x > flagAorB {
		return &m.a
	}
	return &m.b
}

func (m *Map) write(x uint64) *store {
	if x > flagAorB {
		return &m.b
	}
	return &m.a
}

func (m *Map) Get(key interface{}) (val interface{}, ok bool) {
	if atomic.LoadUint32(&m.init) == 0 {
		return nil, false
	}

	x := atomic.AddUint64(&m.count, 1)
	read := m.read(x)
	defer atomic.AddUint64(&read.count, 1)

	val, ok = read.values[key]

	// Check if we have items in the queue.
	l := read.view.Len()
	if l == 1 {
		return val, ok
	}

	if m.writeFlag.GetLock() {
		write := m.write(x)

		// Spin until all readers have left.
		for wc := write.loadCount(); wc < m.writeCount; wc = write.loadCount() {
			runtime.Gosched()
		}

		write.flush()
		write.count = 0
		val, ok = write.values[key]

		// Switch A and B.
		x = atomic.AddUint64(&m.count, flagAorB-m.writeCount)
		m.writeCount = x & (^flagAorB)

		m.writeFlag.Unlock()
		return val, ok
	}

	// Skip the first value, already in map.
	for i, v := 1, read.view; i < l; i++ {
		v = v.Next()

		e := v.Value.(entry)
		if e.key != key {
			continue
		}

		if e.del {
			val, ok = nil, false
		} else {
			val, ok = e.val, true
		}
	}
	return val, ok
}

func (m *Map) set(key, val interface{}, del bool) {
	e := entry{key: key, val: val, del: del}

	if !m.writeFlag.GetLock() {
		// Spin until first view is set.
		for d := atomic.LoadUint32(&m.init); d == 0; d = atomic.LoadUint32(&m.init) {
			runtime.Gosched()
		}

		m.queue.Set(e)
		return
	}
	defer m.writeFlag.Unlock()

	x := atomic.LoadUint64(&m.count)
	write := m.write(x)

	// Init condition, create read & write maps for the first value.
	view := m.queue.Set(e)
	if atomic.LoadUint32(&m.init) == 0 {
		write.values = make(map[interface{}]interface{})
		write.view = view

		read := m.read(x)
		read.values = make(map[interface{}]interface{})
		read.view = view

		if !del {
			write.values[key] = val
			read.values[key] = val
		}

		atomic.StoreUint32(&m.init, 1)
		return
	}

	// Spin until all readers have left.
	for wc := write.loadCount(); wc < m.writeCount; wc = write.loadCount() {
		runtime.Gosched()
	}

	// Write as many elements as possible.
	write.flush()
	write.count = 0

	// Switch A and B.
	x = atomic.AddUint64(&m.count, flagAorB-m.writeCount)
	m.writeCount = x & (^flagAorB)
}

func (m *Map) Set(key, val interface{}) { m.set(key, val, false) }
func (m *Map) Del(key interface{})      { m.set(key, nil, true) }
