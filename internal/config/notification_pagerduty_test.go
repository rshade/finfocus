package config_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestPagerDutyDestinationValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fields, wantField string
	}{
		{"default", `"routing_key":"secret"`, ""},
		{"EU", `"routing_key":"${FINFOCUS_NOTIFY_PD_KEY}","severity":"critical","dedup_key":"per-run",
			"url":"https://events.eu.pagerduty.com/v2/enqueue"`, ""},
		{"loopback", `"routing_key":"secret","url":"https://127.0.0.1:5000/v2/enqueue"`, ""},
		{"missing key", `"severity":"warning"`, "routing_key"},
		{"blank key", `"routing_key":" "`, "routing_key"},
		{"disallowed variable", `"routing_key":"${CI_TOKEN}"`, "routing_key"},
		{"bad severity", `"routing_key":"secret","severity":"urgent"`, "severity"},
		{"bad dedup", `"routing_key":"secret","dedup_key":"custom"`, "dedup_key"},
		{"HTTP", `"routing_key":"secret","url":"http://events.pagerduty.com/v2/enqueue"`, "url"},
		{"untrusted host", `"routing_key":"secret","url":"https://events.pagerduty.com.example.com/v2/enqueue"`, "url"},
		{"userinfo", `"routing_key":"secret","url":"https://user@events.pagerduty.com/v2/enqueue"`, "url"},
		{"query", `"routing_key":"secret","url":"https://events.pagerduty.com/v2/enqueue?debug=1"`, "url"},
		{"wrong path", `"routing_key":"secret","url":"https://events.pagerduty.com/v2/change/enqueue"`, "url"},
		{"webhook method", `"routing_key":"secret","method":"PUT"`, "method"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var dest config.NotificationDestination
			require.NoError(t, json.Unmarshal([]byte(`{"type":"pagerduty",`+tc.fields+`}`), &dest))
			err := dest.Validate()
			if tc.wantField == "" {
				require.NoError(t, err)
				return
			}
			var fieldErr *config.NotificationFieldError
			require.ErrorAs(t, err, &fieldErr)
			assert.Equal(t, tc.wantField, fieldErr.Field)
			assert.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestPagerDutyRoutingKeyInProjectConfig(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"cost":{"budgets":{"global":{"amount":100,"currency":"USD",
		"alerts":[{"type":"actual","threshold":80,"notifications":[{"type":"pagerduty",
			"routing_key":"${FINFOCUS_NOTIFY_PD_KEY}"}]}]}}}}`)
	global, _ := config.ValidateConfigSource("config.hujson", raw)
	require.True(t, global.Valid, "%+v", global.Errors)
	project, _ := config.ValidateProjectConfigSource("config.hujson", raw)
	require.False(t, project.Valid)
	require.Len(t, project.Errors, 1)
	assert.Equal(t, "cost.budgets.global.alerts[0].notifications[0].routing_key", project.Errors[0].Path)
}
