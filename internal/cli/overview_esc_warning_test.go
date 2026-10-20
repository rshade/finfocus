package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type escReleased struct{}

type escHoldModel struct {
	started chan struct{}
	release <-chan struct{}
}

func (m escHoldModel) Init() tea.Cmd {
	return func() tea.Msg {
		close(m.started)
		<-m.release
		return escReleased{}
	}
}

func (m escHoldModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(escReleased); ok {
		return m, tea.Quit
	}
	return m, nil
}

func (escHoldModel) View() tea.View { return tea.NewView("") }

func TestOverviewDefersESCWarningsUntilQuit(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	started := make(chan struct{})
	var stderr bytes.Buffer
	cmd := &cobra.Command{Use: "overview"}
	cmd.SetErr(&stderr)
	sink := newESCWarnSink(cmd)
	ctx := contextWithESCWarn(t.Context(), sink)
	HoldESCWarnings(ctx)

	program := tea.NewProgram(
		escHoldModel{started: started, release: release},
		tea.WithoutRenderer(),
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)
	done := make(chan error, 1)
	go func() {
		_, err := program.Run()
		done <- err
	}()

	<-started
	sink.Warn("ESC environment acme/billing/prod skipped (not_found); plugins use shell and config values")
	assert.Empty(t, stderr.String())
	close(release)
	require.NoError(t, <-done)
	assert.Empty(t, stderr.String())
	ReleaseESCWarnings(ctx)
	assert.Equal(t, 1, strings.Count(stderr.String(), "Warning: ESC"))
	assert.Contains(t, stderr.String(), "acme/billing/prod")
}
