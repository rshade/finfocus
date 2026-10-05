package cli

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/notification"
)

func notifyCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(notifyFlag, false, "")
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func envLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestResolveNotifyOptIn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		env         map[string]string
		want        bool
		wantWarning bool
	}{
		{name: "nothing set", want: false},
		{name: "flag on", args: []string{"--notify"}, want: true},
		{name: "env true fills unset flag", env: map[string]string{notifyEnvVar: "true"}, want: true},
		{name: "env 1 fills unset flag", env: map[string]string{notifyEnvVar: "1"}, want: true},
		{name: "env false", env: map[string]string{notifyEnvVar: "false"}, want: false},
		{
			name: "explicit flag false beats env true",
			args: []string{"--notify=false"}, env: map[string]string{notifyEnvVar: "true"}, want: false,
		},
		{
			name: "explicit flag true beats env false",
			args: []string{"--notify"}, env: map[string]string{notifyEnvVar: "false"}, want: true,
		},
		{
			name: "invalid env warns and counts as false",
			env:  map[string]string{notifyEnvVar: "yes please"}, want: false, wantWarning: true,
		},
		{
			name: "explicit flag ignores invalid env",
			args: []string{"--notify"}, env: map[string]string{notifyEnvVar: "yes please"}, want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, warning := resolveNotifyOptIn(notifyCmd(t, tc.args...), envLookup(tc.env))
			assert.Equal(t, tc.want, got)
			if tc.wantWarning {
				assert.Contains(t, warning, notifyEnvVar)
				assert.NotContains(t, warning, "yes please")
				return
			}
			assert.Empty(t, warning)
		})
	}
}

func TestResolveNotifyOptInWithoutFlag(t *testing.T) {
	t.Parallel()

	got, warning := resolveNotifyOptIn(&cobra.Command{}, envLookup(map[string]string{notifyEnvVar: "true"}))
	assert.True(t, got)
	assert.Empty(t, warning)
}

func dest(url string) []config.NotificationDestination {
	return []config.NotificationDestination{{Type: config.NotificationTypeSlack, URL: url}}
}

func TestCollectExceededAlertsLegacy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)
	status := &engine.BudgetStatus{
		Budget:             config.BudgetConfig{Amount: 100, Currency: "USD"},
		CurrentSpend:       105,
		Percentage:         105,
		ForecastedSpend:    150,
		ForecastPercentage: 150,
		Currency:           "USD",
		Alerts: []engine.ThresholdStatus{
			{Threshold: 50, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded},
			{
				Threshold: 80, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded,
				Notifications: dest("https://a.example"),
			},
			{
				Threshold: 100, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded,
				Notifications: append(dest("https://a.example"), dest("https://b.example")...),
			},
			{
				Threshold: 110, Type: config.AlertTypeActual, Status: engine.ThresholdStatusApproaching,
				Notifications: dest("https://c.example"),
			},
			{
				Threshold: 200, Type: config.AlertTypeForecasted, Status: engine.ThresholdStatusOK,
				Notifications: dest("https://d.example"),
			},
		},
	}

	alerts := collectExceededAlerts(&BudgetRenderResult{LegacyStatus: status}, "USD", now)
	require.Len(t, alerts, 2)
	assert.InDelta(t, 80.0, alerts[0].Event.ThresholdPercent, 1e-9)
	assert.InDelta(t, 100.0, alerts[1].Event.ThresholdPercent, 1e-9)
	assert.Equal(t, notification.ScopeGlobal, alerts[0].Event.Scope)
	assert.InDelta(t, 105.0, alerts[0].Event.Spend, 1e-9)
	assert.Equal(t, "USD", alerts[0].Event.Currency)

	deliveries := deliveriesFor(alerts)
	require.Len(t, deliveries, 3)
	assert.Equal(t, "https://a.example", deliveries[0].Destination.URL)
	assert.Equal(t, "https://a.example", deliveries[1].Destination.URL)
	assert.Equal(t, "https://b.example", deliveries[2].Destination.URL)
}

func scopedStatus(
	scope engine.ScopeType,
	key string,
	currency string,
	alerts ...engine.ThresholdStatus,
) *engine.ScopedBudgetStatus {
	return &engine.ScopedBudgetStatus{
		ScopeType:    scope,
		ScopeKey:     key,
		Budget:       config.ScopedBudget{Amount: 100, Currency: currency},
		CurrentSpend: 90,
		Percentage:   90,
		Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
		Alerts:       alerts,
		Currency:     currency,
	}
}

func exceeded(url string) engine.ThresholdStatus {
	return engine.ThresholdStatus{
		Threshold: 80, Type: config.AlertTypeActual, Status: engine.ThresholdStatusExceeded,
		Notifications: dest(url),
	}
}

func TestCollectExceededAlertsScopedOrder(t *testing.T) {
	t.Parallel()

	result := &engine.ScopedBudgetResult{
		Global: scopedStatus(engine.ScopeTypeGlobal, "", "USD", exceeded("https://global.example")),
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"gcp": scopedStatus(engine.ScopeTypeProvider, "gcp", "", exceeded("https://gcp.example")),
			"aws": scopedStatus(engine.ScopeTypeProvider, "aws", "", exceeded("https://aws.example")),
		},
		ByTag: []*engine.ScopedBudgetStatus{
			scopedStatus(engine.ScopeTypeTag, "team:b", "", exceeded("https://tag-b.example")),
			scopedStatus(engine.ScopeTypeTag, "team:a", "", exceeded("https://tag-a.example")),
		},
		ByType: map[string]*engine.ScopedBudgetStatus{
			"aws:s3/bucket": scopedStatus(engine.ScopeTypeType, "aws:s3/bucket", "",
				exceeded("https://s3.example"),
				engine.ThresholdStatus{Threshold: 95, Type: config.AlertTypeActual, Status: engine.ThresholdStatusOK,
					Notifications: dest("https://never.example")}),
			"aws:ec2/instance": scopedStatus(engine.ScopeTypeType, "aws:ec2/instance", "",
				exceeded("https://ec2.example")),
		},
	}

	alerts := collectExceededAlerts(&BudgetRenderResult{ScopedResult: result}, "EUR", time.Now())
	urls := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		urls = append(urls, alert.Destinations[0].URL)
	}
	assert.Equal(t, []string{
		"https://global.example",
		"https://aws.example",
		"https://gcp.example",
		"https://tag-b.example",
		"https://tag-a.example",
		"https://ec2.example",
		"https://s3.example",
	}, urls)
	assert.Equal(t, "USD", alerts[0].Event.Currency)
	assert.Equal(t, "provider", alerts[1].Event.Scope)
	assert.Equal(t, "aws", alerts[1].Event.ScopeKey)
	assert.Equal(t, "USD", alerts[1].Event.Currency, "scoped budgets inherit the global currency")
	assert.Equal(t, "tag", alerts[3].Event.Scope)
	assert.Equal(t, "type", alerts[5].Event.Scope)
}

func TestCollectExceededAlertsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, collectExceededAlerts(nil, "USD", time.Now()))
	assert.Empty(t, collectExceededAlerts(&BudgetRenderResult{}, "USD", time.Now()))
	assert.Empty(t, collectExceededAlerts(&BudgetRenderResult{ScopedResult: &engine.ScopedBudgetResult{
		ByProvider: map[string]*engine.ScopedBudgetStatus{"aws": nil},
	}}, "USD", time.Now()))

	noGlobalCurrency := &engine.ScopedBudgetResult{
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"aws": scopedStatus(engine.ScopeTypeProvider, "aws", "", exceeded("https://aws.example")),
		},
	}
	alerts := collectExceededAlerts(&BudgetRenderResult{ScopedResult: noGlobalCurrency}, "EUR", time.Now())
	require.Len(t, alerts, 1)
	assert.Equal(t, "EUR", alerts[0].Event.Currency)
}
