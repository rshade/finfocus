package notification_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
)

type capturedRequest struct {
	Method  string
	Path    string
	Header  http.Header
	Body    []byte
	Payload map[string]any
}

type captureServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []capturedRequest
}

func newCaptureServer(t *testing.T, status int) *captureServer {
	t.Helper()
	cs := &captureServer{}
	cs.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		cs.mu.Lock()
		cs.requests = append(cs.requests, capturedRequest{
			Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body, Payload: payload,
		})
		cs.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *captureServer) received() []capturedRequest {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return append([]capturedRequest(nil), cs.requests...)
}

func deliverOne(
	t *testing.T,
	srv *httptest.Server,
	dest config.NotificationDestination,
	in notification.BudgetAlertInput,
	lookup func(string) (string, bool),
) notification.DeliveryResult {
	t.Helper()
	d := notification.NewDispatcher(srv.Client(), lookup)
	results := d.Deliver(context.Background(),
		[]notification.Delivery{{Destination: dest, Event: notification.NewBudgetAlertEvent(in)}},
		notification.Options{})
	require.Len(t, results, 1)
	return results[0]
}

func slackFields(t *testing.T, payload map[string]any) []string {
	t.Helper()
	blocks, ok := payload["blocks"].([]any)
	require.True(t, ok, "blocks: %v", payload)
	require.Len(t, blocks, 2)
	header := blocks[0].(map[string]any)
	assert.Equal(t, "header", header["type"])
	assert.Equal(t, map[string]any{"type": "plain_text", "text": "Budget alert"}, header["text"])
	section := blocks[1].(map[string]any)
	assert.Equal(t, "section", section["type"])
	fields := section["fields"].([]any)
	texts := make([]string, 0, len(fields))
	for _, field := range fields {
		f := field.(map[string]any)
		assert.Equal(t, "mrkdwn", f["type"])
		texts = append(texts, f["text"].(string))
	}
	return texts
}

func TestSlackMessageActualGlobal(t *testing.T) {
	t.Parallel()

	srv := newCaptureServer(t, http.StatusOK)
	result := deliverOne(t, srv.Server,
		config.NotificationDestination{Type: config.NotificationTypeSlack, URL: srv.URL + "/services/T/B/X"},
		notification.BudgetAlertInput{
			Amount: 100, Currency: "USD", Threshold: 80, AlertType: config.AlertTypeActual,
			CurrentSpend: 85, CurrentPercent: 85,
		}, nil)

	assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
	reqs := srv.received()
	require.Len(t, reqs, 1)
	assert.Equal(t, http.MethodPost, reqs[0].Method)
	assert.Equal(t, "/services/T/B/X", reqs[0].Path)
	assert.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
	payload := reqs[0].Payload
	assert.NotContains(t, payload, "channel")
	assert.Equal(t, "Budget alert: global budget crossed its 80% actual threshold", payload["text"])
	assert.Equal(t, []string{
		"*Budget:*\nglobal, $100.00/month",
		"*Current spend:*\n$85.00 (85%)",
		"*Threshold:*\n80% actual",
		"*Status:*\nExceeded",
	}, slackFields(t, payload))
}

func TestSlackMessageVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		channel    string
		in         notification.BudgetAlertInput
		wantText   string
		wantFields []string
	}{
		{
			name:    "forecasted provider with channel",
			channel: "#finops-alerts",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeProvider, ScopeKey: "aws", Amount: 50, Currency: "EUR",
				Threshold: 100, AlertType: config.AlertTypeForecasted,
				CurrentSpend: 20, CurrentPercent: 40, ForecastedSpend: 61.234, ForecastPercent: 122.468,
			},
			wantText: "Budget alert: provider aws budget crossed its 100% forecasted threshold",
			wantFields: []string{
				"*Budget:*\nprovider aws, €50.00/month",
				"*Forecasted spend:*\n€61.23 (122.5%)",
				"*Threshold:*\n100% forecasted",
				"*Status:*\nExceeded",
			},
		},
		{
			name: "tag scope with unknown currency",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeTag, ScopeKey: "team:platform", Amount: 1000, Currency: "CHF",
				Threshold: 87.5, AlertType: config.AlertTypeActual, CurrentSpend: 900, CurrentPercent: 90,
			},
			wantText: "Budget alert: tag team:platform budget crossed its 87.5% actual threshold",
			wantFields: []string{
				"*Budget:*\ntag team:platform, CHF 1000.00/month",
				"*Current spend:*\nCHF 900.00 (90%)",
				"*Threshold:*\n87.5% actual",
				"*Status:*\nExceeded",
			},
		},
		{
			name: "type scope",
			in: notification.BudgetAlertInput{
				Scope: notification.ScopeType, ScopeKey: "aws:ec2/instance", Amount: 10, Currency: "GBP",
				Threshold: 50, AlertType: config.AlertTypeActual, CurrentSpend: 5, CurrentPercent: 50,
			},
			wantText: "Budget alert: type aws:ec2/instance budget crossed its 50% actual threshold",
			wantFields: []string{
				"*Budget:*\ntype aws:ec2/instance, £10.00/month",
				"*Current spend:*\n£5.00 (50%)",
				"*Threshold:*\n50% actual",
				"*Status:*\nExceeded",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := newCaptureServer(t, http.StatusNoContent)
			result := deliverOne(t, srv.Server, config.NotificationDestination{
				Type: config.NotificationTypeSlack, URL: srv.URL, Channel: tc.channel,
			}, tc.in, nil)
			assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
			reqs := srv.received()
			require.Len(t, reqs, 1)
			if tc.channel != "" {
				assert.Equal(t, tc.channel, reqs[0].Payload["channel"])
			}
			assert.Equal(t, tc.wantText, reqs[0].Payload["text"])
			assert.Equal(t, tc.wantFields, slackFields(t, reqs[0].Payload))
		})
	}
}
