package weblog

import (
	"bytes"
	"encoding/json"
	"time"
	"unicode/utf8"
)

const (
	levelInfo     = "info"
	levelWarn     = "warn"
	sourceCore    = "core"
	sourceUnknown = "unknown"
)

// NewEntry parses one log line into an Entry. A zerolog JSON object yields its
// time, level, message, and redacted fields; anything else becomes a raw entry
// at level info. The source is the plugin_name field, then tag, then the
// component field, then "core". A raw line with an empty tag is "core".
func NewEntry(line []byte, tag string, now time.Time) Entry {
	return buildEntry(line, tag, now, sourceCore)
}

func buildEntry(line []byte, tag string, now time.Time, rawFallback string) Entry {
	e := Entry{Level: levelInfo, Time: now.UTC().Format(time.RFC3339Nano)}
	var obj map[string]any
	dec := json.NewDecoder(bytes.NewReader(line))
	if err := dec.Decode(&obj); err != nil || obj == nil || dec.More() {
		e.Raw = true
		e.Message = string(line)
		e.Source, e.SourceKind = rawSource(tag, rawFallback)
	} else {
		fillFromObject(&e, obj, tag)
	}
	if len(e.Message) > MaxLineBytes {
		e.Message = cutUTF8(e.Message, MaxLineBytes)
		e.Truncated = true
	}
	e.Display = Display(e)
	return e
}

func rawSource(tag, fallback string) (string, string) {
	if tag != "" {
		return tag, KindPlugin
	}
	return fallback, KindCore
}

func fillFromObject(e *Entry, obj map[string]any, tag string) {
	if text, ok := obj["time"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			e.Time = parsed.UTC().Format(time.RFC3339Nano)
		}
	}
	e.Level = normalizeLevel(obj["level"])
	if msg, ok := obj["message"].(string); ok {
		e.Message = msg
	}
	name, _ := obj["plugin_name"].(string)
	component, _ := obj["component"].(string)
	switch {
	case name != "":
		e.Source, e.SourceKind = name, KindPlugin
	case tag != "":
		e.Source, e.SourceKind = tag, KindPlugin
	case component != "":
		e.Source, e.SourceKind = component, KindCore
	default:
		e.Source, e.SourceKind = sourceCore, KindCore
	}
	delete(obj, "time")
	delete(obj, "level")
	delete(obj, "message")
	if fields := redactFields(obj); len(fields) > 0 {
		e.Fields = fields
	}
}

func normalizeLevel(value any) string {
	text, _ := value.(string)
	switch text {
	case "warning":
		return levelWarn
	case "trace", "debug", levelInfo, levelWarn, "error", "fatal", "panic":
		return text
	default:
		return levelInfo
	}
}

func cutUTF8(s string, limit int) string {
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
