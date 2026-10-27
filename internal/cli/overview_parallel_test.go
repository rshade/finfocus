package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

const parallelTestTimeout = 5 * time.Second

var errFakePulumi = errors.New("fake pulumi failure")

// gatedPulumi is a pulumi runner whose export and preview calls run under test control.
// Each call counts itself in started, then follows its own behavior.
type gatedPulumi struct {
	export  []byte
	preview []byte

	exportFails  bool
	previewFails bool

	// rendezvous makes each call wait until both have started, so a sequential
	// caller can never finish.
	rendezvous bool
	// blockUntilCancel makes a call that does not fail wait for its context.
	blockUntilCancel bool

	started         sync.WaitGroup
	exportCanceled  atomic.Bool
	previewCanceled atomic.Bool
}

func newGatedPulumi(export, preview []byte) *gatedPulumi {
	g := &gatedPulumi{export: export, preview: preview}
	g.started.Add(2)
	return g
}

func (g *gatedPulumi) Run(
	ctx context.Context, _, name string, _ []string, args ...string,
) ([]byte, []byte, error) {
	if name != "pulumi" {
		return nil, []byte(name), errors.New("unexpected command")
	}
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "stack ls"):
		return []byte(`[{"name":"dev","current":true}]`), nil, nil
	case strings.Contains(joined, "stack export"):
		return g.call(ctx, g.export, g.exportFails, &g.exportCanceled)
	case strings.Contains(joined, "preview"):
		return g.call(ctx, g.preview, g.previewFails, &g.previewCanceled)
	default:
		return nil, []byte(joined), errors.New("unexpected pulumi args")
	}
}

func (g *gatedPulumi) call(
	ctx context.Context, out []byte, fails bool, canceled *atomic.Bool,
) ([]byte, []byte, error) {
	g.started.Done()
	if g.rendezvous {
		both := make(chan struct{})
		go func() {
			g.started.Wait()
			close(both)
		}()
		select {
		case <-both:
		case <-time.After(parallelTestTimeout):
			return nil, []byte("the other pulumi command never started"), errors.New("not concurrent")
		}
	}
	if fails {
		return nil, []byte("boom"), errFakePulumi
	}
	if g.blockUntilCancel {
		select {
		case <-ctx.Done():
			canceled.Store(true)
			return nil, nil, ctx.Err()
		case <-time.After(parallelTestTimeout):
			return nil, []byte("context was never canceled"), errors.New("not canceled")
		}
	}
	return out, nil, nil
}

func setupParallelOverviewProject(t *testing.T, g *gatedPulumi) {
	t.Helper()
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	project := t.TempDir()
	t.Chdir(project)
	installFakePulumi(t)
	require.NoError(t, os.WriteFile(
		filepath.Join(project, "Pulumi.yaml"),
		[]byte("name: test\nruntime: yaml\n"),
		0o600,
	))
	t.Cleanup(pulumidetect.SetRunnerForTest(g))
}

func parallelOverviewFixtures(t *testing.T) ([]byte, []byte) {
	t.Helper()
	export, err := os.ReadFile(overviewFixture(t, "state-no-changes.json"))
	require.NoError(t, err)
	preview, err := os.ReadFile(overviewFixture(t, "plan-no-changes.json"))
	require.NoError(t, err)
	return export, preview
}

//nolint:paralleltest // changes the working directory and swaps the pulumi runner
func TestLoadOverviewFromAutoDetect_RunsExportAndPreviewConcurrently(t *testing.T) {
	export, preview := parallelOverviewFixtures(t)
	g := newGatedPulumi(export, preview)
	g.rendezvous = true
	setupParallelOverviewProject(t, g)

	state, steps, stack, err := testResolveOverviewData(t.Context(), overviewParams{stack: "dev"})

	require.NoError(t, err)
	assert.Equal(t, "dev", stack)
	assert.NotEmpty(t, state)
	assert.Empty(t, steps)
}

//nolint:paralleltest // changes the working directory and swaps the pulumi runner
func TestLoadOverviewFromAutoDetect_PlanFileSkipsPreview(t *testing.T) {
	export, _ := parallelOverviewFixtures(t)
	g := newGatedPulumi(export, nil)
	g.started.Done()
	setupParallelOverviewProject(t, g)

	state, _, _, err := testResolveOverviewData(t.Context(), overviewParams{
		stack:      "dev",
		pulumiJSON: overviewFixture(t, "plan-no-changes.json"),
	})

	require.NoError(t, err)
	assert.NotEmpty(t, state)
}

//nolint:paralleltest // changes the working directory and swaps the pulumi runner
func TestLoadOverviewFromAutoDetect_Errors(t *testing.T) {
	t.Run("export failure retains preview inventory", func(t *testing.T) {
		export, preview := parallelOverviewFixtures(t)
		g := newGatedPulumi(export, preview)
		g.exportFails = true
		g.rendezvous = true
		setupParallelOverviewProject(t, g)
		snapshot, err := loadOverviewSnapshot(t.Context(), overviewParams{stack: "dev"}, nil, false)
		require.NoError(t, err)
		require.NotNil(t, snapshot)
		assert.False(t, snapshot.StateStatus.Loaded)
		assert.True(t, snapshot.PlanStatus.Loaded)
		assert.False(t, g.previewCanceled.Load())
	})
	t.Run("preview failure cancels export", func(t *testing.T) {
		export, preview := parallelOverviewFixtures(t)
		g := newGatedPulumi(export, preview)
		g.previewFails = true
		g.rendezvous = true
		g.blockUntilCancel = true
		setupParallelOverviewProject(t, g)
		_, _, _, err := testResolveOverviewData(t.Context(), overviewParams{stack: "dev"})
		require.ErrorContains(t, err, "loading Pulumi plan")
		assert.True(t, g.exportCanceled.Load())
	})
}

//nolint:paralleltest // changes the working directory and swaps the pulumi runner
func TestLoadOverviewFromAutoDetect_ParentCancellationStopsBoth(t *testing.T) {
	export, preview := parallelOverviewFixtures(t)
	g := newGatedPulumi(export, preview)
	g.rendezvous = true
	g.blockUntilCancel = true
	setupParallelOverviewProject(t, g)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		g.started.Wait()
		cancel()
	}()

	_, _, _, err := testResolveOverviewData(ctx, overviewParams{stack: "dev"})

	require.ErrorIs(t, err, context.Canceled)
	assert.True(t, g.exportCanceled.Load())
	assert.True(t, g.previewCanceled.Load())
}
