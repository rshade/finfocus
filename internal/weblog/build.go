package weblog

import (
	"bytes"
	"encoding/json"
	"time"
	"unicode/utf8"
)

const (
	levelTrace    = "trace"
	levelDebug    = "debug"
	levelInfo     = "info"
	levelWarn     = "warn"
	levelError    = "error"
	levelFatal    = "fatal"
	levelPanic    = "panic"
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
	obj, ok := decodeObject(line)
	if !ok {
		obj, ok = parseConsole(line)
	}
	if ok {
		fillFromObject(&e, obj, tag)
	} else {
		e.Raw = true
		e.Message = rawMessage(line)
		e.Source, e.SourceKind = rawSource(tag, rawFallback)
	}
	if len(e.Message) > MaxLineBytes {
		e.Message = cutUTF8(e.Message, MaxLineBytes)
		e.Truncated = true
	}
	e.Display = Display(e)
	return e
}

func decodeObject(line []byte) (map[string]any, bool) {
	var obj map[string]any
	dec := json.NewDecoder(bytes.NewReader(line))
	if err := dec.Decode(&obj); err != nil || obj == nil || dec.More() {
		return nil, false
	}
	return obj, true
}

const withheldMessage = "structured log line withheld: it is not a complete JSON object, so its fields cannot be redacted"

// rawMessage is the text of a line that is not a JSON object. A line that
// starts like one but does not parse, such as an event cut at MaxLineBytes, may
// carry credential fields that redaction can no longer find, so its text is
// withheld.
func rawMessage(line []byte) string {
	if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 && trimmed[0] == '{' {
		return withheldMessage
	}
	return string(line)
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
	case levelTrace, levelDebug, levelInfo, levelWarn, levelError, levelFatal, levelPanic:
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
