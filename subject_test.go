package observer

import (
	"strconv"
	"sync"
	"testing"
)

func TestObserver(t *testing.T) {
	var s Subject[int]
	v := s.Set(1)

	if v.Value() != 1 {
		t.Fatal("required", 1)
	}

	v2, v2n := s.Set(2), v.Next()
	if v2 != v2n {
		t.Fatalf("%v != %v", v2, v2n)
	}

	var wg sync.WaitGroup
	threes := make([]int, 8)
	for i := range threes {
		i := i
		wg.Add(1)
		go func() {
			threes[i] = v2.Next().Value()
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
	if v65.Value() != 65 {
		t.Fatal("required", 65)
	}
	//t.Logf("%+v", v.frame)
	//t.Logf("%+v", v65.frame)

	// Check length matches.
	l := v.Len()
	if v65.Value() != l {
		t.Fatalf("%v !=len(v) -> %v", v65, l)
	}

	for i := 0; i < 1000; i++ {
		s.Set(66 + i)
		l = v.Len()
		if l != 66+i {
			t.Fatalf("Got %v want %v for len", l, 66+i)
		}
	}
}

var cases = []int{1, 8, 32}

func BenchmarkObserver(b *testing.B) {
	for _, i := range cases {
		b.Run(strconv.Itoa(i), func(b *testing.B) {
			s := &Subject[int]{}
			s.Set(0)
			var wg sync.WaitGroup
			for w := 0; w < i; w++ {
				wg.Add(1)
				v := s.View()
				go func() {
					var sum int
					for sum < b.N {
						v = v.Next()
						sum += v.Value()
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
