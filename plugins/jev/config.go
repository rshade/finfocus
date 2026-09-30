package jev

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/rshade/finfocus/plugins/jev/internal/scoring"
)

const (
	// EnvAPIKey holds the TypeSafe AI API key. It is the only place the key is
	// read from.
	EnvAPIKey = "TYPESAFE_API_KEY" //nolint:gosec // The name of an environment variable, not a credential.
	// EnvBaseURL overrides the API origin.
	EnvBaseURL = "JEV_BASE_URL"
	// EnvModel overrides the model id.
	EnvModel = "JEV_MODEL"
	// EnvTimeout overrides the per-attempt request timeout, as a Go duration.
	EnvTimeout = "JEV_TIMEOUT"
	// EnvBatchSize overrides how many recommendations go in one backend
	// request.
	EnvBatchSize = "JEV_BATCH_SIZE"
	// EnvDuplicateThreshold overrides the yes probability at which two
	// recommendations count as duplicates.
	EnvDuplicateThreshold = "JEV_DUPLICATE_THRESHOLD"

	// DefaultBaseURL is the public TypeSafe AI API.
	DefaultBaseURL = "https://api.typesafe.ai"
	// DefaultModel is a versioned model id rather than the moving jev-latest
	// alias, so scores stay comparable until the operator opts into a new one.
	DefaultModel = "jev-1.13.0"
	// MaxBatchSize is the largest accepted JEV_BATCH_SIZE; larger batches
	// were not measured.
	MaxBatchSize = 80
	// DefaultTimeout bounds each backend attempt.
	DefaultTimeout = 60 * time.Second
)

// Config configures the plugin. The zero value is not usable; start from
// ConfigFromEnv or DefaultConfig.
type Config struct {
	// APIKey authenticates to the backend. Empty leaves the plugin running but
	// every scoring call returns UNAUTHENTICATED.
	APIKey string
	// BaseURL is the API origin.
	BaseURL string
	// Model is the model id sent with every request.
	Model string
	// Timeout bounds each backend attempt.
	Timeout time.Duration
	// BatchSize is the number of recommendations per backend request.
	BatchSize int
	// DuplicateThreshold is the yes probability that marks two recommendations
	// as duplicates.
	DuplicateThreshold float64
	// Logger receives counts and timings only.
	Logger zerolog.Logger
}

// DefaultConfig returns the defaults with no API key.
func DefaultConfig() Config {
	return Config{
		BaseURL:            DefaultBaseURL,
		Model:              DefaultModel,
		Timeout:            DefaultTimeout,
		BatchSize:          scoring.DefaultBatchSize,
		DuplicateThreshold: scoring.DefaultDuplicateThreshold,
	}
}

// ConfigFromEnv reads the configuration through getenv, which is [os.Getenv] in
// production. A missing API key is not an error.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	cfg := DefaultConfig()
	cfg.APIKey = strings.TrimSpace(getenv(EnvAPIKey))
	if v := strings.TrimSpace(getenv(EnvBaseURL)); v != "" {
		cfg.BaseURL = v
	}
	if v := strings.TrimSpace(getenv(EnvModel)); v != "" {
		cfg.Model = v
	}
	if v := strings.TrimSpace(getenv(EnvTimeout)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("%s must be a positive duration such as 30s, got %q", EnvTimeout, v)
		}
		cfg.Timeout = d
	}
	if v := strings.TrimSpace(getenv(EnvBatchSize)); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxBatchSize {
			return Config{}, fmt.Errorf("%s must be a whole number from 1 to %d, got %q", EnvBatchSize, MaxBatchSize, v)
		}
		cfg.BatchSize = n
	}
	if v := strings.TrimSpace(getenv(EnvDuplicateThreshold)); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 || f > 1 {
			return Config{}, fmt.Errorf("%s must be a number above 0 and at most 1, got %q", EnvDuplicateThreshold, v)
		}
		cfg.DuplicateThreshold = f
	}
	return cfg, nil
}
