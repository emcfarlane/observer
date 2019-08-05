package observer

import (
	"runtime"
	"sync/atomic"
)

type spin struct {
	lock uintptr
}

func (s *spin) Lock() {
	for !atomic.CompareAndSwapUintptr(&s.lock, 0, 1) {
		runtime.Gosched()
	}
}

func (s *spin) Unlock() {
	atomic.StoreUintptr(&s.lock, 0)
}
