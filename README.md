# observer

[![GoDoc](https://godoc.org/github.com/afking/observer?status.svg)](https://godoc.org/github.com/afking/observer)

Experimental package to broadcast events to multiple viewers for Go.
Replaces a list of channels.

```
s := &observer.Subject{}
go s.Set("hello")

go func() {
	for v := s.View(); ; v = v.Next() {
		fmt.Println(v.Value)
	}
}()
```

```
BenchmarkObserver/1-8           29919490                38.8 ns/op            28 B/op          0 allocs/op
BenchmarkObserver/8-8           18135234                74.1 ns/op            28 B/op          0 allocs/op
BenchmarkObserver/32-8           9482469               129 ns/op              28 B/op          0 allocs/op
BenchmarkChannel/1-8            20000000                74.2 ns/op             0 B/op          0 allocs/op
BenchmarkChannel/8-8             1000000              1353 ns/op               0 B/op          0 allocs/op
BenchmarkChannel/32-8             200000              6261 ns/op               0 B/op          0 allocs/op
```
