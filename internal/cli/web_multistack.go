package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/webui"
)

var (
	errEmptyStackElement     = errors.New("--stack list has an empty element")
	errStackListNeedsProject = errors.New(
		"a stack list needs a Pulumi project's own stacks: remove --pulumi-json and --pulumi-state, " +
			"and run from a Pulumi project directory")
)

// normalizeStackList turns the raw --stack values into the ordered, de-duplicated
// stack names. Each value is split on commas and trimmed, so `a,b` and
// `--stack a --stack b` give the same list. A lone empty value is no stack, as
// it was before the flag took a list. An empty element beside another value is
// an error.
//
// A name is checked as a URL path segment (webui.ValidStackName) only when two
// or more names remain, because only then is it registered under /s/<name>/. A
// single name takes the single-stack path unchanged and may be a qualified
// Pulumi name such as org/project/stack.
func normalizeStackList(values []string) ([]string, error) {
	var elements []string
	for _, value := range values {
		for element := range strings.SplitSeq(value, ",") {
			elements = append(elements, strings.TrimSpace(element))
		}
	}
	if len(elements) == 0 || (len(elements) == 1 && elements[0] == "") {
		return nil, nil
	}

	names := make([]string, 0, len(elements))
	seen := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		if element == "" {
			return nil, errEmptyStackElement
		}
		if _, dup := seen[element]; dup {
			continue
		}
		seen[element] = struct{}{}
		names = append(names, element)
	}
	if len(names) < webui.MinTabbedStacks {
		return names, nil
	}
	for _, name := range names {
		if !webui.ValidStackName(name) {
			return nil, fmt.Errorf(
				"%q is not a valid stack name in a list: use letters, digits, '.', '_' or '-' (at most %d characters)",
				name, webui.MaxStackNameLen)
		}
	}
	return names, nil
}

// stackFlagList reads the root command's --stack list as normalized names. It
// is false when the command declares --stack as a string, as subcommands do.
func stackFlagList(cmd *cobra.Command) ([]string, bool, error) {
	if values, err := cmd.Flags().GetStringArray("stack"); err == nil {
		names, normErr := normalizeStackList(values)
		return names, true, normErr
	}
	return nil, false, nil
}

// stackFlagName reads the --stack value as the one stack name a command
// operates on. The root command declares --stack as a list for the web
// dashboard while subcommands declare it as a string, so this accepts both.
// A list that does not hold exactly one name has no single name.
func stackFlagName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if names, isList, err := stackFlagList(cmd); isList {
		if err != nil || len(names) != 1 {
			return ""
		}
		return names[0]
	}
	name, _ := cmd.Flags().GetString("stack")
	return name
}

// validateStackList checks a list of two or more stacks before any server
// starts. A list names stacks of the project, so it cannot be combined with a
// saved plan or state, and the project must be a Pulumi one. Each name must be
// one of the project's stacks; the error lists the available names. User-input
// problems exit 2, while a failure to list the stacks is an ordinary error.
func validateStackList(ctx context.Context, flags *webFlagValues, names []string) error {
	if flags.pulumiJSON != "" || flags.pulumiState != "" {
		return toValidationError(ctx, errStackListNeedsProject)
	}
	source, err := DetectProjectSource(".")
	if err != nil {
		return toValidationError(ctx, fmt.Errorf("find Pulumi or Terraform project: %w", err))
	}
	if source.Kind != ProjectKindPulumi {
		return toValidationError(ctx, errStackListNeedsProject)
	}
	if _, _, err = detectPulumiProject(ctx, names[0]); err != nil {
		return fmt.Errorf("auto-detecting Pulumi project: %w", err)
	}
	available, err := pulumi.ListStacks(ctx, source.Dir)
	if err != nil {
		return fmt.Errorf("listing Pulumi stacks: %w", err)
	}
	if err = pulumi.ValidateStacks(names, available); err != nil {
		return toValidationError(ctx, err)
	}
	return nil
}

// runWebStacks serves two or more stacks in one dashboard. Every stack gets its
// own runtime, registered under /s/<name>/, but none starts here: a stack's
// pipeline begins when the browser activates its tab, so the plugin processes
// follow the tabs a person opens. The root session is empty; the page loads the
// stack list from it and then talks to one stack at a time.
func runWebStacks(
	cmd *cobra.Command,
	flags *webFlagValues,
	names []string,
	params overviewParams,
	dateRange engine.DateRange,
	audit *auditContext,
) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	stores := openSharedStores(ctx, cmd, params.cfg)
	defer stores.Close(ctx)
	stacks := make([]*webStack, 0, len(names))
	defer func() {
		for _, stack := range stacks {
			stack.stop()
		}
	}()
	for _, name := range names {
		stackParams := params
		stackParams.stack = name
		stacks = append(stacks, newWebStack(ctx, cmd, stackParams, dateRange, audit, stores))
	}

	root := webui.NewSession(ctx, webui.SessionOptions{})
	defer root.Close()
	srv, err := listenAndPrintWeb(ctx, cmd, flags, root, audit)
	if err != nil {
		return err
	}
	for i, name := range names {
		options := webui.StackOptions{Session: stacks[i].session, Activate: stacks[i].start}
		if err = srv.AddStack(name, options); err != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), webShutdownTimeout)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
			return fmt.Errorf("registering stack %q: %w", name, err)
		}
	}

	return serveDashboard(ctx, flags, srv, audit, func() int {
		for _, stack := range stacks {
			stack.stop()
		}
		return startedRowsEnriched(stacks)
	})
}

// startedRowsEnriched totals the rows priced by the stacks that were started. A
// stack nobody opened priced nothing.
func startedRowsEnriched(stacks []*webStack) int {
	total := 0
	for _, stack := range stacks {
		if stack.started.Load() {
			total += stack.pipe.Result().RowsEnriched
		}
	}
	return total
}
