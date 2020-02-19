package observer

import (
	"fmt"
	"runtime"
	"sync/atomic"
	_ "unsafe"
)

type entry struct {
	key, val interface{} // []byte
	del      bool
	hash     uintptr
}

const flagAorB uint64 = 1 << 63

//go:linkname goEFaceHash runtime.efaceHash
func goEFaceHash(i interface{}, seed uintptr) uintptr

type store struct {
	values map[interface{}]interface{}
	count  uint64
	view   *View
}

func (s *store) search(key interface{}, max int) (value interface{}, ok, del bool) {
	hash := goEFaceHash(key, 0)
	//fmt.Println("searching for", hash, key)

	var i int
	s.view.Range(func(val interface{}) bool {
		// Last write wins.
		if i != 0 {
			e := val.(entry)
			if e.hash == hash && e.key == key {
				value, ok, del = e.val, !e.del, e.del
				//fmt.Println("search found", e)
			}
		}
		i++
		return i <= max
	})
	return
}

func (s *store) commit(max int) int {
	var i int
	s.view = s.view.Range(func(val interface{}) bool {
		if i != 0 {
			if e := val.(entry); e.del {
				delete(s.values, e.key)
				//f/mt.Println("deleted", e)
			} else {
				s.values[e.key] = e.val
				//fmt.Println("set", e)
			}
		}
		i++
		//fmt.Println("i <= max", i <= max)
		return i <= max
	})
	return i - 1
}

type Map struct {
	//once    sync.Once
	queue   Subject
	state   uint64
	a, b    store
	pending uint64

	lock   uint64
	want   uint64
	writes int
}

func (m *Map) String() string {
	return fmt.Sprintf("c = %64b\nA = %64b %v\nB = %64b %v\nw = %64b\np = %64b", m.state, m.a.count, len(m.a.values), m.b.count, len(m.b.values), m.want, m.pending)
}

func (m *Map) aOrB(i uint64) *store {
	if i >= flagAorB {
		return &m.a
	}
	return &m.b
}

/*func (m *Map) commit() {
	state := atomic.LoadUint64(&m.state)

	x := m.aOrB(state ^ flagAorB) // Get the write state.
	count := atomic.LoadUint64(&x.count)

	if m.want != count {
		return // waiting
	}

	if x.values == nil {
		x.values = make(map[interface{}]interface{})
	}
	if x.view == nil {
		x.view = m.queue.Set(entry{}) // sentinel
		y := m.aOrB(state)
		y.view = x.view
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
	//fmt.Println("STATE\n" + m.String())
}

func (m *Map) tryCommit() {
	for i := atomic.AddUint32(&m.lock, 1); i != 1; i = atomic.AddUint32(&m.lock, 1) {
		if i <= 64 {
			return
		}
		//fmt.Println("throttle")
		runtime.Gosched() // throttle
	}
	//if !atomic.CompareAndSwapUint32(&m.lock, 0, 1) {
	//	return // busy
	//}
	m.commit()
	atomic.StoreUint32(&m.lock, 0)
}*/

/*func (m *Map) init() {
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
}*/

func (m *Map) Get(key interface{}) (val interface{}, ok bool) {
	//m.init()
	//m.tryCommit()

	// TODO: should it just be up to writers to flush?
	// If there are pending writes try take the lock and commit pending.
	/*p := atomic.LoadUint64(&m.pending)
	if p > 0 && atomic.AddUint64(&m.lock, 1) == 1 {
		//fmt.Println("HAVE LOCK")
		atomic.StoreUint64(&m.lock, 0)
	}*/

	// Increment the state.
	state := atomic.AddUint64(&m.state, 1)
	//fmt.Printf("READ ")
	r := m.aOrB(state)

	if r.values != nil {
		var del bool
		p := atomic.LoadUint64(&m.pending)
		if p > 0 {
			fmt.Println("in pending????", p)
			val, ok, del = r.search(key, int(p))
		}
		// Search queue first, have to check.
		//var del bool
		//val, ok, del = searchView(s.view, key)
		//if !ok && !del {
		if !ok && !del {
			val, ok = r.values[key]
			//fmt.Println("found", val)
		}
		//}
	}

	atomic.AddUint64(&r.count, 1)
	return val, ok
}

const maxWrites = 8

func (m *Map) set(key, val interface{}, del bool) {
	hash := goEFaceHash(key, 0)
	e := entry{key: key, val: val, del: del, hash: hash}
	//atomic.AddUint64(&m.pending, 1)
	//v := m.queue.Set(e)

	if i := atomic.AddUint64(&m.lock, 1); i != 1 {
		// Set
		m.queue.View() // Wait for first event
		m.queue.Set(e)
		atomic.AddUint64(&m.pending, 1)

		return // TODO: throttle?
	}
	//fmt.Println("trying to set")

	state := atomic.LoadUint64(&m.state)
	w := m.aOrB(state ^ flagAorB) // Get the write state, get it.
	//count := atomic.LoadUint64(&x.count)

	// Spin wait for readers to leave.
	count := atomic.LoadUint64(&w.count)
	for ; m.want != count; count = atomic.LoadUint64(&w.count) {
		runtime.Gosched() // TODO: throttle?
		//fmt.Println("spin wait!", m.want, "!=", count)
	}

	if w.view == nil {
		//x.view = v
		w.view = m.queue.Set(entry{}) // sentinel
		r := m.aOrB(state)
		r.view = w.view
	}
	m.queue.Set(e) // We are somewhere in the queue.
	atomic.AddUint64(&m.pending, 1)

	if w.values == nil {
		w.values = make(map[interface{}]interface{})
	}

	//fmt.Println("trying to commit!")
	n := w.commit(maxWrites + m.writes)
	//fmt.Println("wrote n", n)
	m.writes = n - m.writes // Update the new number of writes.

	w.count = 0 // Don't need atomic as all readers have left
	newState := atomic.AddUint64(&m.state, flagAorB+^(m.want-1))
	m.want = newState &^ flagAorB

	//fmt.Println("PENDING", m.pending, -m.writes)
	atomic.AddUint64(&m.pending, ^uint64(m.writes-1))
	atomic.StoreUint64(&m.lock, 0)
}

func (m *Map) Set(key, val interface{}) {
	m.set(key, val, false)
}

func (m *Map) Del(key interface{}) {
	m.set(key, nil, true)
}
