package notification_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
)

const pagerDutyTestKey = "private-routing-key"

func TestPagerDutyRequest(t *testing.T) {
	t.Parallel()
	requests := make(chan map[string]any, 2)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v2/enqueue", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	event := testEvent(80)
	event.AlertType = config.AlertTypeForecasted
	event.Scope = notification.ScopeProvider
	event.ScopeKey = "aws"
	event.Spend, event.SpendPercent = 110, 110

	dest := config.NotificationDestination{
		Type: "pagerduty", URL: srv.URL + "/v2/enqueue",
		RoutingKey: "${FINFOCUS_NOTIFY_PD_KEY}",
	}
	dispatcher := notification.NewDispatcher(srv.Client(), func(name string) (string, bool) {
		return pagerDutyTestKey, name == "FINFOCUS_NOTIFY_PD_KEY"
	})

	for _, mode := range []string{"", "per-run"} {
		dest.DedupKey = mode
		delivery := notification.Delivery{Destination: dest, Event: event}
		result := dispatcher.Deliver(t.Context(), []notification.Delivery{delivery}, notification.Options{})
		require.Equal(t, notification.OutcomeSent, result[0].Outcome, result[0].Reason)
		body := <-requests
		assert.Equal(t, pagerDutyTestKey, body["routing_key"])
		assert.Equal(t, "trigger", body["event_action"])
		assert.Equal(t, "FinFocus", body["client"])
		if mode == "per-run" {
			assert.NotContains(t, body, "dedup_key")
		} else {
			assert.Contains(t, body["dedup_key"], ":provider/aws:80:forecasted:")
		}
		payload := body["payload"].(map[string]any)
		assert.Contains(t, payload["summary"], "forecasted")
		assert.Equal(t, "warning", payload["severity"])
		assert.Equal(t, "budget/provider/aws", payload["component"])
		details := payload["custom_details"].(map[string]any)
		assert.InEpsilon(t, 110, details["current"].(map[string]any)["spend"], 0.001)
	}
}

func TestPagerDutyResponseDiagnostics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"invalid", `{"message":"bad key private-routing-key\nsecond line","errors":["field is invalid"],"extra":"hide me"}`,
			"bad key [REDACTED] second line; field is invalid", http.StatusBadRequest},
		{"rate limited", `{"message":"rate limited"}`, "rate limited", http.StatusTooManyRequests},
		{"unexpected success", "", "status 200", http.StatusOK},
		{"upstream failure", "not JSON", "status 500", http.StatusInternalServerError},
		{"redirect", "", "status 307", http.StatusTemporaryRedirect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/v2/enqueue", r.URL.Path)
				w.Header().Set("Location", "/must-not-follow")
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			dispatcher := notification.NewDispatcher(srv.Client(), nil)
			delivery := notification.Delivery{
				Destination: config.NotificationDestination{
					Type: "pagerduty", URL: srv.URL + "/v2/enqueue", RoutingKey: pagerDutyTestKey,
				},
				Event: testEvent(80),
			}
			result := dispatcher.Deliver(t.Context(), []notification.Delivery{delivery}, notification.Options{})[0]
			assert.Equal(t, notification.OutcomeFailed, result.Outcome)
			assert.Contains(t, result.Reason, tc.want)
			assert.NotContains(t, result.Reason, pagerDutyTestKey)
			assert.NotContains(t, result.Reason, "hide me")
			assert.NotContains(t, result.Reason, "\n")
		})
	}
}

func TestPagerDutyDeliveryBounds(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Consume the request before waiting for its client to cancel.
		var body json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			w.WriteHeader(http.StatusAccepted)
		}
	}))
	defer srv.Close()
	dispatcher := notification.NewDispatcher(srv.Client(), nil)
	delivery := notification.Delivery{
		Destination: config.NotificationDestination{
			Type: "pagerduty", URL: srv.URL + "/v2/enqueue", RoutingKey: pagerDutyTestKey,
		},
		Event: testEvent(80),
	}
	result := dispatcher.Deliver(t.Context(), []notification.Delivery{delivery},
		notification.Options{Timeout: 50 * time.Millisecond})[0]
	assert.Equal(t, notification.OutcomeFailed, result.Outcome)
	assert.Contains(t, result.Reason, "timed out")
}

func TestPagerDutyPreviewAndReferenceRules(t *testing.T) {
	t.Parallel()
	lookup := func(name string) (string, bool) {
		if name == "FINFOCUS_NOTIFY_PD_KEY" {
			return pagerDutyTestKey, true
		}
		return "https://untrusted.example/v2/enqueue", true
	}
	delivery := notification.Delivery{
		Destination: config.NotificationDestination{Type: "pagerduty", RoutingKey: "${FINFOCUS_NOTIFY_PD_KEY}"},
		Event:       testEvent(80),
	}
	dispatcher := notification.NewDispatcher(nil, lookup)
	options := notification.Options{DryRun: true}
	result := dispatcher.Deliver(t.Context(), []notification.Delivery{delivery}, options)[0]
	require.Equal(t, notification.OutcomeDryRun, result.Outcome, result.Reason)
	assert.NotContains(t, fmt.Sprintf("%+v", result), pagerDutyTestKey)

	delivery.Destination.URL = "${FINFOCUS_NOTIFY_PD_URL}"
	result = dispatcher.Deliver(t.Context(), []notification.Delivery{delivery}, options)[0]
	assert.Equal(t, notification.OutcomeSkipped, result.Outcome)
	assert.Contains(t, result.Reason, "/v2/enqueue")

	delivery.Destination = delivery.Destination.AsProject()
	noLookup := notification.NewDispatcher(nil, func(string) (string, bool) {
		t.Error("project lookup would read a secret")
		return "", false
	})
	result = noLookup.Deliver(t.Context(), []notification.Delivery{delivery}, options)[0]
	assert.Equal(t, notification.OutcomeSkipped, result.Outcome)
	assert.Contains(t, result.Reason, "project config")

	delivery.Destination = config.NotificationDestination{
		Type:       "pagerduty",
		RoutingKey: "${FINFOCUS_NOTIFY_PD_MISSING}",
	}
	result = notification.NewDispatcher(nil, nil).Deliver(t.Context(), []notification.Delivery{delivery}, options)[0]
	assert.Equal(t, notification.OutcomeSkipped, result.Outcome)
	assert.Contains(t, result.Reason, "FINFOCUS_NOTIFY_PD_MISSING")
}

func TestPagerDutyBoundsLongSummary(t *testing.T) {
	t.Parallel()
	requests := make(chan map[string]any, 1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	event := testEvent(100)
	event.Scope = notification.ScopeType
	event.ScopeKey = strings.Repeat("cost🌍", 300)
	delivery := notification.Delivery{
		Destination: config.NotificationDestination{
			Type: "pagerduty", URL: srv.URL + "/v2/enqueue", RoutingKey: pagerDutyTestKey, Severity: "critical",
		},
		Event: event,
	}
	result := notification.NewDispatcher(srv.Client(), nil).
		Deliver(t.Context(), []notification.Delivery{delivery}, notification.Options{})[0]
	require.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
	body := <-requests
	payload := body["payload"].(map[string]any)
	summary := payload["summary"].(string)
	assert.LessOrEqual(t, len(summary), 1024)
	assert.True(t, utf8.ValidString(summary))
	assert.Contains(t, summary, "100% actual threshold")
	assert.Contains(t, body["dedup_key"], "finfocus:sha256:")
	assert.LessOrEqual(t, len(body["dedup_key"].(string)), 255)
}

func TestPagerDutyAcceptedWithSlowResponseBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusAccepted)
		if !assert.NoError(t, http.NewResponseController(w).Flush()) {
			return
		}
		<-r.Context().Done()
	}))
	defer srv.Close()
	dispatcher := notification.NewDispatcher(srv.Client(), nil)
	delivery := notification.Delivery{
		Destination: config.NotificationDestination{
			Type: "pagerduty", URL: srv.URL + "/v2/enqueue", RoutingKey: pagerDutyTestKey,
		},
		Event: testEvent(80),
	}
	result := dispatcher.Deliver(t.Context(), []notification.Delivery{delivery},
		notification.Options{Timeout: 150 * time.Millisecond})[0]
	assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
}

func TestPagerDutyConnectionRefused(t *testing.T) {
	t.Parallel()
	// Nothing listens on a privileged port; a closed ephemeral port can be
	// taken by a parallel test's server.
	delivery := notification.Delivery{
		Destination: config.NotificationDestination{
			Type: "pagerduty", URL: "https://127.0.0.1:2/v2/enqueue", RoutingKey: pagerDutyTestKey,
		},
		Event: testEvent(80),
	}
	result := notification.NewDispatcher(nil, nil).
		Deliver(t.Context(), []notification.Delivery{delivery}, notification.Options{})[0]
	assert.Equal(t, notification.OutcomeFailed, result.Outcome)
	// The rest of the text is the operating system's: Windows does not say
	// "connection refused".
	assert.Contains(t, result.Reason, "dial tcp [REDACTED]: ")
	assert.NotContains(t, result.Reason, pagerDutyTestKey)
}
