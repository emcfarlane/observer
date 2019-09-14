package observer

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type entry struct {
	key, val interface{} // []byte
	del      bool
}

const flagAorB uint64 = 1 << 63

type store struct {
	values map[interface{}]interface{}
	count  uint64
	view   *View
}

type Map struct {
	once    sync.Once
	queue   Subject
	state   uint64
	a, b    *store
	pending uint64

	lock   uint32
	want   uint64
	writes int
}

func (m *Map) String() string {
	return fmt.Sprintf("c = %64b\nA = %64b %v\nB = %64b %v\nw = %64b", m.state, m.a.count, len(m.a.values), m.b.count, len(m.b.values), m.want)
}

func (m *Map) aOrB(i uint64) *store {
	if i >= flagAorB {
		return m.a
	}
	return m.b
}

func (m *Map) commit() {
	state := atomic.LoadUint64(&m.state)

	x := m.aOrB(state ^ flagAorB) // Get the write state.
	count := atomic.LoadUint64(&x.count)

	if m.want != count {
		return // waiting
	}

	var i int
	x.view = x.view.Range(func(val interface{}) bool {
		if i != 0 { // Ignore sentinel.
			if e := val.(entry); e.del {
				delete(x.values, e.key)
			} else {
				x.values[e.key] = e.val
			}
		}
		i++
		return i < (64 + m.writes)
	})
	m.writes = i
	x.count = 0

	newState := atomic.AddUint64(&m.state, flagAorB+^(m.want-1))
	m.want = newState &^ flagAorB
}

func (m *Map) tryCommit() {
	if !atomic.CompareAndSwapUint32(&m.lock, 0, 1) {
		return // busy
	}
	m.commit()
	atomic.StoreUint32(&m.lock, 0)
}

func (m *Map) init() {
	m.once.Do(func() {
		m.queue.deadlock = true
		v := m.queue.Set(entry{}) // sentinel

		m.a = &store{
			values: make(map[interface{}]interface{}),
			view:   v,
		}
		m.b = &store{
			values: make(map[interface{}]interface{}),
			view:   v,
		}
	})
}

func searchView(v *View, key interface{}) (value interface{}, ok, del bool) {
	var i int
	v.Range(func(val interface{}) bool {
		if i != 0 {
			// Last write wins.
			if e := val.(entry); e.key == key {
				value, ok, del = e.val, !e.del, e.del
			}
		}
		i++
		return true
	})
	return
}

func (m *Map) Get(key interface{}) (interface{}, bool) {
	m.init()
	m.tryCommit()

	// Increment the state.
	c := atomic.AddUint64(&m.state, 1)
	//fmt.Printf("READ ")
	s := m.aOrB(c)

	// Search queue first, have to check.
	//val, ok, deleted := searchView(s.view, key)
	//if !ok && !deleted {
	val, ok := s.values[key]
	//}

	atomic.AddUint64(&s.count, 1)
	return val, ok
}

func (m *Map) set(key, val interface{}, del bool) {
	m.init()
	m.queue.Set(entry{key: key, val: val, del: del})
	m.tryCommit()
}

func (m *Map) Set(key, val interface{}) {
	m.set(key, val, false)
}

func (m *Map) Del(key interface{}) {
	m.set(key, nil, true)
}
