// Package scoring turns recommendations into Jev questions, runs them in
// batches and maps the answers back to RecommendationScorerService results.
package scoring

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	defaultMaxString   = 600
	defaultMaxKey      = 64
	defaultMaxMapValue = 200
	defaultMaxEntries  = 32
	defaultMaxListLen  = 8

	maxDepth = 8
)

// Limits caps every free-text part of a recommendation before it leaves the
// host. Zero fields take the defaults.
type Limits struct {
	// MaxString caps descriptions, reasoning lines and every other string.
	MaxString int
	// MaxKey caps tag and metadata keys.
	MaxKey int
	// MaxMapValue caps tag and metadata values.
	MaxMapValue int
	// MaxEntries caps the number of tags and of metadata entries; extra
	// entries are dropped in key order.
	MaxEntries int
	// MaxListLen caps repeated fields such as reasoning.
	MaxListLen int
}

func (l Limits) withDefaults() Limits {
	if l.MaxString <= 0 {
		l.MaxString = defaultMaxString
	}
	if l.MaxKey <= 0 {
		l.MaxKey = defaultMaxKey
	}
	if l.MaxMapValue <= 0 {
		l.MaxMapValue = defaultMaxMapValue
	}
	if l.MaxEntries <= 0 {
		l.MaxEntries = defaultMaxEntries
	}
	if l.MaxListLen <= 0 {
		l.MaxListLen = defaultMaxListLen
	}
	return l
}

type renderer struct {
	limits Limits
}

// render converts a recommendation into a plain map for the request state. The
// recommendation id and creation time carry no scoring signal and are left
// out; every string is stripped of control characters and capped.
func (r renderer) render(rec *pbc.Recommendation) (map[string]any, error) {
	raw, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(rec)
	if err != nil {
		return nil, fmt.Errorf("encode recommendation: %w", err)
	}
	var doc map[string]any
	if err = json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode recommendation: %w", err)
	}
	delete(doc, "id")
	delete(doc, "created_at")
	out, _ := r.object("", doc, 0).(map[string]any)
	return out, nil
}

func (r renderer) object(key string, in map[string]any, depth int) any {
	if depth > maxDepth {
		return nil
	}
	if key == "tags" || key == "metadata" {
		return r.freeMap(in)
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = r.value(k, v, depth+1)
	}
	return out
}

func (r renderer) freeMap(in map[string]any) map[string]any {
	keys := slices.Sorted(maps.Keys(in))
	out := make(map[string]any, min(len(keys), r.limits.MaxEntries))
	for _, k := range keys {
		if len(out) >= r.limits.MaxEntries {
			break
		}
		clean := cleanText(k, r.limits.MaxKey)
		if clean == "" {
			continue
		}
		text, _ := in[k].(string)
		out[clean] = cleanText(text, r.limits.MaxMapValue)
	}
	return out
}

func (r renderer) value(key string, v any, depth int) any {
	switch x := v.(type) {
	case string:
		return cleanText(trimEnumPrefix(key, x), r.limits.MaxString)
	case []any:
		out := make([]any, 0, min(len(x), r.limits.MaxListLen))
		for _, item := range x {
			if len(out) >= r.limits.MaxListLen {
				break
			}
			out = append(out, r.value(key, item, depth+1))
		}
		return out
	case map[string]any:
		return r.object(key, x, depth)
	default:
		return v
	}
}

func trimEnumPrefix(key, value string) string {
	var prefix string
	switch key {
	case "category":
		prefix = "RECOMMENDATION_CATEGORY_"
	case "action_type":
		prefix = "RECOMMENDATION_ACTION_TYPE_"
	case "priority":
		prefix = "RECOMMENDATION_PRIORITY_"
	case "primary_reason", "secondary_reasons":
		prefix = "RECOMMENDATION_REASON_"
	default:
		return value
	}
	return strings.TrimPrefix(value, prefix)
}

// cleanText replaces whitespace with spaces, drops control, format (zero
// width, bidirectional), private-use and surrogate code points, and caps the
// result at limit runes.
func cleanText(s string, limit int) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	b.Grow(min(len(s), limit*utf8.UTFMax))
	count := 0
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			r = ' '
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), unicode.Is(unicode.Co, r), unicode.Is(unicode.Cs, r):
			continue
		}
		if count == limit {
			break
		}
		b.WriteRune(r)
		count++
	}
	return strings.TrimSpace(b.String())
}
