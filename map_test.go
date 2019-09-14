package observer

import (
	"sync"
	"testing"
)

func TestMap(t *testing.T) {
	m := Map{}
	key, val := "hello", "world"
	m.Set(key, val)
	//m.Set("one", "two")
	//m.Set("3", "4")
	//t.Fatal()

	got, ok := m.Get(key)
	t.Logf("%s %t", val, ok)
	if !ok {
		t.Fatalf("should be ok")
	}
	if got.(string) != val {
		t.Fatalf("expected %s, got %s", val, got)
	}

	val2 := "map"
	m.Set(key, val2)
	got, ok = m.Get(key)
	if got.(string) != val2 {
		t.Fatalf("expected %s, got %s", val2, got)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	m.Del(key)
	got, ok = m.Get(key)
	if got != nil {
		t.Fatalf("expected nil, got %s", got)
	}
	if ok {
		t.Fatalf("shouldn't be ok")
	}

	//t.Fatal()
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%4 == 0 {
				m.Set(i, "test")
			} else {
				m.Get(i - 1)
			}
		}(i)
	}
	wg.Wait()
	t.Log("\n" + m.String())
}
