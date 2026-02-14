package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rshade/finfocus/internal/logging"
)

const (
	minURNParts = 3
)

// PulumiPlan represents the top-level structure of a Pulumi preview JSON output.
type PulumiPlan struct {
	Steps []PulumiStep `json:"steps"`
}

// PulumiStep represents a single resource operation step in a Pulumi plan.
type PulumiStep struct {
	Op       string                 `json:"op"`
	URN      string                 `json:"urn"`
	Type     string                 `json:"type"`
	Provider string                 `json:"provider"`
	Inputs   map[string]interface{} `json:"inputs"`
	Outputs  map[string]interface{} `json:"outputs"`
	NewState *PulumiState           `json:"newState,omitempty"`
	OldState *PulumiState           `json:"oldState,omitempty"`
}

// PulumiState represents the state of a resource in a Pulumi step.
type PulumiState struct {
	Type     string                 `json:"type"`
	URN      string                 `json:"urn"`
	Inputs   map[string]interface{} `json:"inputs"`
	Outputs  map[string]interface{} `json:"outputs"`
	Provider string                 `json:"provider"`
}

// PulumiResource contains the detailed information about a resource in a Pulumi step.
type PulumiResource struct {
	Type     string
	URN      string
	Provider string
	Inputs   map[string]interface{}
	Outputs  map[string]interface{}
}

// ParsePulumiPlan parses a Pulumi plan from JSON bytes.
// The data parameter should contain the raw Pulumi plan JSON output.
// It returns the parsed *PulumiPlan, or an error if the data cannot be unmarshaled into a PulumiPlan.
func ParsePulumiPlan(data []byte) (*PulumiPlan, error) {
	return ParsePulumiPlanWithContext(context.Background(), data)
}

// ParsePulumiPlanWithContext parses Pulumi plan JSON from the provided byte slice using the given context to obtain a logger.
// The ctx is used only for logging; data should contain the raw Pulumi plan JSON.
// ParsePulumiPlanWithContext parses a Pulumi preview JSON document from the provided byte slice and returns the resulting PulumiPlan.
// ctx provides cancellation and carries logging context used during parsing.
// data is the raw JSON bytes of a Pulumi preview plan.
// On success, the parsed *PulumiPlan is returned; if the JSON cannot be unmarshaled, an error wrapping the unmarshal failure is returned.
func ParsePulumiPlanWithContext(ctx context.Context, data []byte) (*PulumiPlan, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "ingest").
		Str("operation", "parse_plan").
		Int("data_size_bytes", len(data)).
		Msg("parsing Pulumi plan from bytes")

	var plan PulumiPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "ingest").
			Str("operation", "parse_plan").
			Err(err).
			Msg("failed to parse plan JSON")
		return nil, fmt.Errorf("parsing plan JSON: %w", err)
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "ingest").
		Int("step_count", len(plan.Steps)).
		Msg("plan parsed successfully")

	return &plan, nil
}

// LoadPulumiPlan loads and parses a Pulumi plan JSON file from the specified path.
func LoadPulumiPlan(path string) (*PulumiPlan, error) {
	return LoadPulumiPlanWithContext(context.Background(), path)
}

// LoadPulumiPlanWithContext loads and parses the Pulumi plan JSON file at the given path using the logger carried in ctx.
// It logs progress and any errors encountered to the context's logger.
// path is the filesystem path to the Pulumi plan JSON file to read and parse.
// It returns the parsed *PulumiPlan on success.
// It returns a non-nil error if the file cannot be read or if parsing the JSON fails.
func LoadPulumiPlanWithContext(ctx context.Context, path string) (*PulumiPlan, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "ingest").
		Str("operation", "load_plan").
		Str("plan_path", path).
		Msg("loading Pulumi plan")

	data, err := os.ReadFile(path)
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "ingest").
			Err(err).
			Str("plan_path", path).
			Msg("failed to read plan file")
		return nil, fmt.Errorf("reading plan file: %w", err)
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "ingest").
		Int("file_size_bytes", len(data)).
		Msg("plan file read successfully")

	return ParsePulumiPlanWithContext(ctx, data)
}

// GetResources extracts all resources from the Pulumi plan steps.
func (p *PulumiPlan) GetResources() []PulumiResource {
	return p.GetResourcesWithContext(context.Background())
}

// GetResourcesWithContext extracts all resources from the Pulumi plan steps with logging context.
func (p *PulumiPlan) GetResourcesWithContext(ctx context.Context) []PulumiResource {
	log := logging.FromContext(ctx)
	var resources []PulumiResource
	var skippedOps []string

	for _, step := range p.Steps {
		//nolint:nestif // Complexity is acceptable for this resource extraction logic
		if step.Op == "create" || step.Op == "update" || step.Op == "same" {
			resType := step.Type
			inputs := step.Inputs

			// Prioritize NewState for Create/Update operations if available
			if step.NewState != nil {
				if resType == "" {
					resType = step.NewState.Type
				}
				if inputs == nil {
					inputs = step.NewState.Inputs
				}
			}

			if resType == "" {
				resType = extractTypeFromURN(step.URN)
			}

			resources = append(resources, PulumiResource{
				Type:     resType,
				URN:      step.URN,
				Provider: extractProviderFromURN(step.URN),
				Inputs:   inputs,
				Outputs:  resolveStepOutputs(step),
			})
			log.Debug().
				Ctx(ctx).
				Str("component", "ingest").
				Str("resource_type", step.Type).
				Str("extracted_type", resType).
				Str("operation", step.Op).
				Str("urn", step.URN).
				Msg("extracted resource from plan")
		} else {
			skippedOps = append(skippedOps, step.Op)
		}
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "ingest").
		Int("total_steps", len(p.Steps)).
		Int("extracted_resources", len(resources)).
		Int("skipped_operations", len(skippedOps)).
		Msg("resource extraction complete")

	return resources
}

// resolveStepOutputs picks the best available Outputs for a step.
// resolveStepOutputs returns the outputs map for a PulumiStep.
// It selects outputs with the following priority: step-level Outputs, NewState.Outputs,
// and for operations "update" or "same" falls back to OldState.Outputs.
// Returns nil if no outputs are available.
func resolveStepOutputs(step PulumiStep) map[string]interface{} {
	switch {
	case len(step.Outputs) > 0:
		return step.Outputs
	case step.NewState != nil && len(step.NewState.Outputs) > 0:
		return step.NewState.Outputs
	case (step.Op == "update" || step.Op == "same") &&
		step.OldState != nil && len(step.OldState.Outputs) > 0:
		return step.OldState.Outputs
	default:
		return nil
	}
}

// extractTypeFromURN extracts the resource type from a Pulumi URN.
// It returns the third '::'-separated segment when the URN contains at least
// minURNParts segments; otherwise it returns an empty string.
func extractTypeFromURN(urn string) string {
	parts := strings.Split(urn, "::")
	if len(parts) >= minURNParts {
		return parts[2]
	}
	return ""
}

func extractProviderFromURN(urn string) string {
	parts := strings.Split(urn, "::")
	if len(parts) >= minURNParts {
		providerParts := strings.Split(parts[2], ":")
		if len(providerParts) > 0 && providerParts[0] != "" {
			return providerParts[0]
		}
	}
	return unknownProvider
}
