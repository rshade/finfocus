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
	"github.com/rshade/finfocus/internal/awsutil"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/skus"
)

// ErrEstimateCostNotSupported indicates the EstimateCost RPC is not yet implemented.
var ErrEstimateCostNotSupported = errors.New("EstimateCost RPC not yet implemented in finfocus-spec v0.5.6")

// ErrPropertiesMultiResource indicates Properties cannot be used with multiple ResourceIDs
// because each resource requires its own cloud ID, ARN, and tag mappings.
var ErrPropertiesMultiResource = errors.New(
	"properties cannot be used with multiple ResourceIDs: each resource requires its own properties",
)

const (
	// maxErrorsToDisplay is the maximum number of errors to show in summary before truncating.
	maxErrorsToDisplay = 5
	// awsProvider is the AWS provider name constant.
	awsProvider = "aws"

	// Cloud identifier property keys injected by ingest.MapStateResource.
	// Duplicated here because proto cannot import ingest (circular dependency via engine).
	// Must stay in sync with ingest.PropertyPulumiCloudID / PropertyPulumiARN.
	propCloudID = "pulumi:cloudId"
	propARN     = "pulumi:arn"
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
//   - Errors: a slice of ErrorDetail with per-resource failure information and timestamps.
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
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Type, resource.Properties)
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

// validateActualCostRequest returns a non-nil *CostResultWithErrors when Properties is provided
// together with more than one ResourceID. Returns nil if the request is valid.
func validateActualCostRequest(pluginName string, req *GetActualCostRequest) *CostResultWithErrors {
	if req.Properties != nil && len(req.ResourceIDs) > 1 {
		return &CostResultWithErrors{
			Results: []*CostResult{},
			Errors: []ErrorDetail{{
				ResourceType: req.ResourceType,
				ResourceID:   strings.Join(req.ResourceIDs, ", "),
				PluginName:   pluginName,
				Error:        ErrPropertiesMultiResource,
				Timestamp:    time.Now(),
			}},
		}
	}
	return nil
}

// appendActualCostPlaceholder appends a zero-valued CostResult with USD currency and the
// provided notes to the given CostResultWithErrors' Results slice.
//
// result is the accumulator to which the placeholder result will be appended.
// notes is an informational string stored in the placeholder's Notes field.
func appendActualCostPlaceholder(result *CostResultWithErrors, notes string) {
	result.Results = append(result.Results, &CostResult{
		Currency:    "USD",
		MonthlyCost: 0,
		HourlyCost:  0,
		Notes:       notes,
	})
}

// recordActualCostValidationError records a pre-flight validation failure for an actual cost lookup.
// It logs a warning, appends an ErrorDetail (including the plugin name, resource type, resource
// and cloud IDs, the wrapped validation error, and current timestamp) to result.Errors, and
// appends a validation placeholder CostResult to result. The provided result is mutated in-place.
func recordActualCostValidationError(
	ctx context.Context,
	result *CostResultWithErrors,
	pluginName string,
	resourceType string,
	resourceID string,
	cloudID string,
	validationErr error,
) {
	log := logging.FromContext(ctx)
	log.Warn().
		Str("resource_type", resourceType).
		Str("resource_id", resourceID).
		Str("cloud_id", cloudID).
		Err(validationErr).
		Msg("pre-flight validation failed for actual cost")

	result.Errors = append(result.Errors, ErrorDetail{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PluginName:   pluginName,
		Error:        fmt.Errorf("pre-flight validation failed: %w", validationErr),
		Timestamp:    time.Now(),
	})

	appendActualCostPlaceholder(result, fmt.Sprintf("VALIDATION: %v", validationErr))
}

// recordActualCostPluginError records a plugin call failure for the specified resource and
// appends a placeholder error result.
func recordActualCostPluginError(
	result *CostResultWithErrors,
	pluginName string,
	resourceType string,
	resourceID string,
	pluginErr error,
) {
	result.Errors = append(result.Errors, ErrorDetail{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		PluginName:   pluginName,
		Error:        fmt.Errorf("plugin call failed: %w", pluginErr),
		Timestamp:    time.Now(),
	})

	appendActualCostPlaceholder(result, fmt.Sprintf("ERROR: %v", pluginErr))
}

// appendActualCostResults converts each ActualCostResult into a CostResult and appends it to result.Results.
// It deep-copies both CostBreakdown and Sustainability metrics into new maps.
// The provided result is mutated in-place.
//
// Parameters:
//   - result: destination CostResultWithErrors whose Results slice will be extended.
//   - actualResults: slice of ActualCostResult values to convert and append.
func appendActualCostResults(result *CostResultWithErrors, actualResults []*ActualCostResult) {
	for _, actual := range actualResults {
		costResult := &CostResult{
			Currency:       actual.Currency,
			MonthlyCost:    actual.TotalCost, // Total cost for the period
			HourlyCost:     0,
			CostBreakdown:  make(map[string]float64, len(actual.CostBreakdown)),
			Sustainability: make(map[string]SustainabilityMetric),
		}

		for k, v := range actual.CostBreakdown {
			costResult.CostBreakdown[k] = v
		}

		for k, v := range actual.Sustainability {
			costResult.Sustainability[k] = v
		}
		result.Results = append(result.Results, costResult)
	}
}

// GetActualCostWithErrors validates the request, then for each ResourceID it resolves cloud
// identifiers and tags (including optional SKU and region enrichment when Provider is set),
// validates the plugin-facing request, and invokes the client's GetActualCost. For each
// resource it appends either the plugin's cost results or a placeholder CostResult and records
// any per-resource validation or plugin errors in the returned ErrorDetail slice.
//
// Parameters:
//   - ctx: request context for cancellation and timeouts.
//   - client: the CostSourceClient used to call plugin GetActualCost.
//   - pluginName: human-readable name of the plugin (used in ErrorDetail entries).
//   - req: parameters for the actual cost query; must include ResourceIDs and time range.
//
// Returns:
//
//	A *CostResultWithErrors containing Results for each resource (actual or placeholder)
//	and any per-resource ErrorDetail entries. If the request is invalid (for example,
//	Properties are provided with multiple ResourceIDs) the returned CostResultWithErrors
//	will contain the validation error and no per-resource processing will be performed.
func GetActualCostWithErrors(
	ctx context.Context,
	client CostSourceClient,
	pluginName string,
	req *GetActualCostRequest,
) *CostResultWithErrors {
	if errResult := validateActualCostRequest(pluginName, req); errResult != nil {
		return errResult
	}

	result := &CostResultWithErrors{
		Results: []*CostResult{},
		Errors:  []ErrorDetail{},
	}

	for _, resourceID := range req.ResourceIDs {
		cloudID, arn, tags := resolveActualCostIdentifiers(resourceID, req.Properties)

		// Inject SKU and region into tags for plugins that need pricing dimensions.
		// The projected cost path calls resolveSKUAndRegion via the proto request builder,
		// but actual cost requests only carry tags. Enrich tags so plugins like aws-public
		// can look up costs by instance type / volume type.
		if req.Provider != "" {
			enrichTagsWithSKUAndRegion(tags, req.Provider, req.ResourceType, req.Properties)
		}

		// Pre-flight validation: construct proto request and validate before gRPC call
		protoReq := &pbc.GetActualCostRequest{
			ResourceId: cloudID,
			Start:      timestamppb.New(time.Unix(req.StartTime, 0)),
			End:        timestamppb.New(time.Unix(req.EndTime, 0)),
			Tags:       tags,
			Arn:        arn,
		}

		if err := pluginsdk.ValidateActualCostRequest(protoReq); err != nil {
			recordActualCostValidationError(ctx, result, pluginName, req.ResourceType, resourceID, cloudID, err)
			continue
		}

		singleReq := &GetActualCostRequest{
			ResourceIDs: []string{resourceID},
			StartTime:   req.StartTime,
			EndTime:     req.EndTime,
			Properties:  req.Properties,
			Provider:    req.Provider,
		}

		resp, err := client.GetActualCost(ctx, singleReq)
		if err != nil {
			recordActualCostPluginError(result, pluginName, req.ResourceType, resourceID, err)
			continue
		}

		// Aggregate total cost from results and convert to CostResult
		if len(resp.Results) > 0 {
			appendActualCostResults(result, resp.Results)
		} else {
			appendActualCostPlaceholder(result, "")
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
	// Properties carries resource context (cloud IDs, ARN, tags) from state.
	// Used by the adapter to populate proto fields (ResourceId, Arn, Tags).
	Properties map[string]interface{}
	// Provider is the cloud provider identifier (e.g., "aws", "azure", "gcp").
	// When set, the adapter resolves SKU and region from Properties and injects
	// them into the tags sent to the plugin, enabling plugins like aws-public
	// to price resources by instance type / volume type.
	Provider string
	// ResourceType is the Pulumi type token (e.g., "aws:eks/cluster:Cluster").
	// Used by resolveSKUAndRegion as a fallback for well-known SKU resolution
	// when property-based extraction returns empty.
	ResourceType string
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

	// Reasoning carries plugin-provided warnings and caveats mapped from
	// proto Recommendation.Reasoning (field 14). These explain prerequisites
	// or risks associated with implementing the recommendation.
	Reasoning []string
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

// resolveSKUAndRegion determines the SKU and region for a resource using provider-specific
// extraction logic. For AWS it attempts AWS-specific SKU extraction, falls back to common
// property names, then to well-known SKU mappings; the region is taken from properties, the
// ARN, or the AWS_REGION/AWS_DEFAULT_REGION environment variables. For Azure and GCP it uses
// their respective extractors. Other providers use generic extraction. Both return values may
// resolveSKUAndRegion determines the SKU and region for a resource based on its provider, type, and stringified properties.
//
// resolveSKUAndRegion examines provider-specific fields and fallbacks to derive a SKU and a region for pricing/enrichment.
// For AWS it attempts AWS-specific SKU/region extraction, parses region from an ARN when present, and finally falls back to well-known SKU mappings.
// For Azure and GCP it uses provider-specific extractors. For other providers it uses generic SKU and region extractors.
// If the region remains empty for AWS resources, the function will also consult AWS environment variables `AWS_REGION` and `AWS_DEFAULT_REGION`.
//
// Parameters:
//   - provider: cloud provider identifier (e.g., "aws", "azure", "gcp").
//   - resourceType: the resource type token used for well-known SKU resolution when direct extraction fails.
//   - properties: map of stringified resource properties used by extractors (keys like ARN, tags, sku fields).
//
// Returns:
//   - sku: the resolved SKU string, or an empty string if none could be determined.
//   - region: the resolved region string, or an empty string if none could be determined.
func resolveSKUAndRegion(provider, resourceType string, properties map[string]string) (string, string) {
	var sku, region string
	switch strings.ToLower(provider) {
	case awsProvider:
		sku = mapping.ExtractAWSSKU(properties)
		if sku == "" {
			// Fallback for RDS and other AWS resources not covered by ExtractAWSSKU
			sku = mapping.ExtractSKU(properties, "dbInstanceClass", "sku", "type", "tier")
		}
		if sku == "" {
			// Fallback to well-known SKU mappings for resources with fixed costs
			// (e.g., EKS clusters have $0.10/hr but no SKU property in state)
			sku = skus.ResolveSKU(provider, resourceType, properties)
		}
		region = mapping.ExtractAWSRegion(properties)
		if region == "" {
			// Fallback: parse region from ARN (arn:aws:service:region:account:...)
			region = awsutil.RegionFromARN(properties[propARN])
		}
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

// resolveActualCostIdentifiers extracts the cloud identifier, ARN, and tags from a resource's properties.
// resourceID is used as the fallback cloud identifier when no cloud ID is present in properties.
//
// Returns the resolved cloudID (or the original resourceID if none found), the ARN (or an empty string),
// and a map of tags (empty if no tags are present).
func resolveActualCostIdentifiers(
	resourceID string,
	properties map[string]interface{},
) (string, string, map[string]string) {
	cloudID := resourceID
	var arn string
	tags := make(map[string]string)

	if properties == nil {
		return cloudID, arn, tags
	}

	// Use cloud ID if available (e.g., "i-0abc123", "db-instance-primary")
	if v, ok := properties[propCloudID]; ok {
		if s, isStr := v.(string); isStr && s != "" {
			cloudID = s
		}
	}

	// Extract ARN from properties
	if v, ok := properties[propARN]; ok {
		if s, isStr := v.(string); isStr && s != "" {
			arn = s
		}
	}

	// Extract tags from properties (prefer tagsAll for AWS completeness)
	tags = extractResourceTags(properties)

	return cloudID, arn, tags
}

// extractResourceTags extracts a flat map[string]string of tags from resource properties.
// It checks "tagsAll" first (AWS complete tag set), then "tags".
func extractResourceTags(properties map[string]interface{}) map[string]string {
	tags := make(map[string]string)

	// Try tagsAll first (AWS-specific: includes default tags + resource tags)
	if tagMap := extractTagMap(properties, "tagsAll"); len(tagMap) > 0 {
		return tagMap
	}

	// Fallback to tags
	if tagMap := extractTagMap(properties, "tags"); len(tagMap) > 0 {
		return tagMap
	}

	return tags
}

// extractTagMap returns a map[string]string of tags stored under the given key in properties.
// It looks up properties[key] and, if present and a map, copies its entries into a string map.
// Non-string values are converted to their string representation; unsupported types or a missing key return an empty map.
// extractTagMap extracts a tag map from properties under the given key.
// It returns a map[string]string when the value at properties[key] is either
// a map[string]string or a map[string]interface{}. For a map[string]interface{},
// each value is converted to its string representation; for non-string values
// the result contains the formatted string. If the key is not present or the
// value is not a supported map type, an empty map is returned.
func extractTagMap(properties map[string]interface{}, key string) map[string]string {
	result := make(map[string]string)
	v, found := properties[key]
	if !found {
		return result
	}

	switch m := v.(type) {
	case map[string]interface{}:
		for k, val := range m {
			if s, isStr := val.(string); isStr {
				result[k] = s
			} else {
				result[k] = fmt.Sprintf("%v", val)
			}
		}
	case map[string]string:
		for k, val := range m {
			result[k] = val
		}
	}

	return result
}

// toStringMap converts a map[string]interface{} to a map[string]string.
// toStringMap converts a map[string]interface{} to a map[string]string.
// For each entry, string values are kept as-is; non-nil non-string values are converted with fmt.Sprintf("%v").
// Entries with nil values are omitted from the returned map.
func toStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		} else if v != nil {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

// enrichTagsWithSKUAndRegion injects SKU and region entries into tags by resolving them from
// the provided properties using the given provider and resourceType. Existing entries in tags
// are preserved and never overwritten; when found, SKU is added under the key "sku" and
// enrichTagsWithSKUAndRegion injects SKU and region keys into the provided tags map when they
// can be resolved from the given provider, resourceType, and properties. It stringifies
// properties before resolution, preserves existing tag keys (does not overwrite), and only
// adds "sku" or "region" when the resolved values are non-empty.
//
// Parameters:
//   - tags: map to be mutated with optional "sku" and "region" entries.
//   - provider: cloud provider identifier (e.g., "aws", "azure") used for resolution.
//   - resourceType: resource type token used to help determine SKU/region.
//   - properties: resource properties that are converted to strings and used for resolution.
func enrichTagsWithSKUAndRegion(
	tags map[string]string,
	provider, resourceType string,
	properties map[string]interface{},
) {
	stringProps := toStringMap(properties)
	sku, region := resolveSKUAndRegion(provider, resourceType, stringProps)
	if sku != "" {
		if _, exists := tags["sku"]; !exists {
			tags["sku"] = sku
		}
	}
	if region != "" {
		if _, exists := tags["region"]; !exists {
			tags["region"] = region
		}
	}
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
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Type, resource.Properties)

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
		// Resolve cloud-specific identifiers from properties
		cloudID, arn, tags := resolveActualCostIdentifiers(resourceID, in.Properties)

		// Inject SKU and region into tags so plugins can price by instance type.
		// Note: This enrichment is also performed in GetActualCostWithErrors for the
		// pre-validated path. It is intentionally idempotent — direct callers of
		// clientAdapter.GetActualCost may not have pre-enriched tags.
		if in.Provider != "" {
			enrichTagsWithSKUAndRegion(tags, in.Provider, in.ResourceType, in.Properties)
		}

		req := &pbc.GetActualCostRequest{
			ResourceId: cloudID,
			Start:      timestamppb.New(time.Unix(in.StartTime, 0)),
			End:        timestamppb.New(time.Unix(in.EndTime, 0)),
			Tags:       tags,
			Arn:        arn,
		}

		resp, err := c.client.GetActualCost(ctx, req, opts...)
		if err != nil {
			// Continue to next resource on error
			continue
		}

		// Skip when plugin returned no cost data for this resource
		if len(resp.GetResults()) == 0 {
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
		aggregateImpactMetrics(result, resp.GetResults())
		results = append(results, result)
	}

	return &GetActualCostResponse{Results: results}, nil
}

// aggregateImpactMetrics sums impact metric values by kind across all actual cost results
// into the sustainability map on the given ActualCostResult.
func aggregateImpactMetrics(result *ActualCostResult, pbcResults []*pbc.ActualCostResult) {
	for _, pbcResult := range pbcResults {
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
		sku, region := resolveSKUAndRegion(resource.Provider, resource.Type, resource.Properties)
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
			Reasoning:   rec.GetReasoning(),
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
