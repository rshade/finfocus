package scoring

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestRender_ShapeAndEnums(t *testing.T) {
	t.Parallel()

	rec := makeRec("rec-1", "res-1")
	rec.Reasoning = []string{"CPU is low"}
	rec.PrimaryReason = pbc.RecommendationReason_RECOMMENDATION_REASON_OVER_PROVISIONED

	got, err := renderer{limits: Limits{}.withDefaults()}.render(rec)
	require.NoError(t, err)

	assert.NotContains(t, got, "id", "the recommendation id carries no signal")
	assert.Equal(t, "COST", got["category"])
	assert.Equal(t, "RIGHTSIZE", got["action_type"])
	assert.Equal(t, "OVER_PROVISIONED", got["primary_reason"])
	assert.Equal(t, "Downsize rec-1", got["description"])
	resource, ok := got["resource"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "res-1", resource["id"])
	assert.Equal(t, map[string]any{"env": "prod"}, resource["tags"])
	assert.Equal(t, []any{"CPU is low"}, got["reasoning"])
}

func TestRender_StripsControlAndInvisibleCharacters(t *testing.T) {
	t.Parallel()

	rec := makeRec("rec-1", "res-1")
	rec.Description = "line one\nline two\t\x00\x07 end\u202e\u200b ok"
	rec.Resource.Tags = map[string]string{"k\x00ey\n": "va\u200dlue\r\nnext"}

	got, err := renderer{limits: Limits{}.withDefaults()}.render(rec)
	require.NoError(t, err)

	assert.Equal(t, "line one line two  end ok", got["description"])
	tags, ok := got["resource"].(map[string]any)["tags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"key": "value  next"}, tags)
}

func TestRender_CapsFieldLengthsAndCounts(t *testing.T) {
	t.Parallel()

	limits := Limits{MaxString: 10, MaxKey: 4, MaxMapValue: 6, MaxEntries: 3, MaxListLen: 2}
	rec := makeRec("rec-1", "res-1")
	rec.Description = strings.Repeat("é", 50)
	rec.Reasoning = []string{"aaaaaaaaaaaaaaaa", "b", "c", "d"}
	rec.Resource.Tags = map[string]string{
		"aaaaaaaa": "vvvvvvvvvv", "b": "2", "c": "3", "d": "4", "e": "5",
	}
	rec.Metadata = map[string]string{"long-key": strings.Repeat("x", 100)}

	got, err := renderer{limits: limits.withDefaults()}.render(rec)
	require.NoError(t, err)

	assert.Equal(t, strings.Repeat("é", 10), got["description"])
	assert.Equal(t, []any{"aaaaaaaaaa", "b"}, got["reasoning"])
	tags, ok := got["resource"].(map[string]any)["tags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"aaaa": "vvvvvv", "b": "2", "c": "3"}, tags)
	assert.Equal(t, map[string]any{"long": strings.Repeat("x", 6)}, got["metadata"])
}

func TestRender_OutputIsJSONSerialisable(t *testing.T) {
	t.Parallel()

	got, err := renderer{limits: Limits{}.withDefaults()}.render(makeRec("rec-1", "res-1"))
	require.NoError(t, err)
	_, err = json.Marshal(got)
	require.NoError(t, err)
}
