package notification

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rshade/finfocus/internal/config"
)

const (
	pagerDutySummaryLimit    = 1024
	pagerDutyKeyLimit        = 255
	pagerDutyDiagnosticLimit = 512
)

type pagerDutySender struct {
	client *http.Client
}

type pagerDutyEvent struct {
	RoutingKey  string           `json:"routing_key"`
	EventAction string           `json:"event_action"`
	DedupKey    string           `json:"dedup_key,omitempty"`
	Payload     pagerDutyPayload `json:"payload"`
	Client      string           `json:"client"`
}

type pagerDutyPayload struct {
	Summary       string           `json:"summary"`
	Source        string           `json:"source"`
	Severity      string           `json:"severity"`
	Component     string           `json:"component"`
	Group         string           `json:"group,omitempty"`
	Class         string           `json:"class"`
	Timestamp     string           `json:"timestamp"`
	CustomDetails pagerDutyDetails `json:"custom_details"`
}

type pagerDutyDetails struct {
	Budget    webhookBudget    `json:"budget"`
	Threshold webhookThreshold `json:"threshold"`
	Current   webhookCurrent   `json:"current"`
	Version   string           `json:"finfocus_version"`
}

func (s *pagerDutySender) Send(ctx context.Context, dest Resolved, event BudgetAlertEvent) error {
	resp, err := requestJSON(ctx, s.client, http.MethodPost, dest.URL, nil, newPagerDutyEvent(dest, event))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		// Acceptance is final as soon as the response headers arrive.
		return nil
	}
	return pagerDutyResponseError(resp)
}

func pagerDutyResponseError(resp *http.Response) error {
	var diagnostics struct {
		Message string   `json:"message"`
		Errors  []string `json:"errors"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxDrainBytes))
	if err := decoder.Decode(&diagnostics); err == nil {
		var parts []string
		for _, field := range append([]string{diagnostics.Message}, diagnostics.Errors...) {
			if strings.TrimSpace(field) != "" {
				parts = append(parts, field)
			}
		}
		if len(parts) != 0 {
			return fmt.Errorf("%w: status %d: %s", ErrUnexpectedStatus, resp.StatusCode, strings.Join(parts, "; "))
		}
	}
	return fmt.Errorf("%w: status %d", ErrUnexpectedStatus, resp.StatusCode)
}

func newPagerDutyEvent(dest Resolved, event BudgetAlertEvent) pagerDutyEvent {
	scope := pagerDutyScope(event)
	facts := newWebhookEvent(event)
	return pagerDutyEvent{
		RoutingKey:  dest.RoutingKey,
		EventAction: "trigger",
		DedupKey:    pagerDutyDedupKey(dest.DedupMode, scope, event),
		Payload: pagerDutyPayload{
			Summary:   pagerDutySummary(event),
			Source:    eventSource,
			Severity:  dest.Severity,
			Component: "budget/" + scope,
			Group:     event.Project,
			Class:     facts.Event,
			Timestamp: facts.Timestamp,
			CustomDetails: pagerDutyDetails{
				Budget: facts.Budget, Threshold: facts.Threshold, Current: facts.Current, Version: event.Version,
			},
		},
		Client: "FinFocus",
	}
}

func pagerDutyScope(event BudgetAlertEvent) string {
	if event.ScopeKey == "" {
		return event.Scope
	}
	key := event.ScopeKey
	if event.Scope == ScopeTag {
		key = strings.Replace(key, ":", "=", 1)
	}
	return event.Scope + "/" + key
}

func pagerDutySummary(event BudgetAlertEvent) string {
	const prefix = "FinFocus budget alert: "
	facts := fmt.Sprintf(" budget at %s%% (%s of %s/%s), %s%% %s threshold exceeded",
		formatPercent(event.SpendPercent), formatMoney(event.Spend, event.Currency),
		formatMoney(event.Amount, event.Currency), periodUnit(event.Period),
		formatPercent(event.ThresholdPercent), event.AlertType)
	labelLimit := max(0, pagerDutySummaryLimit-len(prefix)-len(facts))
	label := truncateUTF8(ScopeLabel(event.Scope, event.ScopeKey), labelLimit)
	return truncateUTF8(prefix+label+facts, pagerDutySummaryLimit)
}

func pagerDutyDedupKey(mode, scope string, event BudgetAlertEvent) string {
	if mode == config.PagerDutyPerRun {
		return ""
	}
	// Escape delimiters so two distinct project or scope values stay distinct.
	escape := strings.NewReplacer("%", "%25", ":", "%3A")
	key := strings.Join([]string{
		eventSource, escape.Replace(event.Project), escape.Replace(scope),
		strconv.FormatFloat(event.ThresholdPercent, 'f', -1, 64), string(event.AlertType), event.BudgetMonth,
	}, ":")
	if len(key) > pagerDutyKeyLimit {
		return fmt.Sprintf("%s:sha256:%x", eventSource, sha256.Sum256([]byte(key)))
	}
	return key
}

// pagerDutyWarning bounds the already-redacted server diagnostic and puts it
// on one line without terminal control or formatting characters.
func pagerDutyWarning(reason string) string {
	words := strings.FieldsFunc(reason, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	})
	return truncateUTF8(strings.Join(words, " "), pagerDutyDiagnosticLimit)
}

func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return text[:limit]
}

func resolvePagerDuty(
	dest config.NotificationDestination, lookup func(string) (string, bool), redactor *Redactor,
) (Resolved, error) {
	redactor.Add(dest.RoutingKey)
	key, err := Expand(dest.RoutingKey, lookup)
	if err != nil {
		return Resolved{}, fmt.Errorf("routing_key: %w", err)
	}
	redactor.Add(key)
	if strings.TrimSpace(key) == "" {
		return Resolved{}, errors.New("PagerDuty routing key is required")
	}

	target := config.PagerDutyDefaultURL
	if dest.URL != "" {
		redactor.Add(dest.URL)
		var expandErr error
		target, expandErr = Expand(dest.URL, lookup)
		if expandErr != nil {
			return Resolved{}, fmt.Errorf("url: %w", expandErr)
		}
	}
	if !config.IsPagerDutyEndpoint(target) {
		return Resolved{}, config.ErrPagerDutyEndpoint
	}
	severity := dest.Severity
	if severity == "" {
		severity = config.PagerDutyDefaultSeverity
	}
	addURLSecrets(redactor, target)
	return Resolved{Type: dest.Type, URL: target, RoutingKey: key, Severity: severity, DedupMode: dest.DedupKey}, nil
}
