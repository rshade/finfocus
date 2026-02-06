package engine

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/rshade/finfocus/internal/spec"
)

// Validation constants for ResourceDescriptor.
const (
	maxProperties      = 100       // Maximum number of properties allowed
	maxPropertyKeyLen  = 128       // Maximum length of property keys
	maxPropertyValLen  = 10 * 1024 // Maximum length of property values (10KB)
	maxResourceTypeLen = 256       // Maximum length of resource type
	maxResourceIDLen   = 1024      // Maximum length of resource ID
)

// ErrResourceValidation is returned when resource validation fails.
var ErrResourceValidation = errors.New("resource validation failed")

const (
	// maxErrorsToDisplay is the maximum number of errors to show in summary before truncating.
	maxErrorsToDisplay = 5
)

// ResourceDescriptor represents a cloud resource with its type, provider, and properties.
type ResourceDescriptor struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Provider   string                 `json:"provider"`
	Properties map[string]interface{} `json:"properties"`
}

// Validate checks that the ResourceDescriptor has valid fields and returns an error if validation fails.
// It validates:
//   - Type is not empty and within length limits
//   - ID is within length limits (can be empty for some resources)
//   - Properties count does not exceed maximum
//   - Property keys are valid identifiers and within length limits
//   - Property values do not exceed size limits
func (r *ResourceDescriptor) Validate() error {
	// Validate Type (required)
	if r.Type == "" {
		return fmt.Errorf("%w: resource type is required", ErrResourceValidation)
	}
	if len(r.Type) > maxResourceTypeLen {
		return fmt.Errorf("%w: resource type too long: %d bytes (max %d)",
			ErrResourceValidation, len(r.Type), maxResourceTypeLen)
	}

	// Validate ID (can be empty but must not exceed limit)
	if len(r.ID) > maxResourceIDLen {
		return fmt.Errorf("%w: resource ID too long: %d bytes (max %d)",
			ErrResourceValidation, len(r.ID), maxResourceIDLen)
	}

	// Validate Properties count
	if len(r.Properties) > maxProperties {
		return fmt.Errorf("%w: too many properties: %d (max %d)",
			ErrResourceValidation, len(r.Properties), maxProperties)
	}

	// Validate each property
	for key, val := range r.Properties {
		if err := validatePropertyKey(key); err != nil {
			return fmt.Errorf("%w: %w", ErrResourceValidation, err)
		}

		valStr := fmt.Sprintf("%v", val)
		if len(valStr) > maxPropertyValLen {
			return fmt.Errorf("%w: property value too large for key %q: %d bytes (max %d)",
				ErrResourceValidation, key, len(valStr), maxPropertyValLen)
		}
	}

	return nil
}

// validatePropertyKey checks if a property key is a valid identifier.
// validatePropertyKey validates a resource property key.
// It ensures the key is not empty, does not exceed the maximum allowed length,
// and contains only letters, digits, underscores (_), hyphens (-), or dots (.).
// Returns an error describing the violation when the key is invalid, or nil when valid.
func validatePropertyKey(key string) error {
	if key == "" {
		return errors.New("property key cannot be empty")
	}
	if len(key) > maxPropertyKeyLen {
		return fmt.Errorf("property key too long: %d bytes (max %d)", len(key), maxPropertyKeyLen)
	}

	for _, ch := range key {
		if !isValidPropertyKeyChar(ch) {
			return fmt.Errorf(
				"invalid character in property key %q: %c (must be alphanumeric, _, -, or .)",
				key,
				ch,
			)
		}
	}
	return nil
}

// isValidPropertyKeyChar reports whether ch is a valid character for a property key.
// Valid characters are letters, digits, underscore ('_'), hyphen ('-'), or dot ('.').
func isValidPropertyKeyChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' || ch == '.'
}

// SustainabilityMetric represents a single sustainability impact measurement.
type SustainabilityMetric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// Recommendation represents a single cost optimization suggestion.
//
// Recommendations are provided by plugins to suggest ways to reduce costs,
// such as right-sizing instances, terminating idle resources, or purchasing
// reserved capacity.
//
// Usage Examples:
//
//	rec := Recommendation{
//		Type:            "Right-sizing",
//		Description:     "Switch to t3.small to reduce costs",
//		EstimatedSavings: 15.00,
//		Currency:        "USD",
//	}
type Recommendation struct {
	// ResourceID identifies the resource this recommendation applies to.
	ResourceID string `json:"resourceId,omitempty"`

	// Type categorizes the recommendation (e.g., "Right-sizing", "Terminate",
	// "Purchase Commitment", "Delete Unused", "Adjust Requests")
	Type string `json:"type"`

	// Description provides actionable text explaining the recommendation
	Description string `json:"description"`

	// EstimatedSavings is the projected monthly savings if the recommendation
	// is implemented. Zero indicates savings cannot be estimated.
	EstimatedSavings float64 `json:"estimatedSavings,omitempty"`

	// Currency is the ISO 4217 code for EstimatedSavings (e.g., "USD").
	// Empty if EstimatedSavings is zero.
	Currency string `json:"currency,omitempty"`

	// Status indicates the lifecycle state of this recommendation.
	// Empty or "Active" for active recommendations, "Dismissed" or "Snoozed"
	// for dismissed/snoozed recommendations shown via --include-dismissed.
	Status RecommendationStatus `json:"status,omitempty"`
}

// RecommendationStatus represents the lifecycle state of a recommendation.
type RecommendationStatus string

const (
	// RecommendationStatusActive indicates an active (non-dismissed) recommendation.
	RecommendationStatusActive RecommendationStatus = "Active"
	// RecommendationStatusDismissed indicates a permanently dismissed recommendation.
	RecommendationStatusDismissed RecommendationStatus = "Dismissed"
	// RecommendationStatusSnoozed indicates a temporarily snoozed recommendation.
	RecommendationStatusSnoozed RecommendationStatus = "Snoozed"
)

// IsValid returns true if the status is a known value.
func (rs RecommendationStatus) IsValid() bool {
	switch rs {
	case RecommendationStatusActive, RecommendationStatusDismissed, RecommendationStatusSnoozed, "":
		return true
	}
	return false
}

// String returns the string representation of the status.
func (rs RecommendationStatus) String() string {
	return string(rs)
}

// CostResult contains the calculated cost information for a single resource.
type CostResult struct {
	ResourceType   string                          `json:"resourceType"`
	ResourceID     string                          `json:"resourceId"`
	Adapter        string                          `json:"adapter"`
	Currency       string                          `json:"currency"`
	Monthly        float64                         `json:"monthly"`
	Hourly         float64                         `json:"hourly"`
	Notes          string                          `json:"notes"`
	Breakdown      map[string]float64              `json:"breakdown"`
	Sustainability map[string]SustainabilityMetric `json:"sustainability,omitempty"`
	// Recommendations contains cost optimization suggestions from plugins.
	// This field is populated when plugins provide actionable recommendations
	// alongside cost estimates (e.g., right-sizing, termination suggestions).
	Recommendations []Recommendation `json:"recommendations,omitempty"`
	// Actual cost specific fields
	TotalCost  float64   `json:"totalCost,omitempty"`
	DailyCosts []float64 `json:"dailyCosts,omitempty"`
	CostPeriod string    `json:"costPeriod,omitempty"`
	StartDate  time.Time `json:"startDate,omitempty"`
	EndDate    time.Time `json:"endDate,omitempty"`

	// Delta represents the cost change (trend) compared to a baseline, in the same currency as the cost.
	// Positive values indicate cost increase, negative values indicate decrease.
	// The baseline depends on context (e.g., previous period for actual costs, budget for projected costs).
	Delta float64 `json:"delta,omitempty"`

	// Confidence indicates the reliability level of this cost estimate.
	// HIGH: Real billing data from cloud APIs
	// MEDIUM: Runtime-based estimate from Pulumi timestamps
	// LOW: Imported resource (timestamp may be inaccurate)
	Confidence Confidence `json:"confidence,omitempty"`
}

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
	Results []CostResult
	Errors  []ErrorDetail
}

// HasErrors returns true if any errors were encountered during cost calculation.
func (c *CostResultWithErrors) HasErrors() bool {
	return len(c.Errors) > 0
}

// ErrorSummary returns a human-readable summary of errors.
// Truncates the output after maxErrorsToDisplay errors to keep it readable.
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

// ActualCostRequest contains parameters for querying historical actual costs with filtering and grouping.
type ActualCostRequest struct {
	Resources          []ResourceDescriptor
	From               time.Time
	To                 time.Time
	Adapter            string
	GroupBy            string
	Tags               map[string]string
	EstimateConfidence bool // Show confidence level in output
}

// CrossProviderAggregation represents daily/monthly cost aggregation across providers.
//
// This type enables comprehensive cost analysis across multiple cloud providers
// (AWS, Azure, GCP, etc.) with time-based aggregation for trend analysis and
// cost optimization insights.
//
// Usage Examples:
//
//	// Daily aggregation showing multi-provider costs
//	agg := CrossProviderAggregation{
//		Period: "2024-01-15",
//		Providers: map[string]float64{
//			"aws":   245.67,
//			"azure": 156.23,
//			"gcp":   89.45,
//		},
//		Total:    491.35,
//		Currency: "USD",
//	}
//
//	// Monthly aggregation for cost trending
//	agg := CrossProviderAggregation{
//		Period: "2024-01",
//		Providers: map[string]float64{
//			"aws":   7620.15,
//			"azure": 4843.67,
//		},
//		Total:    12463.82,
//		Currency: "USD",
//	}
type CrossProviderAggregation struct {
	Period    string             `json:"period"`    // Date (2024-01-01) or Month (2024-01)
	Providers map[string]float64 `json:"providers"` // Provider name -> cost
	Total     float64            `json:"total"`     // Total cost for this period
	Currency  string             `json:"currency"`  // Currency for all costs
}

// GroupBy defines the available grouping strategies for cost result aggregation.
//
// This type enables flexible cost analysis by grouping results along different
// dimensions such as resources, providers, or time periods. Each grouping type
// provides unique insights for cost optimization and analysis.
type GroupBy string

// GroupBy constants define all supported aggregation strategies.
//
// Resource Groupings:
//   - GroupByResource: Groups by individual resource (ResourceType/ResourceID)
//   - GroupByType: Groups by resource type (e.g., "aws:ec2:Instance")
//   - GroupByProvider: Groups by cloud provider (e.g., "aws", "azure", "gcp")
//
// Time-Based Groupings:
//   - GroupByDaily: Groups by calendar date ("2006-01-02") for daily trends
//   - GroupByMonthly: Groups by month ("2006-01") for monthly analysis
//   - GroupByDate: Deprecated legacy date-key grouping (non time-based for cross-provider).
//     Prefer GroupByDaily for time-based aggregations.
//
// Special Values:
//   - GroupByNone: No grouping (empty string) - returns results as-is
//
// Usage Guidelines:
//   - Use resource groupings for cost attribution and resource optimization
//   - Use time-based groupings for trend analysis and forecasting
//   - Time-based groupings require results with valid StartDate/EndDate fields
//   - Cross-provider aggregation only supports time-based groupings
const (
	GroupByResource GroupBy = "resource"
	GroupByType     GroupBy = "type"
	GroupByProvider GroupBy = "provider"
	GroupByDate     GroupBy = "date" // Deprecated: use GroupByDaily
	GroupByDaily    GroupBy = "daily"
	GroupByMonthly  GroupBy = "monthly"
	GroupByNone     GroupBy = ""
)

// IsValid returns true if the GroupBy value is valid.
//
// This method provides compile-time safety for GroupBy values and enables
// validation before processing. All defined GroupBy constants are considered
// valid, including the empty string (GroupByNone).
//
// Returns:
//   - true: For all defined GroupBy constants
//   - false: For any other string values
//
// Usage Examples:
//
//	// Validate user input
//	groupBy := GroupBy(userInput)
//	if !groupBy.IsValid() {
//		return fmt.Errorf("invalid groupBy: %s", userInput)
//	}
//
//	// Safe to use in switch statements
//	switch groupBy {
//	case GroupByDaily, GroupByMonthly:
//		// Time-based processing
//	case GroupByResource, GroupByType, GroupByProvider:
//		// Resource-based processing
//	}
func (g GroupBy) IsValid() bool {
	switch g {
	case GroupByResource,
		GroupByType,
		GroupByProvider,
		GroupByDate,
		GroupByDaily,
		GroupByMonthly,
		GroupByNone:
		return true
	default:
		return false
	}
}

// IsTimeBasedGrouping returns true if the GroupBy requires time-based aggregation.
//
// Time-based groupings require cost results with valid StartDate and EndDate fields
// and are the only grouping types supported by cross-provider aggregation functions.
// This method is used internally to validate aggregation requests and determine
// processing strategies.
//
// Time-Based GroupBy Values:
//   - GroupByDaily: Requires daily cost data aggregation
//   - GroupByMonthly: Requires monthly cost data aggregation
//   - GroupByDate: Deprecated legacy date-key grouping (non time-based for cross-provider)
//
// Non-Time-Based GroupBy Values:
//   - GroupByResource: Groups by resource identity
//   - GroupByType: Groups by resource type
//   - GroupByProvider: Groups by cloud provider
//   - GroupByNone: No grouping applied
//
// Returns:
//   - true: For GroupByDaily and GroupByMonthly only
//   - false: For all other GroupBy values
//
// Usage Examples:
//
//	// Validate for cross-provider aggregation
//	if !groupBy.IsTimeBasedGrouping() {
//		return ErrInvalidGroupBy
//	}
//
//	// Route to appropriate processing
//	if groupBy.IsTimeBasedGrouping() {
//		return CreateCrossProviderAggregation(results, groupBy)
//	} else {
//		return engine.GroupResults(results, groupBy)
//	}
func (g GroupBy) IsTimeBasedGrouping() bool {
	return g == GroupByDaily || g == GroupByMonthly
}

// String returns the string representation of the GroupBy.
//
// This method implements the Stringer interface and provides a consistent
// string representation for logging, debugging, and serialization.
//
// Returns:
//   - string: The underlying string value of the GroupBy
//   - "": Empty string for GroupByNone
//
// Usage Examples:
//
//	// Logging and debugging
//	log.Printf("Processing with groupBy: %s", groupBy.String())
//
//	// CLI flag validation
//	if groupBy.String() == "" {
//		groupBy = GroupByResource // Default
//	}
//
//	// JSON serialization (automatic via json package)
//	type Request struct {
//		GroupBy GroupBy `json:"groupBy"`
//	}
func (g GroupBy) String() string {
	return string(g)
}

// ProjectedCostRequest contains resources for which projected costs should be calculated.
type ProjectedCostRequest struct {
	Resources []ResourceDescriptor
	SpecDir   string
	Adapter   string
}

// PricingSpec is an alias to the PricingSpec from the spec package to ensure type consistency.
type PricingSpec = spec.PricingSpec

// CostSummary provides aggregated cost totals grouped by provider, service, and adapter.
type CostSummary struct {
	TotalMonthly float64            `json:"totalMonthly"`
	TotalHourly  float64            `json:"totalHourly"`
	Currency     string             `json:"currency"`
	ByProvider   map[string]float64 `json:"byProvider"`
	ByService    map[string]float64 `json:"byService"`
	ByAdapter    map[string]float64 `json:"byAdapter"`
	Resources    []CostResult       `json:"resources"`
}

// AggregatedResults contains cost results with summary and aggregation data.
type AggregatedResults struct {
	Summary   CostSummary  `json:"summary"`
	Resources []CostResult `json:"resources"`
}

// RecommendationError captures error information when fetching recommendations from a plugin.
type RecommendationError struct {
	PluginName string `json:"pluginName"`
	Error      string `json:"error"`
}

// RecommendationsResult contains the results of fetching recommendations from multiple plugins.
type RecommendationsResult struct {
	Recommendations []Recommendation      `json:"recommendations"`
	Errors          []RecommendationError `json:"errors"`
	TotalSavings    float64               `json:"totalSavings"`
	Currency        string                `json:"currency"`
}

// HasErrors returns true if any errors were encountered.
func (r *RecommendationsResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// ErrorSummary returns a string summary of errors.
func (r *RecommendationsResult) ErrorSummary() string {
	if !r.HasErrors() {
		return ""
	}
	var summaries []string
	for _, e := range r.Errors {
		summaries = append(summaries, fmt.Sprintf("%s: %s", e.PluginName, e.Error))
	}
	return strings.Join(summaries, "; ")
}

// EstimateResult represents the result of a what-if cost estimation.
// It contains baseline and modified costs along with per-property deltas.
//
// Usage Examples:
//
//	result := EstimateResult{
//		Resource: &ResourceDescriptor{Provider: "aws", Type: "ec2:Instance"},
//		Baseline: &CostResult{Monthly: 8.32, Currency: "USD"},
//		Modified: &CostResult{Monthly: 83.22, Currency: "USD"},
//		TotalChange: 74.90,
//		Deltas: []CostDelta{
//			{Property: "instanceType", OriginalValue: "t3.micro", NewValue: "m5.large", CostChange: 74.90},
//		},
//	}
type EstimateResult struct {
	// Resource is the resource being estimated
	Resource *ResourceDescriptor `json:"resource"`

	// Baseline is the cost with original properties
	Baseline *CostResult `json:"baseline"`

	// Modified is the cost with property overrides applied
	Modified *CostResult `json:"modified"`

	// TotalChange is the difference between modified and baseline monthly costs
	// Positive = increase, negative = savings
	TotalChange float64 `json:"totalChange"`

	// Deltas contains per-property cost impact breakdown
	Deltas []CostDelta `json:"deltas"`

	// UsedFallback indicates if EstimateCost RPC was unavailable
	// and the result was computed from two GetProjectedCost calls
	UsedFallback bool `json:"usedFallback,omitempty"`
}

// CostDelta represents the cost impact of changing a single property.
//
// Usage Examples:
//
//	delta := CostDelta{
//		Property:      "instanceType",
//		OriginalValue: "t3.micro",
//		NewValue:      "m5.large",
//		CostChange:    65.70, // Positive means cost increase
//	}
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

// EstimateRequest encapsulates parameters for EstimateCost.
//
// This is the internal request structure used by the engine layer
// for what-if cost analysis operations.
type EstimateRequest struct {
	// Resource is the base resource descriptor
	Resource *ResourceDescriptor `json:"resource,omitempty"`

	// PropertyOverrides are the changes to evaluate
	PropertyOverrides map[string]string `json:"propertyOverrides,omitempty"`

	// UsageProfile optionally provides context (dev, prod, etc.)
	UsageProfile string `json:"usageProfile,omitempty"`
}

// DismissRequest contains parameters for dismissing a recommendation.
type DismissRequest struct {
	// RecommendationID is the unique identifier of the recommendation to dismiss.
	RecommendationID string `json:"recommendationId"`

	// Reason is the CLI flag value (e.g., "business-constraint").
	Reason string `json:"reason"`

	// CustomReason is the free-text explanation from the --note flag.
	CustomReason string `json:"customReason,omitempty"`

	// ExpiresAt is the snooze expiry date; nil means permanent dismissal.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`

	// Recommendation is the current recommendation details for the LastKnown snapshot.
	Recommendation *Recommendation `json:"recommendation,omitempty"`
}

// DismissResult contains the outcome of a dismiss operation.
type DismissResult struct {
	// RecommendationID is the ID of the dismissed recommendation.
	RecommendationID string `json:"recommendationId"`

	// PluginDismissed is true if a plugin accepted the dismissal via RPC.
	PluginDismissed bool `json:"pluginDismissed"`

	// PluginName identifies which plugin handled the dismissal.
	PluginName string `json:"pluginName,omitempty"`

	// PluginMessage is the plugin's response message.
	PluginMessage string `json:"pluginMessage,omitempty"`

	// PluginFailed is true if a plugin dismiss RPC was attempted but failed.
	// Local persistence still proceeds, but the upstream state may be inconsistent.
	PluginFailed bool `json:"pluginFailed"`

	// LocalPersisted is true if the dismissal was saved locally.
	LocalPersisted bool `json:"localPersisted"`

	// Warning contains a non-fatal warning (e.g., plugin failed but local succeeded).
	Warning string `json:"warning,omitempty"`
}

// UndismissResult contains the outcome of an undismiss operation.
type UndismissResult struct {
	// RecommendationID is the ID of the undismissed recommendation.
	RecommendationID string `json:"recommendationId"`

	// WasDismissed is false if the recommendation wasn't dismissed.
	WasDismissed bool `json:"wasDismissed"`

	// Message provides information about the undismiss operation.
	Message string `json:"message,omitempty"`
}
