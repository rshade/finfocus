package tui

import (
	"cmp"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/resourcetype"
)

const (
	historyDefaultHeight = 24
	historyZoomFactor    = 2
	historyMinZoomPoints = 2
)

// HistoryViewModel navigates a selected currency's deployment cost history.
type HistoryViewModel struct {
	doc                   history.ViewDocument
	options               history.ChartOptions
	budgets               []history.ChartBudget
	providers             []string
	markers               []int
	notes                 map[int]history.CostAnnotation
	cursor, span, active  int
	width, height         int
	split, budget, detail bool
}

// NewHistoryViewModel creates an interactive view of an already filtered document.
func NewHistoryViewModel(
	doc history.ViewDocument, opt history.ChartOptions, budgets []history.ChartBudget,
) *HistoryViewModel {
	opt.Provider = resourcetype.NormalizeProvider(opt.Provider)
	doc.Snapshots = slices.Clone(doc.Snapshots)
	slices.SortStableFunc(doc.Snapshots, func(a, b history.CostSnapshot) int {
		return cmp.Or(a.Timestamp.Compare(b.Timestamp), cmp.Compare(a.Version, b.Version))
	})
	m := &HistoryViewModel{
		doc:       doc,
		options:   opt,
		budgets:   budgets,
		providers: history.ProviderNames(doc.Snapshots),
		notes:     make(map[int]history.CostAnnotation),
		span:      len(doc.Snapshots),
		cursor:    max(0, len(doc.Snapshots)-1),
		width:     DefaultTerminalWidth,
		height:    historyDefaultHeight,
		split:     opt.SplitProviders && opt.Provider == "",
		budget:    !opt.NoBudget,
	}
	if !opt.NoAnnotations {
		for _, note := range doc.Annotations {
			m.notes[note.Version] = note
		}
		for i, snapshot := range doc.Snapshots {
			if _, ok := m.notes[snapshot.Version]; ok {
				m.markers = append(m.markers, i)
			}
		}
	}
	return m
}

// Init waits for input.
func (*HistoryViewModel) Init() tea.Cmd { return nil }

// Update handles keyboard controls and terminal size changes.
func (m *HistoryViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		return m, m.key(msg.String())
	}
	return m, nil
}

func (m *HistoryViewModel) key(key string) tea.Cmd {
	last := max(0, len(m.doc.Snapshots)-1)
	switch key {
	case "q", "ctrl+c":
		return tea.Quit
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = last
	case "left":
		m.cursor = max(0, m.cursor-1)
	case "right":
		m.cursor = min(last, m.cursor+1)
	case "+", "=", "pgup":
		m.span = min(len(m.doc.Snapshots), max(historyMinZoomPoints, (m.span+1)/historyZoomFactor))
	case "-", "pgdown":
		m.span = min(len(m.doc.Snapshots), m.span*historyZoomFactor)
	default:
		m.toggle(key)
	}
	return nil
}

func (m *HistoryViewModel) toggle(key string) {
	switch key {
	case "s":
		if m.options.Provider == "" {
			m.split = !m.split
		}
	case "b":
		m.budget = !m.budget
	case "d":
		m.detail = !m.detail
	case "tab":
		if m.split && len(m.providers) > 0 {
			m.active = (m.active + 1) % len(m.providers)
		}
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			i := int(key[0] - '1')
			if i < len(m.markers) {
				m.cursor = m.markers[i]
			}
		}
	}
}

func (m *HistoryViewModel) visible() (int, int) {
	start := min(max(0, m.cursor-m.span/2), len(m.doc.Snapshots)-m.span)
	return start, start + m.span
}

func (m *HistoryViewModel) nearestMarker() int {
	if len(m.markers) == 0 {
		return -1
	}
	i, _ := slices.BinarySearch(m.markers, m.cursor)
	switch {
	case i == 0:
		return m.markers[0]
	case i == len(m.markers):
		return m.markers[i-1]
	}
	stamp := m.doc.Snapshots[m.cursor].Timestamp
	left := stamp.Sub(m.doc.Snapshots[m.markers[i-1]].Timestamp)
	right := m.doc.Snapshots[m.markers[i]].Timestamp.Sub(stamp)
	if right < left || (right == 0 && m.markers[i] == m.cursor) {
		return m.markers[i]
	}
	return m.markers[i-1]
}

func historyTimeRange(snapshots []history.CostSnapshot) (time.Time, time.Time) {
	first, last := snapshots[0].Timestamp, snapshots[len(snapshots)-1].Timestamp
	if !last.After(first) || last.Sub(first) < time.Second {
		first = first.Add(-time.Second)
		last = last.Add(time.Second)
	}
	return first, last
}
