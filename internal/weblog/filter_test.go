package weblog_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

type filterCase struct {
	Name      string         `json:"name"`
	Entries   []weblog.Entry `json:"entries"`
	Filter    weblog.Filter  `json:"filter"`
	ExpectSeq []uint64       `json:"expectSeq"`
}

func TestFilterMatchCases(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/filter_cases.json")
	require.NoError(t, err)
	var cases []filterCase
	require.NoError(t, json.Unmarshal(data, &cases))
	require.NotEmpty(t, cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			got := []uint64{}
			for _, e := range tc.Entries {
				if tc.Filter.Match(e) {
					got = append(got, e.Seq)
				}
			}
			assert.Equal(t, tc.ExpectSeq, got)
		})
	}
}

func TestFilterNilFieldValue(t *testing.T) {
	t.Parallel()
	e := weblog.Entry{Level: "info", Message: "m", Fields: map[string]any{"k": nil}}
	assert.False(t, weblog.Filter{Text: "nil"}.Match(e))
}
