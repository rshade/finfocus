package webui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/weblog"
)

const logsPathPrefix = "/api/logs/"

// LogSource supplies the captured log lines the Logs page shows. *weblog.Hub
// satisfies it; a nil source means the feature is disabled.
type LogSource interface {
	Status() weblog.Status
	Subscribe(afterSeq uint64) (snapshot weblog.Snapshot, live <-chan weblog.Entry, cancel func())
}

func isNilLogSource(source LogSource) bool {
	v := reflect.ValueOf(source)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

type logSnapshotBody struct {
	Entries  []weblog.Entry  `json:"entries"`
	Sources  []weblog.Source `json:"sources"`
	Dropped  uint64          `json:"dropped"`
	Capacity int             `json:"capacity"`
}

func (s *Server) registerLogs(mux *http.ServeMux) {
	if s.logs == nil {
		return
	}
	mux.HandleFunc(logsPathPrefix+"status", s.handleLogsStatus)
	mux.HandleFunc(logsPathPrefix+"stream", s.handleLogsStream)
}

func (s *Server) requireGet(w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		return true
	}
	w.Header().Set("Allow", http.MethodGet)
	s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET only")
	return false
}

func (s *Server) handleLogsStatus(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	status := s.logs.Status()
	if status.Sources == nil {
		status.Sources = []weblog.Source{}
	}
	s.writePlainJSON(w, http.StatusOK, status)
}

func parseLastEventID(r *http.Request) (uint64, bool) {
	raw := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if raw == "" {
		return 0, false
	}
	seq, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return seq, true
}

func writeLogFrame(w http.ResponseWriter, controller *http.ResponseController, id, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_ = controller.SetWriteDeadline(time.Now().Add(streamWriteTimeout))
	var frame strings.Builder
	if id != "" {
		fmt.Fprintf(&frame, "id: %s\n", id)
	}
	fmt.Fprintf(&frame, "event: %s\ndata: %s\n\n", name, data)
	if _, writeErr := fmt.Fprint(w, frame.String()); writeErr != nil {
		return writeErr
	}
	return controller.Flush()
}

func writeLogSnapshot(
	write func(id, name string, value any) error,
	snapshot weblog.Snapshot,
	afterSeq uint64,
	hasLastID bool,
) error {
	body := logSnapshotBody{
		Entries:  snapshot.Entries,
		Sources:  snapshot.Sources,
		Dropped:  snapshot.Dropped,
		Capacity: snapshot.Capacity,
	}
	if body.Entries == nil {
		body.Entries = []weblog.Entry{}
	}
	if body.Sources == nil {
		body.Sources = []weblog.Source{}
	}
	id := ""
	switch {
	case len(body.Entries) > 0:
		id = strconv.FormatUint(body.Entries[len(body.Entries)-1].Seq, 10)
	case hasLastID:
		id = strconv.FormatUint(afterSeq, 10)
	}
	if err := write(id, "snapshot", body); err != nil {
		return err
	}
	if hasLastID && snapshot.Missed > 0 {
		return write("", "gap", map[string]uint64{"missed": snapshot.Missed})
	}
	return nil
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	if !s.requireGet(w, r) {
		return
	}
	afterSeq, hasLastID := parseLastEventID(r)
	snapshot, live, cancel := s.logs.Subscribe(afterSeq)
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	write := func(id, name string, value any) error {
		return writeLogFrame(w, controller, id, name, value)
	}

	if err := writeLogSnapshot(write, snapshot, afterSeq, hasLastID); err != nil {
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
		case entry, ok := <-live:
			if !ok {
				return
			}
			if err := write(strconv.FormatUint(entry.Seq, 10), "log", entry); err != nil {
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
