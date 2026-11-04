package pluginhost

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPluginNameFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "installed layout",
			path: filepath.Join("home", "plugins", "aws-public", "v1.2.3", "finfocus-plugin-aws-public"),
			want: "aws-public",
		},
		{
			name: "exe suffix",
			path: filepath.Join("home", "plugins", "kubernetes", "v0.1.0", "finfocus-plugin-kubernetes.exe"),
			want: "kubernetes",
		},
		{
			name: "bare binary",
			path: "finfocus-plugin-x",
			want: "x",
		},
		{
			name: "no prefix falls back to the base name",
			path: filepath.Join("opt", "tools", "my-plugin"),
			want: "my-plugin",
		},
		{
			name: "no prefix with exe suffix",
			path: filepath.Join("opt", "tools", "custom.exe"),
			want: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, pluginNameFromPath(tt.path))
		})
	}
}
