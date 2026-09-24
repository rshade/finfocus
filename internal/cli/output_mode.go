package cli

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
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

// suppressAuxiliaryOutput reports whether this invocation requests machine output,
// either through --output/-o in the process argv or through --format json.
func suppressAuxiliaryOutput(cmd *cobra.Command, argv []string) bool {
	return shouldSuppressAuxiliaryOutput(argv) || machineOutputRequested(cmd)
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

// axFormatFlag is the persistent agent-mode flag ax.Execute mounts on the root
// command. The MCP dispatcher injects --format=json on every tools/call.
const axFormatFlag = "format"

// agentModeEnv is the environment variable ax-go reads as an explicit
// agent-mode opt-in (after --format, before TTY detection).
const agentModeEnv = "AGENT_MODE"

// resolveOutputFormat returns the effective value of a command's output-style
// flag (flagName, currently holding current). Precedence:
//
//  1. an explicitly set flag always wins, so an invalid explicit value still
//     reaches the command's own validation and is rejected before state loads;
//  2. an explicit machine-mode request selects JSON (see machineOutputRequested);
//  3. otherwise the flag's default (current) is kept.
func resolveOutputFormat(cmd *cobra.Command, flagName, current string) string {
	if cmd.Flags().Changed(flagName) {
		return current
	}
	if machineOutputRequested(cmd) {
		return outputFormatJSON
	}
	return current
}

// resolveJSONFlag is resolveOutputFormat for commands that expose a boolean
// --json flag instead of a string --output flag.
func resolveJSONFlag(cmd *cobra.Command, flagName string, current bool) bool {
	if cmd.Flags().Changed(flagName) {
		return current
	}
	return current || machineOutputRequested(cmd)
}

// machineOutputRequested reports whether this invocation explicitly asks for
// machine (JSON) output: --format json, or else AGENT_MODE, resolved through
// ax-go's own precedence rules.
//
// Only explicit signals count. ax-go falls back to TTY detection, which reports
// JSON whenever stdout is piped; that would silently change the output of
// redirected human invocations, so the TTY branch is disabled by always passing
// stdoutIsTTY=true. An invalid --format or AGENT_MODE value is not a request.
func machineOutputRequested(cmd *cobra.Command) bool {
	format := ""
	if flag := cmd.Flag(axFormatFlag); flag != nil {
		format = flag.Value.String()
	}
	mode, err := ax.ResolveMode(format, os.Getenv(agentModeEnv), true)
	return err == nil && mode == ax.ModeJSON
}

// actionResult is the machine-readable outcome of a mutating command run with
// --format json, reported instead of its human progress lines.
type actionResult struct {
	Action  string `json:"action"`
	DryRun  bool   `json:"dry_run"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

// writeJSON writes v to the command's stdout as indented JSON.
func writeJSON(cmd *cobra.Command, v any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
