// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintOverviewSummaryLine(t *testing.T) {
	t.Parallel()

	const (
		resources = 3
		changes   = 1
		plugins   = 2
	)

	tests := []struct {
		name        string
		skip        bool
		terminal    bool
		input       string
		hasChanges  bool
		wantProceed bool
		wantOut     []string
		notWant     []string
	}{
		{
			name:        "yes skips the prompt and still prints the summary",
			skip:        true,
			terminal:    true,
			input:       "n\n",
			hasChanges:  true,
			wantProceed: true,
			wantOut:     []string{"Overview: 3 resources, 1 pending changes, 2 plugins\n"},
			notWant:     []string{"Continue?", "Cancelled."},
		},
		{
			name:        "non-tty skips the prompt without reading stdin",
			terminal:    false,
			input:       "n\n",
			wantProceed: true,
			wantOut:     []string{"Overview: 3 resources, 2 plugins\n"},
			notWant:     []string{"Continue?", "Cancelled.", "pending"},
		},
		{
			name:        "enter continues",
			terminal:    true,
			input:       "\n",
			hasChanges:  true,
			wantProceed: true,
			wantOut:     []string{"Continue? [Y/n] ", "Overview: 3 resources, 1 pending changes, 2 plugins\n"},
			notWant:     []string{"Cancelled."},
		},
		{
			name:        "y continues",
			terminal:    true,
			input:       "y\n",
			wantProceed: true,
			wantOut:     []string{"Continue? [Y/n] "},
			notWant:     []string{"Cancelled."},
		},
		{
			name:        "n cancels",
			terminal:    true,
			input:       "n\n",
			wantProceed: false,
			wantOut:     []string{"Cancelled."},
		},
		{
			name:        "uppercase N cancels",
			terminal:    true,
			input:       "N\n",
			wantProceed: false,
			wantOut:     []string{"Cancelled."},
		},
		{
			name:        "no cancels",
			terminal:    true,
			input:       "no\n",
			wantProceed: false,
			wantOut:     []string{"Cancelled."},
		},
		{
			name:        "eof continues",
			terminal:    true,
			input:       "",
			wantProceed: true,
			wantOut:     []string{"Continue? [Y/n] "},
			notWant:     []string{"Cancelled."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out strings.Builder
			proceed, err := printOverviewSummaryLine(
				&out,
				strings.NewReader(tt.input),
				tt.skip,
				tt.terminal,
				resources,
				tt.hasChanges,
				changes,
				plugins,
			)
			require.NoError(t, err)
			assert.Equal(t, tt.wantProceed, proceed)
			got := out.String()
			for _, want := range tt.wantOut {
				assert.Contains(t, got, want)
			}
			for _, notWant := range tt.notWant {
				assert.NotContains(t, got, notWant)
			}
		})
	}
}

func TestOverviewInteractive_RespectsWriterAndFlags(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	table := overviewParams{output: outputFormatTable}
	assert.False(t, overviewInteractive(&buf, table))

	forced := table
	forced.forceColor = true
	assert.False(t, overviewInteractive(&buf, forced), "force-color styles text; it does not launch the TUI")

	plain := table
	plain.plain = true
	assert.False(t, overviewInteractive(&buf, plain))

	noColor := table
	noColor.noColor = true
	noColor.forceColor = true
	assert.False(t, overviewInteractive(&buf, noColor))

	jsonOut := overviewParams{output: outputFormatJSON, forceColor: true}
	assert.False(t, overviewInteractive(os.Stdout, jsonOut))
}

func TestStdinIsTerminal_NonFile(t *testing.T) {
	t.Parallel()
	assert.False(t, stdinIsTerminal(strings.NewReader("")))
}

func TestStdinIsTerminal_DevNull(t *testing.T) {
	t.Parallel()
	f, err := os.Open("/dev/null")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	assert.False(t, stdinIsTerminal(f))
}
