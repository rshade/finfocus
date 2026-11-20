package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSecurePlugins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value   string
		want    bool
		wantErr bool
	}{
		{value: "", want: false},
		{value: "false", want: false},
		{value: "0", want: false},
		{value: "true", want: true},
		{value: " TRUE ", want: true},
		{value: "1", want: true},
		{value: "yes", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveSecurePlugins(func(string) string { return tt.value })

			if tt.wantErr {
				require.ErrorContains(t, err, EnvSecurePlugins)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
