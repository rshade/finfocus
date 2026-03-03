package cli

import (
	"context"
	"strings"
)

// suppressAuxOutputKey stores whether auxiliary human-facing messages (tips/log path)
// should be suppressed for machine-readable output modes.
type suppressAuxOutputKey struct{}

// is used as the parent context.
func contextWithSuppressAuxOutput(ctx context.Context, suppress bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, suppressAuxOutputKey{}, suppress)
}

// suppressAuxOutputFromContext reports whether auxiliary human-facing output should be suppressed for ctx.
// It returns true only if ctx contains a boolean value `true` stored under the `suppressAuxOutputKey{}` key.
// If ctx is nil, the key is absent, or the value is not a boolean `true`, it returns false.
func suppressAuxOutputFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(suppressAuxOutputKey{}).(bool)
	return ok && v
}

// shouldSuppressAuxiliaryOutput returns true when CLI argv requests a structured
// shouldSuppressAuxiliaryOutput determines whether auxiliary human-facing output should be suppressed based on CLI arguments.
// argv is the command-line argument list to scan for output format flags (for example `--output` or `-o`).
// It returns true if the detected output format is a structured, machine-readable format such as JSON or NDJSON, and false otherwise.
func shouldSuppressAuxiliaryOutput(argv []string) bool {
	return isStructuredOutputFormat(detectOutputFormatFromArgs(argv))
}

// isStructuredOutputFormat reports whether the provided format string identifies a machine-readable output format.
// It compares the case-insensitive, trimmed value against "json" and "ndjson".
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
// detectOutputFormatFromArgs scans argv for an output-format option and returns the specified format value.
// It supports these forms: `--output=<value>`, `--output <value>`, `-o=<value>`, and `-o <value>`.
// The scan stops if a standalone `--` is encountered. Leading and trailing whitespace around the detected
// value is trimmed. If no output option is found or a flag is present without a following value, an empty
// string is returned.
// argv is the list of command-line arguments to inspect (commonly os.Args[1:]).
// The returned string is the detected output format (trimmed), or `""` if none was found.
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
