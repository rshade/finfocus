package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList_ShowsDatabases(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedHistory(t, dir, "dev", viewSnap(1, time.January, 10, map[string]float64{"aws": 10}))
	seedHistory(t, dir, "staging", viewSnap(2, time.March, 20, map[string]float64{"aws": 20}))

	cmd, stdout := preparedHistoryCmd(t, NewCostHistoryListCmd())
	require.NoError(t, runList(cmd, dir))
	out := stdout.String()
	assert.Contains(t, out, "dev")
	assert.Contains(t, out, "staging")
	assert.Contains(t, out, "Snapshots")
	assert.Contains(t, out, "Size")

	empty, emptyOut := preparedHistoryCmd(t, NewCostHistoryListCmd())
	require.NoError(t, runList(empty, t.TempDir()))
	assert.Contains(t, emptyOut.String(), "No cost history databases.")

	jsonCmd, jsonOut := preparedHistoryCmd(t, NewCostHistoryListCmd(), "--format", "json")
	require.NoError(t, runList(jsonCmd, dir))
	body := jsonOut.String()
	assert.Contains(t, body, `"stack": "dev"`)
	assert.Contains(t, body, `"snapshots"`)
	assert.Contains(t, body, `"size"`)
}

func TestCostHistoryCommands(t *testing.T) {
	t.Parallel()
	cmd := NewCostHistoryCmd()
	names := map[string]bool{}
	for _, child := range cmd.Commands() {
		names[child.Name()] = true
	}
	assert.True(t, names["collect"])
	assert.True(t, names["view"])
	assert.True(t, names["list"])
	assert.True(t, names["prune"])
	assert.True(t, names["diff"])
	assert.True(t, names["export"])
	assert.Nil(t, cmd.RunE)
}
