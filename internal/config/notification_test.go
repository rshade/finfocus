package config_test

import (
	"encoding/json"
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
			dest:      config.NotificationDestination{Type: "sms", URL: "https://example.com"},
			wantErr:   config.ErrNotificationTypeInvalid,
			wantField: "type",
			contains:  "slack, webhook, email",
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

func TestEmailDestinationValidate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fields, wantField string
	}{
		{"valid", `"smtp":{"host":"smtp.example.com"},"from":"FinOps <alerts@example.com>","to":["owner@example.com"]`, ""},
		{"loopback", `"smtp":{"host":"127.0.0.2","tls":"none","port":1025},"from":"a@example.com","to":["b@example.com"]`, ""},
		{"references", `"smtp":{"host":"${FINFOCUS_NOTIFY_SMTP_HOST}","username":"${FINFOCUS_NOTIFY_USER}"},"from":"${FINFOCUS_NOTIFY_FROM}","to":["${FINFOCUS_NOTIFY_TO}"]`, ""},
		{"missing smtp", `"from":"a@example.com","to":["b@example.com"]`, "smtp.host"},
		{"missing host", `"smtp":{},"from":"a@example.com","to":["b@example.com"]`, "smtp.host"},
		{"invalid host", `"smtp":{"host":"https://host.example/path"},"from":"a@example.com","to":["b@example.com"]`, "smtp.host"},
		{"unknown mode", `"smtp":{"host":"smtp.example.com","tls":"opportunistic"},"from":"a@example.com","to":["b@example.com"]`, "smtp.tls"},
		{"plaintext remote", `"smtp":{"host":"smtp.example.com","tls":"none"},"from":"a@example.com","to":["b@example.com"]`, "smtp.host"},
		{"zero port", `"smtp":{"host":"localhost","port":0},"from":"a@example.com","to":["b@example.com"]`, "smtp.port"},
		{"high port", `"smtp":{"host":"localhost","port":65536},"from":"a@example.com","to":["b@example.com"]`, "smtp.port"},
		{"missing from", `"smtp":{"host":"localhost"},"to":["b@example.com"]`, "from"},
		{"missing to", `"smtp":{"host":"localhost"},"from":"a@example.com"`, "to"},
		{"invalid cc", `"smtp":{"host":"localhost"},"from":"a@example.com","to":["b@example.com"],"cc":["invalid"]`, "cc[0]"},
		{"unsafe scope", `"smtp":{"host":"localhost","password":"${GITHUB_TOKEN}"},"from":"a@example.com","to":["b@example.com"]`, "smtp.password"},
		{"header newline", `"smtp":{"host":"localhost"},"from":"a@example.com","to":["b@example.com"],"subject":"bad\r\nheader"`, "subject"},
		{"foreign field", `"smtp":{"host":"localhost"},"from":"a@example.com","to":["b@example.com"],"url":"https://example.com"`, "url"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var dest config.NotificationDestination
			require.NoError(t, json.Unmarshal([]byte(`{"type":"email",`+tc.fields+`}`), &dest))
			err := dest.Validate()
			if tc.wantField == "" {
				require.NoError(t, err)
				return
			}
			var fieldErr *config.NotificationFieldError
			require.ErrorAs(t, err, &fieldErr)
			assert.Equal(t, tc.wantField, fieldErr.Field)
		})
	}
}

func TestEmailConfigMasking(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
	  {"type":"actual","threshold":80,"notifications":[{"type":"email","from":"a@example.com",
	  "to":["b@example.com"],"smtp":{"host":"localhost","username":"login-value","password":"password-value"}}]}
	]}}}}`)
	var cfg config.Config
	require.NoError(t, json.Unmarshal(raw, &cfg))

	masked, err := json.Marshal(cfg.MaskedForDisplay())
	require.NoError(t, err)
	assert.NotContains(t, string(masked), "login-value")
	assert.NotContains(t, string(masked), "password-value")
	assert.Contains(t, string(masked), "b@example.com")
	assert.Contains(t, string(masked), config.RedactedValue)
	original, err := json.Marshal(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(original), "login-value")
	assert.Contains(t, string(original), "password-value")
}

func TestEmailConfigAcrossScopes(t *testing.T) {
	t.Parallel()
	notification := `{"type":"email","smtp":{"host":"localhost","tls":"none"},
	    "from":"alerts@example.com","to":["owner@example.com"],
	    "cc":["${FINFOCUS_NOTIFY_CC}"]}`
	alerts := `"alerts":[{"threshold":80,"type":"actual","notifications":[` + notification + `]}]`
	scope := `{"amount":100,"currency":"USD",` + alerts + `}`
	raw := []byte(`{"cost":{"budgets":{"global":` + scope + `,"providers":{"aws":` + scope +
		`},"tags":[{"selector":"team:platform","amount":100,` + alerts +
		`}],"types":{"aws:ec2/instance":` + scope + `}}}}`)

	global, _ := config.ValidateConfigSource("config.hujson", raw)
	require.True(t, global.Valid, "%+v", global.Errors)
	assert.Len(t, global.Warnings, 1) // Existing tag allocation warning.
	project, _ := config.ValidateProjectConfigSource("config.hujson", raw)
	require.False(t, project.Valid)
	require.Len(t, project.Errors, 4)
	for _, validationErr := range project.Errors {
		assert.Contains(t, validationErr.Path, ".notifications[0].cc[0]")
		assert.Contains(t, validationErr.Message, "project config destinations cannot use variables")
	}
}
