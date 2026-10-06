package webui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

// pulumiSecretFixture is a Pulumi secret input as it appears in exported
// state: the secret signature key plus a ciphertext payload.
func pulumiSecretFixture(ciphertext string) map[string]any {
	return map[string]any{
		"4dabf18193072939515e22adb298388d": "1",
		"ciphertext":                       ciphertext,
	}
}

func TestRedactPropertiesDropsSecretsAndCredentials(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	props := map[string]any{
		"instanceType": "t3.large",
		"region":       "us-east-1",
		"dbPassword":   "hunter2-plaintext",
		"apiToken":     "tok-live-9f8e7d",
		"nested": map[string]any{
			"secretValue":   "nested-secret-123",
			"instanceClass": "db.t3.large",
			"deeper": map[string]any{
				"privateKey": "-----BEGIN-PRIVATE-KEY-----",
			},
		},
		"size": pulumiSecretFixture("v1:encrypted-blob-aabbcc"),
		"list": []any{"keep-me", pulumiSecretFixture("v1:encrypted-in-list")},
	}

	redacted := RedactProperties(ctx, props)
	data, err := json.Marshal(redacted)
	require.NoError(t, err)
	out := string(data)

	// Sensitive values never appear.
	assert.NotContains(t, out, "hunter2-plaintext")
	assert.NotContains(t, out, "tok-live-9f8e7d")
	assert.NotContains(t, out, "nested-secret-123")
	assert.NotContains(t, out, "-----BEGIN-PRIVATE-KEY-----")
	assert.NotContains(t, out, "v1:encrypted-blob-aabbcc")
	assert.NotContains(t, out, "v1:encrypted-in-list")
	// Credential-like keys are gone, not just emptied.
	assert.NotContains(t, out, "dbPassword")
	assert.NotContains(t, out, "apiToken")
	assert.NotContains(t, out, "secretValue")
	assert.NotContains(t, out, "privateKey")
	assert.NotContains(t, out, "4dabf18193072939515e22adb298388d")

	// Non-sensitive data survives untouched.
	assert.Equal(t, "t3.large", redacted["instanceType"])
	assert.Equal(t, "us-east-1", redacted["region"])
	nested, ok := redacted["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "db.t3.large", nested["instanceClass"])
	assert.Equal(t, []any{"keep-me"}, redacted["list"])
}

func TestRedactValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  any
	}{
		{
			name:  "scalar passthrough",
			input: "t3.large",
			want:  "t3.large",
		},
		{
			name:  "credential-like key dropped",
			input: map[string]any{"password": "pw-value", "ami": "ami-123"},
			want:  map[string]any{"ami": "ami-123"},
		},
		{
			name:  "token-like key dropped case-insensitively",
			input: map[string]any{"APIToken": "tok-value", "region": "us-east-1"},
			want:  map[string]any{"region": "us-east-1"},
		},
		{
			name:  "pulumi secret value dropped",
			input: map[string]any{"size": pulumiSecretFixture("cipher-1"), "ami": "ami-123"},
			want:  map[string]any{"ami": "ami-123"},
		},
		{
			name: "nested maps redacted at depth",
			input: map[string]any{
				"outer": map[string]any{
					"connectionString": "conn-value",
					"ok":               true,
				},
			},
			want: map[string]any{"outer": map[string]any{"ok": true}},
		},
		{
			name: "secret inside array dropped",
			input: map[string]any{
				"items": []any{"keep", pulumiSecretFixture("cipher-2")},
			},
			want: map[string]any{"items": []any{"keep"}},
		},
		{
			name:  "array elements preserved in order",
			input: map[string]any{"azs": []any{"a", "b", "c"}},
			want:  map[string]any{"azs": []any{"a", "b", "c"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, RedactValue(tt.input))
		})
	}
}

// TestRedactJSONPayloads exercises the helper handlers use for every payload
// the server emits (overview, detail, cost, recommendations, estimate): the
// shared rules strip Pulumi secrets and credential-like properties no matter
// how deeply the payload nests them.
func TestRedactJSONPayloads(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"urn":  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		"type": "aws:ec2/instance:Instance",
		"properties": map[string]any{
			"instanceType":   "t3.large",
			"adminPassword":  "payload-pw-456",
			"userDataSecret": pulumiSecretFixture("v1:payload-cipher"),
		},
		"breakdown": []any{
			map[string]any{"component": "compute", "monthly": 12.34},
		},
		"estimate": map[string]any{
			"overrides": map[string]any{"sessionToken": "override-token"},
			"monthly":   56.78,
		},
	}

	data, err := RedactJSON(payload)
	require.NoError(t, err)
	out := string(data)

	assert.NotContains(t, out, "payload-pw-456")
	assert.NotContains(t, out, "v1:payload-cipher")
	assert.NotContains(t, out, "override-token")
	assert.NotContains(t, out, "adminPassword")
	assert.NotContains(t, out, "sessionToken")
	assert.Contains(t, out, "t3.large")
	assert.Contains(t, out, "urn:pulumi:dev::app::aws:ec2/instance:Instance::web")
	assert.Contains(t, out, "compute")
}

func TestBudgetHealthPayloadOmitsNotificationDestinations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cfg := &config.BudgetsConfig{
		Global: &config.ScopedBudget{
			Amount:   100,
			Currency: "USD",
			Alerts: []config.AlertConfig{{
				Threshold: 80,
				Type:      config.AlertTypeActual,
				Notifications: []config.NotificationDestination{
					{
						Type:    config.NotificationTypeSlack,
						URL:     "https://hooks.slack.com/services/T0SECRET/B0SECRET/deadbeef",
						Channel: "#fin-alerts",
					},
					{
						Type: config.NotificationTypeWebhook,
						URL:  "https://notify.internal.example/hooks/budget",
						Headers: map[string]string{
							"Authorization": "Bearer notify-secret-header",
						},
					},
				},
			}},
		},
	}

	result := engine.BuildConfigBudgetResult(ctx, cfg, 50.0)
	require.NotNil(t, result, "config budget fixture must evaluate")

	payload := BudgetHealthPayload(ctx, result)
	require.NotEmpty(t, payload)

	data, err := json.Marshal(payload)
	require.NoError(t, err)
	out := string(data)

	// No notification destination (webhook URL, Slack channel, header) leaks.
	assert.NotContains(t, out, "hooks.slack.com")
	assert.NotContains(t, out, "T0SECRET")
	assert.NotContains(t, out, "#fin-alerts")
	assert.NotContains(t, out, "notify.internal.example")
	assert.NotContains(t, out, "notify-secret-header")
	assert.NotContains(t, out, "notifications")

	// The payload is no wider than the budget shape of
	// `finfocus overview --output json` (BudgetHealthResult fields only).
	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.NotEmpty(t, decoded)
	allowed := map[string]bool{
		"budgetID": true, "budgetName": true, "provider": true, "health": true,
		"utilization": true, "forecasted": true, "currency": true, "limit": true,
		"currentSpend": true,
	}
	for _, entry := range decoded {
		for key := range entry {
			assert.True(t, allowed[key], "budget payload field %q is wider than the CLI JSON output", key)
		}
	}

	assert.Equal(t, "USD", payload[0].Currency)
	assert.InDelta(t, 100.0, payload[0].Limit, 1e-9)
}

func TestBudgetHealthPayloadNilResult(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BudgetHealthPayload(context.Background(), nil))
}
