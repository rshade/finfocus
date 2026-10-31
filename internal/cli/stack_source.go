package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/stackload"
)

// buildStackSource resolves command inputs without moving history or ESC into the loader.
func buildStackSource(
	ctx context.Context,
	cmd *cobra.Command,
	planPath, statePath string,
) (stackload.Source, error) {
	src := stackload.Source{PlanPath: planPath, StatePath: statePath, Stack: stackFlagValue(cmd)}
	if statePath != "" {
		return src, nil
	}
	if planPath != "" {
		// Saved previews optionally add state from the surrounding project. Export
		// selects its current stack itself when no explicit --stack was supplied.
		src.ProjectDir, _ = pulumidetect.FindProject(".")
		return src, nil
	}
	dir, stack, err := detectPulumiProject(ctx, src.Stack)
	if err != nil {
		return src, err
	}
	src.ProjectDir, src.Stack = dir, stack
	return src, nil
}

func loadPulumiProjected(
	ctx context.Context,
	cmd *cobra.Command,
	params costProjectedParams,
) ([]engine.ResourceDescriptor, *stackload.Snapshot, error) {
	src, err := buildStackSource(ctx, cmd, params.pulumiJSONPath, params.pulumiState)
	if err != nil {
		return nil, nil, err
	}
	snapshot, err := stackload.Load(ctx, src)
	if err != nil {
		return nil, nil, err
	}
	printStackNotices(cmd, snapshot)
	return snapshot.Descriptors(), snapshot, nil
}

func printStackNotices(cmd *cobra.Command, snapshot *stackload.Snapshot) {
	reason := snapshot.StateStatus.Reason
	switch {
	case reason == stackload.ReasonNotRequested && snapshot.OmittedUnchanged > 0:
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"Note: the preview omits %d unchanged resources, so they are not priced. Make the preview with `--show-sames` or run `finfocus overview`.\n",
			snapshot.OmittedUnchanged,
		)
	case !snapshot.StateStatus.Loaded && reason != stackload.ReasonNotRequested:
		if snapshot.OmittedUnchanged > 0 {
			fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Warning: stack state unavailable (%s): %d unchanged resources are not included in the stack total.\n",
				reason,
				snapshot.OmittedUnchanged,
			)
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: stack state unavailable (%s).\n", reason)
		}
	}
	if len(snapshot.Mismatches) > 0 {
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"Warning: the preview does not match the stack state for %d resources; they are priced as the preview lists them.\n",
			len(snapshot.Mismatches),
		)
	}
}

func setProjectedStackSummary(diff *engine.DiffResult, snapshot *stackload.Snapshot) {
	if snapshot == nil {
		return
	}
	diff.IncludeStack = true
	diff.Summary.StackComplete = snapshot.StateStatus.Loaded
	if !snapshot.StateStatus.Loaded {
		diff.Summary.StackIncompleteReason = snapshot.StateStatus.Reason
		diff.Summary.StackTotal = diff.Summary.TotalAfter
		diff.Summary.StackCurrency = diff.Summary.Currency
		diff.Summary.StackResources = len(diff.Entries)
		diff.Summary.OmittedUnchanged = snapshot.OmittedUnchanged
		diff.StackUnchanged = nil
		diff.StackErrors = nil
	}
}

func loadDeployedPulumi(ctx context.Context, stack string) ([]engine.ResourceDescriptor, error) {
	dir, resolved, err := detectPulumiProject(ctx, stack)
	if err != nil {
		return nil, err
	}
	snapshot, err := stackload.LoadState(ctx, stackload.Source{ProjectDir: dir, Stack: resolved})
	if err != nil {
		var safe *stackload.Error
		if errors.As(err, &safe) && safe.Reason == stackload.ReasonPassphraseRequired {
			return nil, fmt.Errorf("set PULUMI_CONFIG_PASSPHRASE to read this stack: %w", err)
		}
		return nil, err
	}
	return snapshot.DeployedDescriptors(), nil
}

// collapseLiveReplacements is the live inventory of the preview: one descriptor
// per replacement, with the old resource of an unpaired delete-replaced step
// dropped. CollapseReplacements merges a replacement's steps but leaves a step
// with no partner alone, and that step is a resource the preview removes.
func collapseLiveReplacements(resources []engine.ResourceDescriptor) []engine.ResourceDescriptor {
	return slices.DeleteFunc(engine.CollapseReplacements(resources), func(resource engine.ResourceDescriptor) bool {
		return resource.Operation == "delete-replaced"
	})
}
