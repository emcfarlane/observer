package observer

import (
	"container/list"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func testListValues(tb testing.TB, l *List, vals ...interface{}) {
	var got []interface{}
	for e := l.Front(); e != nil; e = e.Next() {
		got = append(got, e.Value)
	}

	if diff := cmp.Diff(vals, got); diff != "" {
		tb.Fatal(diff)
	}
}

func TestList(t *testing.T) {
	l := List{}

	l.PushFront(0)
	testListValues(t, &l, 0)

	e1 := l.PushFront(1)
	testListValues(t, &l, 1, 0)

	l.PushFront(2)
	testListValues(t, &l, 2, 1, 0)

	l.Remove(e1)
	testListValues(t, &l, 2, 0)

	l.Remove(l.Back())
	testListValues(t, &l, 2)

	l.Remove(l.Front())
	testListValues(t, &l)
}

func BenchmarkList(b *testing.B) {
	l := List{}
	for n := 0; n < b.N; n++ {
		l.PushFront(n)
	}
	if n := l.Front().Value.(int); n != b.N-1 {
		b.Fatalf("got %d, want %d", n, b.N)
	}
}

func BenchmarkListParallel(b *testing.B) {
	l := List{}
	var wg sync.WaitGroup
	for n := 0; n < b.N; n++ {
		wg.Add(1)

		go func(n int) {
			l.PushFront(n)
			wg.Done()
		}(n)
	}
	wg.Wait()
	if n := l.Len(); n != b.N {
		b.Fatalf("got %d, want %d", n, b.N)
	}
}

func BenchmarkListRemoveParallel(b *testing.B) {
	l := List{}
	for n := 0; n < b.N; n++ {
		l.PushFront(n)
	}

	b.ResetTimer()
	var wg sync.WaitGroup
	for e := l.Front(); e != nil; e = e.Next() {
		wg.Add(1)

		go func(e *Element) {
			l.Remove(e)
			wg.Done()
		}(e)
	}
	wg.Wait()
	if n := l.Len(); n != 0 {
		b.Fatalf("got %d, want %d", n, b.N)
	}
}

func BenchmarkListContainerParallel(b *testing.B) {
	mu := sync.Mutex{}
	l := list.List{}
	var wg sync.WaitGroup
	for n := 0; n < b.N; n++ {
		wg.Add(1)

		go func(n int) {
			mu.Lock()
			l.PushFront(n)
			mu.Unlock()
			wg.Done()
		}(n)
	}
	wg.Wait()
	if n := l.Len(); n != b.N {
		b.Fatalf("got %d, want %d", n, b.N)
	}
}
