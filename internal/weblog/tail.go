package weblog

import (
	"bytes"
	"fmt"
	"io"
)

// tailProbeSize is how much of the file each backward step reads while
// searching for the suffix. Those bytes are not retained.
const tailProbeSize = 32 * 1024

// TailDecode returns the last n entries of r for which match returns true,
// oldest first. It reads backward from the end and decodes only the suffix
// that holds those entries, so a large file is not parsed from the start.
// A nil match keeps every entry. n must be positive.
//
// Seq is the position in that suffix. The prefix is not scanned, so the
// numbers are not positions from the start of the file. A line longer than
// MaxLineBytes inside the window is decoded from byte 0 so its truncation
// matches Decode, which splits long lines on its own read boundaries. In
// that case seq stays Decode's whole-file index and is not renumbered.
func TailDecode(r io.ReadSeeker, n int, tag string, match func(Entry) bool) ([]Entry, error) {
	if n <= 0 {
		return nil, fmt.Errorf("tail count must be positive, got %d", n)
	}
	if match == nil {
		match = func(Entry) bool { return true }
	}
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, fmt.Errorf("read log tail: %w", err)
	}
	start, err := tailOffset(r, size, n, tag, match)
	if err != nil {
		return nil, fmt.Errorf("read log tail: %w", err)
	}
	if _, err = r.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("read log tail: %w", err)
	}
	ring := newTailRing(n)
	for e, err := range Decode(r, tag) {
		if err != nil {
			return nil, fmt.Errorf("read log tail: %w", err)
		}
		if match(e) {
			ring.add(e)
		}
	}
	return ring.entries(), nil
}

// tailOffset is the file offset where the suffix containing the last n
// matching entries begins. It is 0 when the file has fewer matches than n,
// or when a line in that suffix is longer than MaxLineBytes.
func tailOffset(r io.ReadSeeker, size int64, n int, tag string, match func(Entry) bool) (int64, error) {
	if size == 0 {
		return 0, nil
	}
	found := 0
	offset := int64(0)
	long := false
	err := linesBackward(r, size, func(start, end int64) (bool, error) {
		if start == end {
			return false, nil
		}
		if end-start > MaxLineBytes {
			long = true
			return true, nil
		}
		matched, err := lineMatches(r, start, end, tag, match)
		if err != nil {
			return false, err
		}
		if !matched {
			return false, nil
		}
		found++
		offset = start
		return found >= n, nil
	})
	if err != nil {
		return 0, err
	}
	if long || found < n {
		return 0, nil
	}
	return offset, nil
}

// linesBackward reports each newline-delimited span from the end of the file
// toward the start. end is exclusive and does not include the newline. A
// blank line is an empty span. fn returns true to stop.
func linesBackward(r io.ReadSeeker, size int64, fn func(start, end int64) (bool, error)) error {
	// nextEnd is the exclusive end of the span that still needs its start.
	nextEnd := size
	pos := size
	buf := make([]byte, tailProbeSize)
	for {
		if pos == 0 {
			if nextEnd > 0 {
				_, err := fn(0, nextEnd)
				return err
			}
			return nil
		}
		step := min(int64(tailProbeSize), pos)
		pos -= step
		if err := readFullAt(r, buf[:step], pos); err != nil {
			return err
		}
		chunk := buf[:step]
		for {
			i := bytes.LastIndexByte(chunk, '\n')
			if i < 0 {
				break
			}
			stop, err := fn(pos+int64(i)+1, nextEnd)
			if err != nil || stop {
				return err
			}
			nextEnd = pos + int64(i)
			chunk = chunk[:i]
		}
	}
}

func lineMatches(r io.ReadSeeker, start, end int64, tag string, match func(Entry) bool) (bool, error) {
	buf := make([]byte, end-start)
	if err := readFullAt(r, buf, start); err != nil {
		return false, err
	}
	for e, err := range Decode(bytes.NewReader(buf), tag) {
		if err != nil {
			return false, err
		}
		if match(e) {
			return true, nil
		}
	}
	return false, nil
}

func readFullAt(r io.ReadSeeker, buf []byte, off int64) error {
	if _, err := r.Seek(off, io.SeekStart); err != nil {
		return err
	}
	_, err := io.ReadFull(r, buf)
	return err
}

type tailRing struct {
	buf   []Entry
	start int
	count int
}

func newTailRing(n int) *tailRing {
	return &tailRing{buf: make([]Entry, n)}
}

func (r *tailRing) add(e Entry) {
	if r.count < len(r.buf) {
		r.buf[r.count] = e
		r.count++
		return
	}
	r.buf[r.start] = e
	r.start = (r.start + 1) % len(r.buf)
}

func (r *tailRing) entries() []Entry {
	if r.count == 0 {
		return nil
	}
	out := make([]Entry, r.count)
	for i := range r.count {
		out[i] = r.buf[(r.start+i)%len(r.buf)]
	}
	return out
}
