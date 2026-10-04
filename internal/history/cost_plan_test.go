package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStackHistory(t *testing.T) {
	t.Parallel()
	raw := []byte(`[
		{"version": 2, "kind": "update", "startTime": "2025-03-01T00:00:00Z",
			"message": "upgrade", "result": "succeeded", "resourceChanges": {"update": 1}},
		{"version": 1, "kind": "destroy", "startTime": "2025-01-15T12:00:00.000Z",
			"message": "gone", "result": "succeeded"}
	]`)
	updates, err := ParseStackHistory(raw)
	require.NoError(t, err)
	require.Len(t, updates, 2)
	assert.Equal(t, 2, updates[0].Version)
	assert.Equal(t, "upgrade", updates[0].Message)
	assert.Equal(t, 1, updates[0].ResourceChanges["update"])
	assert.True(t, updates[1].Start.Equal(time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)))
}

func TestParseStackHistory_Invalid(t *testing.T) {
	t.Parallel()
	_, err := ParseStackHistory([]byte(`{"version": 1}`))
	require.Error(t, err)
	_, err = ParseStackHistory([]byte(`[{"version": 1, "startTime": "yesterday"}]`))
	require.Error(t, err)
}

func TestSelectUpdates(t *testing.T) {
	t.Parallel()
	updates := []StackUpdate{
		{Version: 1, Kind: "update", Result: "failed", Start: date(2025, 1, 1)},
		{Version: 2, Kind: "update", Result: "succeeded", Start: date(2025, 1, 2)},
		{Version: 3, Kind: "refresh", Result: "succeeded", Start: date(2025, 1, 3)},
		{Version: 4, Kind: "destroy", Result: "succeeded", Start: date(2025, 1, 4)},
		{Version: 5, Kind: "update", Result: "succeeded", Start: date(2025, 2, 1)},
	}
	selected := SelectUpdates(updates, SelectOptions{})
	require.Len(t, selected, 3)
	assert.Equal(t, []int{2, 4, 5}, versionsOf(selected))

	skipped := SelectUpdates(updates, SelectOptions{SkipDestroy: true})
	assert.Equal(t, []int{2, 5}, versionsOf(skipped))

	from := SelectUpdates(updates, SelectOptions{From: date(2025, 1, 4)})
	assert.Equal(t, []int{4, 5}, versionsOf(from))

	limited := SelectUpdates(updates, SelectOptions{Limit: 1})
	assert.Equal(t, []int{5}, versionsOf(limited))

	incremental := SelectUpdates(updates, SelectOptions{LastVersion: 4})
	assert.Equal(t, []int{5}, versionsOf(incremental))

	stored := SelectUpdates(updates, SelectOptions{Have: map[int]struct{}{2: {}, 4: {}}})
	assert.Equal(t, []int{5}, versionsOf(stored))

	gap := SelectUpdates(updates, SelectOptions{Have: map[int]struct{}{5: {}}, LastVersion: 5})
	assert.Equal(t, []int{2, 4}, versionsOf(gap))
}

func TestVersionReset(t *testing.T) {
	t.Parallel()
	assert.True(t, VersionReset(1, 50))
	assert.False(t, VersionReset(50, 50))
	assert.False(t, VersionReset(51, 50))
	assert.False(t, VersionReset(0, 50))
	assert.False(t, VersionReset(1, 0))
	assert.Equal(t, 3, MaxVersion([]StackUpdate{{Version: 1}, {Version: 3}, {Version: 2}}))
	assert.Contains(t, VersionResetWarning(1, 50), "v1 after v50")
}

func TestCostFileName(t *testing.T) {
	t.Parallel()
	name, err := CostFileName("acme/web-app/production")
	require.NoError(t, err)
	assert.Equal(t, "acme-web-app-production.history.db", name)
	name, err = CostFileName("dev")
	require.NoError(t, err)
	assert.Equal(t, "dev.history.db", name)
	_, err = CostFileName("../dev")
	require.Error(t, err)
	_, err = CostFileName(" ")
	require.Error(t, err)
	assert.True(t, IsCostHistoryFile("dev.history.db"))
	assert.False(t, IsCostHistoryFile("history.db"))
	assert.False(t, IsCostHistoryFile(".history.db"))
}

func TestSnapshotFromResults(t *testing.T) {
	t.Parallel()
	when := date(2025, 6, 15)
	resources := []PriceResource{{
		Type:     "aws:ec2/instance:Instance",
		ID:       "urn:web",
		Provider: "aws",
		Properties: map[string]any{
			"instanceType":     "m5.large",
			"availabilityZone": "us-east-1a",
		},
	}}
	snapshot, err := SnapshotFromResults(when, 12, resources, []PriceQuote{{
		ResourceID:   "urn:web",
		ResourceType: "aws:ec2/instance:Instance",
		Adapter:      "aws-public",
		Currency:     "USD",
		Monthly:      0,
	}})
	require.NoError(t, err)
	assert.InDelta(t, 0, snapshot.TotalMonthly, 0.001)
	assert.Equal(t, "m5.large", snapshot.Resources[0].SKU)
	assert.Equal(t, "us-east-1", snapshot.Resources[0].Region)
	assert.Equal(t, "ec2:Instance", func() string {
		key := ""
		for name := range snapshot.ByType {
			key = name
		}
		return key
	}())
	assert.Equal(t, 1, snapshot.ResourceCount)

	_, err = SnapshotFromResults(when, 12, resources, []PriceQuote{{
		ResourceID: "urn:web",
		Adapter:    "none",
		Currency:   "USD",
	}})
	require.ErrorIs(t, err, ErrNoPlugin)
	require.ErrorContains(t, err, "aws:ec2/instance:Instance")

	_, err = SnapshotFromResults(when, 12, resources, []PriceQuote{{
		ResourceID: "urn:web",
		Adapter:    "aws-public",
		Notes:      "No pricing information available",
	}})
	require.ErrorIs(t, err, ErrNoPlugin)

	_, err = SnapshotFromResults(when, 12, resources, []PriceQuote{{
		ResourceID: "urn:web",
		Adapter:    "aws-public",
		Failed:     true,
	}})
	require.ErrorIs(t, err, ErrNoPlugin)

	mixed := []PriceResource{
		{Type: "aws:ec2/instance:Instance", ID: "a", Provider: "aws"},
		{Type: "gcp:compute:Instance", ID: "b", Provider: "gcp"},
	}
	_, err = SnapshotFromResults(when, 1, mixed, []PriceQuote{
		{ResourceID: "a", Adapter: "aws", Currency: "USD", Monthly: 1},
		{ResourceID: "b", Adapter: "gcp", Currency: "EUR", Monthly: 1},
	})
	require.ErrorIs(t, err, ErrMixedCurrencies)
}

func TestEncryptedProperty(t *testing.T) {
	t.Parallel()
	secret := map[string]any{"4dabf18193072939515e22adb298388d": "sig"}
	name := EncryptedProperty([]PriceResource{{
		Properties: map[string]any{"instanceType": secret, "region": "us-east-1"},
	}})
	assert.Equal(t, "instanceType", name)
	assert.Empty(t, EncryptedProperty([]PriceResource{{
		Properties: map[string]any{"instanceType": "m5.large"},
	}}))
	err := EncryptedError("instanceType")
	require.ErrorContains(t, err, "required property 'instanceType' is encrypted")
}

func TestIsPulumiSecret(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"secret map", map[string]any{secretSignature: "1", "ciphertext": "x"}, true},
		{"plain map", map[string]any{"value": "x"}, false},
		{"string", secretSignature, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsPulumiSecret(tt.value))
		})
	}
}

func TestAnnotationFrom_Destroy(t *testing.T) {
	t.Parallel()
	annotation := AnnotationFrom(
		StackUpdate{Version: 4, Kind: "destroy", Message: "gone", ResourceChanges: map[string]int{"delete": 2}},
	)
	assert.Equal(t, "Stack destroyed", annotation.Message)
	assert.Equal(t, 2, annotation.ResourceChanges["delete"])
	zero := ZeroSnapshot(time.Time{}, 1)
	assert.Empty(t, zero.Resources)
	assert.Equal(t, "USD", zero.Currency)
}

func versionsOf(updates []StackUpdate) []int {
	out := make([]int, len(updates))
	for i, update := range updates {
		out[i] = update.Version
	}
	return out
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
