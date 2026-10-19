package webui

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
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
			name: "pulumi internal keys dropped",
			input: map[string]any{
				"__defaults": []any{"name"},
				"__provider": "urn:pulumi:dev::app::pulumi:providers:aws::default",
				"ami":        "ami-123",
			},
			want: map[string]any{"ami": "ami-123"},
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
			"__defaults":     []any{"payload-default-name"},
			"__provider":     "payload-provider-urn",
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
	assert.NotContains(t, out, "__defaults")
	assert.NotContains(t, out, "payload-default-name")
	assert.NotContains(t, out, "payload-provider-urn")
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

func TestRedactPropertyDiffAndCostDeltaNames(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"propertyDiffs": []engine.PropertyDiff{
			{Key: "dbPassword", OldValue: "old-password", NewValue: "new-password"},
			{Key: "__defaults", OldValue: "old-default", NewValue: "new-default"},
			{
				Key:      "size",
				OldValue: `{"4dabf18193072939515e22adb298388d":"1","ciphertext":"old-cipher"}`,
				NewValue: "safe",
			},
		},
		"deltas": []engine.CostDelta{
			{Property: "apiToken", OriginalValue: "old-token", NewValue: "new-token"},
			{Property: "instanceType", OriginalValue: "t3.small", NewValue: "t3.large"},
		},
	}
	data, err := RedactJSON(payload)
	require.NoError(t, err)
	for _, secret := range []string{"dbPassword", "old-password", "new-password", "old-default", "new-default", "apiToken", "old-token", "new-token", "old-cipher"} {
		assert.NotContains(t, string(data), secret)
	}
	assert.Contains(t, string(data), "instanceType")
}

func TestRedactionPreservesPassphrasePromptMetadata(t *testing.T) {
	t.Parallel()
	data, err := RedactJSON(OverviewSnapshot{PassphraseRequired: true})
	require.NoError(t, err)
	assert.Contains(t, string(data), `"passphraseRequired":true`)
	assert.NotContains(t, string(data), "secret-value")
	properties, propErr := RedactJSON(
		map[string]any{"passphraseRequired": true, "password": false, "passphrase": "secret-value"},
	)
	require.NoError(t, propErr)
	assert.JSONEq(t, `{}`, string(properties))
}

func TestRedactionRootSecretsAndStructuredDiffSecrets(t *testing.T) {
	t.Parallel()
	assert.Nil(t, RedactProperties(context.Background(), nil))
	assert.Nil(t, RedactValue(pulumiSecretFixture("root-cipher")))
	data, err := RedactJSON(
		map[string]any{
			"diffs": []any{
				map[string]any{"key": "size", "oldValue": pulumiSecretFixture("diff-cipher"), "newValue": "safe"},
			},
		},
	)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "diff-cipher")
	assert.JSONEq(t, `{"diffs":[]}`, string(data))
}

func TestRedactEstimateDeltasRetainsSourceSensitivity(t *testing.T) {
	t.Parallel()
	payload := map[string]any{
		"resource": map[string]any{
			"properties": map[string]any{"wrapped": pulumiSecretFixture("original-secret"), "plain": "original"},
		},
		"deltas": []any{
			map[string]any{"property": "wrapped", "originalValue": "", "newValue": "edited-secret"},
			map[string]any{"property": "plain", "originalValue": "original", "newValue": "  exact plain  "},
		},
	}
	raw, err := RedactJSON(payload)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "original-secret")
	assert.NotContains(t, string(raw), "edited-secret")
	assert.Contains(t, string(raw), "  exact plain  ")
	// Non-estimate consumers do not lose unrelated delta data when no source
	// descriptor exists; recursive credential filtering still applies normally.
	ordinary := map[string]any{"deltas": []any{map[string]any{"property": "plain", "newValue": "preserved"}}}
	raw, err = RedactJSON(ordinary)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "preserved")
	withoutProps := map[string]any{"resource": map[string]any{"id": "r"}, "deltas": []any{"preserved"}}
	raw, err = RedactJSON(withoutProps)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "preserved")
}

// Data-keyed maps keep URN and breakdown keys. Property maps still drop
// credential names, and Pulumi secret values inside a data-keyed map are removed.
func TestRedactJSONKeepsDataKeyedTrendAndBreakdown(t *testing.T) {
	t.Parallel()

	const secretURN = "urn:pulumi:dev::shop::aws:secretsmanager/secret:Secret::db"
	const instanceURN = "urn:pulumi:dev::shop::aws:ec2/instance:Instance::web"
	cost := engine.CostResult{
		ResourceID:   secretURN,
		ResourceType: "aws:secretsmanager/secret:Secret",
		Currency:     "USD",
		TotalCost:    5.7,
		Breakdown:    map[string]float64{"compute": 1.5, "input_tokens": 4.2},
	}
	page, err := viewmodel.BuildActualCostPage(
		context.Background(),
		[]engine.CostResult{cost},
		engine.GroupByNone,
		"",
		viewmodel.SortByCost,
		1,
		map[string]string{secretURN: "up", instanceURN: "flat"},
		"flat",
	)
	require.NoError(t, err)

	data, err := RedactJSON(actualCostQueryResponse{ActualCostPage: page})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))

	trends, ok := decoded["trends"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "up", trends[secretURN])
	assert.Equal(t, "flat", trends[instanceURN])

	results, ok := decoded["results"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, results)
	row, ok := results[0].(map[string]any)
	require.True(t, ok)
	breakdown, ok := row["breakdown"].(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, 4.2, breakdown["input_tokens"], 1e-9)
	assert.InDelta(t, 1.5, breakdown["compute"], 1e-9)

	detail := viewmodel.BuildActualCostDetail(cost)
	detailJSON, err := RedactJSON(detail)
	require.NoError(t, err)
	var gotDetail viewmodel.ActualCostDetail
	require.NoError(t, json.Unmarshal(detailJSON, &gotDetail))
	assert.Equal(t, viewmodel.BreakdownDisplay(cost.Breakdown, true), gotDetail.Breakdown)
	assert.Contains(t, string(detailJSON), "input_tokens")

	overview, err := RedactJSON(engine.ActualCostData{
		Breakdown: map[string]float64{"input_tokens": 1.25, "compute": 2},
	})
	require.NoError(t, err)
	assert.Contains(t, string(overview), `"input_tokens":1.25`)

	type markedUsage struct {
		Usage engine.DataKeyed[any] `json:"usage"`
	}
	usage, err := RedactJSON(markedUsage{Usage: engine.DataKeyed[any]{
		"input_tokens": 4.2,
		"wrapped":      pulumiSecretFixture("usage-cipher"),
	}})
	require.NoError(t, err)
	assert.Contains(t, string(usage), `"input_tokens":4.2`)
	assert.NotContains(t, string(usage), "usage-cipher")
	assert.NotContains(t, string(usage), "4dabf18193072939515e22adb298388d")

	properties, err := RedactJSON(map[string]any{
		"properties": map[string]any{"dbPassword": "hunter2", "instanceType": "t3.large"},
		"diffs": []any{
			map[string]any{"key": "apiToken", "oldValue": "old-token", "newValue": "new-token"},
		},
	})
	require.NoError(t, err)
	assert.NotContains(t, string(properties), "hunter2")
	assert.NotContains(t, string(properties), "dbPassword")
	assert.NotContains(t, string(properties), "apiToken")
	assert.NotContains(t, string(properties), "old-token")
	assert.Contains(t, string(properties), "t3.large")
}

// Trusted numeric summary labels must survive without exempting property maps.
func TestRedactJSONTrustedCostSummaryLabels(t *testing.T) {
	t.Parallel()
	response := actualCostQueryResponse{ActualCostPage: viewmodel.ActualCostPage{
		Summary: viewmodel.ActualCostSummary{CostSummary: engine.CostSummary{
			ByService:  map[string]float64{"secretsmanager": 12.5},
			ByProvider: map[string]float64{"secret-provider": 12.5},
			ByAdapter:  map[string]float64{"token-adapter": 12.5},
		}},
	}}
	data, err := RedactJSON(response)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"secretsmanager":12.5`)
	assert.Contains(t, string(data), `"secret-provider":12.5`)
	assert.Contains(t, string(data), `"token-adapter":12.5`)
	data, err = RedactJSON(
		map[string]any{
			"properties": map[string]any{"secretsmanager": "credential"},
			"summary":    map[string]any{"byService": map[string]any{"secretsmanager": 12.5}},
		},
	)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "secretsmanager")
	assert.NotContains(t, string(data), "credential")
}
