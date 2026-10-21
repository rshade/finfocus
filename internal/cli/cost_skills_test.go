package cli

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func costSkillDir(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", name)
}

func readCostSkill(t *testing.T, name string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(costSkillDir(t, name), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return walkErr
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	return b.String()
}

func costSubcommand(t *testing.T, name string) *cobra.Command {
	t.Helper()
	for _, sub := range newCostCmd().Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	require.Failf(t, "missing subcommand", "cost %s", name)
	return nil
}

func lookupSkillFlag(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Lookup(name) != nil || cmd.InheritedFlags().Lookup(name) != nil
}

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return keys
}

func TestCostSkillsDocumentLiveFlags(t *testing.T) {
	t.Parallel()

	check := readCostSkill(t, "cost-check")
	drift := readCostSkill(t, "cost-drift")

	projected := costSubcommand(t, "projected")
	forecast := costSubcommand(t, "forecast")
	overview := NewOverviewCmd()
	list := NewPluginListCmd()

	for _, flag := range []string{
		"pulumi-json", "terraform-state", "stack", "output", "adapter", "filter",
		"exit-on-threshold", "exit-code",
	} {
		assert.Truef(t, lookupSkillFlag(projected, flag), "cost projected has --%s", flag)
		assert.Contains(t, check, "--"+flag)
	}
	for _, flag := range []string{"pulumi-json", "output", "months"} {
		assert.Truef(t, lookupSkillFlag(forecast, flag), "cost forecast has --%s", flag)
	}
	assert.Contains(t, check, "finfocus cost forecast")
	assert.Contains(t, check, "--months")

	for _, flag := range []string{
		"output", "yes", "stack", "from", "to", "state-only", "pulumi-state", "pulumi-json", "adapter",
	} {
		assert.Truef(t, lookupSkillFlag(overview, flag), "overview has --%s", flag)
		assert.Contains(t, drift, "--"+flag)
	}
	assert.Nil(t, overview.Flags().Lookup("terraform-state"), "overview has no Terraform input")
	assert.Contains(t, drift, "`overview` has no `--terraform-state` flag")

	require.NotNil(t, list.Flags().Lookup("output"))
	assert.Contains(t, check, "finfocus plugin list --output json")
	assert.Contains(t, drift, "finfocus plugin list --output json")

	for _, cmd := range []*cobra.Command{projected, overview} {
		assert.Nil(t, cmd.Flags().ShorthandLookup("o"))
	}
	assert.Contains(t, check, "`-o` is not a flag")
	assert.Contains(t, drift, "`-o` is not a flag")

	exitCode := projected.InheritedFlags().Lookup("exit-code")
	require.NotNil(t, exitCode)
	assert.Equal(t, "1", exitCode.DefValue)
	assert.Equal(t, 1, ax.ExitInternal)
	assert.Equal(t, 2, ax.ExitValidation)
	assert.Contains(t, check, "| `2` | Invalid flag, value, or input file.")
	assert.Contains(t, drift, "| `2` | Invalid flag, value, or file.")
}

func TestCostSkillsDocumentLiveJSONFields(t *testing.T) {
	t.Parallel()

	check := readCostSkill(t, "cost-check")
	drift := readCostSkill(t, "cost-drift")

	for _, key := range jsonKeys(t, engine.DiffSummary{TotalBefore: 1}) {
		if key == "currency" {
			continue
		}
		assert.Contains(t, check, key)
	}
	for _, key := range jsonKeys(t, engine.StructuredError{}) {
		assert.Contains(t, check, key)
	}
	for _, code := range []string{
		engine.ErrCodeNoCostData, engine.ErrCodePluginError,
		engine.ErrCodeValidationError, engine.ErrCodeTimeoutError,
	} {
		assert.Contains(t, check, code)
	}
	for _, key := range []string{
		"totalMonthly", "byProvider", "byService", "byAdapter", "resourceType", "resourceId",
		"monthly", "operation", "beforeMonthly", "deltaMonthly", "pluginName",
	} {
		assert.Contains(t, check, key)
	}

	for _, key := range jsonKeys(t, engine.CostDriftData{}) {
		assert.Contains(t, drift, key)
	}
	for _, key := range jsonKeys(t, engine.OverviewSummary{}) {
		if key == "potentialSavings" || key == "projectedDelta" {
			continue
		}
		assert.Contains(t, drift, key)
	}
	for _, key := range jsonKeys(t, engine.OverviewRowError{}) {
		assert.Contains(t, drift, key)
	}
	assert.Contains(t, drift, "actualCost.mtdCost")
	assert.Contains(t, drift, "projectedCost.monthlyCost")
	assert.Contains(t, drift, "metadata.timeWindow")
}

func TestCostDriftSkillMatchesDriftOmissionRules(t *testing.T) {
	t.Parallel()

	drift := readCostSkill(t, "cost-drift")

	within, err := engine.CalculateCostDriftWithElapsedDays(100, 100*30.0/(730.0/24.0), 30, 30)
	require.NoError(t, err)
	assert.Nil(t, within, "a small drift is omitted")

	over, err := engine.CalculateCostDriftWithElapsedDays(200, 100*30.0/(730.0/24.0), 30, 30)
	require.NoError(t, err)
	require.NotNil(t, over)
	assert.True(t, over.IsWarning)

	newResource, err := engine.CalculateCostDriftWithElapsedDays(0, 10, 30, 30)
	require.NoError(t, err)
	assert.Nil(t, newResource, "a new resource is omitted")

	deleted, err := engine.CalculateCostDriftWithElapsedDays(10, 0, 30, 30)
	require.NoError(t, err)
	assert.Nil(t, deleted, "a deleted resource is omitted")

	_, err = engine.CalculateCostDriftWithElapsedDays(10, 10, 1.5, 30)
	require.ErrorIs(t, err, engine.ErrOverviewValidation, "under 2 days is insufficient")

	assert.Contains(t, drift, "within 10%")
	assert.Contains(t, drift, "less than 2 days")
	assert.Contains(t, drift, "new or deleted")
	assert.Contains(t, drift, "insufficient data")
	assert.InDelta(t, 30.4167, float64(engine.HoursPerMonth)/24.0, 0.0001)
}

func TestCostSkillsDeclareSupportedVersion(t *testing.T) {
	t.Parallel()

	root := filepath.Join(costSkillDir(t, "cost-check"), "..", "..")
	manifest, err := os.ReadFile(filepath.Join(root, ".release-please-manifest.json"))
	require.NoError(t, err)
	var releases map[string]string
	require.NoError(t, json.Unmarshal(manifest, &releases))
	released := releases["."]
	require.NotEmpty(t, released)

	declared := regexp.MustCompile(`\*\*Required CLI: finfocus v(\d+\.\d+\.\d+) or newer\.\*\*`)
	for _, name := range []string{"cost-check", "cost-drift"} {
		body, readErr := os.ReadFile(filepath.Join(costSkillDir(t, name), "SKILL.md"))
		require.NoError(t, readErr)
		match := declared.FindSubmatch(body)
		require.NotNilf(t, match, "%s declares its required CLI version", name)
		assert.LessOrEqualf(t, compareSemver(t, string(match[1]), released), 0,
			"%s requires a version newer than the last release %s", name, released)
	}
}

func compareSemver(t *testing.T, a, b string) int {
	t.Helper()
	left := strings.Split(a, ".")
	right := strings.Split(b, ".")
	require.Len(t, left, 3)
	require.Len(t, right, 3)
	for i := range left {
		l, err := strconv.Atoi(left[i])
		require.NoError(t, err)
		r, err := strconv.Atoi(right[i])
		require.NoError(t, err)
		if l != r {
			if l < r {
				return -1
			}
			return 1
		}
	}
	return 0
}

func TestCostSkillsDocumentDefaultThresholds(t *testing.T) {
	t.Parallel()

	check := readCostSkill(t, "cost-check")
	drift := readCostSkill(t, "cost-drift")
	for _, row := range []string{
		"| `thresholds.resource` | 100 |",
		"| `thresholds.increase` | 50 |",
		"| `thresholds.increase_pct` | 20 |",
		"| `thresholds.total` | 1000 |",
	} {
		assert.Contains(t, check, row)
	}
	for _, row := range []string{
		"| `drift.drift_pct` | 10 |",
		"| `drift.drift_absolute` | 50 |",
	} {
		assert.Contains(t, drift, row)
	}
	assert.NotContains(t, drift, "total_drift_pct`  |")
	assert.NotContains(t, drift, "Overall drift (derived)")
	assert.Contains(t, check, "empty-plan")
	assert.Contains(t, check, "COST_CHECK_RESULT:")
	assert.Contains(t, drift, "COST_DRIFT_RESULT:")
}

func TestCostSkillsArchives(t *testing.T) {
	t.Parallel()

	for name, wantFiles := range map[string][]string{
		"cost-check": {
			"SKILL.md", "references/cli-contract.md", "references/failure-modes.md",
			"references/report-template.md", "references/thresholds.md",
		},
		"cost-drift": {
			"SKILL.md", "references/cli-contract.md", "references/drift-explanations.md",
			"references/failure-modes.md", "references/report-template.md", "references/thresholds.md",
		},
	} {
		dir := costSkillDir(t, name)
		archive, err := zip.OpenReader(filepath.Join(dir, name+".skill"))
		require.NoError(t, err)

		names := map[string]bool{}
		for _, file := range archive.File {
			names[file.Name] = true
			rel := strings.TrimPrefix(file.Name, name+"/")
			packed, openErr := file.Open()
			require.NoError(t, openErr)
			got, readErr := io.ReadAll(packed)
			require.NoError(t, packed.Close())
			require.NoError(t, readErr)
			want, fileErr := os.ReadFile(filepath.Join(dir, rel))
			require.NoError(t, fileErr)
			assert.Equal(t, string(want), string(got), file.Name)
		}
		require.NoError(t, archive.Close())
		for _, rel := range wantFiles {
			assert.Truef(t, names[name+"/"+rel], "%s archive missing %s", name, rel)
		}
		assert.Len(t, names, len(wantFiles), "%s archive has no extra files", name)
	}
}

func TestCostSkillsMentionOnlyLiveFlags(t *testing.T) {
	t.Parallel()

	known := map[string]bool{"json": true, "format": true, "version": true, "show-sames": true}
	usageFlag := regexp.MustCompile(`--([a-z][a-z-]*)`)
	for _, cmd := range []*cobra.Command{
		costSubcommand(t, "projected"), costSubcommand(t, "forecast"), NewOverviewCmd(), NewPluginListCmd(),
	} {
		usage := cmd.Flags().FlagUsages() + cmd.InheritedFlags().FlagUsages()
		for _, match := range usageFlag.FindAllStringSubmatch(usage, -1) {
			known[match[1]] = true
		}
	}

	flagToken := regexp.MustCompile(`(?:^|[\s` + "`" + `(])--([a-z][a-z-]*)`)
	for _, name := range []string{"cost-check", "cost-drift"} {
		for _, match := range flagToken.FindAllStringSubmatch(readCostSkill(t, name), -1) {
			assert.Truef(t, known[match[1]], "%s mentions --%s, which no documented command has", name, match[1])
		}
	}
}
