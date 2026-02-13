package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ResourceStatus
// ---------------------------------------------------------------------------

func TestResourceStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status ResourceStatus
		want   string
	}{
		{"active", StatusActive, "active"},
		{"creating", StatusCreating, "creating"},
		{"updating", StatusUpdating, "updating"},
		{"deleting", StatusDeleting, "deleting"},
		{"replacing", StatusReplacing, "replacing"},
		{"unknown value", ResourceStatus(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.status.String())
		})
	}
}

func TestIsValidResourceStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ResourceStatus
		valid  bool
	}{
		{"active", StatusActive, true},
		{"replacing", StatusReplacing, true},
		{"negative", ResourceStatus(-1), false},
		{"out of range", ResourceStatus(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidResourceStatus(tt.status))
		})
	}
}

// ---------------------------------------------------------------------------
// ErrorType
// ---------------------------------------------------------------------------

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		name  string
		eType ErrorType
		want  string
	}{
		{"auth", ErrorTypeAuth, "auth"},
		{"network", ErrorTypeNetwork, "network"},
		{"rate_limit", ErrorTypeRateLimit, "rate_limit"},
		{"unknown", ErrorTypeUnknown, "unknown"},
		{"out of range", ErrorType(99), "unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.eType.String())
		})
	}
}

func TestIsValidErrorType(t *testing.T) {
	tests := []struct {
		name  string
		eType ErrorType
		valid bool
	}{
		{"auth", ErrorTypeAuth, true},
		{"unknown", ErrorTypeUnknown, true},
		{"negative", ErrorType(-1), false},
		{"out of range", ErrorType(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, isValidErrorType(tt.eType))
		})
	}
}

// ---------------------------------------------------------------------------
// DateRange
// ---------------------------------------------------------------------------

func TestDateRange_Validate(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)

	tests := []struct {
		name        string
		dr          DateRange
		wantErr     bool
		errContains string
	}{
		{
			name: "valid range",
			dr:   DateRange{Start: now, End: later},
		},
		{
			name:        "same start and end",
			dr:          DateRange{Start: now, End: now},
			wantErr:     true,
			errContains: "zero-length date range",
		},
		{
			name:        "zero start",
			dr:          DateRange{End: later},
			wantErr:     true,
			errContains: "start time is required",
		},
		{
			name:        "zero end",
			dr:          DateRange{Start: now},
			wantErr:     true,
			errContains: "end time is required",
		},
		{
			name:        "end before start",
			dr:          DateRange{Start: later, End: now},
			wantErr:     true,
			errContains: "end time",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.dr.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ActualCostData
// ---------------------------------------------------------------------------

func TestActualCostData_Validate(t *testing.T) {
	validPeriod := DateRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name        string
		data        ActualCostData
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			data: ActualCostData{MTDCost: 42.50, Currency: "USD", Period: validPeriod},
		},
		{
			name: "valid with breakdown",
			data: ActualCostData{
				MTDCost:   100.0,
				Currency:  "EUR",
				Period:    validPeriod,
				Breakdown: map[string]float64{"compute": 80, "storage": 20},
			},
		},
		{
			name:        "negative MTDCost",
			data:        ActualCostData{MTDCost: -1, Currency: "USD", Period: validPeriod},
			wantErr:     true,
			errContains: "MTDCost must be >= 0",
		},
		{
			name:        "currency too short",
			data:        ActualCostData{MTDCost: 0, Currency: "US", Period: validPeriod},
			wantErr:     true,
			errContains: "currency must be exactly 3 characters",
		},
		{
			name:        "currency too long",
			data:        ActualCostData{MTDCost: 0, Currency: "USDD", Period: validPeriod},
			wantErr:     true,
			errContains: "currency must be exactly 3 characters",
		},
		{
			name:        "invalid period",
			data:        ActualCostData{MTDCost: 0, Currency: "USD", Period: DateRange{}},
			wantErr:     true,
			errContains: "start time is required",
		},
		{
			name: "negative breakdown value",
			data: ActualCostData{
				MTDCost:   10,
				Currency:  "USD",
				Period:    validPeriod,
				Breakdown: map[string]float64{"compute": -5},
			},
			wantErr:     true,
			errContains: "breakdown value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ProjectedCostData
// ---------------------------------------------------------------------------

func TestProjectedCostData_Validate(t *testing.T) {
	tests := []struct {
		name        string
		data        ProjectedCostData
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			data: ProjectedCostData{MonthlyCost: 100.0, Currency: "USD"},
		},
		{
			name: "valid with breakdown",
			data: ProjectedCostData{
				MonthlyCost: 200.0,
				Currency:    "GBP",
				Breakdown:   map[string]float64{"compute": 150, "network": 50},
			},
		},
		{
			name: "zero cost is valid",
			data: ProjectedCostData{MonthlyCost: 0, Currency: "USD"},
		},
		{
			name:        "negative MonthlyCost",
			data:        ProjectedCostData{MonthlyCost: -1, Currency: "USD"},
			wantErr:     true,
			errContains: "MonthlyCost must be >= 0",
		},
		{
			name:        "empty currency",
			data:        ProjectedCostData{MonthlyCost: 10, Currency: ""},
			wantErr:     true,
			errContains: "currency must be exactly 3 characters",
		},
		{
			name: "negative breakdown value",
			data: ProjectedCostData{
				MonthlyCost: 10,
				Currency:    "USD",
				Breakdown:   map[string]float64{"storage": -1},
			},
			wantErr:     true,
			errContains: "breakdown value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CostDriftData
// ---------------------------------------------------------------------------

func TestCostDriftData_Validate(t *testing.T) {
	tests := []struct {
		name        string
		data        CostDriftData
		wantErr     bool
		errContains string
	}{
		{
			name: "valid with warning",
			data: CostDriftData{
				ExtrapolatedMonthly: 120,
				Projected:           100,
				Delta:               20,
				PercentDrift:        20.0,
				IsWarning:           true,
			},
		},
		{
			name: "drift below threshold without warning",
			data: CostDriftData{
				ExtrapolatedMonthly: 105,
				Projected:           100,
				Delta:               5,
				PercentDrift:        5.0,
				IsWarning:           false,
			},
		},
		{
			name: "exactly at threshold without warning is ok",
			data: CostDriftData{
				ExtrapolatedMonthly: 110,
				Projected:           100,
				Delta:               10,
				PercentDrift:        10.0,
				IsWarning:           false,
			},
		},
		{
			name:        "negative ExtrapolatedMonthly",
			data:        CostDriftData{ExtrapolatedMonthly: -1},
			wantErr:     true,
			errContains: "ExtrapolatedMonthly must be >= 0",
		},
		{
			name:        "negative Projected",
			data:        CostDriftData{ExtrapolatedMonthly: 10, Projected: -1},
			wantErr:     true,
			errContains: "Projected must be >= 0",
		},
		{
			name: "high drift without IsWarning",
			data: CostDriftData{
				ExtrapolatedMonthly: 150,
				Projected:           100,
				Delta:               50,
				PercentDrift:        50.0,
				IsWarning:           false,
			},
			wantErr:     true,
			errContains: "IsWarning must be true",
		},
		{
			name: "negative high drift without IsWarning",
			data: CostDriftData{
				ExtrapolatedMonthly: 50,
				Projected:           100,
				Delta:               -50,
				PercentDrift:        -50.0,
				IsWarning:           false,
			},
			wantErr:     true,
			errContains: "IsWarning must be true",
		},
		{
			name: "IsWarning true but drift below threshold",
			data: CostDriftData{
				ExtrapolatedMonthly: 105,
				Projected:           100,
				Delta:               5,
				PercentDrift:        5.0,
				IsWarning:           true,
			},
			wantErr:     true,
			errContains: "IsWarning must be false",
		},
		{
			name: "IsWarning true at exactly threshold",
			data: CostDriftData{
				ExtrapolatedMonthly: 110,
				Projected:           100,
				Delta:               10,
				PercentDrift:        10.0,
				IsWarning:           true,
			},
			wantErr:     true,
			errContains: "IsWarning must be false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// OverviewRowError
// ---------------------------------------------------------------------------

func TestOverviewRowError_Validate(t *testing.T) {
	tests := []struct {
		name        string
		err         OverviewRowError
		wantErr     bool
		errContains string
	}{
		{
			name: "valid",
			err: OverviewRowError{
				URN:       "urn:pulumi:stack::project::type::name",
				ErrorType: ErrorTypeNetwork,
				Message:   "connection refused",
			},
		},
		{
			name: "valid retryable",
			err:  OverviewRowError{URN: "urn:x", ErrorType: ErrorTypeRateLimit, Message: "throttled", Retryable: true},
		},
		{
			name:        "missing URN",
			err:         OverviewRowError{ErrorType: ErrorTypeAuth, Message: "unauthorized"},
			wantErr:     true,
			errContains: "URN is required",
		},
		{
			name:        "missing message",
			err:         OverviewRowError{URN: "urn:x", ErrorType: ErrorTypeAuth},
			wantErr:     true,
			errContains: "Message is required",
		},
		{
			name: "message too long",
			err: OverviewRowError{
				URN:       "urn:x",
				ErrorType: ErrorTypeAuth,
				Message:   strings.Repeat("a", maxMessageLen+1),
			},
			wantErr:     true,
			errContains: "Message too long",
		},
		{
			name:        "invalid error type",
			err:         OverviewRowError{URN: "urn:x", ErrorType: ErrorType(99), Message: "bad"},
			wantErr:     true,
			errContains: "invalid ErrorType",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.err.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// OverviewRow
// ---------------------------------------------------------------------------

func TestOverviewRow_Validate(t *testing.T) {
	validPeriod := DateRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name        string
		row         OverviewRow
		wantErr     bool
		errContains string
	}{
		{
			name: "minimal valid",
			row: OverviewRow{
				URN:    "urn:pulumi:stack::project::type::name",
				Type:   "aws:ec2:Instance",
				Status: StatusActive,
			},
		},
		{
			name: "full valid row",
			row: OverviewRow{
				URN:           "urn:pulumi:stack::project::type::name",
				Type:          "aws:ec2:Instance",
				ResourceID:    "i-1234567890abcdef0",
				Status:        StatusUpdating,
				ActualCost:    &ActualCostData{MTDCost: 50, Currency: "USD", Period: validPeriod},
				ProjectedCost: &ProjectedCostData{MonthlyCost: 100, Currency: "USD"},
				CostDrift: &CostDriftData{
					ExtrapolatedMonthly: 120,
					Projected:           100,
					Delta:               20,
					PercentDrift:        20.0,
					IsWarning:           true,
				},
			},
		},
		{
			name:        "missing URN",
			row:         OverviewRow{Type: "aws:ec2:Instance"},
			wantErr:     true,
			errContains: "URN is required",
		},
		{
			name:        "missing Type",
			row:         OverviewRow{URN: "urn:x"},
			wantErr:     true,
			errContains: "Type is required",
		},
		{
			name:        "Type too long",
			row:         OverviewRow{URN: "urn:x", Type: strings.Repeat("a", maxResourceTypeLen+1)},
			wantErr:     true,
			errContains: "Type too long",
		},
		{
			name:        "ResourceID too long",
			row:         OverviewRow{URN: "urn:x", Type: "t", ResourceID: strings.Repeat("a", maxResourceIDLen+1)},
			wantErr:     true,
			errContains: "ResourceID too long",
		},
		{
			name:        "invalid status",
			row:         OverviewRow{URN: "urn:x", Type: "t", Status: ResourceStatus(99)},
			wantErr:     true,
			errContains: "invalid Status",
		},
		{
			name: "invalid nested ActualCost",
			row: OverviewRow{
				URN:        "urn:x",
				Type:       "t",
				ActualCost: &ActualCostData{MTDCost: -1, Currency: "USD", Period: validPeriod},
			},
			wantErr:     true,
			errContains: "MTDCost must be >= 0",
		},
		{
			name: "invalid nested ProjectedCost",
			row: OverviewRow{
				URN:           "urn:x",
				Type:          "t",
				ProjectedCost: &ProjectedCostData{MonthlyCost: -1, Currency: "USD"},
			},
			wantErr:     true,
			errContains: "MonthlyCost must be >= 0",
		},
		{
			name: "invalid nested CostDrift",
			row: OverviewRow{
				URN:  "urn:x",
				Type: "t",
				CostDrift: &CostDriftData{
					ExtrapolatedMonthly: -1,
				},
			},
			wantErr:     true,
			errContains: "ExtrapolatedMonthly must be >= 0",
		},
		{
			name: "invalid nested Error",
			row: OverviewRow{
				URN:   "urn:x",
				Type:  "t",
				Error: &OverviewRowError{},
			},
			wantErr:     true,
			errContains: "URN is required for OverviewRowError",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.row.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// StackContext
// ---------------------------------------------------------------------------

func TestStackContext_Validate(t *testing.T) {
	validWindow := DateRange{
		Start: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name        string
		ctx         StackContext
		wantErr     bool
		errContains string
	}{
		{
			name: "valid no changes",
			ctx: StackContext{
				StackName:      "dev",
				TimeWindow:     validWindow,
				TotalResources: 10,
			},
		},
		{
			name: "valid with changes",
			ctx: StackContext{
				StackName:      "prod",
				Region:         "us-east-1",
				TimeWindow:     validWindow,
				HasChanges:     true,
				TotalResources: 5,
				PendingChanges: 2,
			},
		},
		{
			name:        "missing stack name",
			ctx:         StackContext{TimeWindow: validWindow},
			wantErr:     true,
			errContains: "StackName is required",
		},
		{
			name: "region too long",
			ctx: StackContext{
				StackName:  "dev",
				Region:     strings.Repeat("a", maxRegionLen+1),
				TimeWindow: validWindow,
			},
			wantErr:     true,
			errContains: "Region too long",
		},
		{
			name: "invalid time window",
			ctx: StackContext{
				StackName:  "dev",
				TimeWindow: DateRange{},
			},
			wantErr:     true,
			errContains: "start time is required",
		},
		{
			name: "negative TotalResources",
			ctx: StackContext{
				StackName:      "dev",
				TimeWindow:     validWindow,
				TotalResources: -1,
			},
			wantErr:     true,
			errContains: "TotalResources must be >= 0",
		},
		{
			name: "HasChanges but zero PendingChanges",
			ctx: StackContext{
				StackName:      "dev",
				TimeWindow:     validWindow,
				HasChanges:     true,
				PendingChanges: 0,
			},
			wantErr:     true,
			errContains: "PendingChanges must be > 0 when HasChanges is true",
		},
		{
			name: "HasChanges but negative PendingChanges",
			ctx: StackContext{
				StackName:      "dev",
				TimeWindow:     validWindow,
				HasChanges:     true,
				PendingChanges: -1,
			},
			wantErr:     true,
			errContains: "PendingChanges must be > 0 when HasChanges is true",
		},
		{
			name: "no changes with zero PendingChanges is valid",
			ctx: StackContext{
				StackName:      "dev",
				TimeWindow:     validWindow,
				HasChanges:     false,
				PendingChanges: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ctx.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.ErrorIs(t, err, ErrOverviewValidation)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
