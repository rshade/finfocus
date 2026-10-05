package notification_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
	"github.com/rshade/finfocus/pkg/version"
)

func TestNewBudgetAlertEvent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("CDT", -5*60*60))
	base := notification.BudgetAlertInput{
		Amount:          100,
		Currency:        "USD",
		Threshold:       80,
		AlertType:       config.AlertTypeActual,
		CurrentSpend:    85,
		CurrentPercent:  85,
		ForecastedSpend: 120,
		ForecastPercent: 120,
		Time:            now,
	}

	tests := []struct {
		name         string
		mutate       func(in *notification.BudgetAlertInput)
		wantScope    string
		wantScopeKey string
		wantSpend    float64
		wantPercent  float64
	}{
		{
			name:        "legacy budget is global and uses current spend",
			mutate:      func(*notification.BudgetAlertInput) {},
			wantScope:   notification.ScopeGlobal,
			wantSpend:   85,
			wantPercent: 85,
		},
		{
			name: "forecasted alert uses forecast",
			mutate: func(in *notification.BudgetAlertInput) {
				in.AlertType = config.AlertTypeForecasted
			},
			wantScope:   notification.ScopeGlobal,
			wantSpend:   120,
			wantPercent: 120,
		},
		{
			name:         "provider scope key passes through",
			mutate:       func(in *notification.BudgetAlertInput) { in.Scope, in.ScopeKey = "provider", "aws" },
			wantScope:    "provider",
			wantScopeKey: "aws",
			wantSpend:    85,
			wantPercent:  85,
		},
		{
			name:         "tag scope key passes through",
			mutate:       func(in *notification.BudgetAlertInput) { in.Scope, in.ScopeKey = "tag", "team:platform" },
			wantScope:    "tag",
			wantScopeKey: "team:platform",
			wantSpend:    85,
			wantPercent:  85,
		},
		{
			name:         "type scope key passes through",
			mutate:       func(in *notification.BudgetAlertInput) { in.Scope, in.ScopeKey = "type", "aws:ec2/instance" },
			wantScope:    "type",
			wantScopeKey: "aws:ec2/instance",
			wantSpend:    85,
			wantPercent:  85,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := base
			tc.mutate(&in)
			event := notification.NewBudgetAlertEvent(in)
			assert.Equal(t, tc.wantScope, event.Scope)
			assert.Equal(t, tc.wantScopeKey, event.ScopeKey)
			assert.InDelta(t, tc.wantSpend, event.Spend, 1e-9)
			assert.InDelta(t, tc.wantPercent, event.SpendPercent, 1e-9)
			assert.InDelta(t, 80.0, event.ThresholdValue, 1e-9)
			assert.InDelta(t, 80.0, event.ThresholdPercent, 1e-9)
			assert.Equal(t, in.AlertType, event.AlertType)
			assert.Equal(t, "monthly", event.Period)
			assert.Equal(t, "USD", event.Currency)
			assert.Equal(t, time.UTC, event.Timestamp.Location())
			assert.True(t, now.Equal(event.Timestamp))
			assert.Equal(t, version.GetVersion(), event.Version)
		})
	}
}

func TestNewBudgetAlertEventThresholdValue(t *testing.T) {
	t.Parallel()

	event := notification.NewBudgetAlertEvent(notification.BudgetAlertInput{
		Amount: 250, Threshold: 90, Period: "monthly", AlertType: config.AlertTypeActual,
	})
	assert.InDelta(t, 225.0, event.ThresholdValue, 1e-9)
	assert.False(t, event.Timestamp.IsZero())
}
