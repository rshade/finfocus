package weblog

import (
	"strings"
	"time"
)

const levelLabelWidth = 5

const (
	rankTrace = iota
	rankDebug
	rankInfo
	rankWarn
	rankError
	rankFatal
	rankPanic
)

//nolint:gochecknoglobals // Immutable level ordering table.
var levelRank = map[string]int{
	"trace":   rankTrace,
	"debug":   rankDebug,
	levelInfo: rankInfo,
	levelWarn: rankWarn,
	"error":   rankError,
	"fatal":   rankFatal,
	"panic":   rankPanic,
}

// Display builds the preformatted strings for an entry: local time as
// 15:04:05.000, an upper-case level label padded to five characters, and the
// source.
func Display(e Entry) DisplayStrings {
	timeText := e.Time
	if parsed, err := time.Parse(time.RFC3339Nano, e.Time); err == nil {
		timeText = parsed.Local().Format("15:04:05.000")
	}
	label := strings.ToUpper(e.Level)
	if pad := levelLabelWidth - len(label); pad > 0 {
		label += strings.Repeat(" ", pad)
	}
	return DisplayStrings{Time: timeText, Level: label, Source: e.Source}
}
