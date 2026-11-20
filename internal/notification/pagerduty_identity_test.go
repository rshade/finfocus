package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestPagerDutyDeduplicationIdentity(t *testing.T) {
	t.Parallel()
	evaluated := time.Date(2026, time.January, 31, 23, 59, 0, 0, time.FixedZone("west", -3600))
	base := NewBudgetAlertEvent(BudgetAlertInput{
		Scope: ScopeTag, ScopeKey: "team:platform", Threshold: 80,
		AlertType: config.AlertTypeActual, Time: evaluated,
	})
	base.Project = "payments"
	key := pagerDutyDedupKey("", pagerDutyScope(base), base)
	require.Equal(t, "finfocus:payments:tag/team=platform:80:actual:2026-01", key)
	assert.Equal(t, time.February, base.Timestamp.Month())
	assert.Equal(t, key, pagerDutyDedupKey("per-threshold", pagerDutyScope(base), base))

	tests := []struct {
		name   string
		change func(*BudgetAlertEvent)
	}{
		{"threshold", func(event *BudgetAlertEvent) { event.ThresholdPercent = 80.25 }},
		{"scope type", func(event *BudgetAlertEvent) { event.Scope = ScopeProvider }},
		{"scope key", func(event *BudgetAlertEvent) { event.ScopeKey = "team:product" }},
		{"project", func(event *BudgetAlertEvent) { event.Project = "checkout" }},
		{"alert type", func(event *BudgetAlertEvent) { event.AlertType = config.AlertTypeForecasted }},
		{"budget month", func(event *BudgetAlertEvent) { event.BudgetMonth = "2026-02" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			event := base
			tc.change(&event)
			assert.NotEqual(t, key, pagerDutyDedupKey("", pagerDutyScope(event), event))
		})
	}

	longEvent := base
	longEvent.Project = strings.Repeat("long-project", 200)
	long := pagerDutyDedupKey("", pagerDutyScope(longEvent), longEvent)
	assert.Equal(t, long, pagerDutyDedupKey("", pagerDutyScope(longEvent), longEvent))
	assert.NotEqual(t, long, pagerDutyDedupKey("", "another scope", longEvent))
}

func TestPagerDutyDeduplicationEscapesSeparators(t *testing.T) {
	t.Parallel()
	first := BudgetAlertEvent{Project: "billing:dev", Scope: ScopeProvider, ScopeKey: "aws"}
	second := BudgetAlertEvent{Project: "billing", Scope: "dev:" + ScopeProvider, ScopeKey: "aws"}
	assert.NotEqual(t, pagerDutyDedupKey("", pagerDutyScope(first), first),
		pagerDutyDedupKey("", pagerDutyScope(second), second))
}
