package terraform

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanFailureWrapping(t *testing.T) {
	t.Parallel()
	source := errors.New("input exceeds 32 MiB")
	err := PlanFailure(source)
	require.ErrorIs(t, err, source)
	assert.Contains(t, err.Error(), "32 MiB")
	assert.Equal(t, hintFailed, PlanFailure(nil).Error())
	var missing *HintError
	assert.Equal(t, hintFailed, missing.Error())
	assert.Empty(t, suffixLines(""))
}
