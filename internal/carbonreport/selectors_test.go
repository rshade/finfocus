package carbonreport

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectorsRequest(t *testing.T) {
	t.Parallel()
	for _, s := range []Selectors{{}, {Start: "2026-01-02", End: "2026-02-01"}, {Start: "2026-01-01", End: "2026-02-01", Method: "x"}, {Start: "2026-01-01", End: "2026-02-01", Scope: "4"}} {
		_, err := s.Request()
		require.Error(t, err)
	}
	req, err := (Selectors{Start: "2026-01-01", End: "2026-03-01", Method: "location-based", Scope: "2", Accounts: []string{"111"}}).Request()
	require.NoError(t, err)
	assert.Equal(t, int32(2), int32(req.GetMethod()))
	assert.Equal(t, int32(3), int32(req.GetScope()))
	assert.Equal(t, []string{"111"}, req.GetAccountIds())
}
