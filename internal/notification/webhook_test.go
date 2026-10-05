package notification_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
	"github.com/rshade/finfocus/pkg/version"
)

func TestWebhookEventBody(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 10, 4, 15, 4, 5, 0, time.UTC)
	tests := []struct {
		name     string
		in       notification.BudgetAlertInput
		wantBody string
	}{
		{
			name: "global actual",
			in: notification.BudgetAlertInput{
				Amount: 100, Currency: "USD", Threshold: 80, AlertType: config.AlertTypeActual,
				CurrentSpend: 85, CurrentPercent: 85, ForecastedSpend: 300, ForecastPercent: 300, Time: when,
			},
			wantBody: `{
  "event": "budget.threshold.exceeded",
  "timestamp": "2026-10-04T15:04:05Z",
  "budget": {"name": "global", "scope": "global", "amount": 100, "currency": "USD", "period": "monthly"},
  "threshold": {"percentage": 80, "type": "actual", "value": 80},
  "current": {"spend": 85, "percentage": 85},
  "metadata": {"source": "finfocus", "version": "` + version.GetVersion() + `"}
}`,
		},
		{
			name: "provider forecasted",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeProvider, ScopeKey: "aws", Amount: 50, Currency: "USD",
				Threshold: 100, AlertType: config.AlertTypeForecasted,
				CurrentSpend: 20, CurrentPercent: 40, ForecastedSpend: 60, ForecastPercent: 120, Time: when,
			},
			wantBody: `{
  "event": "budget.threshold.exceeded",
  "timestamp": "2026-10-04T15:04:05Z",
  "budget": {"name": "provider:aws", "scope": "provider", "scope_key": "aws", "amount": 50,
             "currency": "USD", "period": "monthly"},
  "threshold": {"percentage": 100, "type": "forecasted", "value": 50},
  "current": {"spend": 60, "percentage": 120},
  "metadata": {"source": "finfocus", "version": "` + version.GetVersion() + `"}
}`,
		},
		{
			name: "tag actual",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeTag, ScopeKey: "team:platform", Amount: 10, Currency: "USD",
				Threshold: 50, AlertType: config.AlertTypeActual, CurrentSpend: 6, CurrentPercent: 60, Time: when,
			},
			wantBody: `{
  "event": "budget.threshold.exceeded",
  "timestamp": "2026-10-04T15:04:05Z",
  "budget": {"name": "tag:team:platform", "scope": "tag", "scope_key": "team:platform", "amount": 10,
             "currency": "USD", "period": "monthly"},
  "threshold": {"percentage": 50, "type": "actual", "value": 5},
  "current": {"spend": 6, "percentage": 60},
  "metadata": {"source": "finfocus", "version": "` + version.GetVersion() + `"}
}`,
		},
		{
			name: "type actual",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeType, ScopeKey: "aws:s3/bucket", Amount: 10, Currency: "USD",
				Threshold: 100, AlertType: config.AlertTypeActual, CurrentSpend: 11, CurrentPercent: 110, Time: when,
			},
			wantBody: `{
  "event": "budget.threshold.exceeded",
  "timestamp": "2026-10-04T15:04:05Z",
  "budget": {"name": "type:aws:s3/bucket", "scope": "type", "scope_key": "aws:s3/bucket", "amount": 10,
             "currency": "USD", "period": "monthly"},
  "threshold": {"percentage": 100, "type": "actual", "value": 10},
  "current": {"spend": 11, "percentage": 110},
  "metadata": {"source": "finfocus", "version": "` + version.GetVersion() + `"}
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newCaptureServer(t, http.StatusAccepted)
			result := deliverOne(t, srv.Server,
				config.NotificationDestination{Type: config.NotificationTypeWebhook, URL: srv.URL + "/budget-alert"},
				tc.in, nil)
			assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
			reqs := srv.received()
			require.Len(t, reqs, 1)
			assert.Equal(t, http.MethodPost, reqs[0].Method)
			assert.Equal(t, "/budget-alert", reqs[0].Path)
			assert.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
			assert.JSONEq(t, tc.wantBody, string(reqs[0].Body))
		})
	}
}

func TestWebhookMethodAndHeaders(t *testing.T) {
	t.Parallel()

	srv := newCaptureServer(t, http.StatusOK)
	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_API_TOKEN": "tok-abc"}}
	result := deliverOne(t, srv.Server, config.NotificationDestination{
		Type:   config.NotificationTypeWebhook,
		URL:    srv.URL,
		Method: "put",
		Headers: map[string]string{
			"Authorization": "Bearer ${FINFOCUS_NOTIFY_API_TOKEN}",
			"Content-Type":  "application/vnd.finfocus+json",
			"X-Team":        "platform",
		},
	}, notification.BudgetAlertInput{Amount: 100, Currency: "USD", Threshold: 80, AlertType: config.AlertTypeActual},
		lookup.lookup)

	assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
	reqs := srv.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, http.MethodPut, reqs[0].Method)
	assert.Equal(t, "Bearer tok-abc", reqs[0].Header.Get("Authorization"))
	assert.Equal(t, "application/vnd.finfocus+json", reqs[0].Header.Get("Content-Type"))
	assert.Equal(t, "platform", reqs[0].Header.Get("X-Team"))
	assert.True(t, json.Valid(reqs[0].Body))
}
