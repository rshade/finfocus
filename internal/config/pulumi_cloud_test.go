package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPulumiCloudMode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw   string
		want  PulumiCloudMode
		valid bool
	}{
		{`false`, Off, true}, {`true`, Required, true}, {`"false"`, Off, true}, {`"auto"`, Auto, true}, {`"true"`, Required, true},
		{`""`, Off, false}, {`"yes"`, Off, false}, {`"AUTO"`, Off, false}, {`1`, Off, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			for _, decode := range []func([]byte, any) error{json.Unmarshal, yaml.Unmarshal} {
				var got PulumiCloudMode
				err := decode([]byte(tc.raw), &got)
				if !tc.valid {
					require.Error(t, err)
					assert.Contains(t, err.Error(), "false, auto, true")
					continue
				}
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
				encoded, err := json.Marshal(got)
				require.NoError(t, err)
				assert.Equal(t, `"`+tc.want.String()+`"`, string(encoded))
			}
		})
	}
	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(`{}`), &cfg))
	assert.Equal(t, Off, cfg.Cost.History.PulumiCloud.Enabled)
	result, _ := ValidateConfigSource("test.hujson", []byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":"auto"}}}}`))
	assert.True(t, result.Valid, "%+v", result.Errors)
}

func TestPulumiCloudModeConfigValidation(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`false`, `true`, `"false"`, `"auto"`, `"true"`} {
		result, _ := ValidateConfigSource(
			"test.hujson",
			[]byte(`{"cost":{"history":{"pulumi_cloud":{"enabled":`+value+`}}}}`),
		)
		assert.True(t, result.Valid, "%s: %+v", value, result.Errors)
	}
}
