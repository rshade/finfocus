package cli

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/notification"
)

const (
	notifyFlag      = "notify"
	notifyEnvVar    = "FINFOCUS_NOTIFY"
	notifyFlagUsage = "Send budget alert notifications for exceeded thresholds " +
		"(default from " + notifyEnvVar + ")"
)

// notificationClient is replaced in tests so deliveries reach an httptest TLS
// server. Nil uses a new client. The dispatcher never follows redirects.
var notificationClient *http.Client //nolint:gochecknoglobals // test seam, set only from export_test.go

// exceededAlert is one exceeded threshold with the destinations to notify.
type exceededAlert struct {
	Event        notification.BudgetAlertEvent
	Destinations []config.NotificationDestination
}

// notifyBudgetAlerts sends notifications for every exceeded alert that has
// destinations, when the run opted in. It writes only to stderr and never
// returns an error, so the command's output and exit code are unchanged.
func notifyBudgetAlerts(cmd *cobra.Command, result *BudgetRenderResult, currency string) {
	optedIn, warning := resolveNotifyOptIn(cmd, os.LookupEnv)
	if warning != "" {
		cmd.PrintErrln(warning)
	}
	alerts := collectExceededAlerts(result, currency, time.Now())
	if len(alerts) == 0 {
		return
	}
	if !optedIn {
		cmd.PrintErrf("budget notifications for %d exceeded threshold(s) were not sent; "+
			"pass --%s or set %s=true\n", len(alerts), notifyFlag, notifyEnvVar)
		return
	}

	ctx := commandContext(cmd)
	dispatcher := notification.NewDispatcher(notificationClient, os.LookupEnv)
	results := dispatcher.Deliver(ctx, deliveriesFor(alerts), notification.Options{
		DryRun:  dryRunRequested(cmd),
		Timeout: notification.DefaultTimeout,
	})
	reportDeliveryResults(cmd, results)
}

// validateNotifyConfig runs the cost pre-run configuration check for commands
// outside the cost group (overview) when the run opted in to notifications, so
// an invalid or unsafe destination fails before anything is sent.
func validateNotifyConfig(cmd *cobra.Command) error {
	optedIn, _ := resolveNotifyOptIn(cmd, os.LookupEnv)
	if !optedIn {
		return nil
	}
	return validateCostConfig(cmd, costConfigPaths(config.GetGlobalConfig()))
}

// resolveNotifyOptIn reports whether this run sends notifications. An explicit
// --notify wins; otherwise FINFOCUS_NOTIFY is parsed with [strconv.ParseBool].
// An invalid value counts as false and returns a warning that does not echo it.
func resolveNotifyOptIn(cmd *cobra.Command, lookupEnv func(string) (string, bool)) (bool, string) {
	if flag := cmd.Flag(notifyFlag); flag != nil && flag.Changed {
		value, err := strconv.ParseBool(flag.Value.String())
		return err == nil && value, ""
	}
	raw, ok := lookupEnv(notifyEnvVar)
	if !ok || raw == "" {
		return false, ""
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Sprintf("warning: ignoring %s: not a boolean value; budget notifications are off",
			notifyEnvVar)
	}
	return value, ""
}

// collectExceededAlerts returns one entry per EXCEEDED alert with destinations,
// in the order global, providers sorted, tags in priority order, types sorted.
// A scope without a currency uses the global budget's, then fallbackCurrency.
func collectExceededAlerts(result *BudgetRenderResult, fallbackCurrency string, now time.Time) []exceededAlert {
	if result == nil {
		return nil
	}
	if status := result.LegacyStatus; status != nil {
		currency := firstNonEmpty(status.Currency, status.Budget.Currency, fallbackCurrency)
		return appendExceeded(nil, status.Alerts, notification.BudgetAlertInput{
			Scope:           notification.ScopeGlobal,
			Amount:          status.Budget.Amount,
			Currency:        currency,
			Period:          status.Budget.GetPeriod(),
			CurrentSpend:    status.CurrentSpend,
			CurrentPercent:  status.Percentage,
			ForecastedSpend: status.ForecastedSpend,
			ForecastPercent: status.ForecastPercentage,
			Time:            now,
		})
	}
	if result.ScopedResult == nil {
		return nil
	}

	globalCurrency := ""
	if global := result.ScopedResult.Global; global != nil {
		globalCurrency = firstNonEmpty(global.Currency, global.Budget.Currency)
	}
	var alerts []exceededAlert
	for _, scope := range result.ScopedResult.AllScopes() {
		if scope == nil {
			continue
		}
		alerts = appendExceeded(alerts, scope.Alerts, notification.BudgetAlertInput{
			Scope:           string(scope.ScopeType),
			ScopeKey:        scope.ScopeKey,
			Amount:          scope.Budget.Amount,
			Currency:        firstNonEmpty(scope.Currency, scope.Budget.Currency, globalCurrency, fallbackCurrency),
			Period:          scope.Budget.GetPeriod(),
			CurrentSpend:    scope.CurrentSpend,
			CurrentPercent:  scope.Percentage,
			ForecastedSpend: scope.ForecastedSpend,
			ForecastPercent: scope.ForecastPercentage,
			Time:            now,
		})
	}
	return alerts
}

func appendExceeded(
	alerts []exceededAlert,
	statuses []engine.ThresholdStatus,
	base notification.BudgetAlertInput,
) []exceededAlert {
	for _, status := range statuses {
		if status.Status != engine.ThresholdStatusExceeded || len(status.Notifications) == 0 {
			continue
		}
		in := base
		in.Threshold = status.Threshold
		in.AlertType = status.Type
		alerts = append(alerts, exceededAlert{
			Event:        notification.NewBudgetAlertEvent(in),
			Destinations: status.Notifications,
		})
	}
	return alerts
}

// deliveriesFor flattens alerts into one delivery per destination, in order.
func deliveriesFor(alerts []exceededAlert) []notification.Delivery {
	var deliveries []notification.Delivery
	for _, alert := range alerts {
		for _, dest := range alert.Destinations {
			deliveries = append(deliveries, notification.Delivery{Destination: dest, Event: alert.Event})
		}
	}
	return deliveries
}

// reportDeliveryResults prints one stderr line per dry-run, failed, or skipped
// delivery and logs failures at WARN. Reasons are already redacted.
func reportDeliveryResults(cmd *cobra.Command, results []notification.DeliveryResult) {
	log := logging.FromContext(commandContext(cmd))
	for _, result := range results {
		label := notification.ScopeLabel(result.Scope, result.ScopeKey)
		threshold := strconv.FormatFloat(result.ThresholdPercent, 'f', -1, 64)
		switch result.Outcome {
		case notification.OutcomeSent:
			log.Debug().
				Str("component", "cli").
				Str("operation", "budget_notify").
				Str("destination_type", string(result.Type)).
				Str("scope", label).
				Msg("budget notification sent")
		case notification.OutcomeDryRun:
			cmd.PrintErrf("dry-run: would notify %s for %s budget, %s%% %s threshold\n",
				result.Type, label, threshold, result.AlertType)
		case notification.OutcomeFailed, notification.OutcomeSkipped:
			cmd.PrintErrf("warning: %s notification for %s budget (%s%% %s) %s: %s\n",
				result.Type, label, threshold, result.AlertType, result.Outcome, result.Reason)
			log.Warn().
				Str("component", "cli").
				Str("operation", "budget_notify").
				Str("destination_type", string(result.Type)).
				Str("scope", label).
				Str("outcome", string(result.Outcome)).
				Str("reason", result.Reason).
				Msg("budget notification not delivered")
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
