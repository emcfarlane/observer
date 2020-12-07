package observer

import (
	"runtime"
	"sync/atomic"
	_ "unsafe"
)

type entry struct {
	key, val interface{}
	del      bool
	hash     uintptr
}

type store struct {
	count  uint64
	values map[interface{}]interface{}
	view   View
}

func (s *store) flush() {
	n := s.view.Len()
	//fmt.Println("N", n, "--------------")

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
	s.count = 0
}

func (s *store) loadCount() uint64 { return atomic.LoadUint64(&s.count) }

//go:linkname goEFaceHash runtime.efaceHash
func goEFaceHash(i interface{}, seed uintptr) uintptr

const (
	flagAorB uint64 = 1 << 63
	seed            = 0
)

type Map struct {
	init  uintptr // Required for setting up view
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

func (m *Map) flush(write *store) uint64 {
	// Spin until all readers have left.
	for c := write.loadCount(); c != m.writeCount; c = write.loadCount() {
		runtime.Gosched()
	}

	write.flush()

	// Switch A and B.
	x := atomic.AddUint64(&m.count, flagAorB-m.writeCount)
	m.writeCount = x & ^flagAorB
	return x
}

func (m *Map) Get(key interface{}) (val interface{}, ok bool) {
	if atomic.LoadUintptr(&m.init) == 0 {
		return nil, false
	}

	x := atomic.AddUint64(&m.count, 1) // rlock
	read := m.read(x)

	// Check if we have items in the queue.
	l := read.view.Len()
	if l == 1 && (x&(1<<62) == 0) {
		val, ok = read.values[key]
		atomic.AddUint64(&read.count, 1)
		return val, ok
	}

	// Attempt to flush on write lock.
	if m.writeFlag.GetLock() { // wlock
		x := atomic.LoadUint64(&m.count)
		write := m.write(x)

		// Ensure we weren't switched out.
		if read != write {
			m.flush(write)
			val, ok = write.values[key]
			m.writeFlag.Unlock()             // wlock
			atomic.AddUint64(&read.count, 1) // rlock
			return val, ok
		}
		m.writeFlag.Unlock() // wlock
	}

	val, ok = read.values[key]
	v := read.view
	atomic.AddUint64(&read.count, 1) // rlock
	h := goEFaceHash(key, seed)

	//fmt.Println("L", l)
	// Skip the first value, already in map.
	for i := 1; i < l; i++ {
		v = v.Next()

		e := v.Value.(entry)
		if e.hash != h && e.key != key {
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
	h := goEFaceHash(key, seed)
	e := entry{key: key, val: val, del: del, hash: h}

	hasLock := m.writeFlag.GetLock()
	for i := 0; !hasLock && i < 5; i++ {
		runtime.Gosched()
		hasLock = m.writeFlag.GetLock()
	}

	if !hasLock {
		// Spin until first view is set.
		for d := atomic.LoadUintptr(&m.init); d == 0; d = atomic.LoadUintptr(&m.init) {
			runtime.Gosched()
		}

		m.queue.Set(e)
		return
	}
	defer m.writeFlag.Unlock() // defer wlock

	// Init condition, create read & write maps for the first value.
	view := m.queue.Set(e)
	if atomic.LoadUintptr(&m.init) == 0 {
		write := m.write(0)
		write.values = make(map[interface{}]interface{})
		write.view = view

		read := m.read(0)
		read.values = make(map[interface{}]interface{})
		read.view = view

		if !del {
			write.values[key] = val
			read.values[key] = val
		}

		atomic.StoreUintptr(&m.init, 1)
		return
	}

	x := atomic.LoadUint64(&m.count)
	write := m.write(x)

	m.flush(write)
}

func (m *Map) Set(key, val interface{}) { m.set(key, val, false) }
func (m *Map) Del(key interface{})      { m.set(key, nil, true) }
