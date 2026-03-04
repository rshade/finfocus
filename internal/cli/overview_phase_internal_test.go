package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/tui"
)

const goosWindows = "windows"

type testNoopModel struct{}

func (testNoopModel) Init() tea.Cmd { return nil }

func (m testNoopModel) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return m, nil }

func (testNoopModel) View() tea.View { return tea.NewView("") }

type stackLSMockRunner struct {
	t      *testing.T
	calls  int
	stack  string
	stderr []byte
}

func (m *stackLSMockRunner) Run(
	_ context.Context,
	_ string,
	name string,
	_ []string,
	args ...string,
) ([]byte, []byte, error) {
	m.calls++
	require.Equal(m.t, "pulumi", name)
	if len(args) >= 3 && args[0] == "stack" && args[1] == "ls" && args[2] == "--json" {
		out := fmt.Sprintf(`[{"name":"%s","current":true}]`, m.stack)
		return []byte(out), nil, nil
	}
	return nil, m.stderr, fmt.Errorf("unexpected pulumi args: %v", args)
}

// TestResolveIsStateOnly verifies that detectErr does not override --yes, and that
// without --yes a detection error correctly forces state-only mode.
func TestResolveIsStateOnly(t *testing.T) {
	detectErr := errors.New("change detection failed")
	noErr := error(nil)

	tests := []struct {
		name        string
		params      overviewParams
		signal      pulumidetect.ChangeSignal
		detectErr   error
		wantIsState bool
	}{
		{
			name:        "yes=true, no error: preview runs",
			params:      overviewParams{yes: true},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   noErr,
			wantIsState: false,
		},
		{
			name:        "yes=true, detect error: preview must still run (bug fix)",
			params:      overviewParams{yes: true},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   detectErr,
			wantIsState: false,
		},
		{
			name:        "yes=false, no error, no changes: state-only",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: false},
			detectErr:   noErr,
			wantIsState: true,
		},
		{
			name:        "yes=false, no error, likely changes: preview runs",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   noErr,
			wantIsState: false,
		},
		{
			name:        "yes=false, detect error: falls back to state-only",
			params:      overviewParams{yes: false},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   detectErr,
			wantIsState: true,
		},
		{
			name:        "explicit state-only (pulumi-state set, no pulumi-json, no yes): state-only",
			params:      overviewParams{yes: false, pulumiState: "state.json", pulumiJSON: ""},
			signal:      pulumidetect.ChangeSignal{HasLikelyChanges: true},
			detectErr:   noErr,
			wantIsState: true,
		},
		{
			name:        "explicit state-only overridden by yes: preview runs",
			params:      overviewParams{yes: true, pulumiState: "state.json", pulumiJSON: ""},
			signal:      pulumidetect.ChangeSignal{},
			detectErr:   noErr,
			wantIsState: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIsStateOnly(tt.params, tt.signal, tt.detectErr)
			assert.Equal(t, tt.wantIsState, got)
		})
	}
}

// TestPhaseConstantsAlignWithTUI ensures the phase index constants stay in sync
// with tui.PhaseNames. If PhaseNames grows or shrinks, this test fails, forcing
// an explicit update to both the constants and the slice.
func TestPhaseConstantsAlignWithTUI(t *testing.T) {
	n := len(tui.GetPhaseNames())
	require.Greater(t, n, 0, "tui.GetPhaseNames() must not be empty")

	assert.Less(t, phaseEnrichResources, n,
		"phaseEnrichResources (%d) must be a valid index into tui.PhaseNames (len=%d)",
		phaseEnrichResources, n)
	assert.Equal(t, n-1, phaseEnrichResources,
		"phaseEnrichResources should be the last phase index; update the constant if PhaseNames changed")
	assert.Equal(t, 0, phaseLoadStackState,
		"phaseLoadStackState should be the first phase index")
}

// ---------------------------------------------------------------------------
// convertStateResources — CreatedAt propagation
// ---------------------------------------------------------------------------

func TestConvertStateResources_CreatedAtPreserved(t *testing.T) {
	createdAt := time.Date(2025, 2, 13, 10, 0, 0, 0, time.UTC)

	resources := []ingest.StackExportResource{
		{
			URN:     "urn:pulumi:prod::app::aws:ebs:Volume::data",
			Type:    "aws:ebs:Volume",
			ID:      "vol-abc123",
			Custom:  true,
			Created: &createdAt,
		},
	}

	result := convertStateResources(resources)
	require.Len(t, result, 1)
	require.NotNil(t, result[0].CreatedAt)
	assert.Equal(t, createdAt, *result[0].CreatedAt)
}

func TestConvertStateResources_NilCreatedAtOK(t *testing.T) {
	resources := []ingest.StackExportResource{
		{
			URN:     "urn:pulumi:prod::app::aws:ec2:Instance::web",
			Type:    "aws:ec2:Instance",
			ID:      "i-abc123",
			Custom:  true,
			Created: nil,
		},
	}

	result := convertStateResources(resources)
	require.Len(t, result, 1)
	assert.Nil(t, result[0].CreatedAt)
}

func TestStackSettingsNameCandidates_QualifiedStack(t *testing.T) {
	candidates := stackSettingsNameCandidates("acme/infra/dev")
	require.Equal(t, []string{"dev", "acme/infra/dev"}, candidates)
}

func TestStackSettingsNameCandidates_QualifiedStack_Windows(t *testing.T) {
	candidates := stackSettingsNameCandidates(`acme\infra\dev`)
	require.Equal(t, []string{"dev", `acme\infra\dev`}, candidates)
}

func TestReadStackSettingsFile_QualifiedStackUsesShortName(t *testing.T) {
	tmpDir := t.TempDir()
	want := "encryptionsalt: v1:abc123\n"
	require.NoError(
		t,
		os.WriteFile(filepath.Join(tmpDir, "Pulumi.dev.yaml"), []byte(want), 0o600),
	)

	got, err := readStackSettingsFile(tmpDir, "acme/infra/dev")
	require.NoError(t, err)
	assert.Equal(t, want, string(got))
}

func TestCheckAndPromptPassphrase_QualifiedStackEndToEnd(t *testing.T) {
	// Ensure passphrase env vars are truly unset; empty values still count as "set".
	unsetEnv := func(key string) {
		orig, ok := os.LookupEnv(key)
		require.NoError(t, os.Unsetenv(key))
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, orig)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	unsetEnv("PULUMI_CONFIG_PASSPHRASE")
	unsetEnv("PULUMI_CONFIG_PASSPHRASE_FILE")

	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Make FindBinary() succeed without depending on a system Pulumi install.
	pulumiName := "pulumi"
	if runtime.GOOS == goosWindows {
		pulumiName += ".exe"
	}
	fakePulumi := filepath.Join(tmpDir, pulumiName)
	require.NoError(t, os.WriteFile(fakePulumi, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Make FindProject() and passphrase detection use local fixture files.
	require.NoError(
		t,
		os.WriteFile(filepath.Join(tmpDir, "Pulumi.yaml"), []byte("name: test\nruntime: yaml\n"), 0o600),
	)
	require.NoError(
		t,
		os.WriteFile(filepath.Join(tmpDir, "Pulumi.dev.yaml"), []byte("encryptionsalt: v1:abc123\n"), 0o600),
	)

	mockRunner := &stackLSMockRunner{t: t, stack: "acme/infra/dev"}
	t.Cleanup(pulumidetect.SetRunnerForTest(mockRunner))

	// Use a canceled program context so p.Send() is non-blocking in tests.
	progCtx, cancelProgram := context.WithCancel(context.Background())
	cancelProgram()
	p := tea.NewProgram(
		testNoopModel{},
		tea.WithContext(progCtx),
		tea.WithoutRenderer(),
		tea.WithInput(nil),
		tea.WithOutput(io.Discard),
	)

	passphraseChan := make(chan string, 1)
	passphraseChan <- "stack-secret"

	pw, err := checkAndPromptPassphrase(context.Background(), p, overviewParams{}, passphraseChan)
	require.NoError(t, err)
	require.NotNil(t, pw)
	assert.Equal(t, "stack-secret", *pw)
	assert.GreaterOrEqual(t, mockRunner.calls, 1)
}
