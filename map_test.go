package observer

import "testing"

func TestMap(t *testing.T) {
	m := Map{}

	m.Set([]byte("hello"), []byte("world"))

	val, ok := m.Get([]byte("hello"))
	t.Logf("%s %t", val, ok)
	t.Fail()
}
