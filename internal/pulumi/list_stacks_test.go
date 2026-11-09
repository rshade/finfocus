package pulumi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Not parallel: withMockRunner replaces the package-level runner.
func TestListStacks(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	mock := &mockRunner{stdout: []byte(
		`[{"name":"dev","current":false,"url":"file://~"},{"name":"prod","current":true,"url":"file://~"}]`)}
	withMockRunner(t, mock)

	stacks, err := ListStacks(context.Background(), "/project")

	require.NoError(t, err)
	assert.Equal(t, []StackInfo{
		{Name: "dev", URL: "file://~"},
		{Name: "prod", Current: true, URL: "file://~"},
	}, stacks)
	assert.Equal(t, "/project", mock.lastDir)
	assert.Equal(t, []string{"stack", "ls", "--json"}, mock.lastArgs)
}

//nolint:paralleltest // withMockRunner replaces the package-level runner.
func TestListStacks_Failures(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	t.Run("runner error is wrapped with stderr", func(t *testing.T) {
		withMockRunner(t, &mockRunner{stderr: []byte("not logged in"), err: errors.New("exit status 255")})

		_, err := ListStacks(context.Background(), "/project")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "running pulumi stack ls")
		assert.Contains(t, err.Error(), "not logged in")
	})

	t.Run("canceled context is preserved", func(t *testing.T) {
		withMockRunner(t, &mockRunner{err: errors.New("signal: killed")})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := ListStacks(ctx, "/project")

		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("malformed output", func(t *testing.T) {
		withMockRunner(t, &mockRunner{stdout: []byte(`{nope`)})

		_, err := ListStacks(context.Background(), "/project")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "parsing pulumi stack ls output")
	})
}

func TestValidateStacks(t *testing.T) {
	t.Parallel()
	available := []StackInfo{{Name: "dev"}, {Name: "prod", Current: true}, {Name: "tailscale"}}

	tests := []struct {
		name        string
		requested   []string
		errContains []string
	}{
		{name: "all present", requested: []string{"dev", "tailscale"}},
		{name: "no request", requested: nil},
		{name: "qualified name matches bare stack", requested: []string{"rshade/demo/dev"}},
		{name: "surrounding whitespace is ignored", requested: []string{" dev", "prod "}},
		{
			name:        "qualified name of an unknown stack",
			requested:   []string{"rshade/demo/nope"},
			errContains: []string{"rshade/demo/nope", "available: dev, prod, tailscale"},
		},
		{
			name:        "one unknown",
			requested:   []string{"dev", "stagging"},
			errContains: []string{"stagging", "available: dev, prod, tailscale"},
		},
		{
			name:        "several unknown",
			requested:   []string{"a", "dev", "b"},
			errContains: []string{"a, b", "available: dev, prod, tailscale"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateStacks(tt.requested, available)
			if len(tt.errContains) == 0 {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrStackNotFound)
			for _, want := range tt.errContains {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestValidateStacks_NoStacks(t *testing.T) {
	t.Parallel()

	err := ValidateStacks([]string{"dev"}, nil)

	require.ErrorIs(t, err, ErrStackNotFound)
	assert.Contains(t, err.Error(), "no stacks found")
}
