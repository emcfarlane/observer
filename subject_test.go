package observer

import (
	"strconv"
	"sync"
	"testing"
)

func TestObserver(t *testing.T) {
	var s Subject
	v := s.Set(1)

	if v.Value.(int) != 1 {
		t.Fatal("required", 1)
	}

	v2, v2n := s.Set(2), v.Next()
	if v2 != v2n {
		t.Fatalf("%p %v != %p %v", v2, v2, v2n, v2n)
	}

	var wg sync.WaitGroup
	threes := make([]int, 8)
	for i := range threes {
		i := i
		wg.Add(1)
		go func() {
			threes[i] = v2.Next().Value.(int)
			wg.Done()
		}()
	}

	s.Set(3)
	wg.Wait()
	for i, three := range threes {
		if three != 3 {
			t.Fatalf("threes[%d] == %d, want 3", i, three)
		}
	}

	for i := 4; i < 66; i++ {
		s.Set(i)
	}

	v65 := s.View()
	if v65.Value.(int) != 65 {
		t.Fatal("required", 65)
	}
	//t.Logf("%+v", v.frame)
	//t.Logf("%+v", v65.frame)

	var i int
	v.Range(func(val interface{}) bool {
		i++
		if i != val.(int) {
			t.Fatalf("range %d, failed %v", i, v)
			return false
		}
		return i < 66
	})
}

var cases = []int{1, 8, 32}

func BenchmarkObserver(b *testing.B) {
	for _, i := range cases {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			s := &Subject{}
			s.Set(0)
			var wg sync.WaitGroup
			for w := 0; w < i; w++ {
				wg.Add(1)
				v := s.View()
				go func() {
					var sum int
					for sum < b.N {
						v = v.Next()
						sum += v.Value.(int)
					}
					wg.Done()
				}()
			}

			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				s.Set(1)
			}
			wg.Wait()
		})
	}
}

func BenchmarkChannel(b *testing.B) {
	for _, i := range cases {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			var wg sync.WaitGroup
			chs := make([]chan int, i)
			for w := 0; w < i; w++ {
				wg.Add(1)
				ch := make(chan int, 8)
				chs[w] = ch
				go func() {
					var sum int
					for sum < b.N {
						sum += <-ch
					}
					wg.Done()
				}()
			}
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				for _, ch := range chs {
					ch <- 1
				}
			}
			wg.Wait()
		})
	}
}
