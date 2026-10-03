package detect

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPulumi_Available(t *testing.T) {
	t.Parallel()
	bin, version, err := Detect(context.Background(),
		func(name string) (string, error) {
			assert.Equal(t, "pulumi", name)
			return "/usr/bin/pulumi", nil
		},
		func(_ context.Context, name string, args ...string) ([]byte, error) {
			assert.Equal(t, "/usr/bin/pulumi", name)
			assert.Equal(t, []string{"version"}, args)
			return []byte("v3.218.0\n"), nil
		},
	)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/pulumi", bin)
	assert.Equal(t, "3.218.0", version)
}

func TestDetectPulumi_NotFound(t *testing.T) {
	t.Parallel()
	_, _, err := Detect(context.Background(),
		func(string) (string, error) { return "", errors.New("missing") },
		func(context.Context, string, ...string) ([]byte, error) {
			t.Fatal("run should not be called")
			return nil, nil
		},
	)
	require.ErrorIs(t, err, ErrPulumiMissing)
	require.ErrorContains(t, err, "https://www.pulumi.com/docs/install/")
}

func TestDetectPulumi_VersionParsing(t *testing.T) {
	t.Parallel()
	version, err := ParsePulumiVersion("v3.100.1\nextra")
	require.NoError(t, err)
	assert.Equal(t, "3.100.1", version)
	_, err = ParsePulumiVersion("not-a-version")
	require.Error(t, err)
}

func TestDetectPulumi_MinVersion(t *testing.T) {
	t.Parallel()
	_, err := ParsePulumiVersion("v2.9.0")
	require.ErrorContains(t, err, "v3.0.0 or newer")
	version, err := ParsePulumiVersion("3.0.0")
	require.NoError(t, err)
	assert.Equal(t, "3.0.0", version)
}

func TestDetectPulumi_RunError(t *testing.T) {
	t.Parallel()
	_, _, err := Detect(context.Background(),
		func(string) (string, error) { return "pulumi", nil },
		func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("boom")
		},
	)
	require.ErrorContains(t, err, "running pulumi version")
}

func TestDefaultRun_Printf(t *testing.T) {
	t.Parallel()
	out, err := defaultRun(context.Background(), "printf", "%s", "ok")
	require.NoError(t, err)
	assert.Equal(t, "ok", string(out))
	_, err = defaultRun(context.Background(), "printf-missing-binary", "x")
	require.Error(t, err)
}
