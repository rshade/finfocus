package carbonreport

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

type pageClient func(context.Context, *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error)

func (f pageClient) GetCarbonEmissions(
	ctx context.Context,
	r *pbc.GetCarbonEmissionsRequest,
) (*pbc.GetCarbonEmissionsResponse, error) {
	return f(ctx, r)
}
func request() *pbc.GetCarbonEmissionsRequest {
	return &pbc.GetCarbonEmissionsRequest{
		Start: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		End:   timestamppb.New(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
	}
}
func record() *pbc.CarbonEmissionRecord {
	r := request()
	return &pbc.CarbonEmissionRecord{
		Id:           "one",
		Provider:     "aws",
		Source:       "aws-sustainability",
		AccountId:    "111",
		Region:       "us-east-1",
		Service:      "AmazonEC2",
		Start:        r.GetStart(),
		End:          timestamppb.New(r.GetStart().AsTime().AddDate(0, 1, 0)),
		Method:       1,
		Scope:        1,
		ModelVersion: "v1",
		Value:        proto.Float64(10),
		Unit:         "gCO2e",
		RetrievedAt:  timestamppb.Now(),
	}
}
func TestQueryAtomicPages(t *testing.T) {
	t.Parallel()
	req := request()
	before := proto.CloneOf(req)
	calls := 0
	client := pageClient(
		func(_ context.Context, r *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
			calls++
			row := record()
			if calls == 1 {
				assert.Empty(t, r.GetPageToken())
				return &pbc.GetCarbonEmissionsResponse{
					Records:       []*pbc.CarbonEmissionRecord{row},
					NextPageToken: "two",
				}, nil
			}
			assert.Equal(t, "two", r.GetPageToken())
			row.Id = "two"
			row.AccountId = "222"
			row.Service = "AmazonS3"
			row.Start = row.GetEnd()
			row.End = req.GetEnd()
			row.ModelVersion = "v2"
			row.Value = proto.Float64(25)
			return &pbc.GetCarbonEmissionsResponse{Records: []*pbc.CarbonEmissionRecord{row}}, nil
		},
	)
	report, err := Query(context.Background(), client, req)
	require.NoError(t, err)
	require.Len(t, report.Records, 2)
	assert.InDelta(t, 35.0, *report.TotalGrams, 0)
	assert.True(t, proto.Equal(before, req))
	assert.Equal(t, "v2", report.Records[1].GetModelVersion())
}
func TestQueryRejectsUntrustworthyPages(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*pbc.CarbonEmissionRecord){
		"method": func(r *pbc.CarbonEmissionRecord) { r.Method = 2 },
		"scope":  func(r *pbc.CarbonEmissionRecord) { r.Scope = 2 },
		"outside": func(r *pbc.CarbonEmissionRecord) {
			r.Start = request().GetEnd()
			r.End = timestamppb.New(r.GetStart().AsTime().AddDate(0, 1, 0))
		},
		"missing value": func(r *pbc.CarbonEmissionRecord) { r.Value = nil },
		"nan":           func(r *pbc.CarbonEmissionRecord) { r.Value = proto.Float64(math.NaN()) },
		"filter":        func(r *pbc.CarbonEmissionRecord) { r.AccountId = "other" },
		"unit":          func(r *pbc.CarbonEmissionRecord) { r.Unit = "tonnes" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := request()
			req.AccountIds = []string{"111"}
			row := record()
			mutate(row)
			out, err := Query(
				context.Background(),
				pageClient(
					func(context.Context, *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
						return &pbc.GetCarbonEmissionsResponse{Records: []*pbc.CarbonEmissionRecord{row}}, nil
					},
				),
				req,
			)
			require.Error(t, err)
			assert.Nil(t, out)
		})
	}
	for _, kind := range []string{"duplicate identity", "duplicate id", "repeated token", "later error", "overflow", "page cap", "nil response"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			calls := 0
			out, err := Query(
				context.Background(),
				pageClient(
					func(_ context.Context, _ *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
						calls++
						row := record()
						row.Id = "arbitrary"
						if calls > 1 {
							row.Id = "different"
						}
						switch kind {
						case "later error":
							if calls > 1 {
								return nil, errors.New("later failure")
							}
						case "nil response":
							return nil, nil //nolint:nilnil // Deliberately invalid provider response.
						case "duplicate id":
							row.Id = "same"
							row.AccountId = string(rune('a' + calls))
						case "overflow":
							row.AccountId = string(rune('a' + calls))
							row.Value = proto.Float64(math.MaxFloat64)
						case "page cap":
							return &pbc.GetCarbonEmissionsResponse{
								NextPageToken: time.Unix(int64(calls), 0).String(),
							}, nil
						case "repeated token":
							return &pbc.GetCarbonEmissionsResponse{NextPageToken: "same"}, nil
						}
						next := ""
						if calls == 1 {
							next = "next"
						}
						return &pbc.GetCarbonEmissionsResponse{
							Records:       []*pbc.CarbonEmissionRecord{row},
							NextPageToken: next,
						}, nil
					},
				),
				request(),
			)
			require.Error(t, err)
			assert.Nil(t, out)
		})
	}
}
func TestQueryEmptyAndZero(t *testing.T) {
	t.Parallel()
	for _, zero := range []bool{false, true} {
		out, err := Query(
			context.Background(),
			pageClient(func(context.Context, *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
				r := &pbc.GetCarbonEmissionsResponse{}
				if zero {
					row := record()
					row.Value = proto.Float64(0)
					r.Records = []*pbc.CarbonEmissionRecord{row}
				}
				return r, nil
			}),
			request(),
		)
		require.NoError(t, err)
		if zero {
			require.NotNil(t, out.TotalGrams)
			assert.Zero(t, *out.TotalGrams)
		} else {
			assert.Nil(t, out.TotalGrams)
			assert.Empty(t, out.Records)
		}
	}
}

func TestReportJSONProvenance(t *testing.T) {
	t.Parallel()
	row := record()
	row.Value = proto.Float64(0)
	report := &Report{Request: request(), Records: []*pbc.CarbonEmissionRecord{row}, TotalGrams: proto.Float64(0)}
	data, err := json.Marshal(report)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"value":0`)
	assert.Contains(t, string(data), `2026-01-01T00:00:00Z`)
	assert.Contains(t, string(data), `"totalGrams":0`)
	assert.Contains(t, string(data), `"modelVersion":"v1"`)
}

func TestQueryBoundsStalledPage(t *testing.T) {
	t.Parallel()
	stalled := pageClient(
		func(ctx context.Context, _ *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)
	report, err := queryWithPageTimeout(context.Background(), stalled, request(), 20*time.Millisecond)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, report)
}
