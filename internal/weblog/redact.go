package weblog

import (
	"strings"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

const redactedValue = "[redacted]"

func redactFields(fields map[string]any) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		if strings.HasPrefix(key, "__") {
			continue
		}
		if config.IsCredentialName(key) {
			out[key] = redactedValue
			continue
		}
		out[key] = redactValue(value)
	}
	return out
}

func redactValue(value any) any {
	if history.IsPulumiSecret(value) {
		return redactedValue
	}
	switch typed := value.(type) {
	case map[string]any:
		return redactFields(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactValue(item)
		}
		return out
	default:
		return value
	}
}
