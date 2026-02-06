package proto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk/mapping"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
)

// ErrEstimateCostNotSupported indicates the EstimateCost RPC is not yet implemented.
var ErrEstimateCostNotSupported = errors.New("EstimateCost RPC not yet implemented in finfocus-spec v0.5.5")

const (
	// maxErrorsToDisplay is the maximum number of errors to show in summary before truncating.
	maxErrorsToDisplay = 5
	// awsProvider is the AWS provider name constant.
	awsProvider = "aws"
)

// ErrorDetail captures information about a failed resource cost calculation.
type ErrorDetail struct {
	ResourceType string
	ResourceID   string
	PluginName   string
	Error        error
	Timestamp    time.Time
}

// CostResultWithErrors wraps results and any errors encountered during cost calculation.
type CostResultWithErrors struct {
	Results []*CostResult
	Errors  []ErrorDetail
}

// HasErrors returns true if any errors were encountered during cost calculation.
func (c *CostResultWithErrors) HasErrors() bool {
	return len(c.Errors) > 0
}

// ErrorSummary returns a human-readable summary of errors.
// Truncates the output after 5 errors to keep it readable.
func (c *CostResultWithErrors) ErrorSummary() string {
	if !c.HasErrors() {
		return ""
	}

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("%d resource(s) failed:\n", len(c.Errors)))

	for i, err := range c.Errors {
		if i >= maxErrorsToDisplay {
			summary.WriteString(
				fmt.Sprintf("  ... and %d more errors\n", len(c.Errors)-maxErrorsToDisplay),
			)
			break
		}
		summary.WriteString(
			fmt.Sprintf("  - %s (%s): %v\n", err.ResourceType, err.ResourceID, err.Error),
		)
	}

	return summary.String()
}

// GetProjectedCostWithErrors queries projected costs for each resource and aggregates successful results
// together with per-resource error details.
func GetProjectedCostWithErrors(
	ctx context.Context,
	client CostSourceClient,
	pluginName string,
	resources []*ResourceDescriptor,
) *CostResultWithErrors {
	result := &CostResultWithErrors{
		Results: []*CostResult{},
		Errors:  []ErrorDetail{},
	}

	for _, resource := range resources {
		// Pre-flight validation: construct proto request and validate before gRPC call
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Properties)
		protoReq := &pbc.GetProjectedCostRequest{
			Resource: &pbc.ResourceDescriptor{
				Id:           resource.ID,
				Provider:     resource.Provider,
				ResourceType: resource.Type,
				Sku:          sku,
				Region:       region,
				Tags:         resource.Properties,
			},
		}

		// Validate request using pluginsdk validation functions
		if err := pluginsdk.ValidateProjectedCostRequest(protoReq); err != nil {
			// Log validation failure at WARN level with context
			log := logging.FromContext(ctx)
			log.Warn().
				Str("resource_type", resource.Type).
				Err(err).
				Msg("pre-flight validation failed")

			// Track error
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				PluginName:   pluginName,
				Error:        fmt.Errorf("pre-flight validation failed: %w", err),
				Timestamp:    time.Now(),
			})

			// Add placeholder result with VALIDATION note
			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
				Notes:       fmt.Sprintf("VALIDATION: %v", err),
			})
			continue
		}

		req := &GetProjectedCostRequest{
			Resources: []*ResourceDescriptor{resource},
		}

		resp, err := client.GetProjectedCost(ctx, req)
		if err != nil {
			// Track error instead of silent failure
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				PluginName:   pluginName,
				Error:        fmt.Errorf("plugin call failed: %w", err),
				Timestamp:    time.Now(),
			})

			// Add placeholder result with error note
			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
				Notes:       fmt.Sprintf("ERROR: %v", err),
			})
			continue
		}

		// Add successful results
		if len(resp.Results) > 0 {
			result.Results = append(result.Results, resp.Results...)
		} else {
			// Add empty result if no results returned
			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
			})
		}
	}

	return result
}

// GetActualCostWithErrors retrieves actual cost data for each resource ID in req
// and returns both successful CostResult entries and per-resource ErrorDetail records.
func GetActualCostWithErrors(
	ctx context.Context,
	client CostSourceClient,
	pluginName string,
	req *GetActualCostRequest,
) *CostResultWithErrors {
	result := &CostResultWithErrors{
		Results: []*CostResult{},
		Errors:  []ErrorDetail{},
	}

	for _, resourceID := range req.ResourceIDs {
		// Pre-flight validation: construct proto request and validate before gRPC call
		protoReq := &pbc.GetActualCostRequest{
			ResourceId: resourceID,
			Start:      timestamppb.New(time.Unix(req.StartTime, 0)),
			End:        timestamppb.New(time.Unix(req.EndTime, 0)),
			Tags:       make(map[string]string),
		}

		// Validate request using pluginsdk validation functions
		if err := pluginsdk.ValidateActualCostRequest(protoReq); err != nil {
			// Log validation failure at WARN level with context
			log := logging.FromContext(ctx)
			log.Warn().
				Str("resource_id", resourceID).
				Err(err).
				Msg("pre-flight validation failed for actual cost")

			// Track error
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceID: resourceID,
				PluginName: pluginName,
				Error:      fmt.Errorf("pre-flight validation failed: %w", err),
				Timestamp:  time.Now(),
			})

			// Add placeholder result with VALIDATION note
			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
				Notes:       fmt.Sprintf("VALIDATION: %v", err),
			})
			continue
		}

		singleReq := &GetActualCostRequest{
			ResourceIDs: []string{resourceID},
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
		}

		resp, err := client.GetActualCost(ctx, singleReq)
		if err != nil {
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceID: resourceID,
				PluginName: pluginName,
				Error:      fmt.Errorf("plugin call failed: %w", err),
				Timestamp:  time.Now(),
			})

			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
				Notes:       fmt.Sprintf("ERROR: %v", err),
			})
			continue
		}

		// Aggregate total cost from results and convert to CostResult
		if len(resp.Results) > 0 {
			for _, actual := range resp.Results {
				costResult := &CostResult{
					Currency:       actual.Currency,
					MonthlyCost:    actual.TotalCost, // Total cost for the period
					HourlyCost:     0,
					CostBreakdown:  actual.CostBreakdown,
					Sustainability: make(map[string]SustainabilityMetric),
				}

				// Map impact metrics
				for k, v := range actual.Sustainability {
					costResult.Sustainability[k] = SustainabilityMetric{
						Value: v.Value,
						Unit:  v.Unit,
					}
				}
				result.Results = append(result.Results, costResult)
			}
		} else {
			result.Results = append(result.Results, &CostResult{
				Currency:    "USD",
				MonthlyCost: 0,
				HourlyCost:  0,
			})
		}
	}

	return result
}

// Empty represents an empty request/response for compatibility with existing engine code.
type Empty struct{}

// ResourceDescriptor describes a cloud resource for cost calculation requests.
// It contains the resource type, provider, and properties needed for pricing lookups.
type ResourceDescriptor struct {
	// ID is a client-generated identifier for request/response correlation.
	// Plugins copy this to recommendation ResourceID for proper matching.
	ID         string
	Type       string
	Provider   string
	Properties map[string]string
}

// GetProjectedCostRequest contains resources for which projected costs should be calculated.
type GetProjectedCostRequest struct {
	Resources []*ResourceDescriptor
}

// CostResult represents the calculated cost information for a single resource.
// It includes monthly and hourly costs, currency, and detailed cost breakdowns.
type CostResult struct {
	Currency       string
	MonthlyCost    float64
	HourlyCost     float64
	Notes          string
	CostBreakdown  map[string]float64
	Sustainability map[string]SustainabilityMetric
}

// SustainabilityMetric represents a single sustainability impact measurement.
type SustainabilityMetric struct {
	Value float64
	Unit  string
}

// GetProjectedCostResponse contains the results of projected cost calculations.
type GetProjectedCostResponse struct {
	Results []*CostResult
}

// GetActualCostRequest contains parameters for querying historical actual costs.
// It includes resource IDs and a time range for cost data retrieval.
type GetActualCostRequest struct {
	ResourceIDs []string
	StartTime   int64
	EndTime     int64
}

// ActualCostResult represents the calculated actual cost data retrieved from cloud providers.
// It includes the total cost and detailed breakdowns by service or resource.
type ActualCostResult struct {
	Currency       string
	TotalCost      float64
	CostBreakdown  map[string]float64
	Sustainability map[string]SustainabilityMetric
}

// GetActualCostResponse contains the results of actual cost queries.
type GetActualCostResponse struct {
	Results []*ActualCostResult
}

// NameResponse contains the plugin name returned by the Name RPC call.
type NameResponse struct {
	Name string
}

// GetName returns the plugin name from the response.
func (n *NameResponse) GetName() string {
	return n.Name
}

// GetRecommendationsRequest contains parameters for retrieving cost optimization recommendations.
// It supports filtering by target resources, pagination, and exclusion of dismissed recommendations.
type GetRecommendationsRequest struct {
	// TargetResources specifies the resources to analyze for recommendations.
	// When empty, plugins return recommendations for all resources in scope.
	TargetResources []*ResourceDescriptor

	// ProjectionPeriod specifies the time period for savings projection.
	// Valid values: "daily", "monthly" (default), "annual".
	ProjectionPeriod string

	// PageSize is the maximum number of recommendations to return (default: 50, max: 1000).
	PageSize int32

	// PageToken is the continuation token from a previous response.
	PageToken string

	// ExcludedRecommendationIDs contains IDs of recommendations to exclude from results.
	ExcludedRecommendationIDs []string
}

// GetRecommendationsResponse contains the recommendations and summary from a GetRecommendations call.
type GetRecommendationsResponse struct {
	// Recommendations is the list of cost optimization recommendations.
	Recommendations []*Recommendation

	// NextPageToken is the token for retrieving the next page (empty if last page).
	NextPageToken string
}

// Recommendation represents a single cost optimization recommendation from a plugin.
// This is the internal representation that maps to the protobuf Recommendation message.
type Recommendation struct {
	// ID is a unique identifier for this recommendation.
	ID string

	// Category classifies the type of recommendation (e.g., "COST", "PERFORMANCE").
	Category string

	// ActionType specifies what action is recommended (e.g., "RIGHTSIZE", "TERMINATE").
	ActionType string

	// Description is a human-readable summary of the recommendation.
	Description string

	// ResourceID identifies the affected resource.
	ResourceID string

	// Source identifies the data source (e.g., "aws", "kubecost", "azure-advisor").
	Source string

	// Impact contains the financial impact assessment.
	Impact *RecommendationImpact

	// Metadata contains additional provider-specific information.
	Metadata map[string]string
}

// RecommendationImpact describes the financial impact of implementing a recommendation.
type RecommendationImpact struct {
	// EstimatedSavings is the estimated cost savings.
	EstimatedSavings float64

	// Currency is the ISO 4217 currency code.
	Currency string

	// CurrentCost is the current cost of the resource.
	CurrentCost float64

	// ProjectedCost is the projected cost after implementing the recommendation.
	ProjectedCost float64

	// SavingsPercentage is the savings as a percentage.
	SavingsPercentage float64
}

// DismissRecommendationRequest contains parameters for dismissing a recommendation via plugin RPC.
type DismissRecommendationRequest struct {
	// RecommendationID is the unique identifier of the recommendation to dismiss.
	RecommendationID string

	// Reason is the DismissalReason proto enum value.
	Reason pbc.DismissalReason

	// CustomReason is the free-text explanation (required when Reason is OTHER).
	CustomReason string

	// ExpiresAt is the snooze expiry; nil means permanent dismissal.
	ExpiresAt *time.Time

	// DismissedBy identifies the user who dismissed the recommendation.
	DismissedBy string
}

// DismissRecommendationResponse contains the result of a dismiss RPC call.
type DismissRecommendationResponse struct {
	// Success indicates whether the plugin accepted the dismissal.
	Success bool

	// Message is the plugin's response message.
	Message string

	// DismissedAt is when the plugin recorded the dismissal.
	DismissedAt time.Time

	// ExpiresAt is the snooze expiry echoed back from the plugin.
	ExpiresAt *time.Time

	// RecommendationID echoes back the dismissed recommendation's ID.
	RecommendationID string
}

// CostSourceClient wraps the generated gRPC client from finfocus-spec.
//
//nolint:dupl // Mock implementation in adapter_test.go intentionally mirrors this interface.
type CostSourceClient interface {
	Name(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*NameResponse, error)
	GetProjectedCost(
		ctx context.Context,
		in *GetProjectedCostRequest,
		opts ...grpc.CallOption,
	) (*GetProjectedCostResponse, error)
	GetActualCost(
		ctx context.Context,
		in *GetActualCostRequest,
		opts ...grpc.CallOption,
	) (*GetActualCostResponse, error)
	GetRecommendations(
		ctx context.Context,
		in *GetRecommendationsRequest,
		opts ...grpc.CallOption,
	) (*GetRecommendationsResponse, error)
	GetPluginInfo(
		ctx context.Context,
		in *Empty,
		opts ...grpc.CallOption,
	) (*pbc.GetPluginInfoResponse, error)
	GetBudgets(
		ctx context.Context,
		in *pbc.GetBudgetsRequest,
		opts ...grpc.CallOption,
	) (*pbc.GetBudgetsResponse, error)
	DryRun(
		ctx context.Context,
		in *pbc.DryRunRequest,
		opts ...grpc.CallOption,
	) (*pbc.DryRunResponse, error)
	DismissRecommendation(
		ctx context.Context,
		in *DismissRecommendationRequest,
		opts ...grpc.CallOption,
	) (*DismissRecommendationResponse, error)
}

// NewCostSourceClient creates a new cost source client using the real proto client.
func NewCostSourceClient(conn *grpc.ClientConn) CostSourceClient {
	return &clientAdapter{
		client: pbc.NewCostSourceServiceClient(conn),
	}
}

// clientAdapter adapts the generated client to our internal interface.
type clientAdapter struct {
	client pbc.CostSourceServiceClient
}

func (c *clientAdapter) Name(
	ctx context.Context,
	_ *Empty,
	opts ...grpc.CallOption,
) (*NameResponse, error) {
	resp, err := c.client.Name(ctx, &pbc.NameRequest{}, opts...)
	if err != nil {
		return nil, err
	}
	return &NameResponse{Name: resp.GetName()}, nil
}

func (c *clientAdapter) GetPluginInfo(
	ctx context.Context,
	_ *Empty,
	opts ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	return c.client.GetPluginInfo(ctx, &pbc.GetPluginInfoRequest{}, opts...)
}

func (c *clientAdapter) GetBudgets(
	ctx context.Context,
	in *pbc.GetBudgetsRequest,
	opts ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	return c.client.GetBudgets(ctx, in, opts...)
}

func (c *clientAdapter) DryRun(
	ctx context.Context,
	in *pbc.DryRunRequest,
	opts ...grpc.CallOption,
) (*pbc.DryRunResponse, error) {
	return c.client.DryRun(ctx, in, opts...)
}

func (c *clientAdapter) DismissRecommendation(
	ctx context.Context,
	in *DismissRecommendationRequest,
	opts ...grpc.CallOption,
) (*DismissRecommendationResponse, error) {
	req := &pbc.DismissRecommendationRequest{
		RecommendationId: in.RecommendationID,
		Reason:           in.Reason,
		CustomReason:     in.CustomReason,
		DismissedBy:      in.DismissedBy,
	}

	if in.ExpiresAt != nil {
		req.ExpiresAt = timestamppb.New(*in.ExpiresAt)
	}

	resp, err := c.client.DismissRecommendation(ctx, req, opts...)
	if err != nil {
		return nil, fmt.Errorf("DismissRecommendation RPC failed: %w", err)
	}

	result := &DismissRecommendationResponse{
		Success:          resp.GetSuccess(),
		Message:          resp.GetMessage(),
		RecommendationID: resp.GetRecommendationId(),
	}

	if resp.GetDismissedAt() != nil {
		result.DismissedAt = resp.GetDismissedAt().AsTime()
	}

	if resp.GetExpiresAt() != nil {
		expiresAt := resp.GetExpiresAt().AsTime()
		result.ExpiresAt = &expiresAt
	}

	return result, nil
}

// resolveSKUAndRegion extracts the SKU and region from resource properties based on the cloud provider.
// It recognizes provider values such as "aws", "azure", "azure-native", "gcp", and "google-native" and
// uses provider-specific extraction; for other providers it uses generic extraction helpers.
// If the region cannot be determined from properties, the function falls back to the AWS_REGION or
// AWS_DEFAULT_REGION environment variables.
// provider is the cloud provider identifier; properties contains resource-specific key/value attributes.
func resolveSKUAndRegion(provider string, properties map[string]string) (string, string) {
	var sku, region string
	switch strings.ToLower(provider) {
	case awsProvider:
		sku = mapping.ExtractAWSSKU(properties)
		if sku == "" {
			// Fallback for RDS and other AWS resources not covered by ExtractAWSSKU
			sku = mapping.ExtractSKU(properties, "dbInstanceClass", "sku", "type", "tier")
		}
		region = mapping.ExtractAWSRegion(properties)
	case "azure", "azure-native":
		sku = mapping.ExtractAzureSKU(properties)
		region = mapping.ExtractAzureRegion(properties)
	case "gcp", "google-native":
		sku = mapping.ExtractGCPSKU(properties)
		region = mapping.ExtractGCPRegion(properties)
	default:
		sku = mapping.ExtractSKU(properties)
		region = mapping.ExtractRegion(properties)
	}

	// Fallback to AWS environment variables for region if still empty
	// IMPORTANT: Only apply AWS-specific env vars to AWS resources to avoid
	// incorrect region assignment for Azure/GCP resources (SC-001 fix)
	if region == "" && strings.ToLower(provider) == "aws" {
		if envReg := os.Getenv("AWS_REGION"); envReg != "" {
			region = envReg
		} else {
			envReg = os.Getenv("AWS_DEFAULT_REGION")
			if envReg != "" {
				region = envReg
			}
		}
	}

	return sku, region
}

func (c *clientAdapter) GetProjectedCost(
	ctx context.Context,
	in *GetProjectedCostRequest,
	opts ...grpc.CallOption,
) (*GetProjectedCostResponse, error) {
	// Convert internal request to proto request
	var results []*CostResult

	for _, resource := range in.Resources {
		// Extract SKU and region from properties using intelligent mapping
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Properties)

		req := &pbc.GetProjectedCostRequest{
			Resource: &pbc.ResourceDescriptor{
				Id:           resource.ID,
				Provider:     resource.Provider,
				ResourceType: resource.Type,
				Sku:          sku,
				Region:       region,
				Tags:         resource.Properties,
			},
		}

		resp, err := c.client.GetProjectedCost(ctx, req, opts...)
		if err != nil {
			// Continue to next resource on error
			continue
		}

		result := &CostResult{
			Currency:    resp.GetCurrency(),
			MonthlyCost: resp.GetCostPerMonth(),
			HourlyCost:  resp.GetUnitPrice(), // Assuming hourly for now
			Notes:       resp.GetBillingDetail(),
			CostBreakdown: map[string]float64{
				"unit_price": resp.GetUnitPrice(),
			},
			Sustainability: make(map[string]SustainabilityMetric),
		}

		// Map impact metrics
		for _, metric := range resp.GetImpactMetrics() {
			var key string
			switch metric.GetKind() {
			case pbc.MetricKind_METRIC_KIND_CARBON_FOOTPRINT:
				key = "carbon_footprint"
			case pbc.MetricKind_METRIC_KIND_ENERGY_CONSUMPTION:
				key = "energy_consumption"
			case pbc.MetricKind_METRIC_KIND_WATER_USAGE:
				key = "water_usage"
			case pbc.MetricKind_METRIC_KIND_UNSPECIFIED:
				key = "unspecified"
			default:
				key = strings.ToLower(metric.GetKind().String())
			}
			result.Sustainability[key] = SustainabilityMetric{
				Value: metric.GetValue(),
				Unit:  metric.GetUnit(),
			}
		}
		results = append(results, result)
	}

	return &GetProjectedCostResponse{Results: results}, nil
}

func (c *clientAdapter) GetActualCost(
	ctx context.Context,
	in *GetActualCostRequest,
	opts ...grpc.CallOption,
) (*GetActualCostResponse, error) {
	// Convert internal request to proto request
	var results []*ActualCostResult

	for _, resourceID := range in.ResourceIDs {
		req := &pbc.GetActualCostRequest{
			ResourceId: resourceID,
			Start:      timestamppb.New(time.Unix(in.StartTime, 0)),
			End:        timestamppb.New(time.Unix(in.EndTime, 0)),
			Tags:       make(map[string]string), // Empty tags for now
		}

		resp, err := c.client.GetActualCost(ctx, req, opts...)
		if err != nil {
			// Continue to next resource on error
			continue
		}

		// Aggregate total cost from results
		totalCost := 0.0
		breakdown := make(map[string]float64)

		for _, result := range resp.GetResults() {
			totalCost += result.GetCost()
			if result.GetSource() != "" {
				breakdown[result.GetSource()] = result.GetCost()
			}
		}

		result := &ActualCostResult{
			Currency:       "USD", // Default to USD if not specified
			TotalCost:      totalCost,
			CostBreakdown:  breakdown,
			Sustainability: make(map[string]SustainabilityMetric),
		}

		// Aggregate impact metrics (summing values for same kind across results)
		for _, pbcResult := range resp.GetResults() {
			for _, metric := range pbcResult.GetImpactMetrics() {
				var key string
				switch metric.GetKind() {
				case pbc.MetricKind_METRIC_KIND_CARBON_FOOTPRINT:
					key = "carbon_footprint"
				case pbc.MetricKind_METRIC_KIND_ENERGY_CONSUMPTION:
					key = "energy_consumption"
				case pbc.MetricKind_METRIC_KIND_WATER_USAGE:
					key = "water_usage"
				case pbc.MetricKind_METRIC_KIND_UNSPECIFIED:
					key = "unspecified"
				default:
					key = strings.ToLower(metric.GetKind().String())
				}

				m := result.Sustainability[key]
				m.Value += metric.GetValue()
				m.Unit = metric.GetUnit()
				result.Sustainability[key] = m
			}
		}
		results = append(results, result)
	}

	return &GetActualCostResponse{Results: results}, nil
}

// EstimateCostRequest represents the internal request for what-if cost estimation.
type EstimateCostRequest struct {
	// Resource is the base resource descriptor
	Resource *ResourceDescriptor `json:"resource,omitempty"`

	// PropertyOverrides are the changes to evaluate
	PropertyOverrides map[string]string `json:"propertyOverrides,omitempty"`

	// UsageProfile optionally provides context (dev, prod, etc.)
	UsageProfile string `json:"usageProfile,omitempty"`
}

// EstimateCostResponse contains the results of a cost estimation.
type EstimateCostResponse struct {
	// Baseline is the cost with original properties
	Baseline *CostResult `json:"baseline,omitempty"`

	// Modified is the cost with property overrides applied
	Modified *CostResult `json:"modified,omitempty"`

	// Deltas contains per-property cost impact breakdown
	Deltas []*CostDelta `json:"deltas,omitempty"`
}

// CostDelta represents the cost impact of a single property change.
type CostDelta struct {
	// Property is the name of the property that was changed
	Property string `json:"property"`

	// OriginalValue is the value before the change
	OriginalValue string `json:"originalValue"`

	// NewValue is the value after the change
	NewValue string `json:"newValue"`

	// CostChange is the monthly cost difference
	// Positive = increase, negative = savings
	CostChange float64 `json:"costChange"`
}

// BuildEstimateCostRequest constructs an EstimateCostRequest proto message.
//
// This is the adapter layer function that converts engine-level types to
// proto-level types for gRPC communication with plugins.
//
// Parameters:
//   - resource: The ResourceDescriptor to estimate costs for
//   - overrides: Property overrides to apply for the modified calculation
//
// Returns:
//   - *EstimateCostRequest: The internal request (nil when RPC not implemented)
//   - error: ErrEstimateCostNotSupported until the RPC is implemented
func BuildEstimateCostRequest(
	_ *ResourceDescriptor,
	_ map[string]string,
) (*EstimateCostRequest, error) {
	return nil, ErrEstimateCostNotSupported
}

func (c *clientAdapter) GetRecommendations(
	ctx context.Context,
	in *GetRecommendationsRequest,
	opts ...grpc.CallOption,
) (*GetRecommendationsResponse, error) {
	// Convert internal request to proto request
	req := &pbc.GetRecommendationsRequest{
		ProjectionPeriod:          in.ProjectionPeriod,
		PageSize:                  in.PageSize,
		PageToken:                 in.PageToken,
		ExcludedRecommendationIds: in.ExcludedRecommendationIDs,
	}

	// Convert target resources if provided
	for _, resource := range in.TargetResources {
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Properties)
		req.TargetResources = append(req.TargetResources, &pbc.ResourceDescriptor{
			Id:           resource.ID,
			Provider:     resource.Provider,
			ResourceType: resource.Type,
			Sku:          sku,
			Region:       region,
			Tags:         resource.Properties,
		})
	}

	resp, err := c.client.GetRecommendations(ctx, req, opts...)
	if err != nil {
		return nil, err
	}

	// Convert proto recommendations to internal format
	var recommendations []*Recommendation
	for _, rec := range resp.GetRecommendations() {
		protoRec := &Recommendation{
			ID:          rec.GetId(),
			Category:    rec.GetCategory().String(),
			ActionType:  rec.GetActionType().String(),
			Description: rec.GetDescription(),
			Source:      rec.GetSource(),
			Metadata:    rec.GetMetadata(),
		}

		// Extract resource ID from resource info if available
		if rec.GetResource() != nil {
			protoRec.ResourceID = rec.GetResource().GetId()
		}

		// Convert impact if available
		if rec.GetImpact() != nil {
			protoRec.Impact = &RecommendationImpact{
				EstimatedSavings:  rec.GetImpact().GetEstimatedSavings(),
				Currency:          rec.GetImpact().GetCurrency(),
				CurrentCost:       rec.GetImpact().GetCurrentCost(),
				ProjectedCost:     rec.GetImpact().GetProjectedCost(),
				SavingsPercentage: rec.GetImpact().GetSavingsPercentage(),
			}
		}

		recommendations = append(recommendations, protoRec)
	}

	return &GetRecommendationsResponse{
		Recommendations: recommendations,
		NextPageToken:   resp.GetNextPageToken(),
	}, nil
}
