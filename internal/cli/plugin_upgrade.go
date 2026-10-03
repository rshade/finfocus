package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/pluginupgrade"
)

type pluginUpgradeOptions struct {
	dir        string
	to         string
	allowDirty bool
	format     string
}

// pluginUpgradeResult is the command's JSON document and the data the table
// output renders.
type pluginUpgradeResult struct {
	Plan      *pluginupgrade.Plan `json:"plan"`
	DryRun    bool                `json:"dry_run"`
	Changed   []string            `json:"changed_files"`
	NextSteps []string            `json:"next_steps,omitempty"`
}

// NewPluginUpgradeCmd returns the "plugin upgrade" command, which moves a
// plugin project's source to a newer finfocus-spec version. It applies the
// edits that need no judgment and lists the manual steps of each hop, with a
// link to that hop's migration guide.
func NewPluginUpgradeCmd() *cobra.Command {
	var (
		opts   pluginUpgradeOptions
		output string
	)

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade a plugin project to a newer finfocus-spec version",
		Long: `Upgrade the source of a plugin project to a newer finfocus-spec version.

This edits a plugin's Go module. To update an installed plugin binary, use
'plugin update'.

The command reads the finfocus-spec requirement from go.mod and plans one hop
per release that needs action from plugin authors, up to the version this
finfocus build uses (or --to). It then rewrites the go.mod requirement and go
directive, and any SpecVersion declaration. Everything else is listed as a manual step with a
migration guide; the finfocus-plugin-upgrade agent skill works through them.

Run with --dry-run first to see the plan. Applying requires a clean git
working tree so the upgrade is one reviewable diff. It never uses the
network: run 'go mod tidy' afterwards.`,
		Example: `  # Show the upgrade plan for the plugin in the current directory
  finfocus plugin upgrade --dry-run

  # Apply the automatic edits
  finfocus plugin upgrade

  # Stop at a specific finfocus-spec version
  finfocus plugin upgrade --dir ../my-plugin --to v0.6.0

  # Machine-readable plan, as used by the agent skill
  finfocus plugin upgrade --dry-run --output json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts.format = resolveOutputFormat(cmd, "output", output)
			if opts.format != outputFormatTable && opts.format != outputFormatJSON {
				return fmt.Errorf("unsupported output format: %s (supported: table, json)", opts.format)
			}
			return runPluginUpgrade(cmd, opts)
		},
	}

	cmd.Flags().StringVar(&opts.dir, "dir", ".", "Plugin project directory (containing go.mod)")
	cmd.Flags().StringVar(&opts.to, "to", "",
		"Target finfocus-spec release: a hop version or this build's own (default: this build's)")
	cmd.Flags().BoolVar(&opts.allowDirty, "allow-dirty", false,
		"Apply even if the directory is not a clean git working tree")
	cmd.Flags().StringVar(&output, "output", outputFormatTable, "Output format (table, json)")

	return cmd
}

func runPluginUpgrade(cmd *cobra.Command, opts pluginUpgradeOptions) error {
	project, err := pluginupgrade.Detect(opts.dir)
	if err != nil {
		return err
	}
	target := opts.to
	if target == "" {
		target = pluginsdk.SpecVersion
	}
	plan, err := pluginupgrade.NewPlan(project, target, pluginsdk.SpecVersion)
	if err != nil {
		return err
	}

	rehearse := func(context.Context) error {
		return renderPluginUpgrade(cmd, opts.format, pluginUpgradeResult{Plan: plan, DryRun: true, Changed: []string{}})
	}
	commit := func(ctx context.Context) error {
		result := pluginUpgradeResult{Plan: plan, Changed: []string{}}
		if plan.UpToDate {
			return renderPluginUpgrade(cmd, opts.format, result)
		}
		if !opts.allowDirty {
			if cleanErr := pluginupgrade.CheckClean(ctx, opts.dir); cleanErr != nil {
				return cleanErr
			}
		}
		changed, applyErr := pluginupgrade.Apply(project, plan)
		if applyErr != nil {
			return applyErr
		}
		result.Changed = changed
		result.NextSteps = upgradeNextSteps(plan)
		return renderPluginUpgrade(cmd, opts.format, result)
	}
	return ax.Perform(cmd.Context(), rehearse, commit)
}

func upgradeNextSteps(plan *pluginupgrade.Plan) []string {
	steps := []string{
		"go mod tidy",
		"go build ./...",
		"go test ./...",
	}
	for _, h := range plan.Hops {
		if len(h.Manual) > 0 {
			steps = append(steps, "Work through the manual steps of each hop, using its guide")
			break
		}
	}
	return steps
}

func renderPluginUpgrade(cmd *cobra.Command, format string, result pluginUpgradeResult) error {
	if format == outputFormatJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	plan := result.Plan
	cmd.Printf("Plugin:  %s %s", pluginupgrade.Module, plan.Current)
	if plan.GoVersion != "" {
		cmd.Printf(" (go %s)", plan.GoVersion)
	}
	cmd.Println()
	if plan.UpToDate {
		cmd.Printf("\nPlugin is up to date (finfocus-spec %s).\n", plan.Target)
		return nil
	}
	cmd.Printf("Target:  %s\n", plan.Target)
	if plan.RequiredGo != "" {
		cmd.Printf("Go:      raise the go directive to %s\n", plan.RequiredGo)
	}

	if len(plan.Hops) > 0 {
		cmd.Println("\nHops:")
	}
	for _, h := range plan.Hops {
		cmd.Printf("  %-7s %s\n", h.To, h.Summary)
		for _, a := range h.Automatic {
			cmd.Printf("          automatic: %s\n", a)
		}
		for _, m := range h.Manual {
			cmd.Printf("          manual:    %s\n", m)
		}
		cmd.Printf("          guide:     %s\n", h.GuideURL)
	}

	if len(plan.Warnings) > 0 {
		cmd.Println("\nWarnings:")
		for _, w := range plan.Warnings {
			cmd.Printf("  ! %s\n", w)
		}
	}

	if result.DryRun {
		cmd.Println("\nDry run: no files changed.")
		return nil
	}
	cmd.Println("\nChanged files:")
	for _, f := range result.Changed {
		cmd.Printf("  %s\n", f)
	}
	cmd.Println("\nNext steps:")
	for i, s := range result.NextSteps {
		cmd.Printf("  %d. %s\n", i+1, s)
	}
	return nil
}
