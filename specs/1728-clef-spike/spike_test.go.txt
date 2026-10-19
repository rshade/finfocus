//go:build clefspike

package jev_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/rshade/go-decide/clefclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jevapi "github.com/rshade/finfocus/plugins/scorer/internal/backends/jev"
)

type replayTransport func(*http.Request) (*http.Response, error)

func (f replayTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClefSpike(t *testing.T) {
	raw, err := os.ReadFile("testdata/clef-response.json")
	require.NoError(t, err)
	base, err := clefclient.BaseURL("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	// Preserve recorded answer values and metadata; only remove the choice
	// answer to isolate the two types the current scorer actually sends.
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &envelope))
	var result map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope["result"], &result))
	var answers map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(result["answers"], &answers))
	delete(answers, "team")
	result["answers"], err = json.Marshal(answers)
	require.NoError(t, err)
	envelope["result"], err = json.Marshal(result)
	require.NoError(t, err)
	projected, err := json.Marshal(envelope)
	require.NoError(t, err)
	envelope["success"] = json.RawMessage(`false`)
	failure, err := json.Marshal(envelope)
	require.NoError(t, err)

	tests := []struct {
		name    string
		body    []byte
		choice  bool
		failure bool
	}{
		{name: "recording_exposes_unsupported_choice", body: raw, choice: true},
		{name: "recorded_noul_and_score_projection", body: projected},
		{name: "mutated_success_false_is_502", body: failure, failure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := jevapi.Request{
				Model: "clef", State: "Checkout is failing for every customer.",
				Questions: map[string]jevapi.Question{
					"urgent":   jevapi.NoulQuestion("Is this urgent?", "Urgent", "Not urgent"),
					"severity": jevapi.ScoreQuestion("How severe is it?", "No impact", "Minor", "Major", "Critical"),
				},
			}
			if tt.choice {
				req.Questions["team"] = jevapi.Question{Type: "choice", Instructions: "Which team?",
					Criteria: map[string]string{"billing": "b", "technical": "t", "sales": "s"}}
			}
			wantBody, err := json.Marshal(req)
			require.NoError(t, err)
			hits := 0
			next := replayTransport(func(r *http.Request) (*http.Response, error) {
				hits++
				assert.Equal(t, base+"/clef", r.URL.String())
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "Bearer spike-dummy-token", r.Header.Get("Authorization"))
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				body, readErr := io.ReadAll(r.Body)
				require.NoError(t, readErr)
				assert.JSONEq(t, string(wantBody), string(body), "adapter preserves the request")
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
					Body: io.NopCloser(bytes.NewReader(tt.body)), Request: r}, nil
			})
			var transport http.RoundTripper = clefclient.NewTransport(next)
			if os.Getenv("CLEF_SPIKE_BREAK") == "adapter" {
				transport = next
			}
			client, err := jevapi.New(jevapi.Config{
				BaseURL: base, APIKey: "spike-dummy-token", MaxRetries: -1,
				HTTPClient: &http.Client{Transport: transport,
					CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
			})
			require.NoError(t, err)
			resp, err := client.SystemOne(t.Context(), req)
			assert.Equal(t, 1, hits)
			if tt.choice {
				require.ErrorIs(t, err, jevapi.ErrMalformedResponse)
				assert.ErrorContains(t, err, `unsupported type "choice"`)
				t.Logf("full recording rejected: %v", err)
				return
			}
			if tt.failure {
				var apiErr *jevapi.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, http.StatusBadGateway, apiErr.Status)
				t.Logf("mutated success:false: HTTP %d", apiErr.Status)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "clef", resp.Model)
			assert.Len(t, resp.Answers, 2)
			assert.True(t, resp.Answers["urgent"].HasNoul())
			assert.InDelta(t, 0.9905, resp.Answers["urgent"].Noul(), 1e-9)
			assert.True(t, resp.Answers["severity"].HasScore())
			assert.InDelta(t, 2.9571, resp.Answers["severity"].ScoreValue(), 1e-9)
			assert.Equal(t, 346, resp.Usage.InputTokens)
			t.Logf("projected recording decoded: model=%s noul=%g score=%g input_tokens=%d",
				resp.Model, resp.Answers["urgent"].Noul(), resp.Answers["severity"].ScoreValue(), resp.Usage.InputTokens)
		})
	}
}
