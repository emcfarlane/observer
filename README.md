# observer

[![GoDoc](https://godoc.org/github.com/emcfarlane/observer?status.svg)](https://godoc.org/github.com/emcfarlane/observer)

Experimental concurrency primatives!

## Subject

Subject is an unbounded concurrent observer.
It's based on an atomic counter per fixed length array. Adding elements increments 
the counter to select the insert position. If the counter is greater than the 
list size a new array is allocated and the process is repeated for the new array.
Readers hold a pointer to a position in the array. Replaces a list of channels.

```
var s observer.Subject
go s.Set("hello")

go func() {
	for v := s.View(); ; v = v.Next() {
		fmt.Println(v.Value)
	}
}()
```

### Benchmark

Observer allocations are amortize over the array length.
Faster than a single channel and cost is linear when increase the number of readers unlike a an array of channels.
Trade off is in the queue becoming unbounded.

```
BenchmarkObserver/1-8           29919490                38.8 ns/op            28 B/op          0 allocs/op
BenchmarkObserver/8-8           18135234                74.1 ns/op            28 B/op          0 allocs/op
BenchmarkObserver/32-8           9482469               129 ns/op              28 B/op          0 allocs/op
BenchmarkChannel/1-8            20000000                74.2 ns/op             0 B/op          0 allocs/op
BenchmarkChannel/8-8             1000000              1353 ns/op               0 B/op          0 allocs/op
BenchmarkChannel/32-8             200000              6261 ns/op               0 B/op          0 allocs/op
```

##  Map

Concurrent map is similar to the builtin `sync.Map` but with a different locking stratergy.
The map is duplicated into a read/write pair that are switched based on three atomic counters.
A counter shared between the two maps and a counter per map.
When loading the maps mutual counter is incremented.
This flags which map to read from.
Once the read is finished the maps counter is incremented.
Writers wait for all readers of the map to leave, then swap the read/write map flag so the next write can be processed.

This has some benifits/trade offs in read/write workloads. See the benchmarks.
There is no concurrent process, like compaction, running alongside the map which has the benifit of maintaing throughput under sustained load.

Range operation isn't support as this would lock the entire map, unlike `sync.Map`.
However we gain a `Tx` method that supports a transaction like read/write`.
Useful for counters and optional updates.

```
var m observer.Map
m.Set("key", 1)

m.Tx("key", func(val interface{}, ok bool) {
  if ok {
    return val.(int) + 1, ok
  }
  return nil, false
})
val, ok := m.Get(key)
```

### Benchmark

Benchmark is based off of Dgraphs [cache testing](https://github.com/dgraph-io/benchmarks/blob/master/cachebench/cache_bench_test.go).

key: Map is this Map, SyncMap is `sync.Map` and RWMap is a Go map with a `sync.Mutex` to protect it.

```
BenchmarkCaches/MapZipfRead
BenchmarkCaches/MapZipfRead-8           25119784                51.05 ns/op            0 B/op          0 allocs/op
BenchmarkCaches/SyncMapZipfRead
BenchmarkCaches/SyncMapZipfRead-8       23019645                52.35 ns/op            0 B/op          0 allocs/op
BenchmarkCaches/RWMapZipfRead
BenchmarkCaches/RWMapZipfRead-8         27033931                45.05 ns/op            0 B/op          0 allocs/op
BenchmarkCaches/MapOneKeyRead
BenchmarkCaches/MapOneKeyRead-8         35277840                35.37 ns/op            0 B/op          0 allocs/op
BenchmarkCaches/SyncMapOneKeyRead
BenchmarkCaches/SyncMapOneKeyRead-8     100000000               11.14 ns/op            0 B/op          0 allocs/op
BenchmarkCaches/MapZipIfWrite
BenchmarkCaches/MapZipIfWrite-8          2615496               458.9 ns/op            56 B/op          4 allocs/op
BenchmarkCaches/SyncMapZipfWrite
BenchmarkCaches/SyncMapZipfWrite-8       2199801               547.9 ns/op            71 B/op          5 allocs/op
BenchmarkCaches/MapOneIfWrite
BenchmarkCaches/MapOneIfWrite-8          2646877               446.9 ns/op            56 B/op          4 allocs/op
BenchmarkCaches/SyncMapOneKeyWrite
BenchmarkCaches/SyncMapOneKeyWrite-8     3882291               307.8 ns/op            72 B/op          5 allocs/op
BenchmarkCaches/MapZipfMixed
BenchmarkCaches/MapZipfMixed-8          16612470                68.57 ns/op            2 B/op          0 allocs/op
BenchmarkCaches/SyncMapZipfMixed
BenchmarkCaches/SyncMapZipfMixed-8      20458893                60.46 ns/op           13 B/op          0 allocs/op
BenchmarkCaches/RWMapZipfMixed
BenchmarkCaches/RWMapZipfMixed-8         2817824               441.3 ns/op             1 B/op          0 allocs/op
BenchmarkCaches/MapOneKeyMixed
BenchmarkCaches/MapOneKeyMixed-8        28267887                42.63 ns/op            2 B/op          0 allocs/op
BenchmarkCaches/SyncMapOneKeyMixed
BenchmarkCaches/SyncMapOneKeyMixed-8    66958126                17.83 ns/op            6 B/op          0 allocs/op
```
