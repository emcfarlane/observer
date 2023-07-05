package observer

import "testing"

func TestChannel(t *testing.T) {
	var c Channel[int]

	n := 10
	go func() {
		for i := 0; i < n; i++ {
			c.Enqueue(i)
		}
		c.Close()
	}()

	for i, ok := c.Dequeue(); ok; i, ok = c.Dequeue() {
		t.Log("i =", i)
	}
	t.Log("done")

	// Should panic on closed write.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic")
			} else {
				t.Log("recover:", r)
			}
		}()
		t.Log("enqueue: -1")
		c.Enqueue(-1)
	}()

	if n := c.Len(); n != 0 {
		t.Fatal("invalid length", n)
	}
}
