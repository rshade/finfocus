package notification

import (
	"context"
	"net/http"
	"time"
)

// EventBudgetThresholdExceeded is the event name sent to webhook destinations.
const EventBudgetThresholdExceeded = "budget.threshold.exceeded"

const eventSource = "finfocus"

type webhookSender struct {
	client *http.Client
}

type webhookBudget struct {
	Name     string  `json:"name"`
	Scope    string  `json:"scope"`
	ScopeKey string  `json:"scope_key,omitempty"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Period   string  `json:"period"`
}

type webhookThreshold struct {
	Percentage float64 `json:"percentage"`
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
}

type webhookCurrent struct {
	Spend      float64 `json:"spend"`
	Percentage float64 `json:"percentage"`
}

type webhookMetadata struct {
	Source  string `json:"source"`
	Version string `json:"version"`
}

type webhookEvent struct {
	Event     string           `json:"event"`
	Timestamp string           `json:"timestamp"`
	Budget    webhookBudget    `json:"budget"`
	Threshold webhookThreshold `json:"threshold"`
	Current   webhookCurrent   `json:"current"`
	Metadata  webhookMetadata  `json:"metadata"`
}

// Send delivers the budget.threshold.exceeded event with the configured
// method (POST by default) and headers.
func (s *webhookSender) Send(ctx context.Context, dest Resolved, event BudgetAlertEvent) error {
	method := dest.Method
	if method == "" {
		method = http.MethodPost
	}
	return sendJSON(ctx, s.client, method, dest.URL, dest.Headers, newWebhookEvent(event))
}

func newWebhookEvent(event BudgetAlertEvent) webhookEvent {
	return webhookEvent{
		Event:     EventBudgetThresholdExceeded,
		Timestamp: event.Timestamp.Format(time.RFC3339),
		Budget: webhookBudget{
			Name:     event.BudgetName(),
			Scope:    event.Scope,
			ScopeKey: event.ScopeKey,
			Amount:   event.Amount,
			Currency: event.Currency,
			Period:   event.Period,
		},
		Threshold: webhookThreshold{
			Percentage: event.ThresholdPercent,
			Type:       string(event.AlertType),
			Value:      event.ThresholdValue,
		},
		Current: webhookCurrent{
			Spend:      event.Spend,
			Percentage: event.SpendPercent,
		},
		Metadata: webhookMetadata{
			Source:  eventSource,
			Version: event.Version,
		},
	}
}
