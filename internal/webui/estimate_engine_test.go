package webui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestEstimateEngineUsesQueryEngine(t *testing.T) {
	t.Parallel()

	fallback := engine.New(nil, nil)
	current := engine.New(nil, nil)
	session := NewSession(context.Background(), SessionOptions{
		QueryEngine: func(context.Context) (*engine.Engine, error) { return current, nil },
	})
	session.SetData(fallback, nil, engine.DateRange{}, "dev")

	got, err := session.estimateEngine(context.Background())
	require.NoError(t, err)
	require.Same(t, current, got)
}
