package engine

import (
	"testing"
	"time"
)

// SetRecommendationHistoryTimeoutForTest shortens one history RPC deadline
// and restores the production value when the test ends.
func SetRecommendationHistoryTimeoutForTest(t *testing.T, timeout time.Duration) {
	t.Helper()
	prev := recommendationHistoryTimeout
	recommendationHistoryTimeout = timeout
	t.Cleanup(func() { recommendationHistoryTimeout = prev })
}
