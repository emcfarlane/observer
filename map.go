package observer

import (
	"runtime"
	"sync"
	"sync/atomic"
)

type entry struct {
	key, val interface{}
	del, ok  bool
}

type store struct {
	count  uint64
	values map[interface{}]interface{}
}

func (s *store) loadCount() uint64 { return atomic.LoadUint64(&s.count) }

const flagAorB uint64 = 1 << 63

type Map struct {
	count uint64
	a, b  store

	writeMu    sync.Mutex
	writeCount uint64
	writeEntry entry
}

func (m *Map) read(x uint64) *store {
	if x >= flagAorB {
		return &m.a
	}
	return &m.b
}

func (m *Map) write(x uint64) *store {
	if x < flagAorB {
		return &m.a
	}
	return &m.b
}

func (m *Map) Get(key interface{}) (val interface{}, ok bool) {
	x := atomic.AddUint64(&m.count, 1) // rlock
	read := m.read(x)
	val, ok = read.values[key]
	atomic.AddUint64(&read.count, 1) // rlock
	return val, ok
}

func (m *Map) set(key, val interface{}, del bool) {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	x := atomic.LoadUint64(&m.count)
	write := m.write(x)

	// Spin until all readers have left.
	for c := write.loadCount(); c != m.writeCount; c = write.loadCount() {
		runtime.Gosched()
	}

	if write.values == nil {
		write.values = make(map[interface{}]interface{})
	}

	if e := m.writeEntry; e.ok {
		if !e.del {
			write.values[e.key] = e.val
		} else {
			delete(write.values, e.key)
		}
	}

	m.writeEntry = entry{key: key, val: val, del: del, ok: true}

	if !del {
		write.values[key] = val
	} else {
		delete(write.values, key)
	}
	write.count = 0

	// Switch A and B.
	x = atomic.AddUint64(&m.count, flagAorB-m.writeCount)
	m.writeCount = x & ^flagAorB
}

func (m *Map) Set(key, val interface{}) { m.set(key, val, false) }
func (m *Map) Del(key interface{})      { m.set(key, nil, true) }
