package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

// renamed gives a client an install name and alias that differ from the name
// it reports, as aws-public v0.2.5 does.
func renamed(client *pluginhost.Client) *pluginhost.Client {
	client.InstallName = "b-plugin"
	client.Aliases = []string{"b-legacy"}
	return client
}

// adapterSpellings are every name --adapter accepts for the renamed plugin.
var adapterSpellings = []string{"b-plugin", "b-legacy", "finfocus-plugin-b"} //nolint:gochecknoglobals // test fixture

func TestActualCostAdapterAcceptsEveryPluginName(t *testing.T) {
	t.Parallel()

	for _, adapter := range adapterSpellings {
		t.Run("batch path "+adapter, func(t *testing.T) {
			t.Parallel()
			from, to, _ := actualWindow()
			first, named := &actualCallRecorder{}, &actualCallRecorder{}
			eng := New([]*pluginhost.Client{
				makeBatchCapableClient("a-plugin", first.api(t)),
				renamed(makeBatchCapableClient("finfocus-plugin-b", named.api(t))),
			}, nil)

			result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
				Resources: []ResourceDescriptor{instanceWithProps("i-001", nil), instanceWithProps("i-002", nil)},
				From:      from, To: to, Adapter: adapter,
			})
			require.NoError(t, err)

			assert.Equal(t, [][]string{{"i-001", "i-002"}}, named.batches)
			assert.Empty(t, first.batches)
			assert.Empty(t, first.unary)
			require.Len(t, result.Results, 2)
			for _, row := range result.Results {
				assert.Equal(t, "finfocus-plugin-b", row.Adapter, "output keeps the reported name")
			}
		})

		t.Run("unary path "+adapter, func(t *testing.T) {
			t.Parallel()
			from, to, _ := actualWindow()
			first, named := &actualCallRecorder{}, &actualCallRecorder{}
			eng := New([]*pluginhost.Client{
				makeNonBatchClient("a-plugin", first.api(t)),
				renamed(makeNonBatchClient("finfocus-plugin-b", named.api(t))),
			}, nil)

			result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
				Resources: []ResourceDescriptor{instanceWithProps("i-001", nil)},
				From:      from, To: to, Adapter: adapter,
			})
			require.NoError(t, err)

			assert.Equal(t, []string{"i-001"}, named.unary)
			assert.Empty(t, first.unary)
			require.Len(t, result.Results, 1)
			assert.Equal(t, "finfocus-plugin-b", result.Results[0].Adapter)
		})

		t.Run("options path "+adapter, func(t *testing.T) {
			t.Parallel()
			from, to, _ := actualWindow()
			first, named := &actualCallRecorder{}, &actualCallRecorder{}
			eng := New([]*pluginhost.Client{
				makeNonBatchClient("a-plugin", first.api(t)),
				renamed(makeNonBatchClient("finfocus-plugin-b", named.api(t))),
			}, nil)

			results, err := eng.GetActualCostWithOptions(context.Background(), ActualCostRequest{
				Resources: []ResourceDescriptor{instanceWithProps("i-001", nil)},
				From:      from, To: to, Adapter: adapter,
			})
			require.NoError(t, err)

			assert.Equal(t, []string{"i-001"}, named.unary)
			assert.Empty(t, first.unary)
			require.Len(t, results, 1)
			assert.Equal(t, "finfocus-plugin-b", results[0].Adapter)
		})
	}
}

func TestActualCostAdapterHonorsNamePrecedence(t *testing.T) {
	t.Parallel()
	from, to, _ := actualWindow()

	impostorRec, namedRec := &actualCallRecorder{}, &actualCallRecorder{}
	impostor := makeNonBatchClient("b-plugin", impostorRec.api(t))
	impostor.InstallName = "custom"
	named := makeNonBatchClient("renamed-b", namedRec.api(t))
	named.InstallName = "b-plugin"
	eng := New([]*pluginhost.Client{impostor, named}, nil)

	result, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-001", nil)},
		From:      from, To: to, Adapter: "b-plugin",
	})
	require.NoError(t, err)
	assert.Empty(t, impostorRec.unary, "a plugin that only reports the name loses to the one installed under it")
	assert.Equal(t, []string{"i-001"}, namedRec.unary)
	require.Len(t, result.Results, 1)
	assert.Equal(t, "renamed-b", result.Results[0].Adapter)

	results, err := eng.GetActualCostWithOptions(context.Background(), ActualCostRequest{
		Resources: []ResourceDescriptor{instanceWithProps("i-002", nil)},
		From:      from, To: to, Adapter: "b-plugin",
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "renamed-b", results[0].Adapter)
	assert.Empty(t, impostorRec.unary)
}
