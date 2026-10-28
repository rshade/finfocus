package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestRenderActualMachineOutputEmptyIncompleteResultsIsArray(t *testing.T) {
	t.Parallel()
	for _, estimateConfidence := range []bool{false, true} {
		t.Run(fmt.Sprintf("estimateConfidence=%t", estimateConfidence), func(t *testing.T) {
			t.Parallel()
			view := &engine.CostResultWithErrors{
				HistoryImport: &engine.HistoryImportStatus{Incomplete: true, UnreadableUpdates: 1},
			}
			var out bytes.Buffer
			require.NoError(t, renderActualMachineOutput(&out, engine.OutputJSON, view, "", estimateConfidence))
			var envelope struct {
				Results json.RawMessage `json:"results"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &envelope), out.String())
			assert.JSONEq(t, `[]`, string(envelope.Results))
		})
	}
}
