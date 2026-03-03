package cli

import (
	"context"
	"strings"
)

// suppressAuxOutputKey stores whether auxiliary human-facing messages (tips/log path)
// should be suppressed for machine-readable output modes.
type suppressAuxOutputKey struct{}

// contextWithSuppressAuxOutput stores the suppression decision in context.
func contextWithSuppressAuxOutput(ctx context.Context, suppress bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, suppressAuxOutputKey{}, suppress)
}

// suppressAuxOutputFromContext returns true when auxiliary output should be suppressed.
func suppressAuxOutputFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(suppressAuxOutputKey{}).(bool)
	return ok && v
}

// shouldSuppressAuxiliaryOutput returns true when CLI argv requests a structured
// output mode where extra human-facing lines would pollute machine-readable output.
func shouldSuppressAuxiliaryOutput(argv []string) bool {
	return isStructuredOutputFormat(detectOutputFormatFromArgs(argv))
}

// isStructuredOutputFormat returns true for machine-readable formats.
func isStructuredOutputFormat(format string) bool {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case outputFormatJSON, outputFormatNDJSON:
		return true
	default:
		return false
	}
}

// detectOutputFormatFromArgs scans argv for --output/-o and returns the value.
// It supports:
//   - --output json
//   - --output=json
//   - -o json
//   - -o=json
func detectOutputFormatFromArgs(argv []string) string {
	for i := range argv {
		arg := strings.TrimSpace(argv[i])
		if arg == "" {
			continue
		}
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--output=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--output="))
		}
		if arg == "--output" || arg == "-o" {
			if i+1 < len(argv) {
				return strings.TrimSpace(argv[i+1])
			}
			return ""
		}
		if strings.HasPrefix(arg, "-o=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "-o="))
		}
	}
	return ""
}
