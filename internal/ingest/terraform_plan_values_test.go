package ingest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitiveValueTrees(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, value, marker, want string }{
		{"whole object", `{"password":"secret"}`, `true`, `null`},
		{"known object", `{"name":"public"}`, `false`, `{"name":"public"}`},
		{"list leaves", `["secret","public","tail"]`, `[true,false]`, `["public","tail"]`},
		{"nested list", `[{"password":"secret","name":"public"}]`, `[{"password":true}]`, `[{"name":"public"}]`},
		{"object marker on null", `null`, `{"password":true}`, `null`},
		{"list marker on null", `null`, `[true]`, `null`},
		{"missing marker", `{"name":"public"}`, `null`, `{"name":"public"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var value, marker, want any
			require.NoError(t, json.Unmarshal([]byte(tc.value), &value))
			require.NoError(t, json.Unmarshal([]byte(tc.marker), &marker))
			require.NoError(t, json.Unmarshal([]byte(tc.want), &want))
			assert.Equal(t, want, stripSensitive(value, marker))
		})
	}
}

func TestUnknownValueTrees(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value, marker, want string
		drop                      bool
	}{
		{"unknown root", `null`, `true`, `null`, true},
		{"known root", `"known"`, `false`, `"known"`, false},
		{"object child", `{"future":null,"name":"known"}`, `{"future":true}`, `{"name":"known"}`, false},
		{"known list", `["known","tail"]`, `[false]`, `["known","tail"]`, false},
		{"unknown list item", `["known",null]`, `[false,true]`, `null`, true},
		{"nested list item", `[[null]]`, `[[true]]`, `null`, true},
		{"nested object list", `[{"future":null}]`, `[{"future":true}]`, `null`, true},
		{"object marker on null", `null`, `{"future":true}`, `null`, false},
		{"list marker on null", `null`, `[true]`, `null`, false},
		{"missing marker", `{"name":"known"}`, `null`, `{"name":"known"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var value, marker, want any
			require.NoError(t, json.Unmarshal([]byte(tc.value), &value))
			require.NoError(t, json.Unmarshal([]byte(tc.marker), &marker))
			require.NoError(t, json.Unmarshal([]byte(tc.want), &want))
			got, drop := applyUnknown(value, marker)
			assert.Equal(t, tc.drop, drop)
			assert.Equal(t, want, got)
		})
	}
}

func TestSensitiveUnknownPaths(t *testing.T) {
	t.Parallel()
	marker := map[string]any{
		"private_key": true,
		"settings":    []any{true, map[string]any{"password": true}},
	}
	paths := map[string]struct{}{}
	collectSensitivePaths(nil, marker, paths)
	assert.Contains(t, paths, "privateKey")
	assert.Contains(t, paths, "settings")
	assert.Contains(t, paths, "settings.password")
	assert.Equal(
		t,
		[]string{"known"},
		withoutPaths([]string{"privateKey", "known", "known"}, paths),
	)
}
