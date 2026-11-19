package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestApplySecurePlugins(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		in      checksumPolicy
		want    checksumPolicy
		wantErr error
	}{
		{
			name: "unset leaves the policy alone",
			env:  "",
			in:   checksumPolicy{skip: true},
			want: checksumPolicy{skip: true},
		},
		{
			name: "false leaves the policy alone",
			env:  "false",
			in:   checksumPolicy{acceptMissing: true},
			want: checksumPolicy{acceptMissing: true},
		},
		{
			name: "true requires the file",
			env:  "true",
			in:   checksumPolicy{},
			want: checksumPolicy{requireFile: true},
		},
		{
			name: "1 requires the file",
			env:  "1",
			in:   checksumPolicy{},
			want: checksumPolicy{requireFile: true},
		},
		{
			name: "true drops accept-missing",
			env:  "TRUE",
			in:   checksumPolicy{acceptMissing: true},
			want: checksumPolicy{requireFile: true},
		},
		{name: "true refuses skip", env: "true", in: checksumPolicy{skip: true}, wantErr: ErrSkipChecksumForbidden},
		{name: "bad value", env: "yes", in: checksumPolicy{}, wantErr: ErrSecurePluginsValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.EnvSecurePlugins, tt.env)

			got, err := applySecurePlugins(tt.in)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInstall_SecurePluginsRequiresChecksumFile(t *testing.T) {
	t.Setenv(config.EnvSecurePlugins, "true")
	pluginDir := t.TempDir()
	installer := policyInstaller(t, pluginDir, policyRelease{
		plugin:       "aws-public",
		version:      "v1.0.0",
		omitChecksum: true,
	})

	_, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{
		AcceptMissingChecksum: true,
		NoSave:                true,
	}, nil)

	require.ErrorIs(t, err, ErrChecksumUnavailable)
	_, statErr := os.Stat(filepath.Join(pluginDir, "aws-public", "v1.0.0"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestInstall_SecurePluginsOffKeepsWarnAndContinue(t *testing.T) {
	t.Setenv(config.EnvSecurePlugins, "false")
	installer := policyInstaller(t, t.TempDir(), policyRelease{
		plugin:       "aws-public",
		version:      "v1.0.0",
		omitChecksum: true,
	})

	result, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{NoSave: true}, nil)

	require.NoError(t, err)
	assert.Equal(t, "v1.0.0", result.Version)
}

func TestInstall_SecurePluginsRefusesSkipChecksum(t *testing.T) {
	t.Setenv(config.EnvSecurePlugins, "true")
	installer := policyInstaller(t, t.TempDir(), policyRelease{plugin: "aws-public", version: "v1.0.0"})

	_, err := installer.Install(context.Background(), "aws-public@v1.0.0", InstallOptions{
		SkipChecksum: true,
		NoSave:       true,
	}, nil)

	require.ErrorIs(t, err, ErrSkipChecksumForbidden)
}

func TestUpdate_SecurePluginsRefusesSkipChecksum(t *testing.T) {
	t.Setenv(config.EnvSecurePlugins, "true")
	installer := policyInstaller(t, t.TempDir(), policyRelease{plugin: "aws-public", version: "v1.0.0"})

	_, err := installer.Update(context.Background(), "aws-public", UpdateOptions{SkipChecksum: true}, nil)

	require.ErrorIs(t, err, ErrSkipChecksumForbidden)
}

func TestCheckSecurePlugins(t *testing.T) {
	t.Setenv(config.EnvSecurePlugins, "true")
	require.NoError(t, CheckSecurePlugins(false))
	require.ErrorIs(t, CheckSecurePlugins(true), ErrSkipChecksumForbidden)
}
