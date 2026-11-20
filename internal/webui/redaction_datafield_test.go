package webui

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func redactRows(t *testing.T, value any) []map[string]any {
	t.Helper()
	data, err := RedactJSON(value)
	require.NoError(t, err)
	var decoded struct {
		Rows []map[string]any `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	return decoded.Rows
}

// A record whose key names domain data (a cluster group, say) is kept when the
// Go field that produced it is tagged datakey:"true"; an untagged record, or
// one decoded with no Go value behind it, still follows the property rules.
func TestRedactJSONKeepsTaggedDataKeyRecords(t *testing.T) {
	t.Parallel()

	type dataRow struct {
		Key   string  `json:"key"   datakey:"true"`
		Total float64 `json:"total"`
	}
	type propertyRow struct {
		Key   string  `json:"key"`
		Total float64 `json:"total"`
	}
	names := []string{"__idle__", "apiToken", "vault-secrets", "web"}

	tagged := struct {
		Rows []dataRow `json:"rows"`
	}{}
	untagged := struct {
		Rows []propertyRow `json:"rows"`
	}{}
	decoded := map[string]any{"rows": []any{}}
	for _, name := range names {
		tagged.Rows = append(tagged.Rows, dataRow{Key: name, Total: 1})
		untagged.Rows = append(untagged.Rows, propertyRow{Key: name, Total: 1})
		decoded["rows"] = append(decoded["rows"].([]any), map[string]any{"key": name, "total": 1})
	}

	keys := func(rows []map[string]any) []any {
		out := []any{}
		for _, row := range rows {
			out = append(out, row["key"])
		}
		return out
	}
	assert.Equal(t, []any{"__idle__", "apiToken", "vault-secrets", "web"}, keys(redactRows(t, tagged)))
	assert.Equal(t, []any{"web"}, keys(redactRows(t, untagged)), "an untagged key is a property name")
	assert.Equal(t, []any{"web"}, keys(redactRows(t, decoded)), "no Go value, no trust")
}

func TestRedactJSONTaggedDataKeyRecordStillDropsSecrets(t *testing.T) {
	t.Parallel()

	type row struct {
		Key      string `json:"key"      datakey:"true"`
		NewValue any    `json:"newValue"`
	}
	rows := redactRows(t, struct {
		Rows []row `json:"rows"`
	}{Rows: []row{
		{Key: "web", NewValue: pulumiSecretFixture("cipher")},
		{Key: "api", NewValue: "plain"},
	}})
	require.Len(t, rows, 1)
	assert.Equal(t, "api", rows[0]["key"])
}

// The dashboard's cluster payload keeps every group, including the idle and
// cluster rows and groups whose names look like credentials, so the groups
// still sum to the total the CLI reports.
func TestRedactJSONKeepsEveryClusterGroup(t *testing.T) {
	t.Parallel()

	idle := 10.0
	report := engine.ClusterReport{
		Currency: "USD", Total: 40, Idle: &idle, Capacity: []engine.CapacityGroup{},
		Groups: []engine.ClusterGroup{
			{Key: "vault-secrets", TotalCost: 10}, {Key: "token-service", TotalCost: 10},
			{Key: "__idle__", TotalCost: 10}, {Key: "__cluster__", TotalCost: 10},
		},
	}
	data, err := RedactJSON(ClusterResponse{Status: clusterStatusOK, ClusterReport: report})
	require.NoError(t, err)
	var got ClusterResponse
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, report.Groups, got.Groups)
}
