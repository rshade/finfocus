package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// failingReader fails the test if the plain overview path reads stdin.
type failingReader struct {
	t *testing.T
}

func (r failingReader) Read([]byte) (int, error) {
	assert.Fail(r.t, "plain overview read stdin")
	return 0, io.EOF
}

func TestOverviewPlain_NoWebHint(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cmd, out := overviewCmd(strings.NewReader(""))
	params := overviewParams{
		pulumiState: overviewFixture(t, "state-no-changes.json"),
		output:      outputFormatTable,
		yes:         true,
		stateOnly:   true,
	}
	require.False(t, overviewInteractive(out, params))

	state, _, stackName, stateOnly, err := testLoadPlainOverviewData(ctx, cmd, params)
	require.NoError(t, err)
	rows, _, _, _, err := testBuildOverviewRows(ctx, stateOnly, state, params, "", stackName, nil)
	require.NoError(t, err)
	result := engine.ComputeOverviewResult(rows, 15)
	require.NoError(t, renderOverviewOutput(cmd, params.output, result, engine.StackContext{StackName: stackName}, nil))

	got := out.String()
	assert.NotEmpty(t, got)
	assert.NotContains(t, got, "[w]")
	assert.NotContains(t, got, "finfocus --web")
}

// Sequential: subtests set NO_COLOR and TERM.
func TestOverviewInteractive_NonInteractiveInputs(t *testing.T) {
	tests := []struct {
		name   string
		params overviewParams
		writer func(t *testing.T) io.Writer
		env    map[string]string
	}{
		{
			name:   "plain",
			params: overviewParams{output: outputFormatTable, plain: true},
			writer: func(*testing.T) io.Writer { return os.Stdout },
		},
		{
			name:   "no-color",
			params: overviewParams{output: outputFormatTable, noColor: true},
			writer: func(*testing.T) io.Writer { return os.Stdout },
		},
		{
			name:   "NO_COLOR",
			params: overviewParams{output: outputFormatTable, noColor: true},
			writer: func(*testing.T) io.Writer { return os.Stdout },
			env:    map[string]string{"NO_COLOR": "1"},
		},
		{
			name:   "TERM=dumb",
			params: overviewParams{output: outputFormatTable},
			writer: terminalOrStdout,
			env:    map[string]string{"TERM": "dumb"},
		},
		{
			name:   "non-terminal writer",
			params: overviewParams{output: outputFormatTable},
			writer: func(*testing.T) io.Writer { return &bytes.Buffer{} },
		},
		{
			name:   "non-table output",
			params: overviewParams{output: outputFormatJSON},
			writer: func(*testing.T) io.Writer { return os.Stdout },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.env {
				t.Setenv(key, value)
			}
			assert.False(t, overviewInteractive(tt.writer(t), tt.params))
		})
	}
}

func terminalOrStdout(t *testing.T) io.Writer {
	t.Helper()
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		t.Logf("no controlling terminal (%v); TERM=dumb is covered by the non-terminal writer", err)
		return os.Stdout
	}
	t.Cleanup(func() { require.NoError(t, tty.Close()) })
	return tty
}

func TestOverviewPlain_ReadsNoInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	cmd, out := overviewCmd(failingReader{t: t})
	params := overviewParams{
		pulumiState: overviewFixture(t, "state-no-changes.json"),
		output:      outputFormatTable,
		yes:         true,
		stateOnly:   true,
	}
	require.False(t, overviewInteractive(out, params))

	state, steps, stackName, stateOnly, err := testLoadPlainOverviewData(ctx, cmd, params)
	require.NoError(t, err)
	rows, _, hasChanges, changeCount, err := testBuildOverviewRows(ctx, stateOnly, state, params, "", stackName, nil)
	require.NoError(t, err)
	_, err = confirmPlainOverview(cmd, params, len(rows), hasChanges, changeCount, 0, stateOnly)
	require.NoError(t, err)
	result := engine.ComputeOverviewResult(rows, 15)
	require.NoError(t, renderOverviewOutput(cmd, params.output, result, engine.StackContext{StackName: stackName}, nil))
	assert.Empty(t, steps)
	assert.NotContains(t, out.String(), "[w]")
}

func TestWebFlagsFromOverviewParams(t *testing.T) {
	t.Parallel()

	params := overviewParams{
		pulumiJSON:  "plan.json",
		pulumiState: "state.json",
		stack:       "dev",
		fromStr:     "2026-01-01",
		toStr:       "2026-01-31",
		adapter:     "aws",
		filter:      []string{"provider=aws", "type=aws:s3/bucket:Bucket"},
	}
	got := webFlagsFromOverview(params)
	require.NotNil(t, got)
	assert.Equal(t, &webFlagValues{
		web:         true,
		port:        0,
		noBrowser:   false,
		pulumiJSON:  "plan.json",
		pulumiState: "state.json",
		stack:       "dev",
		from:        "2026-01-01",
		to:          "2026-01-31",
		adapter:     "aws",
		filter:      []string{"provider=aws", "type=aws:s3/bucket:Bucket"},
	}, got)
}

//nolint:paralleltest // swaps the web start seam
func TestInteractiveOverview_HandsOffToWebAfterCleanup(t *testing.T) {
	original := startWebSession
	t.Cleanup(func() { startWebSession = original })

	params := overviewParams{
		pulumiJSON:  "plan.json",
		pulumiState: "state.json",
		stack:       "dev",
		fromStr:     "2026-01-01",
		toStr:       "2026-01-31",
		adapter:     "aws",
		filter:      []string{"provider=aws"},
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	audit := newAuditContext(context.Background(), "overview", nil)

	t.Run("requested", func(t *testing.T) {
		var order []string
		var got *webFlagValues
		startWebSession = func(_ *cobra.Command, flags *webFlagValues) error {
			order = append(order, "web")
			got = flags
			return nil
		}
		model := overviewModelRequestingWeb(t)
		err := completeInteractiveOverview(
			context.Background(), cmd, params, audit, model, nil, 1, time.Now(),
			closedEnrichDone(), cleanupThatRecords(&order),
		)
		require.NoError(t, err)
		assert.Equal(t, []string{"cleanup", "web"}, order)
		require.NotNil(t, got)
		assert.True(t, got.web)
		assert.Equal(t, 0, got.port)
		assert.False(t, got.noBrowser)
		assert.Equal(t, params.pulumiJSON, got.pulumiJSON)
		assert.Equal(t, params.pulumiState, got.pulumiState)
		assert.Equal(t, params.stack, got.stack)
		assert.Equal(t, params.fromStr, got.from)
		assert.Equal(t, params.toStr, got.to)
		assert.Equal(t, params.adapter, got.adapter)
		assert.Equal(t, params.filter, got.filter)
	})

	t.Run("not requested", func(t *testing.T) {
		called := false
		startWebSession = func(*cobra.Command, *webFlagValues) error {
			called = true
			return nil
		}
		model, _ := tui.NewOverviewModel(context.Background(), nil, 0, nil, nil)
		err := completeInteractiveOverview(
			context.Background(), cmd, params, audit, model, nil, 0, time.Now(),
			closedEnrichDone(), cleanupThatRecords(nil),
		)
		require.NoError(t, err)
		assert.False(t, called)
		assert.False(t, model.WebRequested())
	})
}

//nolint:paralleltest // swaps the web start seam
func TestInteractiveOverview_WebStartErrorPropagates(t *testing.T) {
	original := startWebSession
	t.Cleanup(func() { startWebSession = original })

	sentinel := errors.New("listen tcp 127.0.0.1:0: bind: address already in use")
	startWebSession = func(*cobra.Command, *webFlagValues) error { return sentinel }

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	audit := newAuditContext(context.Background(), "overview", nil)
	err := completeInteractiveOverview(
		context.Background(), cmd, overviewParams{}, audit, overviewModelRequestingWeb(t), nil, 0, time.Now(),
		closedEnrichDone(), cleanupThatRecords(nil),
	)
	require.ErrorIs(t, err, sentinel)
	assert.ErrorContains(t, err, "running web session")
}

func overviewModelRequestingWeb(t *testing.T) tui.OverviewModel {
	t.Helper()
	model, _ := tui.NewOverviewModel(context.Background(), []engine.OverviewRow{
		{URN: "urn:test", Type: "aws:ec2:Instance", Status: engine.StatusActive},
	}, 1, nil, nil)
	updated, _ := model.Update(tui.OverviewAllResourcesLoadedMsg{})
	model = updated.(tui.OverviewModel)
	updated, _ = model.Update(tea.KeyPressMsg{Text: "w"})
	got, ok := updated.(tui.OverviewModel)
	require.True(t, ok)
	require.True(t, got.WebRequested())
	return got
}

func closedEnrichDone() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func cleanupThatRecords(order *[]string) <-chan func() {
	ch := make(chan func(), 1)
	ch <- func() {
		if order != nil {
			*order = append(*order, "cleanup")
		}
	}
	return ch
}
