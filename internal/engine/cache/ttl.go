package cache

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// TTL configuration constants and defaults.
const (
	// DefaultTTLSeconds is the default cache TTL (1 hour).
	DefaultTTLSeconds = 3600

	// MinTTLSeconds is the minimum allowed TTL (1 minute).
	MinTTLSeconds = 60

	// MaxTTLSeconds is the maximum allowed TTL (7 days).
	MaxTTLSeconds = 604800

	// DefaultCacheMaxSizeMB is the default maximum cache size in MB.
	DefaultCacheMaxSizeMB = 100

	// minutesPerHour is used for duration formatting calculations.
	minutesPerHour = 60

	// hoursPerDay is used for duration formatting calculations.
	hoursPerDay = 24

	// EnvTTLSeconds is the environment variable for overriding TTL.
	EnvTTLSeconds = "FINFOCUS_CACHE_TTL_SECONDS"

	// EnvCacheEnabled is the environment variable for enabling/disabling cache.
	EnvCacheEnabled = "FINFOCUS_CACHE_ENABLED"

	// EnvCacheDir is the environment variable for cache directory.
	EnvCacheDir = "FINFOCUS_CACHE_DIR"

	// EnvCacheMaxSize is the environment variable for max cache size in MB.
	EnvCacheMaxSize = "FINFOCUS_CACHE_MAX_SIZE_MB"
)

// TTL validation errors.
var (
	ErrInvalidTTL = fmt.Errorf("TTL must be between %d and %d seconds", MinTTLSeconds, MaxTTLSeconds)
)

// TTLConfig holds cache TTL configuration with validation.
type TTLConfig struct {
	// Seconds is the TTL duration in seconds.
	Seconds int

	// Duration is the TTL as a time.Duration.
	Duration time.Duration
}

// NewTTLConfig creates a TTL configuration with validation.
// Returns an error if the TTL is outside the valid range.
func NewTTLConfig(seconds int) (*TTLConfig, error) {
	if seconds < MinTTLSeconds || seconds > MaxTTLSeconds {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidTTL, seconds)
	}

	return &TTLConfig{
		Seconds:  seconds,
		Duration: time.Duration(seconds) * time.Second,
	}, nil
}

// DefaultTTLConfig returns the default TTL configuration.
func DefaultTTLConfig() *TTLConfig {
	return &TTLConfig{
		Seconds:  DefaultTTLSeconds,
		Duration: time.Duration(DefaultTTLSeconds) * time.Second,
	}
}

// GetTTLFromEnv reads the TTL from environment variable or returns the default.
// If the environment variable is invalid, returns the default and logs a warning.
func GetTTLFromEnv() int {
	envVal := os.Getenv(EnvTTLSeconds)
	if envVal == "" {
		return DefaultTTLSeconds
	}

	ttl, err := strconv.Atoi(envVal)
	if err != nil {
		return DefaultTTLSeconds
	}

	// Validate range
	if ttl < MinTTLSeconds || ttl > MaxTTLSeconds {
		return DefaultTTLSeconds
	}

	return ttl
}

// GetCacheEnabledFromEnv reads the cache enabled flag from environment variable.
// Returns true by default if the variable is not set.
func GetCacheEnabledFromEnv() bool {
	envVal := os.Getenv(EnvCacheEnabled)
	if envVal == "" {
		return true // Enabled by default
	}

	enabled, err := strconv.ParseBool(envVal)
	if err != nil {
		return true // Default to enabled on parse error
	}

	return enabled
}

// GetCacheDirFromEnv reads the cache directory from environment variable.
// Returns an empty string if not set (caller should use default).
func GetCacheDirFromEnv() string {
	return os.Getenv(EnvCacheDir)
}

// GetCacheMaxSizeFromEnv reads the max cache size from environment variable.
// Returns DefaultCacheMaxSizeMB if not set or invalid.
func GetCacheMaxSizeFromEnv() int {
	envVal := os.Getenv(EnvCacheMaxSize)
	if envVal == "" {
		return DefaultCacheMaxSizeMB
	}

	maxSize, err := strconv.Atoi(envVal)
	if err != nil {
		return DefaultCacheMaxSizeMB
	}

	if maxSize < 0 {
		return DefaultCacheMaxSizeMB // Negative is invalid, use default
	}

	return maxSize
}

// FormatDuration formats a duration in a human-readable way.
// Examples: "1h", "30m", "5m30s".
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < hoursPerDay*time.Hour {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % minutesPerHour
		if minutes == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	days := int(d.Hours()) / hoursPerDay
	hours := int(d.Hours()) % hoursPerDay
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd%dh", days, hours)
}

// ParseTTL parses a TTL string in various formats:
// - Integer seconds: "3600".
// - Duration string: "1h", "30m", "1h30m".
func ParseTTL(s string) (int, error) {
	// Try parsing as integer seconds first
	if seconds, err := strconv.Atoi(s); err == nil {
		if seconds < MinTTLSeconds || seconds > MaxTTLSeconds {
			return 0, fmt.Errorf("%w: got %d", ErrInvalidTTL, seconds)
		}
		return seconds, nil
	}

	// Try parsing as duration
	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL format: %w", err)
	}

	seconds := int(duration.Seconds())
	if seconds < MinTTLSeconds || seconds > MaxTTLSeconds {
		return 0, fmt.Errorf("%w: got %d", ErrInvalidTTL, seconds)
	}

	return seconds, nil
}
