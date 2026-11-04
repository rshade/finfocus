package weblog

import "bytes"

// MaxLineBytes caps one line and one message.
const MaxLineBytes = 64 * 1024

// assembler splits a byte stream into lines. It keeps the unterminated tail
// between writes and flushes a tail over MaxLineBytes as one truncated line.
type assembler struct {
	tail []byte
	emit func(line []byte, truncated bool)
}

func newAssembler(emit func(line []byte, truncated bool)) *assembler {
	return &assembler{emit: emit}
}

func (a *assembler) write(p []byte) {
	for len(p) > 0 {
		idx := bytes.IndexByte(p, '\n')
		if idx < 0 {
			a.tail = append(a.tail, p...)
			if len(a.tail) > MaxLineBytes {
				a.emitLine(a.tail[:MaxLineBytes], true)
				a.tail = a.tail[:0]
			}
			return
		}
		a.tail = append(a.tail, p[:idx]...)
		a.emitLine(a.tail, len(a.tail) > MaxLineBytes)
		a.tail = a.tail[:0]
		p = p[idx+1:]
	}
}

func (a *assembler) close() {
	if len(a.tail) > 0 {
		a.emitLine(a.tail, false)
		a.tail = nil
	}
}

func (a *assembler) emitLine(line []byte, truncated bool) {
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 {
		return
	}
	if len(line) > MaxLineBytes {
		line = line[:MaxLineBytes]
		truncated = true
	}
	a.emit(line, truncated)
}
