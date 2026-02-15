package config

// Cache configuration constants. These live in the config package to avoid a
// dependency inversion where config would otherwise import internal/engine/cache.
// The cache package re-exports them for backward compatibility.
const (
	// CacheDefaultTTLSeconds is the default cache TTL (1 hour).
	CacheDefaultTTLSeconds = 3600

	// CacheDefaultMaxSizeMB is the default maximum cache size in MB (0 = unlimited).
	CacheDefaultMaxSizeMB = 100

	// CacheEnvTTLSeconds is the environment variable for overriding TTL.
	CacheEnvTTLSeconds = "FINFOCUS_CACHE_TTL"

	// CacheEnvTTLSecondsLegacy is a backward-compatible alias for CacheEnvTTLSeconds.
	CacheEnvTTLSecondsLegacy = "FINFOCUS_CACHE_TTL_SECONDS"

	// CacheEnvEnabled is the environment variable for enabling/disabling cache.
	CacheEnvEnabled = "FINFOCUS_CACHE_ENABLED"

	// CacheEnvDir is the environment variable for cache directory.
	CacheEnvDir = "FINFOCUS_CACHE_DIR"

	// CacheEnvMaxSize is the environment variable for max cache size in MB.
	CacheEnvMaxSize = "FINFOCUS_CACHE_MAX_SIZE_MB"
)
