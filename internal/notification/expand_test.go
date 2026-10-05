package notification_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/notification"
)

type recordingLookup struct {
	mu     sync.Mutex
	values map[string]string
	asked  []string
}

func (r *recordingLookup) lookup(name string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, name)
	value, ok := r.values[name]
	return value, ok
}

func TestExpand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "single reference", value: "${FINFOCUS_NOTIFY_URL}", want: "https://hooks.example/abc"},
		{name: "several references", value: "Bearer ${FINFOCUS_NOTIFY_A}-${FINFOCUS_NOTIFY_B}", want: "Bearer one-two"},
		{
			name:  "dollar name stays literal",
			value: "$NAME and $FINFOCUS_NOTIFY_A",
			want:  "$NAME and $FINFOCUS_NOTIFY_A",
		},
		{name: "bare dollar stays literal", value: "cost $ 5", want: "cost $ 5"},
		{name: "double dollar stays literal", value: "a$$b", want: "a$$b"},
		{name: "no references", value: "https://example.com", want: "https://example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookup := &recordingLookup{values: map[string]string{
				"FINFOCUS_NOTIFY_URL": "https://hooks.example/abc",
				"FINFOCUS_NOTIFY_A":   "one",
				"FINFOCUS_NOTIFY_B":   "two",
			}}
			got, err := notification.Expand(tc.value, lookup.lookup)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExpandRefusesOtherVariables(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "HOME"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lookup := &recordingLookup{values: map[string]string{name: "leaked", "FINFOCUS_NOTIFY_A": "a"}}
			_, err := notification.Expand("${FINFOCUS_NOTIFY_A}https://x/?k=${"+name+"}", lookup.lookup)
			require.ErrorIs(t, err, config.ErrNotificationVariableNotAllowed)
			assert.Contains(t, err.Error(), "not a FINFOCUS_NOTIFY_ variable")
			assert.Contains(t, err.Error(), name)
			assert.Empty(t, lookup.asked, "lookup must not run when any reference is disallowed")
		})
	}
}

func TestExpandUnsetVariable(t *testing.T) {
	t.Parallel()

	for _, values := range []map[string]string{{}, {"FINFOCUS_NOTIFY_TOKEN": ""}} {
		lookup := &recordingLookup{values: values}
		_, err := notification.Expand("https://user:pw@example.com/${FINFOCUS_NOTIFY_TOKEN}", lookup.lookup)
		require.ErrorIs(t, err, notification.ErrUnsetVariable)
		assert.Contains(t, err.Error(), "FINFOCUS_NOTIFY_TOKEN")
		assert.NotContains(t, err.Error(), "example.com")
		assert.NotContains(t, err.Error(), "pw")
	}
}

func TestExpandMalformedReference(t *testing.T) {
	t.Parallel()

	lookup := &recordingLookup{}
	_, err := notification.Expand("https://x/${FINFOCUS_NOTIFY_A", lookup.lookup)
	require.ErrorIs(t, err, config.ErrNotificationReferenceMalformed)
	assert.Empty(t, lookup.asked)
}

func TestReferenceHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, notification.IsSingleReference("${FINFOCUS_NOTIFY_URL}"))
	assert.True(t, notification.IsSingleReference("${GITHUB_TOKEN}"))
	assert.False(t, notification.IsSingleReference("Bearer ${FINFOCUS_NOTIFY_TOKEN}"))
	assert.False(t, notification.IsSingleReference("${FINFOCUS_NOTIFY_A}${FINFOCUS_NOTIFY_B}"))
	assert.False(t, notification.IsSingleReference("https://example.com"))
	assert.False(t, notification.IsSingleReference("${FINFOCUS_NOTIFY_A"))

	assert.True(t, notification.HasReference("x${"))
	assert.True(t, notification.HasReference("Bearer ${FINFOCUS_NOTIFY_TOKEN}"))
	assert.False(t, notification.HasReference("$NAME"))
}
