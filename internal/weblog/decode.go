package weblog

import (
	"errors"
	"io"
	"iter"
	"time"
)

const decodeReadSize = 32 * 1024

// Decode streams a log file through the line assembler and entry builder and
// yields entries in file order with Seq numbered 1..n. Raw lines with an empty
// tag get the source "unknown" because no writer tagged them. Memory is
// bounded by the line cap, not by file size. A read error is yielded once and
// ends the sequence.
func Decode(r io.Reader, tag string) iter.Seq2[Entry, error] {
	return func(yield func(Entry, error) bool) {
		var seq uint64
		stopped := false
		asm := newAssembler(func(line []byte, truncated bool) {
			if stopped {
				return
			}
			seq++
			e := buildEntry(line, tag, time.Now(), sourceUnknown)
			e.Seq = seq
			if truncated {
				e.Truncated = true
			}
			if !yield(e, nil) {
				stopped = true
			}
		})
		buf := make([]byte, decodeReadSize)
		for !stopped {
			n, err := r.Read(buf)
			asm.write(buf[:n])
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				asm.close()
			} else if !stopped {
				yield(Entry{}, err)
			}
			return
		}
	}
}
