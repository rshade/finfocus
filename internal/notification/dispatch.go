package notification

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/config"
)

// DefaultTimeout bounds each destination's delivery.
const DefaultTimeout = 10 * time.Second

// Outcome is the result of one delivery.
type Outcome string

// Delivery outcomes.
const (
	// OutcomeSent means the destination answered with a 2xx status.
	OutcomeSent Outcome = "sent"
	// OutcomeSkipped means nothing was sent: an unresolved or disallowed variable,
	// a project destination with a reference, or a non-HTTPS URL after expansion.
	OutcomeSkipped Outcome = "skipped"
	// OutcomeFailed means the request failed, timed out, or returned a non-2xx status.
	OutcomeFailed Outcome = "failed"
	// OutcomeDryRun means the run was a dry run and nothing was sent.
	OutcomeDryRun Outcome = "dry-run"
)

const reasonProjectVariable = "project config cannot use variables; move this destination to the global config"

// Resolved is a destination with its variables expanded, ready to send.
type Resolved struct {
	Type    config.NotificationType
	URL     string
	Channel string
	Method  string
	Headers map[string]string
}

// Sender delivers an event to one resolved destination.
type Sender interface {
	Send(ctx context.Context, dest Resolved, event BudgetAlertEvent) error
}

// Delivery pairs one destination with the event to send to it.
type Delivery struct {
	Destination config.NotificationDestination
	Event       BudgetAlertEvent
}

// DeliveryResult is the outcome of one Delivery. Reason never contains a
// resolved URL, header value, or response body.
type DeliveryResult struct {
	Type             config.NotificationType
	Scope            string
	ScopeKey         string
	ThresholdPercent float64
	AlertType        config.AlertType
	Outcome          Outcome
	Reason           string
}

// Options controls one Deliver call.
type Options struct {
	// DryRun resolves destinations but sends nothing.
	DryRun bool
	// Timeout bounds each destination. Zero means DefaultTimeout.
	Timeout time.Duration
}

// Dispatcher sends deliveries to their destination types' senders.
type Dispatcher struct {
	lookup  func(string) (string, bool)
	senders map[config.NotificationType]Sender
}

// NewDispatcher returns a Dispatcher with the Slack and webhook senders
// registered. client is copied and never follows redirects; nil uses a new
// client. lookup resolves ${FINFOCUS_NOTIFY_*} variables; nil resolves none.
func NewDispatcher(client *http.Client, lookup func(string) (string, bool)) *Dispatcher {
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	httpClient := noRedirectClient(client)
	d := &Dispatcher{lookup: lookup, senders: map[config.NotificationType]Sender{}}
	d.register(config.NotificationTypeSlack, &slackSender{client: httpClient})
	d.register(config.NotificationTypeWebhook, &webhookSender{client: httpClient})
	return d
}

// register sets the sender for a destination type, replacing any existing one.
func (d *Dispatcher) register(notificationType config.NotificationType, sender Sender) {
	d.senders[notificationType] = sender
}

// Deliver sends every delivery concurrently, each bounded by opts.Timeout, and
// returns one result per delivery in input order.
func (d *Dispatcher) Deliver(ctx context.Context, deliveries []Delivery, opts Options) []DeliveryResult {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	results := make([]DeliveryResult, len(deliveries))
	var wg sync.WaitGroup
	for i := range deliveries {
		wg.Go(func() {
			results[i] = d.deliverOne(ctx, deliveries[i], opts.DryRun, timeout)
		})
	}
	wg.Wait()
	return results
}

func (d *Dispatcher) deliverOne(
	ctx context.Context,
	delivery Delivery,
	dryRun bool,
	timeout time.Duration,
) DeliveryResult {
	dest := delivery.Destination
	result := DeliveryResult{
		Type:             dest.Type,
		Scope:            delivery.Event.Scope,
		ScopeKey:         delivery.Event.ScopeKey,
		ThresholdPercent: delivery.Event.ThresholdPercent,
		AlertType:        delivery.Event.AlertType,
	}

	sender, ok := d.senders[dest.Type]
	if !ok {
		return withOutcome(result, OutcomeFailed, "unsupported destination type")
	}
	if dest.FromProject() && config.DestinationHasReference(dest) {
		return withOutcome(result, OutcomeSkipped, reasonProjectVariable)
	}

	redactor := &Redactor{}
	resolved, err := d.resolve(dest, redactor)
	if err != nil {
		return withOutcome(result, OutcomeSkipped, redactor.Redact(err.Error()))
	}
	if !config.IsHTTPSURL(resolved.URL) {
		return withOutcome(result, OutcomeSkipped, "url is not HTTPS after expansion")
	}
	if dryRun {
		return withOutcome(result, OutcomeDryRun, "")
	}

	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if sendErr := sender.Send(sendCtx, resolved, delivery.Event); sendErr != nil {
		return withOutcome(result, OutcomeFailed, redactor.Redact(failureReason(sendErr, timeout)))
	}
	return withOutcome(result, OutcomeSent, "")
}

// resolve expands dest's variables. The URL, its host (network errors name the
// host alone), every variable value, and every value built from a variable are
// registered with redactor. A literal header
// value is registered only when it looks like a secret (see secretHeader), so
// a short value such as "1" does not redact unrelated text.
func (d *Dispatcher) resolve(dest config.NotificationDestination, redactor *Redactor) (Resolved, error) {
	lookup := func(name string) (string, bool) {
		value, ok := d.lookup(name)
		if ok {
			redactor.Add(value)
		}
		return value, ok
	}

	redactor.Add(dest.URL)
	resolvedURL, err := Expand(dest.URL, lookup)
	if err != nil {
		return Resolved{}, fmt.Errorf("url: %w", err)
	}
	redactor.Add(resolvedURL)
	if parsed, parseErr := url.Parse(resolvedURL); parseErr == nil {
		redactor.Add(parsed.Host)
		redactor.Add(parsed.Hostname())
	}

	headers := make(map[string]string, len(dest.Headers))
	names := make([]string, 0, len(dest.Headers))
	for name := range maps.Keys(dest.Headers) {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		raw := dest.Headers[name]
		value, expandErr := Expand(raw, lookup)
		if expandErr != nil {
			return Resolved{}, fmt.Errorf("header %s: %w", name, expandErr)
		}
		if config.HasNotificationReference(raw) || secretHeader(name, raw) {
			redactor.Add(raw)
			redactor.Add(value)
		}
		headers[name] = value
	}

	return Resolved{
		Type:    dest.Type,
		URL:     resolvedURL,
		Channel: dest.Channel,
		Method:  strings.ToUpper(dest.Method),
		Headers: headers,
	}, nil
}

// minSecretHeaderLength is the shortest literal header value treated as a
// secret regardless of the header name.
const minSecretHeaderLength = 8

// secretHeader reports whether a literal header value should be redacted: it is
// long enough to be a credential, or the header name says it carries one.
func secretHeader(name, value string) bool {
	if len(value) >= minSecretHeaderLength {
		return true
	}
	lower := strings.ToLower(name)
	switch lower {
	case "authorization", "proxy-authorization", "cookie":
		return true
	}
	for _, marker := range []string{"token", "key", "secret"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func withOutcome(result DeliveryResult, outcome Outcome, reason string) DeliveryResult {
	result.Outcome = outcome
	result.Reason = reason
	return result
}

func failureReason(err error, timeout time.Duration) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("timed out after %s", timeout)
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return err.Error()
}
