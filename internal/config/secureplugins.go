package config

import (
	"fmt"
	"strings"
)

// EnvSecurePlugins is the environment variable that makes plugin installs and
// updates require a verifiable checksums.txt. It is off unless set.
const EnvSecurePlugins = "FINFOCUS_SECURE_PLUGINS"

// ResolveSecurePlugins reports whether secure plugin mode is on. The
// FINFOCUS_SECURE_PLUGINS value read through getenv decides: 1 or true enables,
// 0, false, or an empty value disables, and any other value is an error.
func ResolveSecurePlugins(getenv func(string) string) (bool, error) {
	value := strings.TrimSpace(getenv(EnvSecurePlugins))
	switch strings.ToLower(value) {
	case "1", "true":
		return true, nil
	case "0", "false", "":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be one of 1, true, 0, false (got %q)", EnvSecurePlugins, value)
	}
}
