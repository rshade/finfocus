package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

const (
	flagPulumiJSON     = "pulumi-json"
	flagPulumiState    = "pulumi-state"
	flagTerraformPlan  = "terraform-plan"
	flagTerraformState = "terraform-state"
)

func validateInputPaths(paths ...string) error {
	count := 0
	for _, path := range paths {
		if path != "" {
			count++
		}
	}
	if count > 1 {
		return errors.New(
			"--terraform-plan, --pulumi-json, --pulumi-state, and --terraform-state are mutually exclusive; use only one",
		)
	}
	return nil
}

func validateInfrastructureInputs(cmd *cobra.Command) error {
	var paths []string
	for _, name := range []string{flagTerraformPlan, flagPulumiJSON, flagPulumiState, flagTerraformState} {
		if cmd.Flag(name) != nil {
			value, err := cmd.Flags().GetString(name)
			if err != nil {
				return err
			}
			paths = append(paths, value)
		}
	}
	// Overview already accepts a Pulumi state and preview together to build a diff.
	// cost anomalies runs the same pipeline and takes the same inputs.
	if cmd.Name() == "overview" || cmd.Name() == "anomalies" {
		plan, _ := cmd.Flags().GetString("terraform-plan")
		state, _ := cmd.Flags().GetString("terraform-state")
		if plan == "" && state == "" {
			return nil
		}
	}
	return validateInputPaths(paths...)
}

func addIncludeWorkspaceFlag(cmd *cobra.Command) {
	cmd.Flags().
		Bool("include-workspace", false, "Include the Terraform workspace in history identity (ignored for Pulumi)")
}

func terraformIncludeWorkspace(cmd *cobra.Command) bool {
	if cmd != nil && cmd.Flag("include-workspace") != nil &&
		cmd.Flags().Changed("include-workspace") {
		value, _ := cmd.Flags().GetBool("include-workspace")
		return value
	}
	cfg := config.GetGlobalConfig()
	return cfg != nil && cfg.Cost.Terraform.IncludeWorkspace
}

func terraformProjectionSelected(cmd *cobra.Command) bool {
	for _, name := range []string{flagPulumiJSON, flagPulumiState, flagTerraformState} {
		if cmd.Flag(name) != nil {
			value, _ := cmd.Flags().GetString(name)
			if value != "" {
				return false
			}
		}
	}
	if cmd.Flag("terraform-plan") != nil {
		value, _ := cmd.Flags().GetString("terraform-plan")
		if value != "" {
			return true
		}
	}
	source, err := DetectProjectSource(".")
	return err == nil && source.Kind == ProjectKindTerraform
}
