package weblog

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	fieldStart = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*=`)
)

//nolint:gochecknoglobals // Immutable lookup of zerolog.ConsoleWriter's level labels.
var consoleLevels = map[string]string{
	"TRC": levelTrace,
	"DBG": levelDebug,
	"INF": levelInfo,
	"WRN": levelWarn,
	"ERR": levelError,
	"FTL": levelFatal,
	"PNC": levelPanic,
}

// parseConsole decodes a line written by zerolog.ConsoleWriter, the default log
// file format: "<RFC3339 time> <LVL> <message> key=value ...", with colour
// escapes. It returns the same object a JSON event would decode to, so source
// selection and redaction treat both forms alike. A line that does not start
// with a time and a level label is not a console line.
func parseConsole(line []byte) (map[string]any, bool) {
	text := strings.TrimSpace(ansiEscape.ReplaceAllString(string(line), ""))
	stamp, rest, ok := strings.Cut(text, " ")
	if !ok {
		return nil, false
	}
	if _, err := time.Parse(time.RFC3339, stamp); err != nil {
		return nil, false
	}
	label, rest, _ := strings.Cut(strings.TrimLeft(rest, " "), " ")
	level, ok := consoleLevels[label]
	if !ok {
		return nil, false
	}
	message, pairs := splitMessage(strings.TrimLeft(rest, " "))
	obj := map[string]any{"time": stamp, "level": level, "message": message}
	for key, value := range parsePairs(pairs) {
		if _, taken := obj[key]; !taken {
			obj[key] = value
		}
	}
	return obj, true
}

// splitMessage cuts rest at the first token that looks like key=value. The
// message is everything before it; the fields are the remainder.
func splitMessage(rest string) (string, string) {
	offset := 0
	for offset <= len(rest) {
		if fieldStart.MatchString(rest[offset:]) {
			return strings.TrimSpace(rest[:offset]), rest[offset:]
		}
		next := strings.IndexByte(rest[offset:], ' ')
		if next < 0 {
			break
		}
		offset += next + 1
	}
	return strings.TrimSpace(rest), ""
}

// parsePairs reads key=value tokens. A value is a Go-quoted string or runs to the
// next space. A trailing token without '=' is dropped: it is the stump of a line
// cut at MaxLineBytes and might be the start of a credential's name.
func parsePairs(text string) map[string]any {
	out := map[string]any{}
	for {
		text = strings.TrimLeft(text, " ")
		if text == "" {
			return out
		}
		key, rest, ok := strings.Cut(text, "=")
		if !ok || strings.ContainsAny(key, " ") {
			return out
		}
		value, remaining := readValue(rest)
		out[key] = value
		text = remaining
	}
}

func readValue(rest string) (string, string) {
	if strings.HasPrefix(rest, `"`) {
		if quoted, err := strconv.QuotedPrefix(rest); err == nil {
			if value, unquoteErr := strconv.Unquote(quoted); unquoteErr == nil {
				return value, rest[len(quoted):]
			}
		}
		return strings.TrimPrefix(rest, `"`), ""
	}
	value, remaining, _ := strings.Cut(rest, " ")
	return value, remaining
}
