package observer

/*
import (
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

type store struct {
	m map[interface{}]interface{}
	v *View
}

type entry struct {
	key, val interface{} // []byte
	del      bool
}

type Map struct {
	readN unsafe.Pointer // *int32
	read  unsafe.Pointer // *store

	once   sync.Once
	mu     sync.Mutex
	cond   sync.Cond // backpressure
	queue  Subject
	length int32

	writeN *int32
	write  *store

	view  *View
	count int
}

func (m *Map) n() int32 {
	return atomic.LoadInt32(m.writeN)
}

func (m *Map) run() {
	for i, v := int32(0), m.queue.View(); ; i, v = 0, v.Next() {

		// Wait for writeN to go to zero
		for n := m.n(); n != 0; n = m.n() {
			runtime.Gosched() // TODO: sync.Cond schedule?
		}

		// Replay entries
		if wv := m.write.v; wv != nil {
			wv.Range(func(val interface{}) bool {
				//return v != wv
				return false
			})

		}
		for wv := m.write.v; wv != nil && v != wv; wv = wv.Load() {
			m.write.set(v)
		}

		// Write new
		for ; v != nil && i < 128; v = v.Load() {
			m.write.set(v)
			i++
		}
		v = m.write.v // rewind to last...

		// Swap
		m.writeN = (*int32)(atomic.SwapPointer(&m.readN, unsafe.Pointer(m.writeN)))
		m.write = (*store)(atomic.SwapPointer(&m.read, unsafe.Pointer(m.write)))

		// Throttle
		if n := atomic.AddInt32(&m.length, -i); n < 128 {
			m.mu.Lock()
			m.cond.Broadcast()
			m.mu.Unlock()
		}
	}
}

func (m *Map) tryCommit() {
	if !atomic.CompareAndSwapInt32(m.writeN, 0, -1) {
		return // busy
	}

	if m.count != 0 {
		var i int
		m.write.v = m.write.v.Range(func(val interface{}) bool {
			if e := v.Value.(entry); e.del {
				delete(s.m, e.key)
			} else {
				s.m[e.key] = e.val
			}

			i++
			return i < m.count
		})
	}

	m.count = 0
	m.write.v

}

func (m *Map) init() {
	m.once.Do(func() {
		m.cond.L = &m.mu

		m.queue.deadlock = true
		v := m.queue.Set(entry{}) // sentinel

		var wN, rN int32

		wS := store{m: make(map[interface{}]interface{}), v: v}
		m.write = &wS
		m.writeN = &wN

		rS := store{m: make(map[interface{}]interface{}), v: v}
		atomic.SwapPointer(&m.read, unsafe.Pointer(&rS))
		atomic.SwapPointer(&m.readN, unsafe.Pointer(&rN))

		go m.run()
	})
}

func searchView(v *View, key interface{}) (val interface{}, ok, deleted bool) {
	v.Range(func(val interface{}) bool {
		e := val.(entry)
		if e.key == key {
			// Last write wins
			val, ok, deleted = e.val, !e.del, e.del
		}
		return true
	})
	return
}

func (m *Map) throttle() {
	m.mu.Lock()
	for {
		if n := atomic.LoadInt32(&m.length); n <= 128 {
			break
		}
		m.cond.Wait()
	}
	m.mu.Unlock()
}

func (m *Map) Get(key interface{}) (interface{}, bool) {
	if n := atomic.LoadInt32(&m.length); n > 128 {
		m.throttle()
	}

	counter := (*int32)(atomic.LoadPointer(&m.readN))
	if counter == nil {
		return nil, false // init
	}
	atomic.AddInt32(counter, 1)

	s := (*store)(atomic.LoadPointer(&m.read))

	// Search queue first, have to check
	val, ok, deleted := searchView(s.v, key)
	if !ok && !deleted {
		val, ok = s.m[key]
	}

	atomic.AddInt32(counter, -1)
	return val, ok
}

func (m *Map) set(key, val interface{}, del bool) {
	m.init()
	if n := atomic.AddInt32(&m.length, 1); n > 128 {
		m.throttle()
	}

	e := entry{key: key, val: val, del: del}
	m.queue.Set(e)
}

func (m *Map) Set(key, val interface{}) { m.set(key, val, false) }

func (m *Map) Del(key interface{}) { m.set(key, nil, true) }*/
