package stackload

import (
	"context"
	"errors"

	"github.com/rshade/finfocus/internal/pulumi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"os"
	"path/filepath"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, string, []string, ...string) ([]byte, []byte, error)

func (f runnerFunc) Run(c context.Context, d, n string, e []string, a ...string) ([]byte, []byte, error) {
	return f(c, d, n, e, a...)
}

const stateJSON = `{"version":3,"deployment":{"resources":[{"urn":"urn:a","type":"aws:s3/bucket:Bucket","custom":true}]}}`
const planJSON = `{"steps":[{"urn":"urn:c","type":"aws:s3/bucket:Bucket","op":"create"}]}`

func inputFile(t *testing.T, name, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(data), 0600))
	return p
}

//nolint:paralleltest // Replaces the process-wide Pulumi runner.
func TestLoadFiles(t *testing.T) {
	t.Cleanup(
		pulumi.SetRunnerForTest(
			runnerFunc(func(context.Context, string, string, []string, ...string) ([]byte, []byte, error) {
				assert.Fail(t, "explicit file invoked subprocess")
				return nil, nil, errors.New("forbidden")
			}),
		),
	)
	src := Source{StatePath: inputFile(t, "state.json", stateJSON), PlanPath: inputFile(t, "plan.json", planJSON)}
	s, err := Load(context.Background(), src)
	require.NoError(t, err)
	assert.Len(t, s.Descriptors(), 2)
	s, err = LoadState(context.Background(), src)
	require.NoError(t, err)
	assert.Nil(t, s.Plan)
	assert.Len(t, s.Descriptors(), 1)
	s, err = LoadPlan(context.Background(), src)
	require.NoError(t, err)
	assert.Empty(t, s.State)
	assert.Equal(t, ReasonNotRequested, s.StateStatus.Reason)
	s, err = Load(context.Background(), Source{StatePath: src.StatePath})
	require.NoError(t, err)
	assert.Nil(t, s.Plan)
	_, err = Load(context.Background(), Source{})
	require.Error(t, err)
	plan := src.PlanPath
	for _, src := range []Source{
		{StatePath: "missing"},
		{PlanPath: "missing"},
		{StatePath: inputFile(t, "bad", "{")},
		{PlanPath: inputFile(t, "bad", "{")},
		{StatePath: "missing", PlanPath: plan},
		{StatePath: inputFile(t, "bad", "{"), PlanPath: plan},
	} {
		_, err = Load(context.Background(), src)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ReasonReadFailed)
	}
}

func TestLoadConcurrentAndPassphrase(t *testing.T) {
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "parent")
	started := make(chan string, 2)
	release := make(chan struct{})
	secret := "child"
	t.Cleanup(
		pulumi.SetRunnerForTest(
			runnerFunc(func(ctx context.Context, dir, _ string, env []string, args ...string) ([]byte, []byte, error) {
				assert.Equal(t, "project", dir)
				assert.Contains(t, env, "PULUMI_CONFIG_PASSPHRASE=child")
				assert.Contains(t, args, "dev")
				started <- args[0]
				select {
				case <-release:
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				}
				if args[0] == "preview" {
					return []byte(planJSON), nil, nil
				}
				return []byte(stateJSON), nil, nil
			}),
		),
	)
	done := make(chan error, 1)
	go func() {
		_, err := Load(context.Background(), Source{ProjectDir: "project", Stack: "dev", Passphrase: &secret})
		done <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			require.FailNow(t, "state and preview did not start together")
		}
	}
	close(release)
	require.NoError(t, <-done)
	assert.Equal(t, "parent", os.Getenv("PULUMI_CONFIG_PASSPHRASE"))
}

//nolint:paralleltest // Parent and subtests replace the process-wide Pulumi runner.
func TestLoadDegradedAndFatalCancellation(t *testing.T) {
	t.Run("degraded", func(t *testing.T) {
		t.Cleanup(
			pulumi.SetRunnerForTest(
				runnerFunc(func(_ context.Context, _, _ string, _ []string, a ...string) ([]byte, []byte, error) {
					if a[0] == "preview" {
						return []byte(planJSON), nil, nil
					}
					return nil, []byte("PULUMI_CONFIG_PASSPHRASE secret-token"), errors.New("failed")
				}),
			),
		)
		s, err := Load(context.Background(), Source{ProjectDir: "project"})
		require.NoError(t, err)
		assert.False(t, s.StateStatus.Loaded)
		assert.Equal(t, ReasonPassphraseRequired, s.StateStatus.Reason)
		assert.Len(t, s.Descriptors(), 1)
	})
	t.Run("fatal", func(t *testing.T) {
		started := make(chan struct{})
		cancelled := make(chan struct{})
		t.Cleanup(
			pulumi.SetRunnerForTest(
				runnerFunc(func(ctx context.Context, _, _ string, _ []string, a ...string) ([]byte, []byte, error) {
					if a[0] == "preview" {
						<-started
						return nil, []byte("login https://private token=abc"), errors.New("failed")
					}
					close(started)
					<-ctx.Done()
					close(cancelled)
					return nil, nil, ctx.Err()
				}),
			),
		)
		_, err := Load(context.Background(), Source{ProjectDir: "project"})
		require.Error(t, err)
		require.ErrorIs(t, err, pulumi.ErrPreviewFailed, "a failed preview keeps Pulumi's error, as before")
		assert.Contains(t, err.Error(), "login https://private")
		select {
		case <-cancelled:
		default:
			require.FailNow(t, "sibling not cancelled")
		}
	})
	t.Run("state failure", func(t *testing.T) {
		t.Cleanup(
			pulumi.SetRunnerForTest(
				runnerFunc(func(context.Context, string, string, []string, ...string) ([]byte, []byte, error) {
					return nil, []byte("unreachable https://private"), errors.New("failed")
				}),
			),
		)
		_, err := LoadState(context.Background(), Source{ProjectDir: "project"})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), "https://")
	})
}

//nolint:paralleltest // Replaces the process-wide Pulumi runner.
func TestLoadParentCancellationPreservesSentinel(t *testing.T) {
	started := make(chan struct{}, 2)
	t.Cleanup(
		pulumi.SetRunnerForTest(
			runnerFunc(func(ctx context.Context, _, _ string, _ []string, _ ...string) ([]byte, []byte, error) {
				started <- struct{}{}
				<-ctx.Done()
				return nil, []byte("https://secret.example token=abc"), ctx.Err()
			}),
		),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := Load(ctx, Source{ProjectDir: "project"})
		result <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			require.FailNow(t, "both inputs must start")
		}
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
		assert.NotContains(t, err.Error(), "https://")
		assert.NotContains(t, err.Error(), "token=")
	case <-time.After(time.Second):
		require.FailNow(t, "load did not stop after cancellation")
	}
}

func TestExplicitInputErrorsRemainUsefulAndRedacted(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"plan", "state"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			missing := filepath.Join(t.TempDir(), "missing.json")
			invalid := inputFile(t, "invalid.json", `{"secret":"token=abc https://private.example", INVALID`)
			load := LoadPlan
			source := func(path string) Source { return Source{PlanPath: path} }
			if kind == "state" {
				load = LoadState
				source = func(path string) Source { return Source{StatePath: path} }
			}
			_, err := load(context.Background(), source(missing))
			require.Error(t, err)
			assert.Contains(t, err.Error(), missing)
			assert.Contains(t, err.Error(), "no such file")
			missingErr := err
			require.NoError(t, errors.Unwrap(err))
			_, err = load(context.Background(), source(invalid))
			require.Error(t, err)
			assert.Contains(t, err.Error(), invalid)
			assert.Contains(t, err.Error(), "invalid JSON")
			assert.NotContains(t, err.Error(), "token=abc")
			assert.NotContains(t, err.Error(), "https://")
			require.NoError(t, errors.Unwrap(err))
			require.ErrorIs(t, missingErr, os.ErrNotExist)
		})
	}
}

func TestLoadComponentStepIsNotMismatch(t *testing.T) {
	t.Parallel()
	state := `{"version":3,"deployment":{"resources":[` +
		`{"urn":"urn:a","type":"aws:s3/bucket:Bucket","custom":true},` +
		`{"urn":"urn:node","type":"example:index:Node","custom":false}]}}`
	plan := `{"steps":[` +
		`{"urn":"urn:a","type":"aws:s3/bucket:Bucket","op":"same"},` +
		`{"urn":"urn:node","type":"example:index:Node","op":"same"},` +
		`{"urn":"urn:gone","type":"example:index:Node","op":"same"}]}`
	s, err := Load(
		context.Background(),
		Source{StatePath: inputFile(t, "state.json", state), PlanPath: inputFile(t, "plan.json", plan)},
	)
	require.NoError(t, err)
	assert.Equal(t, []string{"urn:gone"}, s.Mismatches, "a component in state is not missing from it")
}
