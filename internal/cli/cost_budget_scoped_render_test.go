package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestNewBudgetScopeFilter(t *testing.T) {
	tests := []struct {
		name             string
		scopeFlag        string
		wantGlobal       bool
		wantProvider     bool
		wantTag          bool
		wantType         bool
		wantProviderList []string
		wantTagList      []string
		wantTypeList     []string
	}{
		{
			name:         "empty shows all",
			scopeFlag:    "",
			wantGlobal:   true,
			wantProvider: true,
			wantTag:      true,
			wantType:     true,
		},
		{
			name:       "global only",
			scopeFlag:  "global",
			wantGlobal: true,
		},
		{
			name:         "provider only",
			scopeFlag:    "provider",
			wantProvider: true,
		},
		{
			name:             "provider with filter",
			scopeFlag:        "provider=aws",
			wantProvider:     true,
			wantProviderList: []string{"aws"},
		},
		{
			name:             "multiple provider filters",
			scopeFlag:        "provider=aws,provider=gcp",
			wantProvider:     true,
			wantProviderList: []string{"aws", "gcp"},
		},
		{
			name:      "tag only",
			scopeFlag: "tag",
			wantTag:   true,
		},
		{
			name:      "type only",
			scopeFlag: "type",
			wantType:  true,
		},
		{
			name:         "multiple scopes",
			scopeFlag:    "global,provider",
			wantGlobal:   true,
			wantProvider: true,
		},
		{
			name:         "all scopes explicit",
			scopeFlag:    "global,provider,tag,type",
			wantGlobal:   true,
			wantProvider: true,
			wantTag:      true,
			wantType:     true,
		},
		{
			name:         "invalid scope defaults to all",
			scopeFlag:    "invalid",
			wantGlobal:   true,
			wantProvider: true,
			wantTag:      true,
			wantType:     true,
		},
		{
			name:         "case insensitive",
			scopeFlag:    "GLOBAL,Provider,TAG",
			wantGlobal:   true,
			wantProvider: true,
			wantTag:      true,
		},
		{
			name:        "tag with filter",
			scopeFlag:   "tag=team:platform",
			wantTag:     true,
			wantTagList: []string{"team:platform"},
		},
		{
			name:         "type with filter",
			scopeFlag:    "type=aws:ec2/instance",
			wantType:     true,
			wantTypeList: []string{"aws:ec2/instance"},
		},
		{
			name:        "multiple tag filters",
			scopeFlag:   "tag=team:platform,tag=env:prod",
			wantTag:     true,
			wantTagList: []string{"team:platform", "env:prod"},
		},
		{
			name:         "multiple type filters",
			scopeFlag:    "type=aws:ec2/instance,type=aws:rds/instance",
			wantType:     true,
			wantTypeList: []string{"aws:ec2/instance", "aws:rds/instance"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewBudgetScopeFilter(tt.scopeFlag)
			assert.Equal(t, tt.wantGlobal, filter.ShowGlobal, "ShowGlobal mismatch")
			assert.Equal(t, tt.wantProvider, filter.ShowProvider, "ShowProvider mismatch")
			assert.Equal(t, tt.wantTag, filter.ShowTag, "ShowTag mismatch")
			assert.Equal(t, tt.wantType, filter.ShowType, "ShowType mismatch")
			if tt.wantProviderList != nil {
				assert.Equal(t, tt.wantProviderList, filter.ProviderFilter, "ProviderFilter mismatch")
			}
			if tt.wantTagList != nil {
				assert.Equal(t, tt.wantTagList, filter.TagFilter, "TagFilter mismatch")
			}
			if tt.wantTypeList != nil {
				assert.Equal(t, tt.wantTypeList, filter.TypeFilter, "TypeFilter mismatch")
			}
		})
	}
}

func TestRenderScopedBudgetStatus_Nil(t *testing.T) {
	var buf bytes.Buffer
	err := RenderScopedBudgetStatus(&buf, nil, nil)
	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRenderPlainScopedBudget_Global(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		Global: &engine.ScopedBudgetStatus{
			ScopeType:    engine.ScopeTypeGlobal,
			Budget:       config.ScopedBudget{Amount: 10000, Currency: "USD"},
			CurrentSpend: 5000,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
	}

	filter := NewBudgetScopeFilter("global")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BUDGET STATUS")
	assert.Contains(t, output, "Overall Health: OK")
	assert.Contains(t, output, "GLOBAL")
	assert.Contains(t, output, "10,000.00")
	assert.Contains(t, output, "5,000.00")
	assert.Contains(t, output, "50.0%")
}

func TestRenderPlainScopedBudget_ByProvider(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"aws": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "aws",
				Budget:       config.ScopedBudget{Amount: 5000, Currency: "USD"},
				CurrentSpend: 3500,
				Percentage:   70,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
			"gcp": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "gcp",
				Budget:       config.ScopedBudget{Amount: 3000, Currency: "USD"},
				CurrentSpend: 2700,
				Percentage:   90,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
	}

	filter := NewBudgetScopeFilter("provider")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BY PROVIDER")
	assert.Contains(t, output, "AWS")
	assert.Contains(t, output, "GCP")
	assert.Contains(t, output, "5,000.00")
	assert.Contains(t, output, "3,000.00")
	assert.Contains(t, output, "70.0%")
	assert.Contains(t, output, "90.0%")
}

func TestRenderPlainScopedBudget_ProviderFilter(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"aws": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "aws",
				Budget:       config.ScopedBudget{Amount: 5000, Currency: "USD"},
				CurrentSpend: 3500,
				Percentage:   70,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
			"gcp": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "gcp",
				Budget:       config.ScopedBudget{Amount: 3000, Currency: "USD"},
				CurrentSpend: 2700,
				Percentage:   90,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL,
	}

	// Filter to AWS only
	filter := NewBudgetScopeFilter("provider=aws")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "AWS")
	assert.NotContains(t, output, "GCP")
}

func TestRenderPlainScopedBudget_ByTag(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByTag: []*engine.ScopedBudgetStatus{
			{
				ScopeType:    engine.ScopeTypeTag,
				ScopeKey:     "team:platform",
				Budget:       config.ScopedBudget{Amount: 2000, Currency: "USD"},
				CurrentSpend: 1600,
				Percentage:   80,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
	}

	filter := NewBudgetScopeFilter("tag")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BY TAG")
	assert.Contains(t, output, "team:platform")
	assert.Contains(t, output, "2,000.00")
	assert.Contains(t, output, "80.0%")
}

func TestRenderPlainScopedBudget_ByType(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByType: map[string]*engine.ScopedBudgetStatus{
			"aws:ec2/instance": {
				ScopeType:    engine.ScopeTypeType,
				ScopeKey:     "aws:ec2/instance",
				Budget:       config.ScopedBudget{Amount: 1000, Currency: "USD"},
				CurrentSpend: 1100,
				Percentage:   110,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
	}

	filter := NewBudgetScopeFilter("type")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "BY TYPE")
	assert.Contains(t, output, "aws:ec2/instance")
	assert.Contains(t, output, "1,000.00")
	assert.Contains(t, output, "110.0%")
}

func TestRenderPlainScopedBudget_TagFilter(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByTag: []*engine.ScopedBudgetStatus{
			{
				ScopeType:    engine.ScopeTypeTag,
				ScopeKey:     "team:platform",
				Budget:       config.ScopedBudget{Amount: 2000, Currency: "USD"},
				CurrentSpend: 1600,
				Percentage:   80,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				Currency:     "USD",
			},
			{
				ScopeType:    engine.ScopeTypeTag,
				ScopeKey:     "team:backend",
				Budget:       config.ScopedBudget{Amount: 3000, Currency: "USD"},
				CurrentSpend: 1500,
				Percentage:   50,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
	}

	// Filter to team:platform only
	filter := NewBudgetScopeFilter("tag=team:platform")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "team:platform")
	assert.NotContains(t, output, "team:backend")
}

func TestRenderPlainScopedBudget_TypeFilter(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		ByType: map[string]*engine.ScopedBudgetStatus{
			"aws:ec2/instance": {
				ScopeType:    engine.ScopeTypeType,
				ScopeKey:     "aws:ec2/instance",
				Budget:       config.ScopedBudget{Amount: 1000, Currency: "USD"},
				CurrentSpend: 800,
				Percentage:   80,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
				Currency:     "USD",
			},
			"aws:rds/instance": {
				ScopeType:    engine.ScopeTypeType,
				ScopeKey:     "aws:rds/instance",
				Budget:       config.ScopedBudget{Amount: 2000, Currency: "USD"},
				CurrentSpend: 1000,
				Percentage:   50,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
	}

	// Filter to aws:ec2/instance only
	filter := NewBudgetScopeFilter("type=aws:ec2/instance")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "aws:ec2/instance")
	assert.NotContains(t, output, "aws:rds/instance")
}

func TestRenderPlainScopedBudget_CriticalScopes(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		CriticalScopes: []string{"provider:gcp", "type:aws:ec2/instance"},
		OverallHealth:  pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED,
	}

	filter := NewBudgetScopeFilter("")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "CRITICAL SCOPES")
	assert.Contains(t, output, "provider:gcp")
	assert.Contains(t, output, "type:aws:ec2/instance")
}

func TestRenderPlainScopedBudget_Warnings(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		Warnings:      []string{"overlapping tag priorities for team:backend and team:platform"},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING,
	}

	filter := NewBudgetScopeFilter("")

	var buf bytes.Buffer
	err := renderPlainScopedBudget(&buf, result, filter)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "WARNINGS:")
	assert.Contains(t, output, "overlapping tag priorities")
}

func TestHealthStatusLabel(t *testing.T) {
	tests := []struct {
		health pbc.BudgetHealthStatus
		want   string
	}{
		{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK, "OK"},
		{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_WARNING, "WARNING"},
		{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_CRITICAL, "CRITICAL"},
		{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_EXCEEDED, "EXCEEDED"},
		{pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_UNSPECIFIED, "UNSPECIFIED"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := healthStatusLabel(tt.health)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		target string
		want   bool
	}{
		{"exact match", []string{"aws", "gcp"}, "aws", true},
		{"case insensitive match", []string{"AWS", "GCP"}, "aws", true},
		{"no match", []string{"aws", "gcp"}, "azure", false},
		{"empty slice", []string{}, "aws", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsIgnoreCase(tt.slice, tt.target)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderScopedProgressBar(t *testing.T) {
	tests := []struct {
		name       string
		percentage float64
		width      int
	}{
		{"zero percent", 0, 20},
		{"half percent", 50, 20},
		{"full percent", 100, 20},
		{"over budget", 150, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bar := renderScopedProgressBar(tt.percentage, tt.width)
			assert.NotEmpty(t, bar)
			// Bar should contain some combination of filled and empty chars
		})
	}
}

func TestRenderStyledScopedBudget_NoError(t *testing.T) {
	result := &engine.ScopedBudgetResult{
		Global: &engine.ScopedBudgetStatus{
			ScopeType:    engine.ScopeTypeGlobal,
			Budget:       config.ScopedBudget{Amount: 10000, Currency: "USD"},
			CurrentSpend: 5000,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
		ByProvider: map[string]*engine.ScopedBudgetStatus{
			"aws": {
				ScopeType:    engine.ScopeTypeProvider,
				ScopeKey:     "aws",
				Budget:       config.ScopedBudget{Amount: 5000, Currency: "USD"},
				CurrentSpend: 3500,
				Percentage:   70,
				Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
				Currency:     "USD",
			},
		},
		OverallHealth: pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
	}

	filter := NewBudgetScopeFilter("")

	var buf bytes.Buffer
	err := renderStyledScopedBudget(&buf, result, filter)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())

	// Verify key content is present
	output := buf.String()
	assert.Contains(t, output, "BUDGET STATUS")
	assert.Contains(t, output, "Overall Health")
}

func TestRenderProviderSection_SortedOutput(t *testing.T) {
	providers := map[string]*engine.ScopedBudgetStatus{
		"gcp": {
			ScopeType:    engine.ScopeTypeProvider,
			ScopeKey:     "gcp",
			Budget:       config.ScopedBudget{Amount: 3000, Currency: "USD"},
			CurrentSpend: 1500,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
		"aws": {
			ScopeType:    engine.ScopeTypeProvider,
			ScopeKey:     "aws",
			Budget:       config.ScopedBudget{Amount: 5000, Currency: "USD"},
			CurrentSpend: 2500,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
		"azure": {
			ScopeType:    engine.ScopeTypeProvider,
			ScopeKey:     "azure",
			Budget:       config.ScopedBudget{Amount: 2000, Currency: "USD"},
			CurrentSpend: 1000,
			Percentage:   50,
			Health:       pbc.BudgetHealthStatus_BUDGET_HEALTH_STATUS_OK,
			Currency:     "USD",
		},
	}

	output := renderProviderSection(providers, nil)

	// AWS should appear before AZURE, which should appear before GCP
	awsIdx := strings.Index(output, "AWS")
	azureIdx := strings.Index(output, "AZURE")
	gcpIdx := strings.Index(output, "GCP")

	require.NotEqual(t, -1, awsIdx, "AWS not found in output")
	require.NotEqual(t, -1, azureIdx, "AZURE not found in output")
	require.NotEqual(t, -1, gcpIdx, "GCP not found in output")

	assert.Less(t, awsIdx, azureIdx, "AWS should come before AZURE")
	assert.Less(t, azureIdx, gcpIdx, "AZURE should come before GCP")
}
