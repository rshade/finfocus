package notification

import (
	"context"
	"fmt"
	"math"
	"net/http"

	"github.com/rshade/finfocus/internal/config"
)

const slackMarkdown = "mrkdwn"

type slackSender struct {
	client *http.Client
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackMessage struct {
	Channel string       `json:"channel,omitempty"`
	Text    string       `json:"text"`
	Blocks  []slackBlock `json:"blocks"`
}

// Send posts the alert to a Slack incoming webhook.
func (s *slackSender) Send(ctx context.Context, dest Resolved, event BudgetAlertEvent) error {
	return sendJSON(ctx, s.client, http.MethodPost, dest.URL, nil, newSlackMessage(dest.Channel, event))
}

func newSlackMessage(channel string, event BudgetAlertEvent) slackMessage {
	label := ScopeLabel(event.Scope, event.ScopeKey)
	threshold := fmt.Sprintf("%s%% %s", formatPercent(event.ThresholdPercent), event.AlertType)
	spendLabel := "Current spend"
	if event.AlertType == config.AlertTypeForecasted {
		spendLabel = "Forecasted spend"
	}
	return slackMessage{
		Channel: channel,
		Text:    fmt.Sprintf("Budget alert: %s budget crossed its %s threshold", label, threshold),
		Blocks: []slackBlock{
			{Type: "header", Text: &slackText{Type: "plain_text", Text: "Budget alert"}},
			{Type: "section", Fields: []slackText{
				{Type: slackMarkdown, Text: fmt.Sprintf("*Budget:*\n%s, %s/%s",
					label, formatMoney(event.Amount, event.Currency), periodUnit(event.Period))},
				{Type: slackMarkdown, Text: fmt.Sprintf("*%s:*\n%s (%s%%)",
					spendLabel, formatMoney(event.Spend, event.Currency), formatPercent(event.SpendPercent))},
				{Type: slackMarkdown, Text: "*Threshold:*\n" + threshold},
				{Type: slackMarkdown, Text: "*Status:*\nExceeded"},
			}},
		},
	}
}

// ScopeLabel names a budget for people: "global", "provider aws", "tag team:platform".
func ScopeLabel(scope, scopeKey string) string {
	if scope == ScopeGlobal || scopeKey == "" {
		return scope
	}
	return scope + " " + scopeKey
}

func periodUnit(period string) string {
	if period == config.DefaultBudgetPeriod {
		return "month"
	}
	return period
}

func formatMoney(amount float64, currency string) string {
	switch currency {
	case "USD", "":
		return fmt.Sprintf("$%.2f", amount)
	case "EUR":
		return fmt.Sprintf("€%.2f", amount)
	case "GBP":
		return fmt.Sprintf("£%.2f", amount)
	case "JPY":
		return fmt.Sprintf("¥%.2f", amount)
	default:
		return fmt.Sprintf("%s %.2f", currency, amount)
	}
}

// formatPercent prints a whole number when the value is integral, otherwise one decimal.
func formatPercent(value float64) string {
	if value == math.Trunc(value) {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}
