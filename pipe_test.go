package observer

import (
	"testing"
	"io/ioutil"
	"io"
)

func TestPipe(t *testing.T) {
	r, w := Pipe()

	type result struct {
		b []byte
		n int
		err error
	}
	rs := make(chan result)

	go func() {
		n, err := w.Write([]byte("hello"))
		if err != nil {
			rs<-result{n: n, err: err}	
		}
		w.Close()
	}()

	go func() {
		b, err := ioutil.ReadAll(r)
		rs<-result{b: b, n: len(b), err: err}
	}()

	x := <-rs
	if x.err != nil {
		t.Error(x)
	}
	//t.Fatalf("%s %v %v", x.b, x.n, x.err)
}

func BenchmarkPipe(b *testing.B) {
	r, w := Pipe()

	type result struct {
		b []byte
		n int
		err error
	}
	rs := make(chan result)

	go func() {
		b, err := ioutil.ReadAll(r)
		rs <- result{b: b, n: len(b), err: err}
	}()

        for n := 0; n < b.N; n++ {
		w.Write([]byte("go"))
        }
	w.Close()

	x := <-rs
	if x.err != nil {
		b.Fatalf("%v %v", x.n, x.err)
	}
}


func BenchmarkPipeIO(b *testing.B) {
	r, w := io.Pipe()

	type result struct {
		b []byte
		n int
		err error
	}
	rs := make(chan result)

	go func() {
		b, err := ioutil.ReadAll(r)
		rs <- result{b: b, n: len(b), err: err}
	}()

        for n := 0; n < b.N; n++ {
		w.Write([]byte("go"))
        }
	w.Close()

	x := <-rs
	if x.err != nil {
		b.Fatalf("%v %v", x.n, x.err)
	}
}
