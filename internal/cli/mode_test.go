package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/cli"
)

func TestDetectPluginMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		env  map[string]string
		want bool
	}{
		{
			name: "Standard binary name, no env",
			args: []string{"/usr/bin/finfocus"},
			env:  map[string]string{},
			want: false,
		},
		{
			name: "Plugin binary name",
			args: []string{"/usr/bin/pulumi-tool-cost"},
			env:  map[string]string{},
			want: true,
		},
		{
			name: "Plugin binary name with .exe",
			args: []string{"/usr/bin/pulumi-tool-cost.exe"},
			env:  map[string]string{},
			want: true,
		},
		{
			name: "Plugin binary name case insensitive",
			args: []string{"/usr/bin/PULUMI-TOOL-COST"},
			env:  map[string]string{},
			want: true,
		},
		{
			name: "Env var set to true",
			args: []string{"/usr/bin/finfocus"},
			env:  map[string]string{"FINFOCUS_PLUGIN_MODE": "true"},
			want: true,
		},
		{
			name: "Env var set to 1",
			args: []string{"/usr/bin/finfocus"},
			env:  map[string]string{"FINFOCUS_PLUGIN_MODE": "1"},
			want: true,
		},
		{
			name: "Env var set to false",
			args: []string{"/usr/bin/finfocus"},
			env:  map[string]string{"FINFOCUS_PLUGIN_MODE": "false"},
			want: false,
		},
		{
			// Binary name triggers plugin mode even if env var is false.
			// Spec uses OR logic: "Detect via EITHER binary name OR env var".
			name: "Both binary name and env set",
			args: []string{"/usr/bin/pulumi-tool-cost"},
			env:  map[string]string{"FINFOCUS_PLUGIN_MODE": "false"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lookupEnv := func(key string) (string, bool) {
				val, ok := tt.env[key]
				return val, ok
			}
			got := cli.DetectPluginMode(tt.args, lookupEnv)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectPluginMode_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("nil lookupEnv does not panic", func(t *testing.T) {
		t.Parallel()
		// Should not panic when lookupEnv is nil - just skip env var detection
		got := cli.DetectPluginMode([]string{"/usr/bin/finfocus"}, nil)
		assert.False(t, got)
	})

	t.Run("nil lookupEnv with plugin binary name", func(t *testing.T) {
		t.Parallel()
		// Binary name detection should still work even with nil lookupEnv
		got := cli.DetectPluginMode([]string{"/usr/bin/pulumi-tool-cost"}, nil)
		assert.True(t, got)
	})

	t.Run("nil args does not panic", func(t *testing.T) {
		t.Parallel()
		lookupEnv := func(_ string) (string, bool) { return "", false }
		got := cli.DetectPluginMode(nil, lookupEnv)
		assert.False(t, got)
	})

	t.Run("empty args does not panic", func(t *testing.T) {
		t.Parallel()
		lookupEnv := func(_ string) (string, bool) { return "", false }
		got := cli.DetectPluginMode([]string{}, lookupEnv)
		assert.False(t, got)
	})

	t.Run("both nil does not panic", func(t *testing.T) {
		t.Parallel()
		got := cli.DetectPluginMode(nil, nil)
		assert.False(t, got)
	})
}
