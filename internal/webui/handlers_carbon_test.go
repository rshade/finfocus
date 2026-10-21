package webui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/carbonreport"
	"github.com/rshade/finfocus/internal/engine"
)

func TestCarbonQueryAuthValidationAndParity(t *testing.T) {
	t.Parallel()
	called := 0
	session := NewSession(
		context.Background(),
		SessionOptions{
			HistoricalCarbon: func(_ context.Context, req *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
				called++
				assert.Equal(t, []string{"111"}, req.GetAccountIds())
				return &carbonreport.Report{Request: req, Records: []*pbc.CarbonEmissionRecord{}}, nil
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})
	body := `{"start":"2026-01-01","end":"2026-03-01","method":"market-based","scope":"total","accounts":["111"]}`
	req, err := http.NewRequest(http.MethodPost, s.BaseURL()+"/api/carbon/actual/query", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Zero(t, called)
	resp = overviewRequest(t, s, http.MethodPost, "/api/carbon/actual/query", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	var got map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &got))
	assert.JSONEq(t, `null`, string(got["totalGrams"]))
	assert.Contains(t, string(got["request"]), "2026-01-01T00:00:00Z")
	assert.Equal(t, 1, called)
	for _, bad := range []string{`{}`, `{"start":"2026-01-02","end":"2026-03-01"}`, `{"start":"2026-01-01","end":"2026-03-01","scope":"bad"}`} {
		resp = overviewRequest(t, s, http.MethodPost, "/api/carbon/actual/query", bad)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.NoError(t, resp.Body.Close())
	}
	assert.Equal(t, 1, called)
	session.Close()
	resp = overviewRequest(t, s, http.MethodPost, "/api/carbon/actual/query", body)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

type carbonPageClient func(context.Context, *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error)

func (f carbonPageClient) GetCarbonEmissions(
	ctx context.Context,
	r *pbc.GetCarbonEmissionsRequest,
) (*pbc.GetCarbonEmissionsResponse, error) {
	return f(ctx, r)
}

func TestCarbonQuerySharedOperationParity(t *testing.T) {
	t.Parallel()
	selectors := carbonreport.Selectors{Start: "2026-01-01", End: "2026-03-01", Method: "market-based", Scope: "total"}
	req, err := selectors.Request()
	require.NoError(t, err)
	client := carbonPageClient(
		func(_ context.Context, q *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
			row := &pbc.CarbonEmissionRecord{
				Id:           "first",
				Provider:     "aws",
				Source:       "aws-sustainability",
				AccountId:    "111",
				Region:       "us-east-1",
				Service:      "AmazonEC2",
				Start:        q.GetStart(),
				End:          timestamppb.New(q.GetStart().AsTime().AddDate(0, 1, 0)),
				Method:       q.GetMethod(),
				Scope:        q.GetScope(),
				ModelVersion: "v1",
				RetrievedAt:  q.GetEnd(),
				Value:        proto.Float64(0),
				Unit:         "gCO2e",
			}
			next := "second"
			if q.GetPageToken() != "" {
				row.Id = "second"
				row.AccountId = "222"
				row.Service = "AmazonS3"
				row.Start = row.GetEnd()
				row.End = q.GetEnd()
				row.Value = proto.Float64(17)
				row.ModelVersion = "v2"
				next = ""
			}
			return &pbc.GetCarbonEmissionsResponse{Records: []*pbc.CarbonEmissionRecord{row}, NextPageToken: next}, nil
		},
	)
	expected, err := carbonreport.Query(context.Background(), client, req)
	require.NoError(t, err)
	expectedJSON, err := json.Marshal(expected)
	require.NoError(t, err)
	session := NewSession(
		context.Background(),
		SessionOptions{
			HistoricalCarbon: func(ctx context.Context, r *pbc.GetCarbonEmissionsRequest) (*carbonreport.Report, error) {
				return carbonreport.Query(ctx, client, r)
			},
		},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "dev")
	defer session.Close()
	s := newRunningServer(t, Options{Session: session})
	body, err := json.Marshal(selectors)
	require.NoError(t, err)
	resp := overviewRequest(t, s, http.MethodPost, "/api/carbon/actual/query", string(body))
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, string(expectedJSON), string(data))
}
