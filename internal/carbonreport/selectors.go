package carbonreport

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// Selectors is the shared CLI and web query input. End is exclusive.
type Selectors struct {
	Start    string   `json:"start"`
	End      string   `json:"end"`
	Method   string   `json:"method"`
	Scope    string   `json:"scope"`
	Accounts []string `json:"accounts,omitempty"`
	Regions  []string `json:"regions,omitempty"`
	Services []string `json:"services,omitempty"`
}

// Request parses selectors and validates complete month bounds before launch.
func (s Selectors) Request() (*pbc.GetCarbonEmissionsRequest, error) {
	start, err := time.Parse(time.DateOnly, s.Start)
	if err != nil {
		return nil, fmt.Errorf("--start must be YYYY-MM-DD at a UTC month boundary: %w", err)
	}
	end, err := time.Parse(time.DateOnly, s.End)
	if err != nil {
		return nil, fmt.Errorf("--end must be YYYY-MM-DD at a UTC month boundary (exclusive): %w", err)
	}
	req := &pbc.GetCarbonEmissionsRequest{
		Start:      timestamppb.New(start),
		End:        timestamppb.New(end),
		AccountIds: append([]string(nil), s.Accounts...),
		Regions:    append([]string(nil), s.Regions...),
		Services:   append([]string(nil), s.Services...),
	}
	switch s.Method {
	case "", "market-based":
		req.Method = pbc.CarbonAccountingMethod_CARBON_ACCOUNTING_METHOD_MARKET_BASED
	case "location-based":
		req.Method = pbc.CarbonAccountingMethod_CARBON_ACCOUNTING_METHOD_LOCATION_BASED
	default:
		return nil, errors.New("method must be market-based or location-based")
	}
	switch s.Scope {
	case "", "total":
		req.Scope = pbc.CarbonEmissionsScope_CARBON_EMISSIONS_SCOPE_TOTAL
	case "1":
		req.Scope = pbc.CarbonEmissionsScope_CARBON_EMISSIONS_SCOPE_SCOPE_1
	case "2":
		req.Scope = pbc.CarbonEmissionsScope_CARBON_EMISSIONS_SCOPE_SCOPE_2
	case "3":
		req.Scope = pbc.CarbonEmissionsScope_CARBON_EMISSIONS_SCOPE_SCOPE_3
	default:
		return nil, errors.New("scope must be total, 1, 2 or 3")
	}
	for _, values := range [][]string{req.GetAccountIds(), req.GetRegions(), req.GetServices()} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return nil, errors.New("account, region and service filters must not be blank")
			}
		}
	}
	if err = pluginsdk.ValidateGetCarbonEmissionsRequest(req); err != nil {
		return nil, fmt.Errorf("invalid carbon query: %w", err)
	}
	return req, nil
}
