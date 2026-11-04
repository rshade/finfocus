package weblog

import (
	"fmt"
	"slices"
	"strings"
)

// Filter selects entries by source, minimum level, and text.
type Filter struct {
	Sources  []string `json:"sources"`
	MinLevel string   `json:"minLevel"`
	Text     string   `json:"text"`
}

// Match reports whether the entry passes every part of the filter. An empty
// Sources list matches all sources, an empty MinLevel means trace, and Text is
// a case-insensitive substring of the message or any field value.
func (f Filter) Match(e Entry) bool {
	if len(f.Sources) > 0 && !slices.Contains(f.Sources, e.Source) {
		return false
	}
	if levelRank[e.Level] < levelRank[f.MinLevel] {
		return false
	}
	if f.Text == "" {
		return true
	}
	needle := strings.ToLower(f.Text)
	if strings.Contains(strings.ToLower(e.Message), needle) {
		return true
	}
	return valueContains(e.Fields, needle)
}

func valueContains(value any, needle string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, item := range typed {
			if valueContains(item, needle) {
				return true
			}
		}
		return false
	case []any:
		return slices.ContainsFunc(typed, func(item any) bool { return valueContains(item, needle) })
	case nil:
		return false
	default:
		return strings.Contains(strings.ToLower(fmt.Sprint(typed)), needle)
	}
}
