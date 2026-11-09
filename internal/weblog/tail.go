package weblog

import (
	"bytes"
	"fmt"
	"io"
)

// tailProbeSize is how much of the file each backward step reads while
// searching for the suffix. A line that crosses a step stays in memory
// until its start is found, and that hold is capped at MaxLineBytes.
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
// matching entries begins. It is the file size when nothing matches, so the
// caller does not decode the file again. It is 0 when the file has some
// matches but fewer than n, so seq stays Decode's whole-file index, and when
// a line in the window is longer than MaxLineBytes.
func tailOffset(r io.ReadSeeker, size int64, n int, tag string, match func(Entry) bool) (int64, error) {
	if size == 0 {
		return 0, nil
	}
	found := 0
	offset := int64(0)
	long := false
	err := linesBackward(r, size, func(start, end int64, line []byte) (bool, error) {
		if start == end {
			return false, nil
		}
		if line == nil || end-start > MaxLineBytes {
			long = true
			return true, nil
		}
		matched, matchErr := lineMatches(line, tag, match)
		if matchErr != nil {
			return false, matchErr
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
	if long || (found > 0 && found < n) {
		return 0, nil
	}
	if found == 0 {
		return size, nil
	}
	return offset, nil
}

// linesBackward reports each newline-delimited span from the end of the file
// toward the start. end is exclusive and does not include the newline. A
// blank line is an empty span. line is the span's bytes and is valid only for
// the call. fn returns true to stop.
func linesBackward(r io.ReadSeeker, size int64, fn func(start, end int64, line []byte) (bool, error)) error {
	// nextEnd is the exclusive end of the span that still needs its start.
	nextEnd := size
	pos := size
	probe := make([]byte, tailProbeSize)
	var pending []byte
	for pos > 0 {
		step := min(int64(tailProbeSize), pos)
		pos -= step
		if err := readFullAt(r, probe[:step], pos); err != nil {
			return err
		}
		var stop bool
		var err error
		pending, nextEnd, stop, err = splitChunk(probe[:step], pos, nextEnd, pending, fn)
		if err != nil || stop {
			return err
		}
	}
	if nextEnd > 0 && len(pending) > 0 {
		_, err := fn(0, nextEnd, pending)
		return err
	}
	return nil
}

// splitChunk emits every complete line in chunk and returns the unfinished
// prefix, joined with pending. pending is the part of that line closer to EOF.
func splitChunk(
	chunk []byte,
	pos, nextEnd int64,
	pending []byte,
	fn func(start, end int64, line []byte) (bool, error),
) ([]byte, int64, bool, error) {
	rest := chunk
	for {
		i := bytes.LastIndexByte(rest, '\n')
		if i < 0 {
			break
		}
		line := rest[i+1:]
		if len(pending) > 0 {
			joined, ok := joinLine(line, pending)
			if !ok {
				stop, err := signalLong(fn)
				return nil, 0, stop, err
			}
			line = joined
		}
		pending = nil
		stop, err := fn(pos+int64(i)+1, nextEnd, line)
		if err != nil || stop {
			return nil, 0, stop, err
		}
		nextEnd = pos + int64(i)
		rest = rest[:i]
	}
	if len(rest) == 0 {
		return pending, nextEnd, false, nil
	}
	held, ok := joinLine(rest, pending)
	if !ok {
		stop, err := signalLong(fn)
		return nil, 0, stop, err
	}
	return held, nextEnd, false, nil
}

func signalLong(fn func(start, end int64, line []byte) (bool, error)) (bool, error) {
	return fn(0, int64(MaxLineBytes)+1, nil)
}

// joinLine concatenates a line that crossed a probe boundary. The result is
// false when the line is longer than MaxLineBytes.
func joinLine(prefix, suffix []byte) ([]byte, bool) {
	if len(prefix)+len(suffix) > MaxLineBytes {
		return nil, false
	}
	out := make([]byte, len(prefix)+len(suffix))
	copy(out, prefix)
	copy(out[len(prefix):], suffix)
	return out, true
}

func lineMatches(line []byte, tag string, match func(Entry) bool) (bool, error) {
	for e, err := range Decode(bytes.NewReader(line), tag) {
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
