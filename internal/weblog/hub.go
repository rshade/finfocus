package weblog

import (
	"slices"
	"sync"
)

const (
	defaultCapacity  = 5000
	subscriberBuffer = 256
)

type subscriber struct {
	ch chan Entry
}

// Hub is a bounded in-memory ring of entries with live subscribers. It is
// safe for concurrent use.
type Hub struct {
	mu       sync.Mutex
	capacity int
	entries  []Entry
	nextSeq  uint64
	dropped  uint64
	sources  []Source
	known    map[string]struct{}
	subs     map[*subscriber]struct{}
}

// NewHub returns a hub holding at most capacity entries; a capacity of zero or
// less selects 5000.
func NewHub(capacity int) *Hub {
	if capacity <= 0 {
		capacity = defaultCapacity
	}
	return &Hub{
		capacity: capacity,
		nextSeq:  1,
		known:    map[string]struct{}{},
		subs:     map[*subscriber]struct{}{},
	}
}

// Append assigns the next sequence number, stores the entry, evicting the
// oldest when full, and delivers it to subscribers. A subscriber whose channel
// is full is closed and removed; Append never blocks. The stored entry is
// returned.
func (h *Hub) Append(e Entry) Entry {
	h.mu.Lock()
	defer h.mu.Unlock()
	e.Seq = h.nextSeq
	h.nextSeq++
	if _, ok := h.known[e.Source]; !ok {
		h.known[e.Source] = struct{}{}
		h.sources = append(h.sources, Source{Name: e.Source, Kind: e.SourceKind})
	}
	if len(h.entries) >= h.capacity {
		copy(h.entries, h.entries[1:])
		h.entries = h.entries[:len(h.entries)-1]
		h.dropped++
	}
	h.entries = append(h.entries, e)
	for sub := range h.subs {
		select {
		case sub.ch <- e:
		default:
			close(sub.ch)
			delete(h.subs, sub)
		}
	}
	return e
}

// Subscribe returns the retained entries with Seq greater than afterSeq, a
// live channel, and an idempotent cancel function. Snapshot.Missed counts
// entries after afterSeq that were already evicted. The channel is closed when
// the subscriber lags or cancels.
func (h *Hub) Subscribe(afterSeq uint64) (Snapshot, <-chan Entry, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	snap := Snapshot{
		Sources:  slices.Clone(h.sources),
		Dropped:  h.dropped,
		Capacity: h.capacity,
	}
	for _, e := range h.entries {
		if e.Seq > afterSeq {
			snap.Entries = append(snap.Entries, e)
		}
	}
	if oldest := h.nextSeq - uint64(len(h.entries)); afterSeq+1 < oldest {
		snap.Missed = oldest - afterSeq - 1
	}
	sub := &subscriber{ch: make(chan Entry, subscriberBuffer)}
	h.subs[sub] = struct{}{}
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[sub]; ok {
			delete(h.subs, sub)
			close(sub.ch)
		}
	}
	return snap, sub.ch, cancel
}

// Status reports the buffer's capacity, retained count, dropped count, and
// registered sources.
func (h *Hub) Status() Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	return Status{
		Enabled:  true,
		Capacity: h.capacity,
		Retained: len(h.entries),
		Dropped:  h.dropped,
		Sources:  slices.Clone(h.sources),
	}
}
