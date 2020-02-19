package observer

import (
	"sync"
	"sync/atomic"
)

type Batch struct {
	mu    sync.Mutex
	cond  sync.Cond
	count uint32
}

func (b *Batch) Exec(f func()) {
	i := atomic.AddUint32(&b.count, 1)

	if i > 8 {

	}
	if i > 1 {

	}

}
