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
		QueryEngine: func(context.Context) (*engine.Engine, func(), error) {
			return current, func() {}, nil
		},
	})
	session.SetData(fallback, nil, engine.DateRange{}, "dev")

	got, release, err := session.estimateEngine(context.Background())
	require.NoError(t, err)
	require.NotNil(t, release)
	release()
	require.Same(t, current, got)
}
