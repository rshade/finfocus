package weblog

// Source kinds.
const (
	KindCore   = "core"
	KindPlugin = "plugin"
)

// DisplayStrings holds the preformatted text shown for an entry.
type DisplayStrings struct {
	Time   string `json:"time"`
	Level  string `json:"level"`
	Source string `json:"source"`
}

// Entry is one captured log line after parsing and redaction.
type Entry struct {
	Seq        uint64         `json:"seq"`
	Time       string         `json:"time"`
	Level      string         `json:"level"`
	Source     string         `json:"source"`
	SourceKind string         `json:"sourceKind"`
	Message    string         `json:"message"`
	Fields     map[string]any `json:"fields,omitempty"`
	Raw        bool           `json:"raw,omitempty"`
	Truncated  bool           `json:"truncated,omitempty"`
	Display    DisplayStrings `json:"display"`
}

// Source is a label entries are filtered by.
type Source struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Status describes the buffer.
type Status struct {
	Enabled  bool     `json:"enabled"`
	Capacity int      `json:"capacity"`
	Retained int      `json:"retained"`
	Dropped  uint64   `json:"dropped"`
	Sources  []Source `json:"sources"`
}

// Snapshot is what a subscriber receives before live entries.
type Snapshot struct {
	Entries  []Entry
	Sources  []Source
	Dropped  uint64
	Capacity int
	// Missed counts entries after the requested sequence that were already
	// evicted.
	Missed uint64
}
