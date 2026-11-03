package config

import (
	"fmt"
	"strconv"
	"strings"
)

// EnvWebLogs is the environment variable that turns the web Logs view on or off.
// It takes precedence over the web.logs.enabled config key.
const EnvWebLogs = "FINFOCUS_WEB_LOGS"

const (
	webKeyLogs    = "logs"
	webKeyEnabled = "enabled"
)

// WebConfig is the web section, which configures the dashboard.
type WebConfig struct {
	Logs WebLogsConfig `yaml:"logs,omitempty" json:"logs,omitzero"`
}

// WebLogsConfig is the web.logs section. A nil Enabled means the key is unset.
type WebLogsConfig struct {
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// ResolveWebLogs reports whether the dashboard Logs view is enabled. The
// FINFOCUS_WEB_LOGS value read through getenv wins when it is set: 1 or true
// enables, 0 or false disables, and any other non-empty value is an error. When
// the variable is empty, configured decides, and an absent setting means false.
func ResolveWebLogs(getenv func(string) string, configured *bool) (bool, error) {
	value := strings.TrimSpace(getenv(EnvWebLogs))
	switch strings.ToLower(value) {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	case "":
		return configured != nil && *configured, nil
	default:
		return false, fmt.Errorf("%s must be one of 1, true, 0, false (got %q)", EnvWebLogs, value)
	}
}

func (c *Config) setWebValue(parts []string, value string) error {
	if len(parts) != 2 || parts[0] != webKeyLogs || parts[1] != webKeyEnabled {
		return fmt.Errorf("unknown web setting: %s", strings.Join(parts, "."))
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("web.logs.enabled must be true or false (got %q)", value)
	}
	if c.Web == nil {
		c.Web = &WebConfig{}
	}
	c.Web.Logs.Enabled = &enabled
	return nil
}

func (c *Config) getWebValue(parts []string) (any, error) {
	if len(parts) != 2 || parts[0] != webKeyLogs || parts[1] != webKeyEnabled {
		return nil, fmt.Errorf("unknown web setting: %s", strings.Join(parts, "."))
	}
	return c.Web != nil && c.Web.Logs.Enabled != nil && *c.Web.Logs.Enabled, nil
}
