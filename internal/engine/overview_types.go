package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// maxMessageLen is the maximum length of an error message in OverviewRowError.
const maxMessageLen = 2048

// maxRegionLen is the maximum length of the Region field in StackContext.
const maxRegionLen = 128

// currencyCodeLen is the required length of ISO 4217 currency codes.
const currencyCodeLen = 3

// driftWarningThreshold is the absolute percent-drift above which IsWarning must be true.
const driftWarningThreshold = 10.0

// ErrOverviewValidation is returned when overview type validation fails.
var ErrOverviewValidation = errors.New("overview validation failed")

// ResourceStatus represents the lifecycle state of a resource in the overview.
//
//nolint:recvcheck // UnmarshalJSON requires pointer receiver; String/MarshalJSON use value receivers.
type ResourceStatus int

const (
	// StatusActive indicates the resource exists and has no pending changes.
	StatusActive ResourceStatus = iota
	// StatusCreating indicates the resource is being created.
	StatusCreating
	// StatusUpdating indicates the resource is being updated.
	StatusUpdating
	// StatusDeleting indicates the resource is being deleted.
	StatusDeleting
	// StatusReplacing indicates the resource is being replaced (delete + create).
	StatusReplacing
)

// String returns the human-readable label for a ResourceStatus.
func (s ResourceStatus) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusCreating:
		return "creating"
	case StatusUpdating:
		return "updating"
	case StatusDeleting:
		return "deleting"
	case StatusReplacing:
		return "replacing"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// MarshalJSON implements json.Marshaler to output ResourceStatus as string.
func (s ResourceStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler to parse ResourceStatus from string.
func (s *ResourceStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("parsing resource status: %w", err)
	}
	switch str {
	case "active":
		*s = StatusActive
	case "creating":
		*s = StatusCreating
	case "updating":
		*s = StatusUpdating
	case "deleting":
		*s = StatusDeleting
	case "replacing":
		*s = StatusReplacing
	default:
		return fmt.Errorf("unknown resource status: %q", str)
	}
	return nil
}

// isValidResourceStatus returns true if the status is within the valid range.
func isValidResourceStatus(s ResourceStatus) bool {
	return s >= StatusActive && s <= StatusReplacing
}

// ErrorType categorises the kind of error encountered for a resource.
//
//nolint:recvcheck // UnmarshalJSON requires pointer receiver; String/MarshalJSON use value receivers.
type ErrorType int

const (
	// ErrorTypeAuth indicates an authentication or authorisation failure.
	ErrorTypeAuth ErrorType = iota
	// ErrorTypeNetwork indicates a network connectivity failure.
	ErrorTypeNetwork
	// ErrorTypeRateLimit indicates a rate-limit / throttle response.
	ErrorTypeRateLimit
	// ErrorTypeUnknown indicates an unclassified error.
	ErrorTypeUnknown
)

// String returns the human-readable label for an ErrorType.
func (e ErrorType) String() string {
	switch e {
	case ErrorTypeAuth:
		return "auth"
	case ErrorTypeNetwork:
		return "network"
	case ErrorTypeRateLimit:
		return "rate_limit"
	case ErrorTypeUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("unknown(%d)", int(e))
	}
}

// MarshalJSON implements json.Marshaler to output ErrorType as string.
func (e ErrorType) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

// UnmarshalJSON implements json.Unmarshaler to parse ErrorType from string.
func (e *ErrorType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("parsing error type: %w", err)
	}
	switch str {
	case "auth":
		*e = ErrorTypeAuth
	case "network":
		*e = ErrorTypeNetwork
	case "rate_limit":
		*e = ErrorTypeRateLimit
	case "unknown":
		*e = ErrorTypeUnknown
	default:
		return fmt.Errorf("unknown error type: %q", str)
	}
	return nil
}

// isValidErrorType returns true if the error type is within the valid range.
func isValidErrorType(e ErrorType) bool {
	return e >= ErrorTypeAuth && e <= ErrorTypeUnknown
}

// DateRange represents a half-open time interval [Start, End).
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Validate checks that the DateRange is well-formed.
func (d DateRange) Validate() error {
	if d.Start.IsZero() {
		return fmt.Errorf("%w: start time is required", ErrOverviewValidation)
	}
	if d.End.IsZero() {
		return fmt.Errorf("%w: end time is required", ErrOverviewValidation)
	}
	if !d.End.After(d.Start) {
		if d.End.Equal(d.Start) {
			return fmt.Errorf("%w: zero-length date range (start and end are both %s)",
				ErrOverviewValidation, d.Start.Format(time.RFC3339))
		}
		return fmt.Errorf("%w: end time (%s) is before start time (%s)",
			ErrOverviewValidation, d.End.Format(time.RFC3339), d.Start.Format(time.RFC3339))
	}
	return nil
}

// ActualCostData holds month-to-date actual cost information for a resource.
type ActualCostData struct {
	MTDCost   float64            `json:"mtdCost"`
	Currency  string             `json:"currency"`
	Period    DateRange          `json:"period"`
	Breakdown map[string]float64 `json:"breakdown,omitempty"`
}

// Validate checks that the ActualCostData fields are consistent.
func (a *ActualCostData) Validate() error {
	if a.MTDCost < 0 {
		return fmt.Errorf("%w: MTDCost must be >= 0, got %f", ErrOverviewValidation, a.MTDCost)
	}
	if len(a.Currency) != currencyCodeLen {
		return fmt.Errorf("%w: currency must be exactly 3 characters, got %q", ErrOverviewValidation, a.Currency)
	}
	if err := a.Period.Validate(); err != nil {
		return err
	}
	for k, v := range a.Breakdown {
		if v < 0 {
			return fmt.Errorf("%w: breakdown value for %q must be >= 0, got %f", ErrOverviewValidation, k, v)
		}
	}
	return nil
}

// ProjectedCostData holds projected monthly cost information for a resource.
type ProjectedCostData struct {
	MonthlyCost float64            `json:"monthlyCost"`
	Currency    string             `json:"currency"`
	Breakdown   map[string]float64 `json:"breakdown,omitempty"`
}

// Validate checks that the ProjectedCostData fields are consistent.
func (p *ProjectedCostData) Validate() error {
	if p.MonthlyCost < 0 {
		return fmt.Errorf("%w: MonthlyCost must be >= 0, got %f", ErrOverviewValidation, p.MonthlyCost)
	}
	if len(p.Currency) != currencyCodeLen {
		return fmt.Errorf("%w: currency must be exactly 3 characters, got %q", ErrOverviewValidation, p.Currency)
	}
	for k, v := range p.Breakdown {
		if v < 0 {
			return fmt.Errorf("%w: breakdown value for %q must be >= 0, got %f", ErrOverviewValidation, k, v)
		}
	}
	return nil
}

// CostDriftData captures the difference between extrapolated actual spend and
// the projected monthly cost. It is only populated when the absolute drift
// exceeds the warning threshold (10%).
type CostDriftData struct {
	ExtrapolatedMonthly float64 `json:"extrapolatedMonthly"`
	Projected           float64 `json:"projected"`
	Delta               float64 `json:"delta"`
	PercentDrift        float64 `json:"percentDrift"`
	IsWarning           bool    `json:"isWarning"`
}

// Validate checks that the CostDriftData fields are consistent.
func (c *CostDriftData) Validate() error {
	if c.ExtrapolatedMonthly < 0 {
		return fmt.Errorf("%w: ExtrapolatedMonthly must be >= 0, got %f", ErrOverviewValidation, c.ExtrapolatedMonthly)
	}
	if c.Projected < 0 {
		return fmt.Errorf("%w: Projected must be >= 0, got %f", ErrOverviewValidation, c.Projected)
	}
	if math.Abs(c.PercentDrift) > driftWarningThreshold && !c.IsWarning {
		return fmt.Errorf("%w: IsWarning must be true when abs(PercentDrift) > %.1f (got %.2f%%)",
			ErrOverviewValidation, driftWarningThreshold, c.PercentDrift)
	}
	if c.IsWarning && math.Abs(c.PercentDrift) <= driftWarningThreshold {
		return fmt.Errorf(
			"%w: IsWarning must be false when abs(PercentDrift) <= %.1f (PercentDrift=%.2f%%, IsWarning=true)",
			ErrOverviewValidation, driftWarningThreshold, c.PercentDrift)
	}
	return nil
}

// OverviewRowError captures an error that occurred while fetching cost data
// for a specific resource.
type OverviewRowError struct {
	URN       string    `json:"urn"`
	ErrorType ErrorType `json:"errorType"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"`
}

// Validate checks that the OverviewRowError fields are well-formed.
func (e *OverviewRowError) Validate() error {
	if e.URN == "" {
		return fmt.Errorf("%w: URN is required for OverviewRowError", ErrOverviewValidation)
	}
	if e.Message == "" {
		return fmt.Errorf("%w: Message is required for OverviewRowError", ErrOverviewValidation)
	}
	if len(e.Message) > maxMessageLen {
		return fmt.Errorf(
			"%w: Message too long: %d chars (max %d)",
			ErrOverviewValidation,
			len(e.Message),
			maxMessageLen,
		)
	}
	if !isValidErrorType(e.ErrorType) {
		return fmt.Errorf("%w: invalid ErrorType: %d", ErrOverviewValidation, e.ErrorType)
	}
	return nil
}

// OverviewRow represents a single resource row in the unified cost overview.
// Each row combines state, plan, actual costs, projected costs, drift, and
// recommendations for a resource.
type OverviewRow struct {
	URN             string             `json:"urn"`
	Type            string             `json:"type"`
	ResourceID      string             `json:"resourceId,omitempty"`
	Status          ResourceStatus     `json:"status"`
	ActualCost      *ActualCostData    `json:"actualCost,omitempty"`
	ProjectedCost   *ProjectedCostData `json:"projectedCost,omitempty"`
	Recommendations []Recommendation   `json:"recommendations,omitempty"`
	CostDrift       *CostDriftData     `json:"costDrift,omitempty"`
	Error           *OverviewRowError  `json:"error,omitempty"`
}

// Validate checks that the OverviewRow fields are well-formed. It validates
// required fields, length constraints, status range, and recursively validates
// nested types when present.
func (r *OverviewRow) Validate() error {
	if r.URN == "" {
		return fmt.Errorf("%w: URN is required", ErrOverviewValidation)
	}
	if r.Type == "" {
		return fmt.Errorf("%w: Type is required", ErrOverviewValidation)
	}
	if len(r.Type) > maxResourceTypeLen {
		return fmt.Errorf(
			"%w: Type too long: %d chars (max %d)",
			ErrOverviewValidation,
			len(r.Type),
			maxResourceTypeLen,
		)
	}
	if len(r.ResourceID) > maxResourceIDLen {
		return fmt.Errorf(
			"%w: ResourceID too long: %d chars (max %d)",
			ErrOverviewValidation,
			len(r.ResourceID),
			maxResourceIDLen,
		)
	}
	if !isValidResourceStatus(r.Status) {
		return fmt.Errorf("%w: invalid Status: %d", ErrOverviewValidation, r.Status)
	}
	if r.ActualCost != nil {
		if err := r.ActualCost.Validate(); err != nil {
			return err
		}
	}
	if r.ProjectedCost != nil {
		if err := r.ProjectedCost.Validate(); err != nil {
			return err
		}
	}
	if r.CostDrift != nil {
		if err := r.CostDrift.Validate(); err != nil {
			return err
		}
	}
	if r.Error != nil {
		if err := r.Error.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// StackContext provides metadata about the Pulumi stack being analysed in
// the overview, including change detection information.
type StackContext struct {
	StackName      string    `json:"stackName"`
	Region         string    `json:"region,omitempty"`
	TimeWindow     DateRange `json:"timeWindow"`
	HasChanges     bool      `json:"hasChanges"`
	TotalResources int       `json:"totalResources"`
	PendingChanges int       `json:"pendingChanges"`
	GeneratedAt    time.Time `json:"generatedAt,omitempty"`
}

// Validate checks that the StackContext fields are well-formed.
func (s *StackContext) Validate() error {
	if s.StackName == "" {
		return fmt.Errorf("%w: StackName is required", ErrOverviewValidation)
	}
	if len(s.Region) > maxRegionLen {
		return fmt.Errorf("%w: Region too long: %d chars (max %d)", ErrOverviewValidation, len(s.Region), maxRegionLen)
	}
	if err := s.TimeWindow.Validate(); err != nil {
		return err
	}
	if s.TotalResources < 0 {
		return fmt.Errorf("%w: TotalResources must be >= 0, got %d", ErrOverviewValidation, s.TotalResources)
	}
	if s.HasChanges && s.PendingChanges <= 0 {
		return fmt.Errorf("%w: PendingChanges must be > 0 when HasChanges is true, got %d",
			ErrOverviewValidation, s.PendingChanges)
	}
	if !s.HasChanges && s.PendingChanges > 0 {
		return fmt.Errorf("%w: HasChanges must be true when PendingChanges > 0, got %d",
			ErrOverviewValidation, s.PendingChanges)
	}
	return nil
}

// OverviewRowUpdate carries a row update on a progress channel, pairing the
// row index with the updated row data.
type OverviewRowUpdate struct {
	Index int
	Row   OverviewRow
}

// StateResource represents a resource from Pulumi state for overview merging.
// This is a lightweight projection of ingest.StackExportResource to avoid an
// import cycle (ingest already imports engine).
type StateResource struct {
	URN    string `json:"urn,omitempty"`
	Type   string `json:"type,omitempty"`
	ID     string `json:"id,omitempty"`
	Custom bool   `json:"custom,omitempty"`
}

// PlanStep represents a step from a Pulumi plan for overview merging.
// This is a lightweight projection of ingest.PulumiStep to avoid an
// import cycle (ingest already imports engine).
type PlanStep struct {
	URN  string `json:"urn,omitempty"`
	Op   string `json:"op,omitempty"`
	Type string `json:"type,omitempty"`
}
