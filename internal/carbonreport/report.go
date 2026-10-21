// Package carbonreport queries complete, provider-reported historical emissions.
package carbonreport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// ErrInvalidResponse identifies provider data or pagination that cannot form a valid report.
var ErrInvalidResponse = errors.New("invalid historical carbon response")

// Client is the optional typed historical emissions interface.
type Client interface {
	GetCarbonEmissions(context.Context, *pbc.GetCarbonEmissionsRequest) (*pbc.GetCarbonEmissionsResponse, error)
}

// Report preserves the resolved query and every source record. TotalGrams sums
// returned records only; nil denotes no available data rather than zero emissions.
type Report struct {
	Request    *pbc.GetCarbonEmissionsRequest `json:"request"`
	Records    []*pbc.CarbonEmissionRecord    `json:"records"`
	TotalGrams *float64                       `json:"totalGrams"`
}

// Query walks at most 100 pages atomically. It never mutates the caller's request
// and returns no partial report on provider, pagination or validation failure.
func Query(ctx context.Context, client Client, req *pbc.GetCarbonEmissionsRequest) (*Report, error) {
	if err := pluginsdk.ValidateGetCarbonEmissionsRequest(req); err != nil {
		return nil, fmt.Errorf("invalid historical carbon query: %w", err)
	}
	query := proto.CloneOf(req)
	if query.GetPageToken() != "" {
		return nil, errors.New("historical report must start without a page token")
	}
	pluginsdk.ApplyGetCarbonEmissionsDefaults(query)
	report := &Report{Request: proto.CloneOf(query), Records: []*pbc.CarbonEmissionRecord{}}
	seen := &recordSet{identities: map[identity]bool{}, ids: map[string]bool{}}
	tokens := map[string]bool{}
	for page := range 100 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := client.GetCarbonEmissions(ctx, proto.CloneOf(query))
		if err != nil {
			return nil, fmt.Errorf("historical carbon page %d: %w", page+1, err)
		}
		if err = validatePage(query, resp); err != nil {
			return nil, fmt.Errorf("%w: page %d: %w", ErrInvalidResponse, page+1, err)
		}
		if err = seen.append(report, query, resp.GetRecords()); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidResponse, err)
		}
		next := resp.GetNextPageToken()
		if next == "" {
			if len(report.Records) > 0 {
				report.TotalGrams = &seen.total
			}
			return report, nil
		}
		if len(next) > pluginsdk.MaxCarbonPageTokenBytes || tokens[next] {
			return nil, fmt.Errorf("%w: invalid or repeated historical carbon page token", ErrInvalidResponse)
		}
		tokens[next] = true
		query.PageToken = next
	}
	return nil, fmt.Errorf("%w: report exceeds 100-page limit", ErrInvalidResponse)
}

type identity struct {
	provider, account, region, service string
	start, end                         int64
	method                             pbc.CarbonAccountingMethod
	scope                              pbc.CarbonEmissionsScope
}

func recordIdentity(r *pbc.CarbonEmissionRecord) identity {
	return identity{
		r.GetProvider(),
		r.GetAccountId(),
		r.GetRegion(),
		r.GetService(),
		r.GetStart().GetSeconds(),
		r.GetEnd().GetSeconds(),
		r.GetMethod(),
		r.GetScope(),
	}
}
func matches(q *pbc.GetCarbonEmissionsRequest, r *pbc.CarbonEmissionRecord) bool {
	return r.GetMethod() == q.GetMethod() && r.GetScope() == q.GetScope() &&
		!r.GetStart().AsTime().Before(q.GetStart().AsTime()) &&
		!r.GetEnd().AsTime().After(q.GetEnd().AsTime()) &&
		selected(q.GetAccountIds(), r.GetAccountId()) &&
		selected(q.GetRegions(), r.GetRegion()) &&
		selected(q.GetServices(), r.GetService())
}
func selected(values []string, value string) bool {
	return len(values) == 0 || slices.Contains(values, value)
}

// MarshalJSON uses canonical protobuf JSON for enums and RFC3339 timestamps.
// Explicit zero presence is retained in records and the optional report total.
func (r *Report) MarshalJSON() ([]byte, error) {
	request, err := protojson.Marshal(r.Request)
	if err != nil {
		return nil, err
	}
	records := make([]json.RawMessage, 0, len(r.Records))
	for _, record := range r.Records {
		encoded, marshalErr := protojson.Marshal(record)
		if marshalErr != nil {
			return nil, marshalErr
		}
		records = append(records, encoded)
	}
	return json.Marshal(struct {
		Request    json.RawMessage   `json:"request"`
		Records    []json.RawMessage `json:"records"`
		TotalGrams *float64          `json:"totalGrams"`
	}{request, records, r.TotalGrams})
}

type recordSet struct {
	identities map[identity]bool
	ids        map[string]bool
	total      float64
}

func (s *recordSet) append(
	report *Report,
	query *pbc.GetCarbonEmissionsRequest,
	rows []*pbc.CarbonEmissionRecord,
) error {
	for _, row := range rows {
		if !matches(query, row) {
			return errors.New("historical carbon record does not match query window, filters, method or scope")
		}
		key := recordIdentity(row)
		if s.ids[row.GetId()] || s.identities[key] {
			return errors.New("duplicate historical carbon identity; model revisions are not additive")
		}
		s.ids[row.GetId()] = true
		s.identities[key] = true
		s.total += row.GetValue()
		if math.IsNaN(s.total) || math.IsInf(s.total, 0) {
			return errors.New("historical carbon total is not finite")
		}
		report.Records = append(report.Records, proto.CloneOf(row))
	}
	return nil
}

func validatePage(query *pbc.GetCarbonEmissionsRequest, resp *pbc.GetCarbonEmissionsResponse) error {
	if err := pluginsdk.ValidateGetCarbonEmissionsResponse(resp); err != nil {
		return err
	}
	if len(resp.GetRecords()) > int(query.GetPageSize()) {
		return errors.New("historical carbon page exceeds requested size")
	}
	return nil
}
