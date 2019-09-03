package observer

import (
	"bytes"
	"fmt"
	"runtime"
	"sync/atomic"
	"unsafe"

	"github.com/cespare/xxhash"
)

type store struct {
	m map[uint64]*Element //uintptr
	v *View
}

type entry struct {
	key, val []byte
	hash     uint64
}

type Map struct {
	readN unsafe.Pointer // *int32
	read  unsafe.Pointer // *store //map[uint64]uintptr

	start uint32
	queue Subject
	list  List // LRU

	writeN *int32
	write  *store
}

func (m *Map) run() {
	for v := m.queue.View(); ; v.Next() {

		// Wait for writeN to go to zero
		for {
			if n := atomic.LoadInt32(m.writeN); n == 0 {
				break
			}
			runtime.Gosched()
		}

		// Dump all current writes TODO: exhaustion?
		for ; v != nil; v = v.Load() {
			e := v.Value().(entry)
			le := m.list.PushFront(e)

			m.write.m[e.hash] = le // uintptr(unsafe.Pointer(le))
			m.write.v = v
		}
		v = m.write.v // last

		// Swap
		r := atomic.LoadPointer(&m.read)
		if !atomic.CompareAndSwapPointer(&m.read, r, unsafe.Pointer(m.write)) {
			panic("read/write swap corruption")
		}
		m.writeN = (*int32)(atomic.SwapPointer(&m.readN, unsafe.Pointer(m.writeN)))
		fmt.Println("wrote!")
	}
}

func (m *Map) flush() {
	if atomic.CompareAndSwapUint32(&m.start, 0, 1) {
		var wN, rN int32
		atomic.SwapPointer(&m.read, unsafe.Pointer(&store{
			m: make(map[uint64]*Element),
		}))
		atomic.SwapPointer(&m.readN, unsafe.Pointer(&rN))

		m.write = &store{
			m: make(map[uint64]*Element),
		}
		m.writeN = &wN

		go m.run()
	}
}

func searchView(v *View, key []byte) (val []byte, ok bool) {
	for ; v != nil; v = v.Load() {
		e := v.Value().(entry)
		if bytes.Compare(e.key, key) == 0 {
			val = e.val
			ok = true
		}
	}
	return
}

func (m *Map) Get(key []byte) ([]byte, bool) {
	fmt.Println("reading...")
	defer fmt.Println("read!")

	counter := (*int32)(atomic.LoadPointer(&m.readN))
	if counter == nil {
		fmt.Println("init")
		return nil, false // init
	}
	atomic.AddInt32(counter, 1)

	s := (*store)(atomic.LoadPointer(&m.read))

	// Search queue first, have to check.
	val, ok := searchView(s.v, key)
	if !ok {
		var le *Element
		le, ok = s.m[xxhash.Sum64(key)]
		if ok {
			// TODO: misuse fixes
			//le := (*Element)(unsafe.Pointer(raw))
			e := le.Value.(entry)
			val = e.val
		}
	}

	atomic.AddInt32(counter, -1)
	return val, ok
}

func (m *Map) Set(key, val []byte) bool {
	e := entry{key: key, val: val, hash: xxhash.Sum64(key)}
	m.queue.Set(e)
	m.flush()
	return true
}

func (m *Map) Del(key []byte) bool {
	// TODO
	return false
}
