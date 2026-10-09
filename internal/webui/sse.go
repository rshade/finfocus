package webui

import (
	"fmt"
	"net/http"
	"time"
)

const streamBufferSize = 64
const streamWriteTimeout = 10 * time.Second
const streamHeartbeatInterval = 15 * time.Second

type streamEvent struct {
	name string
	data []byte
}

// subscribe captures the snapshot and registers the subscriber atomically;
// an update can never fall between the first snapshot and registration.
func (s *Session) subscribe() (streamEvent, <-chan streamEvent, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := RedactJSON(s.snapshotLocked())
	ch := make(chan streamEvent, streamBufferSize)
	if s.closed {
		close(ch)
	} else {
		s.subscribers[ch] = struct{}{}
	}
	return streamEvent{name: "snapshot", data: data}, ch, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}
func (s *Session) publishLocked(name string, value any) {
	if s.closed {
		return
	}
	data, err := RedactJSON(value)
	if err != nil {
		return
	}
	event := streamEvent{name: name, data: data}
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			close(ch)
			delete(s.subscribers, ch)
		}
	}
}
func (s *Server) handleOverviewStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	first, events, unsubscribe := s.session.subscribe()
	defer unsubscribe()
	controller := http.NewResponseController(w)
	write := func(event streamEvent) error {
		_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.name, event.data); err != nil {
			return err
		}
		return controller.Flush()
	}
	if err := write(first); err != nil {
		return
	}
	heartbeat := time.NewTicker(streamHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.session.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := write(event); err != nil {
				return
			}
		case <-heartbeat.C:
			_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}
