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

const (
	flagAorB uint64 = 1 << 63
	flagBusy uint64 = 1 << 62
)

type store struct {
	values map[interface{}]interface{}
	count  uint64
	view   *View
}

type Map struct {
	once   sync.Once
	queue  Subject
	state  uint64
	a, b   store
	want   uint64
	writes int

	///lock uintptr
	//writeN unsafe.Pointer // *int32
	//writeN *int32
	//write  *store
	//ktotal int
}

func (m *Map) String() string {
	return fmt.Sprintf("c = %64b\nA = %64b %v\nB = %64b %v\nw = %64b", m.state, m.a.count, len(m.a.values), m.b.count, len(m.b.values), m.want)
}

func (m *Map) aOrB(i uint64) (*store, uint64) {
	if i >= flagAorB {
		fmt.Println(i, "--------- A")
		return &m.a, flagAorB
	}
	fmt.Println(i, "--------- B")
	return &m.b, 0
}

//atomic.AddUint64(&m.want, flagBusy)
//fmt.Printf("busy %64b\n", uint64(1<<60))
//fmt.Printf("busy %64b\n", ^uint64((1<<60)-1))
//fmt.Println(uint64(uint64(1<<60) - ^uint64((1<<60)-1)))
//fmt.Println("writing", val)

//count := atomic.LoadUint64(&m.count)
//s, other := m.aOrB(count ^ flagAorB)
//sCount := atomic.LoadUint64(&s.count)

//if !atomic.CompareAndSwapUint64(&m.want, sCount+other, flagAorB) {
//	return // busy
//}

func (m *Map) tryCommit() {
	// ---------------
	//fmt.Println(m)

	want := atomic.LoadUint64(&m.want) // 1000001
	wantClean := want & ^flagAorB
	//fmt.Println("wantClean", wantClean)
	fmt.Println("TRY\n" + m.String())

	fmt.Printf("WRITE ")
	x, other := m.aOrB(want ^ flagAorB)
	if !atomic.CompareAndSwapUint64(&x.count, wantClean, flagAorB) {
		return // busy
	}
	fmt.Println("LOCKED", wantClean, "\n"+m.String())

	// ---------------
	//fmt.Println(m)

	var i int
	x.view = x.view.Range(func(val interface{}) bool {
		if i != 0 { // Ignore sentinel.
			if e := val.(entry); e.del {
				delete(x.values, e.key)
			} else {
				//fmt.Println("set", e.key, e.val)
				x.values[e.key] = e.val
			}
		}
		i++
		return i < (64 + m.writes)
	})
	m.writes = i

	y, _ := m.aOrB(want)

	count := atomic.AddUint64(&m.state, ^(want + other - 1))
	//atomic.AddUint64(&x.count, ^(flagAorB - 1))
	fmt.Println("STATE\n" + m.String())

	atomic.StoreUint64(&m.want, count)
	fmt.Println("WANT\n" + m.String())

	// Relase other map counter.
	atomic.AddUint64(&y.count, ^(flagAorB - 1))
	fmt.Println("RELEASE\n" + m.String())
}

func (m *Map) init() {
	m.once.Do(func() {
		m.queue.deadlock = true
		v := m.queue.Set(entry{}) // sentinel

		m.a.values = make(map[interface{}]interface{})
		m.a.view = v
		m.b.values = make(map[interface{}]interface{})
		m.b.view = v
		m.b.count = flagAorB
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

	// Increment the state
	c := atomic.AddUint64(&m.state, 1)
	fmt.Printf("READ ")
	s, _ := m.aOrB(c)

	// Search queue first, have to check.
	val, ok, deleted := searchView(s.view, key)
	if !ok && !deleted {
		val, ok = s.values[key]
	}

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
