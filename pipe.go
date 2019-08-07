package observer

import (
	"io"
	"sync"
)

type pipeState int

const (
	pipeRead pipeState = iota - 1
	pipeOpen
	pipeWrite
)

type pipe struct {
	mu    sync.Mutex
	cond  sync.Cond
	state pipeState

	rwMu sync.Mutex
	b    []byte
	n    int
	err  error
}

func (p *pipe) copy(b []byte, state pipeState) (n int, err error) {
	p.mu.Lock()
	for p.state == state {
		p.cond.Wait() // Queue of same types.
	}
	if p.err != nil {
		p.mu.Unlock()
		return 0, p.err
	}

	if p.state == pipeOpen {
		p.b, p.state = b, state
		p.rwMu.Lock()
		p.mu.Unlock()

		p.rwMu.Lock() // Sync.
		n, err = p.n, p.err
		p.state = pipeOpen
		p.rwMu.Unlock()
		p.cond.Signal()
		p.mu.Unlock()
		return
	}

	dst, src := b, p.b
	if state == pipeWrite {
		dst, src = src, dst
	}

	n = copy(dst, src)
	p.n = n
	p.rwMu.Unlock()
	return
}

func (p *pipe) close(err error) error {
	p.mu.Lock()
	if p.err == nil {
		p.n, p.err = 0, err
	}
	p.cond.Broadcast()
	if p.state != pipeOpen {
		p.rwMu.Unlock()
		return p.err
	}
	p.mu.Unlock()
	return p.err
}

type PipeReader struct {
	p *pipe
}

func (pr *PipeReader) Read(b []byte) (int, error) {
	return pr.p.copy(b, pipeRead)
}
func (pr *PipeReader) Close() error {
	return pr.p.close(io.ErrClosedPipe)
}

type PipeWriter struct {
	p *pipe
}

func (pw *PipeWriter) Write(b []byte) (int, error) {
	return pw.p.copy(b, pipeWrite)
}
func (pw *PipeWriter) Close() error {
	return pw.p.close(io.EOF)
}

// Pipe is an io.Pipe implementation.
func Pipe() (*PipeReader, *PipeWriter) {
	p := &pipe{}
	p.cond.L = &p.mu
	return &PipeReader{p}, &PipeWriter{p}
}
