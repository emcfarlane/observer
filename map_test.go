package observer

import (
	"sync"
	"testing"
)

func TestMap(t *testing.T) {
	m := Map{}
	key, val := "hello", "world"
	t.Log("Set", key, val)
	m.Set(key, val)
	//m.Set("one", "two")
	//m.Set("3", "4")
	//t.Fatal()
	//t.Logf("\n%s", m.String())

	t.Log("Get", key)
	got, ok := m.Get(key)
	t.Logf("%s %t", val, ok)
	if !ok {
		t.Fatalf("should be ok")
	}
	if got.(string) != val {
		t.Fatalf("expected %s, got %s", val, got)
	}
	//t.Logf("\n%s", m.String())

	val2 := "map"
	t.Log("Set", key, val2)
	m.Set(key, val2)
	t.Log("Get", key)
	got, ok = m.Get(key)
	if got.(string) != val2 {
		//t.Log(m.a.values, m.b.values, m.writeEntry)
		t.Fatalf("expected %s, got %s", val2, got)
	}
	if !ok {
		t.Fatalf("should be ok")
	}

	t.Log("Del", key)
	m.Del(key)
	got, ok = m.Get(key)
	if got != nil {
		t.Fatalf("expected nil, got %s", got)
	}
	if ok {
		t.Fatalf("shouldn't be ok")
	}

	//t.Fatal()
	t.Log("Map set/get loop")
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
	//t.Logf("\n%s", m.String())
}

func TestMapTx(t *testing.T) {
	m := Map{}

	key, val := "counter", 2
	m.Set(key, val)
	m.Tx(key, func(mval interface{}, ok bool) (interface{}, bool) {
		if !ok {
			t.Fatal("not valid", mval, ok)
		}

		n := mval.(int)
		if val != mval {
			t.Fatal("invalid val", n)
		}
		return n + 1, ok
	})

	mval, ok := m.Get(key)
	if !ok {
		t.Fatal("not valid")
	}
	n := mval.(int)
	if n != val+1 {
		t.Fatal("invalid value", n)
	}
}
