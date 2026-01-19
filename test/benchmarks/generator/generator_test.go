package generator

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBenchmarkConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  BenchmarkConfig
		wantErr error
	}{
		{
			name: "valid config",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        5,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			wantErr: nil,
		},
		{
			name: "zero resource count",
			config: BenchmarkConfig{
				ResourceCount:   0,
				MaxDepth:        5,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			wantErr: ErrInvalidResourceCount,
		},
		{
			name: "negative resource count",
			config: BenchmarkConfig{
				ResourceCount:   -1,
				MaxDepth:        5,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			wantErr: ErrInvalidResourceCount,
		},
		{
			name: "negative max depth",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        -1,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			wantErr: ErrInvalidMaxDepth,
		},
		{
			name: "zero max depth is valid",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        0,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			wantErr: nil,
		},
		{
			name: "dependency ratio below zero",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        5,
				DependencyRatio: -0.1,
				Seed:            42,
			},
			wantErr: ErrInvalidDependencyRatio,
		},
		{
			name: "dependency ratio above one",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        5,
				DependencyRatio: 1.1,
				Seed:            42,
			},
			wantErr: ErrInvalidDependencyRatio,
		},
		{
			name: "dependency ratio at boundaries",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        5,
				DependencyRatio: 1.0,
				Seed:            42,
			},
			wantErr: nil,
		},
		{
			name:    "preset small is valid",
			config:  PresetSmall,
			wantErr: nil,
		},
		{
			name:    "preset medium is valid",
			config:  PresetMedium,
			wantErr: nil,
		},
		{
			name:    "preset large is valid",
			config:  PresetLarge,
			wantErr: nil,
		},
		{
			name:    "preset deep nesting is valid",
			config:  PresetDeepNesting,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Validate() error = %v, wantErr nil", err)
				}
			} else if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGeneratePlan(t *testing.T) {
	tests := []struct {
		name      string
		config    BenchmarkConfig
		wantErr   bool
		checkPlan func(t *testing.T, plan SyntheticPlan)
	}{
		{
			name:   "generates correct resource count",
			config: PresetSmall,
			checkPlan: func(t *testing.T, plan SyntheticPlan) {
				if len(plan.Resources) != PresetSmall.ResourceCount {
					t.Errorf("expected %d resources, got %d", PresetSmall.ResourceCount, len(plan.Resources))
				}
			},
		},
		{
			name: "invalid config returns error",
			config: BenchmarkConfig{
				ResourceCount:   0,
				MaxDepth:        5,
				DependencyRatio: 0.3,
			},
			wantErr: true,
		},
		{
			name: "deterministic with same seed",
			config: BenchmarkConfig{
				ResourceCount:   10,
				MaxDepth:        2,
				DependencyRatio: 0.5,
				Seed:            12345,
			},
			checkPlan: func(t *testing.T, plan SyntheticPlan) {
				// Generate again with same seed
				plan2, err := GeneratePlan(BenchmarkConfig{
					ResourceCount:   10,
					MaxDepth:        2,
					DependencyRatio: 0.5,
					Seed:            12345,
				})
				require.NoError(t, err)
				for i := range plan.Resources {
					if plan.Resources[i].Name != plan2.Resources[i].Name {
						t.Errorf("resource %d name mismatch: %s vs %s",
							i, plan.Resources[i].Name, plan2.Resources[i].Name)
					}
					if plan.Resources[i].Type != plan2.Resources[i].Type {
						t.Errorf("resource %d type mismatch: %s vs %s",
							i, plan.Resources[i].Type, plan2.Resources[i].Type)
					}
				}
			},
		},
		{
			name: "resources have valid types",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        3,
				DependencyRatio: 0.3,
				Seed:            42,
			},
			checkPlan: func(t *testing.T, plan SyntheticPlan) {
				validTypes := make(map[string]bool)
				for _, rt := range resourceTypes {
					validTypes[rt] = true
				}
				for _, r := range plan.Resources {
					if !validTypes[r.Type] {
						t.Errorf("invalid resource type: %s", r.Type)
					}
				}
			},
		},
		{
			name: "dependencies reference earlier resources only",
			config: BenchmarkConfig{
				ResourceCount:   50,
				MaxDepth:        2,
				DependencyRatio: 0.8, // High ratio to ensure some dependencies
				Seed:            42,
			},
			checkPlan: func(t *testing.T, plan SyntheticPlan) {
				nameToIdx := make(map[string]int)
				for i, r := range plan.Resources {
					nameToIdx[r.Name] = i
				}
				for i, r := range plan.Resources {
					for _, dep := range r.DependsOn {
						depIdx, ok := nameToIdx[dep]
						if !ok {
							t.Errorf("resource %d references unknown dependency: %s", i, dep)
						}
						if depIdx >= i {
							t.Errorf("resource %d references later resource %d as dependency", i, depIdx)
						}
					}
				}
			},
		},
		{
			name: "zero dependency ratio means no dependencies",
			config: BenchmarkConfig{
				ResourceCount:   100,
				MaxDepth:        3,
				DependencyRatio: 0.0,
				Seed:            42,
			},
			checkPlan: func(t *testing.T, plan SyntheticPlan) {
				for _, r := range plan.Resources {
					if len(r.DependsOn) > 0 {
						t.Errorf("expected no dependencies with ratio 0.0, got %v", r.DependsOn)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := GeneratePlan(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("GeneratePlan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.checkPlan != nil && err == nil {
				tt.checkPlan(t, plan)
			}
		})
	}
}

func TestGeneratePlan_Variables(t *testing.T) {
	config := BenchmarkConfig{
		ResourceCount:   10,
		MaxDepth:        2,
		DependencyRatio: 0.3,
		Seed:            42,
	}

	plan, err := GeneratePlan(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Variables["environment"] != "benchmark" {
		t.Errorf("expected environment=benchmark, got %v", plan.Variables["environment"])
	}

	if plan.Variables["generated_count"] != config.ResourceCount {
		t.Errorf("expected generated_count=%d, got %v", config.ResourceCount, plan.Variables["generated_count"])
	}
}
