package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestNotificationDestinationValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dest      config.NotificationDestination
		wantErr   error
		wantField string
		contains  string
	}{
		{
			name: "slack accepted",
			dest: config.NotificationDestination{Type: "slack", URL: "https://hooks.slack.com/services/T/B/X"},
		},
		{
			name: "slack with channel accepted",
			dest: config.NotificationDestination{
				Type: "slack", URL: "https://hooks.slack.com/services/T/B/X", Channel: "#finops",
			},
		},
		{
			name: "webhook accepted",
			dest: config.NotificationDestination{
				Type:    "webhook",
				URL:     "https://api.example.com/hook",
				Method:  "post",
				Headers: map[string]string{"Authorization": "Bearer ${FINFOCUS_NOTIFY_TOKEN}"},
			},
		},
		{
			name: "webhook put any case",
			dest: config.NotificationDestination{Type: "webhook", URL: "https://api.example.com", Method: "Put"},
		},
		{
			name:      "unknown type lists supported types",
			dest:      config.NotificationDestination{Type: "email", URL: "https://example.com"},
			wantErr:   config.ErrNotificationTypeInvalid,
			wantField: "type",
			contains:  "slack, webhook",
		},
		{
			name:      "empty url",
			dest:      config.NotificationDestination{Type: "slack"},
			wantErr:   config.ErrNotificationURLRequired,
			wantField: "url",
		},
		{
			name:      "http url",
			dest:      config.NotificationDestination{Type: "webhook", URL: "http://api.example.com/hook"},
			wantErr:   config.ErrNotificationHTTPSRequired,
			wantField: "url",
			contains:  "HTTPS is required",
		},
		{
			name:      "hostless url",
			dest:      config.NotificationDestination{Type: "webhook", URL: "https:///path"},
			wantErr:   config.ErrNotificationHTTPSRequired,
			wantField: "url",
		},
		{
			name: "reference skips scheme check",
			dest: config.NotificationDestination{Type: "slack", URL: "${FINFOCUS_NOTIFY_SLACK_URL}"},
		},
		{
			name:      "invalid method",
			dest:      config.NotificationDestination{Type: "webhook", URL: "https://api.example.com", Method: "GET"},
			wantErr:   config.ErrNotificationMethodInvalid,
			wantField: "method",
		},
		{
			name: "channel on webhook",
			dest: config.NotificationDestination{
				Type: "webhook", URL: "https://api.example.com", Channel: "#finops",
			},
			wantErr:   config.ErrNotificationFieldNotAllowed,
			wantField: "channel",
			contains:  "slack",
		},
		{
			name:      "method on slack",
			dest:      config.NotificationDestination{Type: "slack", URL: "https://hooks.slack.com/x", Method: "POST"},
			wantErr:   config.ErrNotificationFieldNotAllowed,
			wantField: "method",
			contains:  "webhook",
		},
		{
			name: "headers on slack",
			dest: config.NotificationDestination{
				Type: "slack", URL: "https://hooks.slack.com/x", Headers: map[string]string{"X": "y"},
			},
			wantErr:   config.ErrNotificationFieldNotAllowed,
			wantField: "headers",
		},
		{
			name: "empty header name",
			dest: config.NotificationDestination{
				Type: "webhook", URL: "https://api.example.com", Headers: map[string]string{"": "y"},
			},
			wantErr:   config.ErrNotificationHeaderNameEmpty,
			wantField: "headers",
		},
		{
			name: "url references a CI secret",
			dest: config.NotificationDestination{
				Type: "webhook",
				URL:  "https://evil.example/?k=${AWS_SECRET_ACCESS_KEY}",
			},
			wantErr:   config.ErrNotificationVariableNotAllowed,
			wantField: "url",
			contains:  "AWS_SECRET_ACCESS_KEY",
		},
		{
			name: "header references GITHUB_TOKEN",
			dest: config.NotificationDestination{
				Type:    "webhook",
				URL:     "https://api.example.com",
				Headers: map[string]string{"Authorization": "${GITHUB_TOKEN}"},
			},
			wantErr:   config.ErrNotificationVariableNotAllowed,
			wantField: "headers.Authorization",
			contains:  "GITHUB_TOKEN",
		},
		{
			name:      "unterminated reference",
			dest:      config.NotificationDestination{Type: "slack", URL: "${FINFOCUS_NOTIFY_X"},
			wantErr:   config.ErrNotificationReferenceMalformed,
			wantField: "url",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.dest.Validate()
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorIs(t, err, tc.wantErr)
			var fieldErr *config.NotificationFieldError
			require.ErrorAs(t, err, &fieldErr)
			assert.Equal(t, tc.wantField, fieldErr.Field)
			if tc.contains != "" {
				assert.Contains(t, err.Error(), tc.contains)
			}
		})
	}
}

func TestNotificationDestinationValidateNeverEchoesValues(t *testing.T) {
	t.Parallel()

	secret := "http://hooks.example.com/T000/B000/s3cr3t"
	err := config.NotificationDestination{Type: "slack", URL: secret}.Validate()
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "s3cr3t")
}

func TestFindNotificationReferences(t *testing.T) {
	t.Parallel()

	refs, err := config.FindNotificationReferences("a${FINFOCUS_NOTIFY_A}b${OTHER}$NAME$$")
	require.NoError(t, err)
	require.Len(t, refs, 2)
	assert.Equal(t, config.NotificationReference{Start: 1, End: 21, Name: "FINFOCUS_NOTIFY_A"}, refs[0])
	assert.Equal(t, "OTHER", refs[1].Name)

	_, err = config.FindNotificationReferences("${FINFOCUS_NOTIFY_A")
	require.ErrorIs(t, err, config.ErrNotificationReferenceMalformed)

	_, err = config.FindNotificationReferences("${not a name}")
	require.ErrorIs(t, err, config.ErrNotificationReferenceMalformed)
	assert.NotContains(t, err.Error(), "not a name")

	assert.True(t, config.IsNotificationVariable("FINFOCUS_NOTIFY_SLACK_URL"))
	assert.False(t, config.IsNotificationVariable("FINFOCUS_NOTIFY_"))
	assert.False(t, config.IsNotificationVariable("GITHUB_TOKEN"))
}

func TestNotificationFieldErrorUnwrap(t *testing.T) {
	t.Parallel()

	err := &config.NotificationFieldError{Field: "url", Err: config.ErrNotificationURLRequired}
	require.ErrorIs(t, err, config.ErrNotificationURLRequired)
	assert.Equal(t, "url: notification url is required", err.Error())
}

func TestDestinationHasReference(t *testing.T) {
	t.Parallel()

	assert.False(t, config.DestinationHasReference(config.NotificationDestination{URL: "https://a.example"}))
	assert.True(t, config.DestinationHasReference(config.NotificationDestination{URL: "${FINFOCUS_NOTIFY_A}"}))
	assert.True(t, config.DestinationHasReference(config.NotificationDestination{
		URL: "https://a.example", Headers: map[string]string{"X": "Bearer ${FINFOCUS_NOTIFY_T}"},
	}))
	assert.False(t, config.DestinationHasReference(config.NotificationDestination{
		URL: "https://a.example", Headers: map[string]string{"X": "literal"},
	}))
}

func TestMaskedForDisplay(t *testing.T) {
	t.Parallel()

	alerts := func(url string) []config.AlertConfig {
		return []config.AlertConfig{{
			Threshold: 80, Type: config.AlertTypeActual,
			Notifications: []config.NotificationDestination{{
				Type: config.NotificationTypeWebhook, URL: url,
				Headers: map[string]string{
					"Authorization": "Bearer literal-secret",
					"X-Ref":         "${FINFOCUS_NOTIFY_TOKEN}",
					"X-Mixed":       "Bearer ${FINFOCUS_NOTIFY_TOKEN}",
				},
			}},
		}}
	}
	cfg := &config.Config{Cost: config.CostConfig{Budgets: &config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 100, Currency: "USD", Alerts: alerts("https://g.example/secret")},
		Providers: map[string]*config.ScopedBudget{
			"aws": {Amount: 5, Alerts: alerts("${FINFOCUS_NOTIFY_AWS}")},
			"gcp": nil,
		},
		Tags: []config.TagBudget{
			{
				Selector:     "team:x",
				ScopedBudget: config.ScopedBudget{Amount: 5, Alerts: alerts("https://t.example/secret")},
			},
		},
		Types: map[string]*config.ScopedBudget{"aws:s3/bucket": {Amount: 5, Alerts: alerts("https://s3.example/x")}},
	}}}

	masked := cfg.MaskedForDisplay()
	budgets := masked.Cost.Budgets
	assertMasked := func(dest config.NotificationDestination, wantURL string) {
		assert.Equal(t, wantURL, dest.URL)
		assert.Equal(t, config.RedactedValue, dest.Headers["Authorization"])
		assert.Equal(t, "${FINFOCUS_NOTIFY_TOKEN}", dest.Headers["X-Ref"])
		assert.Equal(t, config.RedactedValue, dest.Headers["X-Mixed"])
	}
	assertMasked(budgets.Global.Alerts[0].Notifications[0], config.RedactedValue)
	assertMasked(budgets.Providers["aws"].Alerts[0].Notifications[0], "${FINFOCUS_NOTIFY_AWS}")
	assert.Nil(t, budgets.Providers["gcp"])
	assertMasked(budgets.Tags[0].Alerts[0].Notifications[0], config.RedactedValue)
	assertMasked(budgets.Types["aws:s3/bucket"].Alerts[0].Notifications[0], config.RedactedValue)

	original := cfg.Cost.Budgets.Global.Alerts[0].Notifications[0]
	assert.Equal(t, "https://g.example/secret", original.URL)
	assert.Equal(t, "Bearer literal-secret", original.Headers["Authorization"])
	assert.Equal(t, "https://t.example/secret", cfg.Cost.Budgets.Tags[0].Alerts[0].Notifications[0].URL)

	assert.Nil(t, (&config.Config{}).MaskedForDisplay().Cost.Budgets)
}

func TestNotificationReferenceHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, config.IsSingleNotificationReference("${FINFOCUS_NOTIFY_URL}"))
	assert.True(t, config.IsSingleNotificationReference("${GITHUB_TOKEN}"))
	assert.False(t, config.IsSingleNotificationReference("Bearer ${FINFOCUS_NOTIFY_TOKEN}"))
	assert.False(t, config.IsSingleNotificationReference("${FINFOCUS_NOTIFY_A}${FINFOCUS_NOTIFY_B}"))
	assert.False(t, config.IsSingleNotificationReference("https://example.com"))
	assert.False(t, config.IsSingleNotificationReference("${FINFOCUS_NOTIFY_A"))

	assert.True(t, config.HasNotificationReference("x${"))
	assert.True(t, config.HasNotificationReference("Bearer ${FINFOCUS_NOTIFY_TOKEN}"))
	assert.False(t, config.HasNotificationReference("$NAME"))
}
