package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestQueryEngineReturnsTheReloadedEngine(t *testing.T) {
	t.Parallel()

	data := newWebData(nil, overviewParams{}, engine.DateRange{}, nil)
	first := engine.New(nil, nil)
	data.eng = first
	got, err := data.queryEngine(context.Background())
	require.NoError(t, err)
	require.Same(t, first, got)

	second := engine.New(nil, nil)
	data.eng = second
	got, err = data.queryEngine(context.Background())
	require.NoError(t, err)
	require.Same(t, second, got)
}
