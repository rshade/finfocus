package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// ModeRunRate labels results computed from point-in-time usage and monthly prices.
const ModeRunRate = "run-rate"

// ModeHistorical labels results priced from actual spend over a window.
const ModeHistorical = "historical"

// Allocation row kinds and subject keys from the finfocus-spec allocation contract.
const (
	rowKindWorkload = "workload"
	rowKindIdle     = "__idle__"
	rowKindCluster  = "__cluster__"
	subjectKeyKind  = "kind"
	subjectKeyNode  = "node"
)

var (
	// ErrConservation reports allocator output that does not sum to the priced total.
	ErrConservation = errors.New("allocator violated cost conservation")
	// ErrHistoricalUnsupported reports historical usage, which needs actual-cost pricing.
	ErrHistoricalUnsupported = errors.New("historical usage is not supported yet")
	// ErrStatsModeMismatch reports a windowed run whose usage source did not
	// return historical mode. The error names both modes.
	ErrStatsModeMismatch = errors.New("usage stats mode does not match the request")
)

const (
	periodMonthly        = "monthly"
	incompletePrefix     = "incomplete:"
	unpricedZeroNote     = "priced at $0; treated as unpriced"
	unpricedMissing      = "no pricing result returned"
	nothingPricedMessage = "no priceable resource could be priced; check that a pricing plugin " +
		"(e.g. aws-public for the nodes' region) is installed"
)

// UsageSource is satisfied by pbc.UsageSourceServiceClient.
type UsageSource interface {
	GetStats(ctx context.Context, in *pbc.GetStatsRequest, opts ...grpc.CallOption) (*pbc.GetStatsResponse, error)
}

// Allocator is satisfied by pbc.AllocatorServiceClient.
type Allocator interface {
	Allocate(ctx context.Context, in *pbc.AllocateRequest, opts ...grpc.CallOption) (*pbc.AllocateResponse, error)
}

// ResourcePricer is satisfied by *Engine.
type ResourcePricer interface {
	GetProjectedCostWithErrors(ctx context.Context, resources []ResourceDescriptor) (*CostResultWithErrors, error)
	// GetWindowCost prices resources from actual spend over [from, to].
	// State-based estimation is off. The amount is CostResult.TotalCost.
	GetWindowCost(ctx context.Context, resources []ResourceDescriptor, from, to time.Time) ([]CostResult, error)
}

// ClusterRequest selects the cluster and policy for an allocation run.
// From and To are both zero for a run-rate report. A window sets both.
type ClusterRequest struct {
	Scope      string
	Namespace  string
	Selector   map[string]string
	PolicyJSON []byte
	From       time.Time
	To         time.Time
}

// ClusterRow is one allocator row.
type ClusterRow struct {
	Subject   map[string]string `json:"subject"`
	CPUCost   float64           `json:"cpu_cost"`
	MemCost   float64           `json:"mem_cost"`
	TotalCost float64           `json:"total_cost"`
	Note      string            `json:"note,omitempty"`
}

// PricedSummary reports how one priceable resource was priced.
type PricedSummary struct {
	Kind         string  `json:"kind"`
	ID           string  `json:"id"`
	ResourceType string  `json:"resource_type"`
	SKU          string  `json:"sku"`
	Monthly      float64 `json:"monthly"`
	Priced       bool    `json:"priced"`
	Note         string  `json:"note,omitempty"`
}

// ClusterResult is the outcome of RunClusterAllocation.
type ClusterResult struct {
	Mode            string
	Period          string
	Currency        string
	Rows            []ClusterRow
	Priced          []PricedSummary
	Total           float64
	Idle            float64
	NamespaceScoped bool
	Incomplete      bool
	PolicyDigest    string
	EffectivePolicy json.RawMessage
	Warnings        []string
}

// RunClusterAllocation gathers usage, prices the reported resources, allocates,
// and verifies that rows sum to the priced total. Both From and To zero is the
// monthly run-rate path. A window prices actual spend with GetWindowCost.
func RunClusterAllocation(
	ctx context.Context,
	usage UsageSource,
	alloc Allocator,
	pricer ResourcePricer,
	req ClusterRequest,
) (*ClusterResult, error) {
	window, err := clusterWindow(req.From, req.To)
	if err != nil {
		return nil, err
	}
	stats, err := usage.GetStats(ctx, usageStatsRequest(req, window))
	if err != nil {
		return nil, fmt.Errorf("get usage stats: %w", err)
	}
	if modeErr := requireStatsMode(window, stats.GetMode()); modeErr != nil {
		return nil, modeErr
	}

	priced, summaries, currency, err := priceClusterResources(ctx, pricer, stats.GetPriceable(), req, window)
	if err != nil {
		return nil, err
	}

	allocReq := &pbc.AllocateRequest{
		Usage: stats.GetRows(), Priced: priced, PolicyJson: req.PolicyJSON, Mode: stats.GetMode(),
	}
	resp, err := alloc.Allocate(ctx, allocReq)
	if err != nil {
		return nil, fmt.Errorf("allocate: %w", err)
	}
	if consErr := VerifyConservation(allocReq, resp); consErr != nil {
		return nil, consErr
	}
	return newClusterResult(req, window, stats, resp, summaries, currency), nil
}

func usageStatsRequest(req ClusterRequest, window bool) *pbc.GetStatsRequest {
	selector := map[string]string{}
	maps.Copy(selector, req.Selector)
	if req.Namespace != "" {
		selector["namespace"] = req.Namespace
	}
	statsReq := &pbc.GetStatsRequest{Scope: req.Scope, Selector: selector}
	if window {
		statsReq.Start = timestamppb.New(req.From)
		statsReq.End = timestamppb.New(req.To)
	}
	return statsReq
}

func priceClusterResources(
	ctx context.Context,
	pricer ResourcePricer,
	descs []*pbc.ResourceDescriptor,
	req ClusterRequest,
	window bool,
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	if window {
		return priceWindowResources(ctx, pricer, descs, req.From, req.To)
	}
	return priceResources(ctx, pricer, descs)
}

func newClusterResult(
	req ClusterRequest,
	window bool,
	stats *pbc.GetStatsResponse,
	resp *pbc.AllocateResponse,
	summaries []PricedSummary,
	currency string,
) *ClusterResult {
	mode, period := ModeRunRate, periodMonthly
	if window {
		mode, period = ModeHistorical, FormatPeriod(req.From, req.To)
	}
	res := &ClusterResult{
		Mode:            mode,
		Period:          period,
		Currency:        currency,
		Priced:          summaries,
		NamespaceScoped: req.Namespace != "",
		PolicyDigest:    resp.GetPolicyDigest(),
		EffectivePolicy: json.RawMessage(resp.GetEffectivePolicyJson()),
		Warnings:        append(append([]string{}, stats.GetWarnings()...), resp.GetWarnings()...),
	}
	for _, w := range stats.GetWarnings() {
		if strings.HasPrefix(w, incompletePrefix) {
			res.Incomplete = true
			break
		}
	}
	for _, s := range summaries {
		if !s.Priced {
			res.Incomplete = true
		}
	}
	for _, r := range resp.GetRows() {
		kind := r.GetSubject()[subjectKeyKind]
		if res.NamespaceScoped && kind != rowKindWorkload {
			continue
		}
		res.Rows = append(res.Rows, ClusterRow{
			Subject: r.GetSubject(), CPUCost: r.GetCpuCost(), MemCost: r.GetMemCost(),
			TotalCost: r.GetTotalCost(), Note: r.GetNote(),
		})
		res.Total += r.GetTotalCost()
		if kind == rowKindIdle {
			res.Idle += r.GetTotalCost()
		}
	}
	return res
}

// clusterWindow reports whether From and To name a historical window.
// Exactly one bound, or an end that is not after the start, is rejected
// before GetStats.
func clusterWindow(from, to time.Time) (bool, error) {
	switch {
	case from.IsZero() && to.IsZero():
		return false, nil
	case from.IsZero() || to.IsZero():
		return false, errors.New("cluster window requires both from and to")
	case !to.After(from):
		return false, errors.New("cluster window end must be after start")
	default:
		return true, nil
	}
}

func requireStatsMode(window bool, mode pbc.StatsMode) error {
	if window {
		if mode != pbc.StatsMode_STATS_MODE_HISTORICAL {
			return fmt.Errorf("%w: requested %s, usage source returned %s",
				ErrStatsModeMismatch, ModeHistorical, statsModeLabel(mode))
		}
		return nil
	}
	if mode != pbc.StatsMode_STATS_MODE_RUN_RATE {
		return fmt.Errorf("%w (usage source returned %s)", ErrHistoricalUnsupported, statsModeLabel(mode))
	}
	return nil
}

// statsModeLabel is the mode name a mismatch error shows. Known modes use the
// same words as ClusterResult.Mode.
func statsModeLabel(mode pbc.StatsMode) string {
	switch mode {
	case pbc.StatsMode_STATS_MODE_HISTORICAL:
		return ModeHistorical
	case pbc.StatsMode_STATS_MODE_RUN_RATE:
		return ModeRunRate
	case pbc.StatsMode_STATS_MODE_UNSPECIFIED:
		fallthrough
	default:
		return mode.String()
	}
}

// ShowAllocationPolicy asks the allocator for its effective policy without usage.
func ShowAllocationPolicy(
	ctx context.Context, alloc Allocator, policyJSON []byte,
) (json.RawMessage, string, error) {
	req := &pbc.AllocateRequest{PolicyJson: policyJSON}
	resp, err := alloc.Allocate(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("allocate: %w", err)
	}
	if err = pluginsdk.ValidateAllocateResponse(req, resp); err != nil {
		return nil, "", fmt.Errorf("invalid allocator response: %w", err)
	}
	return json.RawMessage(resp.GetEffectivePolicyJson()), resp.GetPolicyDigest(), nil
}

// VerifyConservation rejects allocator output that breaks the allocation
// contract (row kinds, idle rows, negative or non-finite costs, currency) or
// does not sum to the successfully priced total. Both checks are the SDK's, so
// core, allocators, and the conformance suite apply one rule.
func VerifyConservation(req *pbc.AllocateRequest, resp *pbc.AllocateResponse) error {
	if err := pluginsdk.ValidateAllocateResponse(req, resp); err != nil {
		return fmt.Errorf("%w: %w", ErrConservation, err)
	}
	if err := pluginsdk.CheckConservation(req, resp, pluginsdk.DefaultConservationEpsilon); err != nil {
		return fmt.Errorf("%w: %w", ErrConservation, err)
	}
	return nil
}

// PriceableToResource converts a usage source's priceable descriptor into the
// engine's resource shape; sku and region travel as properties.
func PriceableToResource(d *pbc.ResourceDescriptor) ResourceDescriptor {
	props := map[string]any{"sku": d.GetSku(), "region": d.GetRegion()}
	for k, v := range d.GetTags() {
		props[k] = v
	}
	return ResourceDescriptor{Type: d.GetResourceType(), ID: d.GetId(), Provider: d.GetProvider(), Properties: props}
}

func descriptorsToResources(descs []*pbc.ResourceDescriptor) []ResourceDescriptor {
	resources := make([]ResourceDescriptor, len(descs))
	for i, d := range descs {
		resources[i] = PriceableToResource(d)
	}
	return resources
}

func priceResources(
	ctx context.Context,
	pricer ResourcePricer,
	descs []*pbc.ResourceDescriptor,
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	results, err := pricer.GetProjectedCostWithErrors(ctx, descriptorsToResources(descs))
	if err != nil {
		return nil, nil, "", fmt.Errorf("price cluster resources: %w", err)
	}
	return priceMatched(descs, results.Results, func(r CostResult) (float64, bool) {
		if r.Monthly <= 0 {
			return 0, false
		}
		return r.Monthly, true
	})
}

// priceWindowResources prices a historical window from CostResult.TotalCost.
// A missing result, a result error, or TotalCost <= 0 stays unpriced. This
// path does not read Monthly and does not call projected pricing.
func priceWindowResources(
	ctx context.Context,
	pricer ResourcePricer,
	descs []*pbc.ResourceDescriptor,
	from, to time.Time,
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	results, err := pricer.GetWindowCost(ctx, descriptorsToResources(descs), from, to)
	if err != nil {
		return nil, nil, "", fmt.Errorf("price cluster resources: %w", err)
	}
	return priceMatched(descs, results, func(r CostResult) (float64, bool) {
		if r.TotalCost <= 0 {
			return 0, false
		}
		return r.TotalCost, true
	})
}

func priceMatched(
	descs []*pbc.ResourceDescriptor,
	results []CostResult,
	amount func(CostResult) (float64, bool),
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	byKey := make(map[string]CostResult, len(results))
	for _, r := range results {
		byKey[r.ResourceType+"\x00"+r.ResourceID] = r
	}

	priced := make([]*pbc.PricedResource, 0, len(descs))
	summaries := make([]PricedSummary, 0, len(descs))
	anyPriced := false
	for _, d := range descs {
		r, found := byKey[d.GetResourceType()+"\x00"+d.GetId()]
		p := &pbc.PricedResource{Resource: d}
		switch {
		case !found:
			p.Note = unpricedMissing
		case r.Error != nil:
			p.Note = r.Error.Message
		default:
			if cost, ok := amount(r); ok {
				p.Priced, p.Cost, p.Currency, p.Note = true, cost, r.Currency, r.Notes
				anyPriced = true
			} else {
				p.Note = firstNonEmpty(r.Notes, unpricedZeroNote)
			}
		}
		priced = append(priced, p)
		summaries = append(summaries, PricedSummary{
			Kind: d.GetTags()[subjectKeyKind], ID: d.GetId(), ResourceType: d.GetResourceType(), SKU: d.GetSku(),
			Monthly: p.GetCost(), Priced: p.GetPriced(), Note: p.GetNote(),
		})
	}
	if len(descs) > 0 && !anyPriced {
		return nil, nil, "", errors.New(nothingPricedMessage)
	}
	currency, err := pluginsdk.ResolveCurrency(priced)
	if err != nil {
		return nil, nil, "", fmt.Errorf("%w: %w", ErrMixedCurrencies, err)
	}
	return priced, summaries, currency, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetWindowCost prices resources from actual spend between from and to.
// State-based estimation is disabled, so a plugin total of zero stays zero.
// The allocated amount is CostResult.TotalCost.
func (e *Engine) GetWindowCost(
	ctx context.Context, resources []ResourceDescriptor, from, to time.Time,
) ([]CostResult, error) {
	return e.GetActualCostWithOptions(ctx, ActualCostRequest{
		Resources:         resources,
		From:              from,
		To:                to,
		SkipStateEstimate: true,
	})
}
