package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/terraform"
)

const historyReadHelp = "Read stored cost history without running a plan. Terraform directories resolve the current project identity without --stack; --include-workspace opts into workspace scoping. Pulumi histories use --stack."

const (
	historyOutputPlain   = "plain"
	historyOutputJSON    = "json"
	historyProgressWidth = 32
	defaultTermWidth     = 80
)

// NewCostHistoryCmd creates the `cost history` command group.
func NewCostHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Collect and view projected cost history",
		Long: `Stores and charts projected-cost timelines offline. collect reads successful
Pulumi updates and needs the Pulumi CLI and cost plugins. Terraform projections
are recorded automatically under a stable project identity. view, list, diff,
prune, and export resolve that identity without running a plan. Use
--include-workspace or cost.terraform.include_workspace to scope Terraform
history to its workspace. The flag is ignored for Pulumi.`,
	}
	cmd.PersistentFlags().
		Bool("include-workspace", false, "Include the Terraform workspace in the history identity (overrides cost.terraform.include_workspace)")
	cmd.AddCommand(
		NewCostHistoryCollectCmd(),
		NewCostHistoryViewCmd(),
		NewCostHistoryListCmd(),
		NewCostHistoryDiffCmd(),
		NewCostHistoryExportCmd(),
		NewCostHistoryPruneCmd(),
	)
	return cmd
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func flagYes(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Flags().Lookup("yes") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("yes")
	return err == nil && value
}

func parseDateFlag(value string, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD)", value)
	}
	parsed = parsed.UTC()
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return parsed, nil
}

func historyDir(dir string) string {
	if dir != "" {
		return dir
	}
	dir, _ = historyDirectory(
		context.Background(),
		config.GetGlobalConfig(),
		config.GetResolvedProjectDir(),
	)
	return dir
}

// historyTarget is the database a --stack value resolves to, plus the project
// and stack name to record in it.
type historyTarget struct {
	Path    string
	Project string
	Stack   string
	Label   string
}

func historyPath(dir, stack string) (string, error) {
	target, err := resolveHistoryTarget(dir, stack)
	if err != nil {
		return "", err
	}
	return target.Path, nil
}

// resolveHistoryTarget finds the cost history database for stack. The project
// comes from a fully qualified "org/project/stack", else from the Pulumi project
// in the working directory, which is the project `pulumi stack export` resolves
// the stack against.
func resolveHistoryTarget(dir, stack string) (historyTarget, error) {
	if target, ok, err := terraformHistoryTarget(dir, terraformWorkspaceFromConfig()); ok ||
		err != nil {
		return target, err
	}
	return resolveHistoryTargetFor(dir, stack, currentProjectName())
}

func terraformHistoryTarget(dir string, includeWorkspace bool) (historyTarget, bool, error) {
	return terraformHistoryTargetContext(context.Background(), dir, includeWorkspace)
}

func terraformHistoryTargetContext(
	ctx context.Context,
	dir string,
	includeWorkspace bool,
) (historyTarget, bool, error) {
	search := "."
	source, detectErr := DetectProjectSource(search)
	if detectErr != nil || source.Kind != ProjectKindTerraform {
		// A missing marker is the Pulumi history path, not a failure.
		return historyTarget{}, false, nil //nolint:nilerr // detectErr only means this directory is not Terraform
	}
	identity, err := terraform.ResolveIdentity(ctx, source.Dir, includeWorkspace)
	if err != nil {
		return historyTarget{}, true, err
	}
	name, err := history.CostFileNameFor(identity.Project, identity.Stack)
	if err != nil {
		return historyTarget{}, true, err
	}
	base := historyDir(dir)
	return historyTarget{
		Path:    filepath.Join(base, name),
		Project: identity.Project,
		Stack:   identity.Stack,
		Label:   identity.Label,
	}, true, nil
}

// resolveHistoryCommandTarget keeps the storage directory separate from the
// project directory and resolves the workspace flag before the config default.
func resolveHistoryCommandTarget(cmd *cobra.Command, dir string) (historyTarget, error) {
	includeWorkspace := terraformIncludeWorkspace(cmd)
	if target, ok, err := terraformHistoryTargetContext(commandContext(cmd), dir, includeWorkspace); ok ||
		err != nil {
		return target, err
	}
	stack, err := requiredStack(cmd)
	if err != nil {
		return historyTarget{}, err
	}
	return resolveHistoryTargetFor(dir, stack, currentProjectName())
}

func openHistoryTargetRead(target historyTarget) (*history.CostDB, error) {
	if target.Label == "" {
		return history.OpenCostDBRead(target.Path)
	}
	db, err := history.OpenCostDBFor(target.Path, target.Project, target.Stack)
	if err != nil {
		return nil, err
	}
	if err = db.SetProjectLabel(target.Label); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func terraformWorkspaceFromConfig() bool {
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		return false
	}
	return cfg.Cost.Terraform.IncludeWorkspace
}

func currentProjectName() string {
	root, err := pulumidetect.FindProject(".")
	if err != nil {
		return ""
	}
	name, err := pulumidetect.GetProjectName(root)
	if err != nil {
		return ""
	}
	return name
}

func resolveHistoryTargetFor(dir, stack, cwdProject string) (historyTarget, error) {
	stack = strings.TrimSpace(stack)
	legacyName, err := history.CostFileName(stack)
	if err != nil {
		return historyTarget{}, err
	}
	base := historyDir(dir)
	legacy := historyTarget{Path: filepath.Join(base, legacyName), Stack: stack}

	project, short := history.SplitStack(stack)
	if project == "" {
		project = cwdProject
	}
	if project == "" {
		return resolveUnscopedTarget(base, legacy, short)
	}
	name, err := history.CostFileNameFor(project, short)
	if err != nil {
		return historyTarget{}, err
	}
	scoped := historyTarget{Path: filepath.Join(base, name), Project: project, Stack: short}
	if fileExists(scoped.Path) {
		return scoped, nil
	}
	if fileExists(legacy.Path) && legacyBelongsTo(legacy.Path, project) {
		legacy.Project = project
		return legacy, nil
	}
	return scoped, nil
}

// resolveUnscopedTarget handles a bare stack name with no project to go on. A
// legacy `<stack>.history.db` wins; otherwise a single `<project>@<stack>`
// database is used, and several are ambiguous.
func resolveUnscopedTarget(base string, legacy historyTarget, short string) (historyTarget, error) {
	if fileExists(legacy.Path) {
		return legacy, nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return legacy, nil //nolint:nilerr // no history directory yet: a fresh legacy-named database
	}
	var found []historyTarget
	for _, entry := range entries {
		project, stack, ok := history.ProjectFromFileName(entry.Name())
		if !ok || entry.IsDir() || stack != short {
			continue
		}
		found = append(
			found,
			historyTarget{Path: filepath.Join(base, entry.Name()), Project: project, Stack: stack},
		)
	}
	switch len(found) {
	case 0:
		return legacy, nil
	case 1:
		return found[0], nil
	default:
		projects := make([]string, len(found))
		for i, target := range found {
			projects[i] = target.Project
		}
		return historyTarget{}, fmt.Errorf(
			"stack %q exists in several projects (%s); run from a project directory or pass --stack org/project/%s",
			short,
			strings.Join(projects, ", "),
			short,
		)
	}
}

// legacyBelongsTo reports whether a `<stack>.history.db` written before
// projects were recorded can be used by project. It can unless its stored
// resource URNs name a different project. A database that cannot be read here is
// left to the real open to report.
func legacyBelongsTo(path, project string) bool {
	db, err := history.OpenCostDBRead(path)
	if err != nil {
		return true
	}
	defer func() { _ = db.Close() }()
	owner, err := db.InferProject()
	if err != nil {
		return true
	}
	return owner == "" || owner == project
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return "~/" + filepath.ToSlash(rel)
}

func stackKnown(name string, listed []string) bool {
	for _, item := range listed {
		if item == name || strings.HasSuffix(item, "/"+name) {
			return true
		}
	}
	return false
}

func parseStackList(data []byte) ([]string, error) {
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing pulumi stack ls: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func joinStacks(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if ok {
			return nil, fmt.Errorf(
				"%s: %w: %s",
				name,
				err,
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return nil, err
	}
	return out, nil
}

func terminalWidth() int {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return defaultTermWidth
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return defaultTermWidth
	}
	return fitTerminalChartWidth(width, true)
}

// fitTerminalChartWidth leaves the non-terminal default unchanged and reserves
// asciigraph's three-column axis when the measured width is a real terminal.
func fitTerminalChartWidth(columns int, isTerminal bool) int {
	const axisOffset = 3
	if !isTerminal || columns <= axisOffset {
		return columns
	}
	return columns - axisOffset
}

func globalBudget(cfg *config.Config) (float64, bool) {
	if cfg == nil {
		cfg = config.GetGlobalConfig()
	}
	if cfg == nil || cfg.Cost.Budgets == nil || !cfg.Cost.Budgets.HasGlobalBudget() {
		return 0, false
	}
	return cfg.Cost.Budgets.Global.Amount, true
}

func formatRemain(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
