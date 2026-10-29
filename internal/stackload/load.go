package stackload

import (
	"context"
	"errors"

	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pulumi"

	"golang.org/x/sync/errgroup"

	"os"
	"strings"
)

// Load reads state and preview concurrently. A failed stack export degrades to
// the preview alone; an explicit state file that cannot be read is an error.
// Project state from a different stack or project than the preview is not
// merged; the preview is then used alone with ReasonNotRequested.
func Load(ctx context.Context, src Source) (*Snapshot, error) {
	if src.SkipPreview || src.StatePath != "" && src.PlanPath == "" {
		return LoadState(ctx, src)
	}
	if src.ProjectDir == "" && src.StatePath == "" {
		return LoadPlan(ctx, src)
	}
	group, child := errgroup.WithContext(ctx)
	var state, plan *Snapshot
	var stateErr error
	group.Go(func() error {
		state, stateErr = LoadState(child, src)
		if src.StatePath != "" {
			// A file the caller named is input, not an unreachable backend.
			return stateErr
		}
		return nil
	})
	group.Go(func() error { var err error; plan, err = LoadPlan(child, src); return err })
	if err := group.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		// A canceled export is the caller's cancellation, not unreachable state.
		return nil, err
	}
	if stateErr != nil {
		if loadErr, ok := errors.AsType[*Error](stateErr); ok {
			plan.StateStatus = Status{Reason: loadErr.Reason}
		}
		return plan, nil
	}
	if src.StatePath == "" && !sameStack(state, plan) {
		// A preview of another stack is projected on its own, as outside a project.
		return plan, nil
	}
	return state.WithPlan(plan.Plan), nil
}
func emptySnapshot(src Source) *Snapshot {
	return &Snapshot{
		StackName:   src.Stack,
		ProjectDir:  src.ProjectDir,
		StateStatus: Status{Reason: ReasonNotRequested},
		PlanStatus:  Status{Reason: ReasonNotRequested},
	}
}

// LoadState reads an explicit export or exports a resolved project without previewing.
func LoadState(ctx context.Context, src Source) (*Snapshot, error) {
	var data []byte
	var err error
	switch {
	case src.StatePath != "":
		data, err = os.ReadFile(src.StatePath)
	case src.ProjectDir != "":
		data, err = pulumi.StackExport(
			ctx,
			pulumi.ExportOptions{ProjectDir: src.ProjectDir, Stack: src.Stack, Passphrase: src.Passphrase},
		)
	default:
		err = errors.New("no state input")
	}
	if err != nil {
		return nil, fileReadError("state", src.StatePath, err)
	}
	state, err := ingest.ParseStackExportWithContext(ctx, data)
	if err != nil {
		return nil, invalidInputError("state", src.StatePath, err)
	}
	result := emptySnapshot(src)
	result.RawState = state.GetCustomResourcesWithContext(ctx)
	for _, r := range state.Deployment.Resources {
		if !r.Custom {
			result.ComponentURNs = append(result.ComponentURNs, r.URN)
		}
	}
	result.stateGraph = BuildDependencyGraph(state.Deployment.Resources)
	result.graph = result.stateGraph
	result.State = convertStateResources(result.RawState)
	result.ManifestTime = state.Deployment.Manifest.Time
	result.StateStatus = Status{Loaded: true, Reason: ReasonNone}
	if result.StackName == "" {
		for _, r := range state.Deployment.Resources {
			if r.Type == "pulumi:pulumi:Stack" {
				parts := strings.Split(r.URN, "::")
				if len(parts) > 1 {
					result.StackName = strings.TrimPrefix(parts[0], "urn:pulumi:")
				}
				break
			}
		}
	}
	return result, nil
}

// LoadPlan reads an explicit preview or runs preview for a resolved project.
func LoadPlan(ctx context.Context, src Source) (*Snapshot, error) {
	var data []byte
	var err error
	switch {
	case src.PlanPath != "":
		data, err = os.ReadFile(src.PlanPath)
	case src.ProjectDir != "":
		data, err = pulumi.Preview(
			ctx,
			pulumi.PreviewOptions{ProjectDir: src.ProjectDir, Stack: src.Stack, Passphrase: src.Passphrase},
		)
	default:
		err = errors.New("no plan input")
	}
	if err != nil && src.PlanPath == "" && src.ProjectDir != "" {
		result := classifiedError("plan", err)
		result.cause = err
		return nil, result
	}
	if err != nil {
		return nil, fileReadError("plan", src.PlanPath, err)
	}
	plan, err := ingest.ParsePulumiPlanWithContext(ctx, data)
	if err != nil {
		return nil, invalidInputError("plan", src.PlanPath, err)
	}
	return emptySnapshot(src).WithPlan(plan), nil
}
