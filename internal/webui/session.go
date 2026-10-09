package webui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

// SessionOptions supplies operations owned by the CLI orchestration layer.
const (
	phaseActive     = "active"
	phaseDone       = "done"
	phaseFailed     = "error"
	previewPhase    = 2
	enrichmentPhase = 6
)

// ErrorEvent carries a safe user-facing error with its source phase.
type ErrorEvent struct {
	Message string `json:"message"`
	Phase   int    `json:"phase"`
}

// SessionOptions supplies the CLI domain pipelines to transport adapters.
type SessionOptions struct {
	EstimateResources func(context.Context) ([]engine.ResourceDescriptor, error)
	ActualCosts       func(context.Context, string, map[string]string) (*engine.CostResultWithErrors, error)
	Recommendations   func(context.Context, bool) (*engine.RecommendationsResult, error)
	Trends            func() (map[string]string, string)
	DayOfMonth        int
	Preview           func(context.Context) ([]engine.OverviewRow, error)
	SubmitPassphrase  func(context.Context, string) error
}

// Session shares engine data and pipeline events among browser tabs. It never
// stores authentication tokens or passphrases and never depends on the CLI.
type Session struct {
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	opts               SessionOptions
	eng                *engine.Engine
	rows               []engine.OverviewRow
	dateRange          engine.DateRange
	stack              string
	phases             []PhaseEvent
	failures           map[int]ErrorEvent
	progress           ProgressEvent
	budget             BudgetPayload
	ready              bool
	passphraseRequired bool
	closed             bool
	subscribers        map[chan streamEvent]struct{}
	preview            PreviewEvent
	previewRunning     bool
	workers            sync.WaitGroup
}

// PhaseEvent describes one loading checklist entry.
type PhaseEvent struct {
	Phase  int    `json:"phase"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ProgressEvent describes enrichment progress.
type ProgressEvent struct {
	Loaded int `json:"loaded"`
	Total  int `json:"total"`
}

// BudgetPayload exposes only the shared CLI budget health schema.
type BudgetPayload struct {
	Display viewmodel.BudgetDisplay     `json:"display"`
	Budgets []engine.BudgetHealthResult `json:"budgets"`
}

// PreviewEvent reports the single preview shared by all tabs.
type PreviewEvent struct {
	Status    string                     `json:"status"`
	ElapsedMs int64                      `json:"elapsedMs"`
	Rows      []engine.OverviewRowResult `json:"rows,omitempty"`
}

// OverviewSnapshot is the complete state sent first on each SSE connection.
type OverviewSnapshot struct {
	Phases             []PhaseEvent               `json:"phases"`
	Rows               []engine.OverviewRowResult `json:"rows"`
	Totals             OverviewTotalsPayload      `json:"totals"`
	Budget             BudgetPayload              `json:"budget"`
	Progress           ProgressEvent              `json:"progress"`
	Ready              bool                       `json:"ready"`
	PassphraseRequired bool                       `json:"passphraseRequired"`
	Stack              string                     `json:"stack"`
	Preview            PreviewEvent               `json:"preview"`
	Errors             []ErrorEvent               `json:"errors"`
}

// OverviewTotalsPayload adds canonical display strings to engine totals.
type OverviewTotalsPayload struct {
	engine.OverviewTotals

	UnavailableReason     string `json:"unavailableReason,omitempty"`
	TotalActualDisplay    string `json:"totalActualDisplay"`
	TotalProjectedDisplay string `json:"totalProjectedDisplay"`
	TotalDeltaDisplay     string `json:"totalDeltaDisplay"`
	TotalSavingsDisplay   string `json:"totalSavingsDisplay"`
}

func totalsPayload(t engine.OverviewTotals) OverviewTotalsPayload {
	if t.MixedCurrencies {
		t.TotalActual, t.TotalProjected, t.TotalDelta, t.TotalSavings = 0, 0, 0, 0
		return OverviewTotalsPayload{
			OverviewTotals:    t,
			UnavailableReason: "Totals unavailable: resources use different currencies.",
		}
	}
	return OverviewTotalsPayload{
		OverviewTotals:        t,
		TotalActualDisplay:    engine.FormatOverviewCurrency(t.TotalActual),
		TotalProjectedDisplay: engine.FormatOverviewCurrency(t.TotalProjected),
		TotalDeltaDisplay:     engine.FormatOverviewDelta(t.TotalDelta),
		TotalSavingsDisplay:   engine.FormatOverviewCurrency(t.TotalSavings),
	}
}

// NewSession creates session state governed by ctx and explicit Close.
func NewSession(ctx context.Context, opts SessionOptions) *Session {
	ctx, cancel := context.WithCancel(ctx)
	if opts.DayOfMonth == 0 {
		opts.DayOfMonth = time.Now().Day()
	}
	return &Session{
		ctx:         ctx,
		cancel:      cancel,
		opts:        opts,
		rows:        []engine.OverviewRow{},
		phases:      []PhaseEvent{},
		failures:    make(map[int]ErrorEvent),
		budget:      BudgetPayload{Budgets: []engine.BudgetHealthResult{}},
		subscribers: make(map[chan streamEvent]struct{}),
	}
}

// Context returns the session lifetime, independent of any browser request.
func (s *Session) Context() context.Context { return s.ctx }

// Engine returns the shared engine, nil until preparation completes.
func (s *Session) Engine() *engine.Engine { s.mu.Lock(); defer s.mu.Unlock(); return s.eng }

// DateRange returns the date interval resolved by the shared CLI pipeline.
func (s *Session) DateRange() engine.DateRange { s.mu.Lock(); defer s.mu.Unlock(); return s.dateRange }

// Rows returns an independent copy, including nested source properties.
func (s *Session) Rows() []engine.OverviewRow {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneData(s.rows)
}

// Resources returns descriptors equivalent to the engine overview enrichment
// inputs. Rows retains projected properties separately for estimate callers.
func (s *Session) Resources() []engine.ResourceDescriptor {
	rows := s.Rows()
	resources := make([]engine.ResourceDescriptor, 0, len(rows))
	for _, row := range rows {
		resources = append(
			resources,
			engine.ResourceDescriptor{
				Type:       row.Type,
				ID:         row.URN,
				Provider:   engine.ExtractProviderFromResourceType(row.Type),
				Properties: row.Properties,
			},
		)
	}
	return resources
}

// SetData installs engine and pre-enrichment rows when loading finishes.
func (s *Session) SetData(eng *engine.Engine, rows []engine.OverviewRow, dateRange engine.DateRange, stack string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eng = eng
	s.rows = cloneData(rows)
	s.dateRange = dateRange
	s.stack = stack
	s.progress.Total = len(rows)
}

// Phase reports a phase start and marks the previous active phase done.
func (s *Session) Phase(phase int, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, phase)
	for i := range s.phases {
		if s.phases[i].Status == phaseActive {
			s.phases[i].Status = phaseDone
			s.publishLocked("phase", s.phases[i])
		}
	}
	event := PhaseEvent{Phase: phase, Name: name, Status: phaseActive}
	found := false
	for i := range s.phases {
		if s.phases[i].Phase == phase {
			s.phases[i] = event
			found = true
		}
	}
	if !found {
		s.phases = append(s.phases, event)
	}
	s.publishLocked("phase", event)
}

// Row installs and publishes a completed enrichment row.
func (s *Session) Row(index int, row engine.OverviewRow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.rows) {
		return
	}
	s.rows[index] = cloneData(row)
	result := engine.ComputeOverviewResult([]engine.OverviewRow{s.rows[index]}, s.opts.DayOfMonth)
	s.publishLocked("row", result.Rows[0])
}

// Progress publishes aggregate enrichment progress.
func (s *Session) Progress(loaded, total int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.progress = ProgressEvent{Loaded: loaded, Total: total}
	s.publishLocked("progress", s.progress)
}

// Budget publishes health results and a sanitized recoverable error.
func (s *Session) Budget(result *engine.BudgetResult, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	budgets := BudgetHealthPayload(s.ctx, result)
	if budgets == nil {
		budgets = []engine.BudgetHealthResult{}
	}
	s.budget = BudgetPayload{Budgets: cloneData(budgets), Display: viewmodel.BuildBudgetDisplay(result)}
	s.publishLocked("budget", s.budget)
	if err != nil {
		s.publishLocked("error", ErrorEvent{Message: "Budget data unavailable", Phase: enrichmentPhase})
	}
}

// Expansion replaces the row set after cluster expansion.
func (s *Session) Expansion(rows []engine.OverviewRow, notes []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = cloneData(rows)
	s.publishLocked("expansion", map[string]any{"rows": s.resultLocked().Rows, "notes": notes})
}

// Error records a safe user-facing failure and its phase for reconnects.
func (s *Session) Error(phase int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	failure := ErrorEvent{Message: message, Phase: phase}
	s.failures[phase] = failure
	found := false
	for i := range s.phases {
		if s.phases[i].Phase == phase {
			s.phases[i].Status = phaseFailed
			s.publishLocked("phase", s.phases[i])
			found = true
			break
		}
	}
	if !found {
		event := PhaseEvent{Phase: phase, Name: fmt.Sprintf("Phase %d", phase), Status: phaseFailed}
		s.phases = append(s.phases, event)
		s.publishLocked("phase", event)
	}
	s.publishLocked("error", failure)
}

// Ready installs final rows and publishes canonical totals.
func (s *Session) Ready(rows []engine.OverviewRow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = cloneData(rows)
	s.ready = true
	for _, phase := range s.phases {
		s.completePhaseLocked(phase.Phase)
	}
	s.publishLocked("ready", map[string]any{"totals": totalsPayload(s.resultLocked().Summary)})
}

// RequirePassphrase prompts all clients to unlock the selected stack.
func (s *Session) RequirePassphrase(stack string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passphraseRequired = true
	if stack != "" {
		s.stack = stack
	}
	s.publishLocked("passphrase_required", map[string]string{"stack": s.stack})
}

// SubscriberCount reports active streams, including bounded slow-client eviction.
func (s *Session) SubscriberCount() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.subscribers) }

// Close cancels preview work and disconnects streams before engine cleanup.
func (s *Session) Close() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		s.cancel()
		for ch := range s.subscribers {
			close(ch)
			delete(s.subscribers, ch)
		}
	}
	s.mu.Unlock()
	s.workers.Wait()
}
func (s *Session) resultLocked() engine.OverviewResult {
	return engine.ComputeOverviewResult(s.rows, s.opts.DayOfMonth)
}
func (s *Session) snapshotLocked() OverviewSnapshot {
	result := s.resultLocked()
	failures := make([]ErrorEvent, 0, len(s.failures))
	for _, phase := range s.phases {
		if failure, ok := s.failures[phase.Phase]; ok {
			failures = append(failures, failure)
		}
	}
	return OverviewSnapshot{
		Phases:             s.phases,
		Rows:               result.Rows,
		Totals:             totalsPayload(result.Summary),
		Budget:             s.budget,
		Progress:           s.progress,
		Ready:              s.ready,
		PassphraseRequired: s.passphraseRequired,
		Stack:              s.stack,
		Preview:            s.preview,
		Errors:             failures,
	}
}
func (s *Session) startPreview() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil {
		return errors.New("session closed")
	}
	if s.opts.Preview == nil {
		return errors.New("preview unavailable")
	}
	if s.previewRunning {
		return nil
	}
	s.previewRunning = true
	s.preview = PreviewEvent{Status: "running"}
	s.publishLocked("preview", s.preview)
	s.workers.Add(1)
	go s.runPreview()
	return nil
}
func (s *Session) runPreview() {
	defer s.workers.Done()
	start := time.Now()
	done := make(chan struct{})
	// One joined ticker publishes elapsed time even when Pulumi takes a while.
	tickerDone := make(chan struct{})
	go func() {
		defer close(tickerDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.mu.Lock()
				s.preview.ElapsedMs = time.Since(start).Milliseconds()
				s.publishLocked("preview", s.preview)
				s.mu.Unlock()
			}
		}
	}()
	rows, err := s.opts.Preview(s.ctx)
	close(done)
	<-tickerDone
	s.mu.Lock()
	s.previewRunning = false
	s.preview = PreviewEvent{Status: phaseDone, ElapsedMs: time.Since(start).Milliseconds()}
	if err != nil {
		s.preview.Status = "error"
		failure := ErrorEvent{Message: "Preview failed", Phase: previewPhase}
		s.failures[previewPhase] = failure
		s.publishLocked("error", failure)
	} else {
		s.rows = cloneData(rows)
		s.ready = true
		s.completePhaseLocked(previewPhase)
		s.preview.Rows = s.resultLocked().Rows
		s.publishLocked("ready", map[string]any{"totals": totalsPayload(s.resultLocked().Summary)})
	}
	s.publishLocked("preview", s.preview)
	s.mu.Unlock()
}

// cloneData preserves non-JSON engine fields used for canonical delta math.
func cloneData[T any](value T) T {
	cloned, _ := reflect.TypeAssert[T](cloneReflect(reflect.ValueOf(value)))
	return cloned
}
func cloneReflect(v reflect.Value) reflect.Value {
	//nolint:exhaustive // Immutable scalar and opaque values are copied by value in default.
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(cloneReflect(v.Elem()))
		return out
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(cloneReflect(v.Elem()))
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), cloneReflect(iter.Value()))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(cloneReflect(v.Index(i)))
		}
		return out
	case reflect.Struct:
		return cloneStruct(v)
	default:
		return v
	}
}

func cloneStruct(v reflect.Value) reflect.Value {
	out := reflect.New(v.Type()).Elem()
	out.Set(v)
	for i := range v.NumField() {
		if v.Type().Field(i).IsExported() {
			out.Field(i).Set(cloneReflect(v.Field(i)))
		}
	}
	return out
}

func (s *Session) completePhaseLocked(phase int) {
	delete(s.failures, phase)
	for i := range s.phases {
		if s.phases[i].Phase == phase && (s.phases[i].Status == phaseActive || s.phases[i].Status == phaseFailed) {
			s.phases[i].Status = phaseDone
			s.publishLocked("phase", s.phases[i])
			return
		}
	}
}
