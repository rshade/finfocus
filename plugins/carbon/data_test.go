package carbon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedDataLoaderLoadsOnce(t *testing.T) {
	t.Parallel()
	calls := 0
	want := dataset{
		Revision:  DatasetRevision,
		Instances: map[string]instance{"t3.medium": {VCPUs: 2}},
		Regions:   map[string]float64{"us-east-1": 0.5},
	}
	loader := newCachedDataLoader(func() (dataset, error) {
		calls++
		return want, nil
	})
	first, err := loader()
	require.NoError(t, err)
	second, err := loader()
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.Equal(t, want, first)
	assert.Equal(t, first, second)
}
