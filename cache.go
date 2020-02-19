package observer

import (
	"github.com/cespare/xxhash"
)

const shard = 64

type Cache struct {
	MaxSize uint64
	filter  [shard]uint32
	index   [shard][]uint64
}

func sum(s []byte) uint64 {
	return xxhash.Sum64(s)
}
