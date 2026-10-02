package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"google.golang.org/grpc"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// ModeRunRate labels results computed from point-in-time usage and monthly prices.
const ModeRunRate = "run-rate"

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
}

// ClusterRequest selects the cluster and policy for an allocation run.
type ClusterRequest struct {
	Scope      string
	Namespace  string
	Selector   map[string]string
	PolicyJSON []byte
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

// RunClusterAllocation gathers usage, prices the reported resources through the
// projected-cost path, allocates, and verifies that rows sum to the priced total.
func RunClusterAllocation(
	ctx context.Context,
	usage UsageSource,
	alloc Allocator,
	pricer ResourcePricer,
	req ClusterRequest,
) (*ClusterResult, error) {
	selector := map[string]string{}
	maps.Copy(selector, req.Selector)
	if req.Namespace != "" {
		selector["namespace"] = req.Namespace
	}
	stats, err := usage.GetStats(ctx, &pbc.GetStatsRequest{Scope: req.Scope, Selector: selector})
	if err != nil {
		return nil, fmt.Errorf("get usage stats: %w", err)
	}
	if stats.GetMode() != pbc.StatsMode_STATS_MODE_RUN_RATE {
		return nil, fmt.Errorf("%w (usage source returned %s)", ErrHistoricalUnsupported, stats.GetMode())
	}

	priced, summaries, currency, err := priceResources(ctx, pricer, stats.GetPriceable())
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

	res := &ClusterResult{
		Mode:            ModeRunRate,
		Currency:        currency,
		Priced:          summaries,
		NamespaceScoped: req.Namespace != "",
		PolicyDigest:    resp.GetPolicyDigest(),
		EffectivePolicy: json.RawMessage(resp.GetEffectivePolicyJson()),
		Warnings:        append(append([]string{}, stats.GetWarnings()...), resp.GetWarnings()...),
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
	return res, nil
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

func priceResources(
	ctx context.Context,
	pricer ResourcePricer,
	descs []*pbc.ResourceDescriptor,
) ([]*pbc.PricedResource, []PricedSummary, string, error) {
	resources := make([]ResourceDescriptor, len(descs))
	for i, d := range descs {
		resources[i] = PriceableToResource(d)
	}
	results, err := pricer.GetProjectedCostWithErrors(ctx, resources)
	if err != nil {
		return nil, nil, "", fmt.Errorf("price cluster resources: %w", err)
	}
	byKey := make(map[string]CostResult, len(results.Results))
	for _, r := range results.Results {
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
			p.Note = "no pricing result returned"
		case r.Error != nil:
			p.Note = r.Error.Message
		case r.Monthly <= 0:
			p.Note = firstNonEmpty(r.Notes, "priced at $0; treated as unpriced")
		default:
			p.Priced, p.Cost, p.Currency, p.Note = true, r.Monthly, r.Currency, r.Notes
			anyPriced = true
		}
		priced = append(priced, p)
		summaries = append(summaries, PricedSummary{
			Kind: d.GetTags()[subjectKeyKind], ID: d.GetId(), ResourceType: d.GetResourceType(), SKU: d.GetSku(),
			Monthly: p.GetCost(), Priced: p.GetPriced(), Note: p.GetNote(),
		})
	}
	if len(descs) > 0 && !anyPriced {
		return nil, nil, "", errors.New("no priceable resource could be priced; check that a pricing plugin " +
			"(e.g. aws-public for the nodes' region) is installed")
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
