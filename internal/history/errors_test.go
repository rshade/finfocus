package history

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryError(t *testing.T) {
	t.Parallel()

	cause := fmt.Errorf("wrapped: %w", ErrMixedCurrencies)
	err := error(&QueryError{Err: cause})

	assert.Equal(t, cause.Error(), err.Error())
	require.ErrorIs(t, err, ErrMixedCurrencies)
	assert.True(t, IsQueryError(fmt.Errorf("outer: %w", err)))
	assert.False(t, IsQueryError(cause))
	assert.False(t, IsQueryError(nil))
}

func TestNoHistoryErrors(t *testing.T) {
	t.Parallel()

	missing := fmt.Errorf("%w for stack '%s'", ErrNoCostHistory, "dev")
	assert.Equal(t, "no cost history for stack 'dev'", missing.Error())
	assert.True(t, IsNoHistory(missing))
	assert.True(t, IsNoHistory(ErrNoSnapshots))
	assert.False(t, IsNoHistory(errors.New("no cost history")))
	assert.False(t, IsNoHistory(nil))
}
