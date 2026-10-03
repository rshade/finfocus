package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyCacheLRUOverrides(t *testing.T) {
	t.Setenv(CacheEnvLRUEnabled, "true")
	t.Setenv(CacheEnvLRUMaxItems, "32")
	cfg := &Config{}
	cfg.applyCacheLRUOverrides()
	assert.True(t, cfg.Cost.Cache.LRUEnabled)
	assert.Equal(t, 32, cfg.Cost.Cache.LRUMaxItems)

	t.Setenv(CacheEnvLRUEnabled, "")
	t.Setenv(CacheEnvLRUMaxItems, "")
	cfg.applyCacheLRUOverrides()
	assert.True(t, cfg.Cost.Cache.LRUEnabled)
	assert.Equal(t, 32, cfg.Cost.Cache.LRUMaxItems)

	t.Setenv(CacheEnvLRUEnabled, "nope")
	t.Setenv(CacheEnvLRUMaxItems, "0")
	cfg.applyCacheLRUOverrides()
	assert.True(t, cfg.Cost.Cache.LRUEnabled)
	assert.Equal(t, 0, cfg.Cost.Cache.LRUMaxItems)

	t.Setenv(CacheEnvLRUEnabled, "false")
	t.Setenv(CacheEnvLRUMaxItems, "-3")
	cfg.Cost.Cache.LRUMaxItems = 32
	cfg.applyCacheLRUOverrides()
	assert.False(t, cfg.Cost.Cache.LRUEnabled)
	assert.Equal(t, 32, cfg.Cost.Cache.LRUMaxItems)
}
