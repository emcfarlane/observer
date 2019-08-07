package observer

import (
	"sync"
	"io"
)

type pipeState int

const (
	pipeRead pipeState = iota - 1
	pipeOpen
	pipeWrite
)

type pipe struct {
	mu sync.Mutex
	state pipeState

	rwMu sync.Mutex
	b []byte
	n int
	err error
}

func (p *pipe) copy(b []byte, state pipeState) (n int, err error) {
	p.mu.Lock()
	if p.err != nil {
		p.mu.Unlock()
		return 0, p.err
	}

	for p.state != -state {
		if p.err != nil {
			p.mu.Unlock()
			return 0, p.err
		}

		next := p.state == pipeOpen
		if next {
			p.rwMu.Lock() // Enqueue
			p.state = state
			p.b = b
		}
		p.mu.Unlock()

		// TODO: sync.Cond.Signal?
		p.rwMu.Lock() // Sync on rwMU
		if !next {
			p.rwMu.Unlock()
			p.mu.Lock()
			continue
		}

		n, err = p.n, p.err
		p.state = pipeOpen
		p.rwMu.Unlock()
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
		p.n = 0
		p.err = err
	}
	if p.state != pipeOpen {
		p.rwMu.Unlock()
		return p.err
	}
	p.mu.Unlock()
	return p.err
}

func (p *pipe) Read(b []byte) (int, error) { return p.copy(b, pipeRead) }
func (p *pipe) Write(b []byte) (int, error) { return p.copy(b, pipeWrite) }

type PipeReader struct {
	p *pipe
}

func (pr *PipeReader) Read(p []byte) (int, error) { return pr.p.Read(p) }
func (pr *PipeReader) Close() error { return pr.p.close(io.ErrClosedPipe) }

type PipeWriter struct {
	p *pipe
}

func (pw *PipeWriter) Write(p []byte) (int, error) { return pw.p.Write(p) }
func (pw *PipeWriter) Close() error { return pw.p.close(io.EOF) }

func Pipe() (*PipeReader, *PipeWriter) {
	p := &pipe{}
	return &PipeReader{p}, &PipeWriter{p}
}
