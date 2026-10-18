package webui

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestRedactJSONDoesNotRestoreCredentialNamedDataKeyedField(t *testing.T) {
	t.Parallel()

	type payload struct {
		Secrets map[string]float64 `json:"secrets" datakey:"true"`
		Totals  map[string]float64 `json:"totals"  datakey:"true"`
	}
	data, err := RedactJSON(payload{
		Secrets: map[string]float64{"urn:db": 1},
		Totals:  map[string]float64{"urn:web": 2},
	})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "secrets")
	assert.NotContains(t, string(data), "urn:db")
	assert.Contains(t, string(data), "urn:web")
}

func collectDataKeyedMapTypes(t reflect.Type, seen map[reflect.Type]bool, found map[string]reflect.Type) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for i := range t.NumField() {
		field := t.Field(i)
		if field.Tag.Get("datakey") == "true" {
			found[t.Name()+"."+field.Name] = field.Type
		}
		collectDataKeyedMapTypes(field.Type, seen, found)
	}
}

// A data-keyed map keeps its keys, so a string value there is never checked
// by name. Only numeric maps may carry the tag, plus the sparkline-only Trends.
func TestDataKeyedFieldsHoldOnlyNumericValues(t *testing.T) {
	t.Parallel()

	found := map[string]reflect.Type{}
	seen := map[reflect.Type]bool{}
	for _, root := range []any{
		engine.CostResult{}, engine.CostSummary{}, engine.CrossProviderAggregation{},
		engine.ActualCostData{}, engine.OverviewRow{}, viewmodel.ActualCostPage{},
	} {
		collectDataKeyedMapTypes(reflect.TypeOf(root), seen, found)
	}
	require.NotEmpty(t, found)
	for name, fieldType := range found {
		require.Equal(t, reflect.Map, fieldType.Kind(), name)
		if name == "ActualCostPage.Trends" {
			continue
		}
		kind := fieldType.Elem().Kind()
		numeric := kind >= reflect.Int && kind <= reflect.Uint64 ||
			kind == reflect.Float32 || kind == reflect.Float64
		assert.True(t, numeric, "%s holds %s", name, fieldType.Elem())
	}
}

func TestRedactJSONKeepsDataKeysWhenSiblingElementIsDropped(t *testing.T) {
	t.Parallel()

	type row struct {
		Key    string             `json:"key"`
		Totals map[string]float64 `json:"totals" datakey:"true"`
	}
	data, err := RedactJSON(map[string]any{"rows": []row{
		{Key: "apiToken", Totals: map[string]float64{"urn:gone": 1}},
		{Key: "web", Totals: map[string]float64{"urn:aws:secretsmanager:db": 2}},
	}})
	require.NoError(t, err)
	var decoded struct {
		Rows []struct {
			Key    string             `json:"key"`
			Totals map[string]float64 `json:"totals"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Len(t, decoded.Rows, 1)
	assert.Equal(t, map[string]float64{"urn:aws:secretsmanager:db": 2}, decoded.Rows[0].Totals)
	assert.NotContains(t, string(data), "urn:gone")
}
