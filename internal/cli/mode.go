package cli

import (
	"path/filepath"
	"strings"
)

// DetectPluginMode determines if the application should run in Pulumi plugin mode.
// It checks the binary name and the FINFOCUS_PLUGIN_MODE environment variable.
// This is a pure function with no side effects and is safe to call multiple times.
//
// args: usually [os.Args] (nil or empty is handled gracefully).
// lookupEnv: function to retrieve environment variables (e.g., [os.LookupEnv]).
//
//	If nil, environment variable detection is skipped.
func DetectPluginMode(args []string, lookupEnv func(string) (string, bool)) bool {
	// 1. Check Env Var (skip if lookupEnv is nil)
	if lookupEnv != nil {
		if val, ok := lookupEnv("FINFOCUS_PLUGIN_MODE"); ok {
			val = strings.ToLower(val)
			if val == "true" || val == "1" {
				return true
			}
		}
	}

	// 2. Check Binary Name (handles nil or empty args gracefully)
	if len(args) > 0 {
		if isPluginBinary(args[0]) {
			return true
		}
	}

	return false
}

// isPluginBinary checks if the binary name matches the expected plugin name.
// It handles both Unix-style forward slashes and Windows-style backslashes.
func isPluginBinary(name string) bool {
	// Use filepath.Base first (handles current OS separator)
	base := filepath.Base(name)

	// Also handle cross-platform case: Windows paths on Unix or vice versa
	// by looking for the last occurrence of either separator
	if idx := strings.LastIndexAny(base, `/\`); idx >= 0 {
		base = base[idx+1:]
	}

	// Remove .exe extension if present (case-insensitive)
	if strings.HasSuffix(strings.ToLower(base), ".exe") {
		base = base[:len(base)-4]
	}
	return strings.EqualFold(base, "pulumi-tool-cost")
}
