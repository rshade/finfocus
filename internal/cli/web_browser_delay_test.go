package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // Swaps the package-level WSL detection seam.
func TestWaitForBrowserRelay(t *testing.T) {
	origWSL, origDelay := runningUnderWSL, wslBrowserOpenDelay
	t.Cleanup(func() { runningUnderWSL, wslBrowserOpenDelay = origWSL, origDelay })
	wslBrowserOpenDelay = 200 * time.Millisecond

	t.Run("skips the wait outside WSL", func(t *testing.T) {
		runningUnderWSL = func() bool { return false }
		start := time.Now()
		waitForBrowserRelay(context.Background())
		assert.Less(t, time.Since(start), wslBrowserOpenDelay/2)
	})

	t.Run("waits under WSL", func(t *testing.T) {
		runningUnderWSL = func() bool { return true }
		start := time.Now()
		waitForBrowserRelay(context.Background())
		assert.GreaterOrEqual(t, time.Since(start), wslBrowserOpenDelay)
	})

	t.Run("returns early when canceled", func(t *testing.T) {
		runningUnderWSL = func() bool { return true }
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		start := time.Now()
		waitForBrowserRelay(ctx)
		assert.Less(t, time.Since(start), wslBrowserOpenDelay/2)
	})
}
