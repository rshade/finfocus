package cli

import (
	"context"
	"sync"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine/cache"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
)

// sharedStores holds the cost cache and the resource history for one process.
// Bolt takes a file lock per open handle, so a second open in the same process
// times out and silently degrades to "no cache" or "no history". Every stack's
// engine and pipeline therefore receives these two handles instead of opening
// its own, and only the owner closes them.
type sharedStores struct {
	cache        cache.Cache
	history      history.Store
	closeOnce    sync.Once
	closeHistory func()
	closeCache   func() error
}

// openSharedStores opens the cache and history once from cfg. A disabled or
// failed store stays nil and is reported once, as a single-stack run does.
func openSharedStores(ctx context.Context, cmd *cobra.Command, cfg *config.Config) *sharedStores {
	if cfg == nil {
		cfg = config.New()
	}
	stores := &sharedStores{cache: initCacheFromConfig(ctx, cmd, cfg)}
	if stores.cache != nil {
		stores.closeCache = stores.cache.Close
	}
	stores.history, stores.closeHistory = initHistoryFromConfig(ctx, cfg, config.GetResolvedProjectDir())
	return stores
}

// Close releases the history and the cache. It is safe to call more than once
// and on a nil receiver.
func (s *sharedStores) Close(ctx context.Context) {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.closeHistory != nil {
			s.closeHistory()
		}
		if s.closeCache != nil {
			if err := s.closeCache(); err != nil {
				logging.FromContext(ctx).Warn().Ctx(ctx).Err(err).
					Str("component", "cache").
					Msg("failed to close cache store")
			}
		}
	})
}

// cacheStore returns the shared cache, nil when s is nil or caching is off.
func (s *sharedStores) cacheStore() cache.Cache {
	if s == nil {
		return nil
	}
	return s.cache
}

// historyStore returns the shared history, nil when s is nil or history is off.
func (s *sharedStores) historyStore() history.Store {
	if s == nil {
		return nil
	}
	return s.history
}
