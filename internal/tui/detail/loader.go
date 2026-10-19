package detail

import (
	"context"
	"errors"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ErrHistorySkipped means the caller has no cost-history source for this
// resource. The detail section is omitted. It is not a failure the user retries.
var ErrHistorySkipped = errors.New("cost history skipped")

// LoadState is the lazy-load state of one detail view.
type LoadState int

const (
	// LoadStateIdle means the detail view has not asked for cost history.
	LoadStateIdle LoadState = iota
	// LoadStateLoading means a fetch is in flight.
	LoadStateLoading
	// LoadStateLoaded means the latest fetch finished.
	LoadStateLoaded
	// LoadStateError means the latest fetch failed and r retries it.
	LoadStateError
)

// Point is one stored cost observation for the resource open in the detail view.
type Point struct {
	Timestamp time.Time
	Amount    float64
	Currency  string
}

// Loader fetches cost history for one resource. The resource ID is the value
// the parent view uses to identify the row, usually a Pulumi URN.
type Loader func(ctx context.Context, resourceID string) ([]Point, error)

// Model is the cost-history subsection of a detail view.
// The parent must not copy a Model value; the mutex and in-flight replies
// belong to this pointer.
type Model struct {
	mu           sync.Mutex
	resourceID   string
	loader       Loader
	points       []Point
	loadState    LoadState
	errorMessage string
	retryCount   int
	generation   int
	ctx          context.Context
}

type historyLoadedMsg struct {
	owner  *Model
	gen    int
	points []Point
}

type historyErrorMsg struct {
	owner *Model
	gen   int
	err   error
}

// New returns a detail history model that has not fetched yet.
func New(resourceID string, loader Loader) *Model {
	return &Model{
		resourceID: resourceID,
		loader:     loader,
		loadState:  LoadStateIdle,
	}
}

// ResourceID returns the resource this model loads.
func (m *Model) ResourceID() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resourceID
}

// State returns the current load state.
func (m *Model) State() LoadState {
	if m == nil {
		return LoadStateIdle
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadState
}

// RetryCount returns how many times the user has pressed r.
func (m *Model) RetryCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.retryCount
}

// Points returns a copy of the loaded series.
func (m *Model) Points() []Point {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Point(nil), m.points...)
}

// Activate starts a fetch unless one is already running or the series is loaded.
// The loading state is set before the command runs.
func (m *Model) Activate(ctx context.Context) tea.Cmd {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadState == LoadStateLoading || m.loadState == LoadStateLoaded {
		return nil
	}
	return m.beginLocked(ctx)
}

// Init satisfies tea.Model. The parent starts the fetch with Activate.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Invalidate ignores a reply already in flight. The parent calls it when the
// user leaves the detail view.
func (m *Model) Invalidate() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.generation++
	m.mu.Unlock()
}

// Update applies a load reply or the r retry key.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m == nil {
		return m, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() != "r" || m.loadState != LoadStateError {
			return m, nil
		}
		m.retryCount++
		return m, m.beginLocked(m.ctx)
	case historyLoadedMsg:
		if msg.owner != m || msg.gen != m.generation {
			return m, nil
		}
		m.points = append([]Point(nil), msg.points...)
		m.loadState = LoadStateLoaded
		m.errorMessage = ""
	case historyErrorMsg:
		if msg.owner != m || msg.gen != m.generation {
			return m, nil
		}
		if msg.err != nil && errors.Is(msg.err, ErrHistorySkipped) {
			m.points = nil
			m.loadState = LoadStateIdle
			m.errorMessage = ""
			return m, nil
		}
		m.loadState = LoadStateError
		m.points = nil
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
		} else {
			m.errorMessage = "unknown failure"
		}
	}
	return m, nil
}

// beginLocked starts a fetch. The caller holds m.mu.
func (m *Model) beginLocked(ctx context.Context) tea.Cmd {
	if m.loader == nil {
		m.loadState = LoadStateError
		m.errorMessage = "cost history loader is not configured"
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.ctx = ctx
	m.loadState = LoadStateLoading
	m.errorMessage = ""
	m.generation++
	gen := m.generation
	resourceID := m.resourceID
	loader := m.loader
	return func() tea.Msg {
		if err := ctx.Err(); err != nil {
			return historyErrorMsg{owner: m, gen: gen, err: err}
		}
		points, err := loader(ctx, resourceID)
		if err != nil {
			return historyErrorMsg{owner: m, gen: gen, err: err}
		}
		return historyLoadedMsg{owner: m, gen: gen, points: points}
	}
}
