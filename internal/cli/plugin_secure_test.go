package cli_test

import (
	"context"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/config"
)

func TestPluginInstallUpdate_SecurePluginsRejectsInput(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		args    []string
		wantErr string
	}{
		{
			name:    "install refuses skip-checksum",
			env:     "true",
			args:    []string{"plugin", "install", "--skip-checksum", "aws-public"},
			wantErr: "checksum verification cannot be skipped",
		},
		{
			name:    "update refuses skip-checksum",
			env:     "1",
			args:    []string{"plugin", "update", "--skip-checksum", "aws-public"},
			wantErr: "checksum verification cannot be skipped",
		},
		{
			name:    "install rejects a bad value",
			env:     "maybe",
			args:    []string{"plugin", "install", "aws-public"},
			wantErr: config.EnvSecurePlugins + " must be one of",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.EnvSecurePlugins, tt.env)
			t.Setenv("FINFOCUS_HOME", t.TempDir())

			result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), tt.args)

			assert.Equal(t, 2, result.ExitCode)
			assert.Contains(t, string(result.Stderr), tt.wantErr)
		})
	}
}
