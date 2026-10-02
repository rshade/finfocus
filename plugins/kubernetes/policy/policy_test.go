package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_EmptyInputsYieldDefaults(t *testing.T) {
	t.Parallel()

	_, defDigest, err := Defaults().Canonical()
	require.NoError(t, err)
	for _, in := range []string{"", "  ", "{}", `{"version": 1}`} {
		p, decodeErr := Decode([]byte(in))
		require.NoError(t, decodeErr, "input %q", in)
		assert.Equal(t, Defaults(), p, "input %q", in)
		_, d, canonErr := p.Canonical()
		require.NoError(t, canonErr)
		assert.Equal(t, defDigest, d, "input %q must digest like defaults", in)
	}
}

func TestDecode_OverridesMergeOntoDefaults(t *testing.T) {
	t.Parallel()

	p, err := Decode([]byte(`{"node_split": {"cpu_core_hour": 0.05}}`))
	require.NoError(t, err)
	assert.InDelta(t, 0.05, p.NodeSplit.CPUCoreHour, 1e-12)
	assert.Equal(t, "unit-price-ratio", p.NodeSplit.Method, "sibling field keeps default")
	assert.InDelta(t, 0.004237, p.NodeSplit.MemGiBHour, 1e-12)
	assert.Equal(t, "separate", p.Idle)
}

func TestDecode_AcceptsShareModes(t *testing.T) {
	t.Parallel()

	p, err := Decode([]byte(`{"idle": "share", "system_workloads": "share"}`))
	require.NoError(t, err)
	assert.True(t, p.ShareIdle())
	assert.True(t, p.ShareSystemWorkloads())
	canon, digest, err := p.Canonical()
	require.NoError(t, err)
	assert.Contains(t, string(canon), `"idle":"share"`)
	assert.Contains(t, string(canon), `"system_workloads":"share"`)
	_, defDigest, err := Defaults().Canonical()
	require.NoError(t, err)
	assert.NotEqual(t, defDigest, digest)

	separate, err := Decode([]byte(`{"idle": "separate", "system_workloads": "separate"}`))
	require.NoError(t, err)
	assert.False(t, separate.ShareIdle())
	assert.Equal(t, Defaults(), separate)
}

func TestDecode_Rejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, wantErr string
	}{
		{"unknown top-level field", `{"idel": "share"}`, "idel"},
		{"unknown nested field reports JSON path", `{"node_split": {"cpu": 1}}`, "node_split.cpu"},
		{"unknown version", `{"version": 2}`, "unsupported policy version 2"},
		{"unsupported idle mode", `{"idle": "spread"}`, `idle: unsupported value "spread"`},
		{"unsupported system mode", `{"system_workloads": "spread"}`, `system_workloads: unsupported value "spread"`},
		{"negative weight", `{"node_split": {"mem_gib_hour": -1}}`, "node_split weights must be >= 0"},
		{"trailing data", `{} {}`, "allocation policy"},
		{"not json", `idle: separate`, "allocation policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode([]byte(tt.in))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCanonical_StableAndDistinct(t *testing.T) {
	t.Parallel()

	a, da, err := Defaults().Canonical()
	require.NoError(t, err)
	b, db, err := Defaults().Canonical()
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.Equal(t, da, db)
	assert.Len(t, da, 64, "hex sha256")

	p := Defaults()
	p.NodeSplit.CPUCoreHour = 0.04
	_, dp, err := p.Canonical()
	require.NoError(t, err)
	assert.NotEqual(t, da, dp)
}
