package weblog

import (
	"io"
	"sync"
	"time"
)

type lineWriter struct {
	mu  sync.Mutex
	asm *assembler
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.asm.write(p)
	return len(p), nil
}

func (w *lineWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.asm.close()
	return nil
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }

// Writer returns a writer that assembles lines and appends each as an entry
// tagged name. Close flushes an unterminated tail. It is safe for concurrent
// use. A nil hub returns a writer that discards its input.
func (h *Hub) Writer(name string) io.WriteCloser {
	if h == nil {
		return nopWriteCloser{io.Discard}
	}
	return &lineWriter{asm: newAssembler(func(line []byte, truncated bool) {
		e := NewEntry(line, name, time.Now())
		if truncated {
			e.Truncated = true
		}
		h.Append(e)
	})}
}

type teeWriter struct{ hub *Hub }

func (t teeWriter) Write(p []byte) (int, error) {
	n := len(p)
	line := p
	for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
		line = line[:len(line)-1]
	}
	if len(line) == 0 {
		return n, nil
	}
	truncated := len(line) > MaxLineBytes
	if truncated {
		line = line[:MaxLineBytes]
	}
	e := NewEntry(line, "", time.Now())
	if truncated {
		e.Truncated = true
	}
	t.hub.Append(e)
	return n, nil
}

// Tee returns a writer for core's logger: each Write is one complete JSON
// event. It is safe for concurrent use. A nil hub returns a discarding writer.
func (h *Hub) Tee() io.Writer {
	if h == nil {
		return io.Discard
	}
	return teeWriter{hub: h}
}
