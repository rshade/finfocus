package notification_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
)

const responseSecret = "body-with-sk_live_123"

func failingServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func assertNoSecrets(t *testing.T, reason string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		assert.NotContains(t, reason, secret)
	}
}

func TestDeliverHTTP500(t *testing.T) {
	t.Parallel()

	srv := failingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(responseSecret))
	})
	for _, typ := range []config.NotificationType{config.NotificationTypeSlack, config.NotificationTypeWebhook} {
		dest := config.NotificationDestination{Type: typ, URL: srv.URL + "/secret-path"}
		if typ == config.NotificationTypeWebhook {
			dest.Headers = map[string]string{"Authorization": "Bearer header-secret"}
		}
		result := deliverOne(t, srv, dest,
			notification.BudgetAlertInput{Amount: 100, Threshold: 80, AlertType: config.AlertTypeActual}, nil)
		assert.Equal(t, notification.OutcomeFailed, result.Outcome)
		assert.Contains(t, result.Reason, "status 500")
		assertNoSecrets(t, result.Reason, srv.URL, "secret-path", "header-secret", responseSecret)
	}
}

func TestDeliverRedirectIsNotFollowed(t *testing.T) {
	t.Parallel()

	var redirectHits atomic.Int32
	target := failingServer(t, func(w http.ResponseWriter, _ *http.Request) {
		redirectHits.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	srv := failingServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/elsewhere", http.StatusFound)
	})

	result := deliverOne(t, srv, config.NotificationDestination{Type: config.NotificationTypeWebhook, URL: srv.URL},
		notification.BudgetAlertInput{Amount: 100, Threshold: 80, AlertType: config.AlertTypeActual}, nil)
	assert.Equal(t, notification.OutcomeFailed, result.Outcome)
	assert.Contains(t, result.Reason, "status 302")
	assert.Zero(t, redirectHits.Load())
}

func TestDeliverTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := failingServer(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	t.Cleanup(func() { close(release) })

	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_HOOK": srv.URL + "/hook-secret"}}
	d := notification.NewDispatcher(srv.Client(), lookup.lookup)
	start := time.Now()
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{Type: config.NotificationTypeSlack, URL: "${FINFOCUS_NOTIFY_HOOK}"},
		Event:       testEvent(80),
	}}, notification.Options{Timeout: 100 * time.Millisecond})
	elapsed := time.Since(start)

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.Contains(t, results[0].Reason, "timed out after 100ms")
	assert.Less(t, elapsed, 5*time.Second)
	assertNoSecrets(t, results[0].Reason, srv.URL, "hook-secret")
}

func TestDeliverConnectionErrorIsRedacted(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.NotFoundHandler())
	client := srv.Client()
	closedURL := srv.URL + "/services/T000/B000/conn-secret"
	srv.Close()

	d := notification.NewDispatcher(client, nil)
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{Type: config.NotificationTypeSlack, URL: closedURL},
		Event:       testEvent(80),
	}}, notification.Options{Timeout: 2 * time.Second})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.NotEmpty(t, results[0].Reason)
	assertNoSecrets(t, results[0].Reason, closedURL, "conn-secret")
}

func TestDeliverCanceledContext(t *testing.T) {
	t.Parallel()

	srv := failingServer(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := notification.NewDispatcher(srv.Client(), nil)
	results := d.Deliver(ctx, []notification.Delivery{{
		Destination: config.NotificationDestination{Type: config.NotificationTypeWebhook, URL: srv.URL},
		Event:       testEvent(80),
	}}, notification.Options{})
	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.Equal(t, "canceled", results[0].Reason)
}

func TestNewHTTPClientDoesNotFollowRedirects(t *testing.T) {
	t.Parallel()

	client := notification.NewHTTPClient()
	require.NotNil(t, client.CheckRedirect)
	assert.ErrorIs(t, client.CheckRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestDeliverRedactsVariableHostOnDNSFailure(t *testing.T) {
	t.Parallel()

	host := "budget-notify-secret-host.invalid"
	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_HOST": host}}
	d := notification.NewDispatcher(nil, lookup.lookup)
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{
			Type: config.NotificationTypeWebhook, URL: "https://${FINFOCUS_NOTIFY_HOST}/hook",
		},
		Event: testEvent(80),
	}}, notification.Options{Timeout: 5 * time.Second})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.Contains(t, results[0].Reason, notification.RedactedText)
	assert.NotContains(t, results[0].Reason, host)
	assert.NotContains(t, results[0].Reason, "budget-notify-secret-host")
}

func TestDeliverRedactsLiteralHostOnDNSFailure(t *testing.T) {
	t.Parallel()

	host := "budget-notify-literal-host.invalid"
	d := notification.NewDispatcher(nil, (&recordingLookup{}).lookup)
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{
			Type: config.NotificationTypeWebhook, URL: "https://" + host + "/hook",
		},
		Event: testEvent(80),
	}}, notification.Options{Timeout: 5 * time.Second})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.Contains(t, results[0].Reason, notification.RedactedText)
	assert.NotContains(t, results[0].Reason, "budget-notify-literal-host")
}

func TestDeliverRedactsURLUserinfoOnDNSFailure(t *testing.T) {
	t.Parallel()

	d := notification.NewDispatcher(nil, (&recordingLookup{}).lookup)
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{
			Type: config.NotificationTypeWebhook,
			URL:  "https://hookuser:hookpass@budget-notify-userinfo.invalid/secret-path",
		},
		Event: testEvent(80),
	}}, notification.Options{Timeout: 5 * time.Second})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	for _, secret := range []string{"hookuser", "hookpass", "secret-path", "budget-notify-userinfo"} {
		assert.NotContains(t, results[0].Reason, secret)
	}
}

// headerEchoSender fails with an error that quotes every resolved header value,
// to show which values the dispatcher redacts.
type headerEchoSender struct{}

func (headerEchoSender) Send(_ context.Context, dest notification.Resolved, _ notification.BudgetAlertEvent) error {
	return fmt.Errorf("x-version=%s auth=%s x-api-key=%s x-trace=%s x-ref=%s count 1",
		dest.Headers["X-Version"], dest.Headers["Authorization"], dest.Headers["X-Api-Key"],
		dest.Headers["X-Trace"], dest.Headers["X-Ref"])
}

func TestDeliverRedactsOnlySecretLookingHeaders(t *testing.T) {
	t.Parallel()

	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_REF": "r1"}}
	d := notification.NewDispatcher(nil, lookup.lookup)
	d.RegisterForTest(fakeType, headerEchoSender{})
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{
			Type: fakeType, URL: "https://a.example",
			Headers: map[string]string{
				"X-Version":     "1",
				"Authorization": "abc",
				"X-Api-Key":     "k9",
				"X-Trace":       "trace-id-12345",
				"X-Ref":         "${FINFOCUS_NOTIFY_REF}",
			},
		},
		Event: testEvent(80),
	}}, notification.Options{})

	require.Len(t, results, 1)
	reason := results[0].Reason
	assert.Contains(t, reason, "x-version=1 ")
	assert.Contains(t, reason, "count 1")
	for _, secret := range []string{"abc", "k9", "trace-id-12345", "r1"} {
		assert.NotContains(t, reason, secret)
	}
	assert.Equal(t, 4, strings.Count(reason, notification.RedactedText), reason)
}
