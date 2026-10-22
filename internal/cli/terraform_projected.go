package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/terraform"
)

func loadTerraformPlanFile(
	ctx context.Context,
	cmd *cobra.Command,
	path string,
) ([]engine.ResourceDescriptor, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("reading terraform plan: %w", err)
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, ingest.TerraformPlanMaxBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("reading terraform plan: %w", err)
	}
	if int64(len(raw)) > ingest.TerraformPlanMaxBytes {
		return nil, false, toValidationError(ctx, errors.New("terraform plan input exceeds 32 MiB"))
	}
	if err = validateTerraformWorkspace(ctx, cmd); err != nil {
		return nil, false, err
	}
	resources, err := descriptorsFromPlan(ctx, cmd, raw, true)
	if err != nil {
		return nil, false, err
	}
	return resources, terraformPlanUnchanged(resources), nil
}

func loadTerraformDirectory(
	ctx context.Context,
	cmd *cobra.Command,
	dir string,
) ([]engine.ResourceDescriptor, error) {
	if err := validateTerraformWorkspace(ctx, cmd); err != nil {
		return nil, err
	}
	raw, err := terraform.RunPlan(ctx, dir)
	if err != nil {
		return nil, err
	}
	return descriptorsFromPlan(ctx, cmd, raw, false)
}

func descriptorsFromPlan(
	ctx context.Context,
	cmd *cobra.Command,
	raw []byte,
	fromUser bool,
) ([]engine.ResourceDescriptor, error) {
	parser := &ingest.TerraformPlanParser{}
	response, err := parser.Parse(ctx, ingest.TerraformPlanRequest(raw))
	if err != nil {
		if fromUser {
			return nil, planInputError(err)
		}
		return nil, terraform.PlanFailure(planInputError(err))
	}
	printSkippedProviders(cmd, parser.SkippedProviders())
	return ingest.AdaptTerraformPlan(response, parser.Operations(), parser.Before()), nil
}

func planInputError(err error) error {
	if _, ok := errors.AsType[*ingest.PlanFormatError](err); ok {
		return fmt.Errorf("terraform plan format_version must be major version 1: %w", err)
	}
	if err != nil && strings.Contains(err.Error(), "parse_max_input_bytes") {
		return fmt.Errorf("terraform plan input exceeds 32 MiB: %w", err)
	}
	return fmt.Errorf("reading terraform plan: %w", err)
}

func printSkippedProviders(cmd *cobra.Command, skipped map[string]int) {
	if cmd == nil {
		return
	}
	names := make([]string, 0, len(skipped))
	for provider := range skipped {
		names = append(names, provider)
	}
	slices.Sort(names)
	for _, provider := range names {
		fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %d %s resources: no cost.\n", skipped[provider], provider)
	}
}

func terraformPlanUnchanged(resources []engine.ResourceDescriptor) bool {
	for _, resource := range resources {
		if resource.Operation != engine.DiffOperationSame {
			return false
		}
	}
	return true
}

// validateTerraformWorkspace rejects the reserved workspace before pricing or writing history.
func validateTerraformWorkspace(ctx context.Context, cmd *cobra.Command) error {
	if !terraformIncludeWorkspace(cmd) {
		return nil
	}
	source, err := DetectProjectSource(".")
	if err != nil || source.Kind != ProjectKindTerraform {
		return nil //nolint:nilerr // A missing marker means no Terraform history identity to validate.
	}
	_, err = terraform.ResolveIdentity(ctx, source.Dir, true)
	if err != nil {
		return toValidationError(ctx, err)
	}
	return nil
}
