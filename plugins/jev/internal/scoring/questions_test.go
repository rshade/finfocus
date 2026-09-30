package scoring

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/plugins/jev/internal/jevapi"
)

func questionsJSON(t *testing.T, req jevapi.Request) string {
	t.Helper()
	b, err := json.Marshal(req.Questions)
	require.NoError(t, err)
	return string(b)
}

func TestQuestionName_UsesPositionOnly(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "risk:#7", questionName("risk", 7))
	assert.Equal(t, "priority:#0", questionName("priority", 0))
}

func TestQuestions_WordingCoversRiskKinds(t *testing.T) {
	t.Parallel()

	risk := riskQuestion(0)
	for _, want := range []string{"downtime", "data loss", "performance regression", "financial commitment"} {
		assert.Contains(t, risk.Instructions, want)
	}
}

func TestQuestions_PriorityIsPlainFourLevelScale(t *testing.T) {
	t.Parallel()

	q := priorityQuestion(0)
	assert.Equal(t, "score", q.Type)
	assert.Equal(t, []string{"Ignore", "Low", "Medium", "High"}, q.Criteria)
	b, err := json.Marshal(q)
	require.NoError(t, err)
	assert.NotContains(t, string(b), "$")
	assert.NotRegexp(t, `(?i)dollar|usd|\$`, q.Instructions)
}

func TestQuestions_NoQuestionPositionsOverlapAcrossSignals(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, sig := range perRecordSignals() {
		q := sig.build(3)
		assert.Contains(t, q.Instructions, "position is 3")
		assert.False(t, seen[q.Instructions])
		seen[q.Instructions] = true
	}
	assert.Contains(t, duplicateQuestion(3).Instructions, "position is 3")
}
