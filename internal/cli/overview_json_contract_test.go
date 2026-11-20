package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/sourcepos"
)

const overviewFieldsInventory = "../../packages/vscode-finfocus/contract/overview-fields.json"

type fieldInventory struct {
	Required map[string]string `json:"required"`
	Optional map[string]string `json:"optional"`
	Failure  map[string]string `json:"failure"`

	Placeholders map[string]unpricedPlaceholder `json:"placeholders"`
}

// unpricedPlaceholder is the extension's "unpriced row" signal: the value a
// row prints in the field at Path when it has no price.
type unpricedPlaceholder struct {
	Path           string   `json:"path"`
	Value          string   `json:"value"`
	ExemptStatuses []string `json:"exemptStatuses"`
}

func loadFieldInventory(t *testing.T) fieldInventory {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(overviewFieldsInventory))
	require.NoError(t, err)
	var inv fieldInventory
	require.NoError(t, json.Unmarshal(raw, &inv))
	require.NotEmpty(t, inv.Required)
	return inv
}

func jsonKind(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case nil:
		return "null"
	}
	return "unknown"
}

// lookup walks a path such as "resources[].display.projected" and returns the
// value at every leaf. A segment ending in "[]" fans out over an array.
func lookup(doc any, path string) ([]any, bool) {
	current := []any{doc}
	for _, segment := range strings.Split(path, ".") {
		fan := strings.HasSuffix(segment, "[]")
		key := strings.TrimSuffix(segment, "[]")
		var next []any
		for _, node := range current {
			object, ok := node.(map[string]any)
			if !ok {
				return nil, false
			}
			child, present := object[key]
			if !present {
				return nil, false
			}
			if fan {
				items, isArray := child.([]any)
				if !isArray {
					return nil, false
				}
				next = append(next, items...)
				continue
			}
			next = append(next, child)
		}
		current = next
	}
	return current, true
}

// lookupPresent is lookup for optional fields: a node that lacks the key is
// skipped instead of ending the walk, so the rows that do carry it are typed.
func lookupPresent(doc any, path string) []any {
	current := []any{doc}
	for _, segment := range strings.Split(path, ".") {
		fan := strings.HasSuffix(segment, "[]")
		key := strings.TrimSuffix(segment, "[]")
		var next []any
		for _, node := range current {
			object, ok := node.(map[string]any)
			if !ok {
				continue
			}
			child, present := object[key]
			if !present {
				continue
			}
			if items, isArray := child.([]any); fan && isArray {
				next = append(next, items...)
				continue
			}
			if !fan {
				next = append(next, child)
			}
		}
		current = next
	}
	return current
}

// inventoryProblems lists every required path that is missing or mistyped and
// every optional path that is present with the wrong type.
func inventoryProblems(doc map[string]any, inv fieldInventory) []string {
	var problems []string
	for path, want := range inv.Required {
		values, ok := lookup(doc, path)
		if !ok {
			problems = append(problems, "missing "+path)
			continue
		}
		for _, v := range values {
			if got := jsonKind(v); got != want {
				problems = append(problems, "type "+path+": "+got)
				break
			}
		}
	}
	for path, want := range inv.Optional {
		for _, v := range lookupPresent(doc, path) {
			if got := jsonKind(v); got != want {
				problems = append(problems, "type "+path+": "+got)
				break
			}
		}
	}
	sort.Strings(problems)
	return problems
}

func removePath(doc any, path string) {
	segments := strings.Split(path, ".")
	current := []any{doc}
	for i, segment := range segments {
		fan := strings.HasSuffix(segment, "[]")
		key := strings.TrimSuffix(segment, "[]")
		var next []any
		for _, node := range current {
			object, ok := node.(map[string]any)
			if !ok {
				continue
			}
			if i == len(segments)-1 {
				delete(object, key)
				continue
			}
			child := object[key]
			if fan {
				if items, isArray := child.([]any); isArray {
					next = append(next, items...)
				}
				continue
			}
			next = append(next, child)
		}
		current = next
	}
}

func contractModified() *time.Time {
	modified := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	return &modified
}

func contractRows() []engine.OverviewRow {
	return []engine.OverviewRow{
		{
			URN: "urn:pulumi:dev::shop::aws:ec2/instance:Instance::web", Type: "aws:ec2/instance:Instance",
			Status:        engine.StatusUpdating,
			ActualCost:    &engine.ActualCostData{MTDCost: 18.4, Currency: "USD"},
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 61.32, Currency: "USD"},
			BaselineProjectedCost: &engine.ProjectedCostData{
				MonthlyCost: 30.66, Currency: "USD",
			},
			PropertyDiffs: []engine.PropertyDiff{{Key: "instanceType", OldValue: "t3.large", NewValue: "t3.xlarge"}},
			Location:      &sourcepos.Location{File: "lib/web.ts", Line: 12, Column: 5},
			ModifiedAt:    contractModified(),
		},
		{
			URN: "urn:pulumi:dev::shop::aws:s3/bucket:Bucket::logs", Type: "aws:s3/bucket:Bucket",
			Status:        engine.StatusCreating,
			ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 0, Currency: "USD"},
			Location:      &sourcepos.Location{File: "main.go", Line: 14},
		},
		{
			URN:    "urn:pulumi:dev::shop::command:local:Command::wait",
			Type:   "command:local:Command",
			Status: engine.StatusActive,
		},
		{
			URN: "urn:pulumi:dev::shop::aws:rds/instance:Instance::db", Type: "aws:rds/instance:Instance",
			Status: engine.StatusActive,
			Error: &engine.OverviewRowError{
				URN: "urn:pulumi:dev::shop::aws:rds/instance:Instance::db", ErrorType: engine.ErrorTypeNetwork,
				Message: "pricing failed",
			},
		},
	}
}

func renderContractDoc(t *testing.T, stateOnly bool, budget *engine.BudgetResult) map[string]any {
	t.Helper()
	cmd, _ := overviewCmd(nil)
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)

	rows := contractRows()
	result := engine.ComputeOverviewResult(rows, 15)
	stackCtx := engine.StackContext{
		StackName:      "dev",
		TimeWindow:     engine.DateRange{Start: time.Now().Add(-24 * time.Hour), End: time.Now()},
		TotalResources: len(rows),
		GeneratedAt:    time.Now(),
		IsStateOnly:    stateOnly,
	}
	require.NoError(t, renderOverviewOutput(cmd, outputFormatJSON, result, stackCtx, budget))

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc), out.String())
	return doc
}

func configBudgetResult(t *testing.T) *engine.BudgetResult {
	t.Helper()
	result := engine.BuildConfigBudgetResult(context.Background(), &config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 100, Currency: "USD", Period: "monthly"},
	}, 61.32)
	require.NotNil(t, result)
	return result
}

func TestOverviewJSONContract_InventoryPathsPresentWithTypes(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)

	cases := map[string]struct {
		stateOnly bool
		budget    *engine.BudgetResult
	}{
		"state only, no budget":      {stateOnly: true},
		"preview, no budget":         {},
		"preview, configured budget": {budget: configBudgetResult(t)},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			doc := renderContractDoc(t, tc.stateOnly, tc.budget)
			assert.Empty(t, inventoryProblems(doc, inv))
		})
	}
}

func TestOverviewJSONContract_StateOnlyMetadataIsOptional(t *testing.T) {
	t.Parallel()

	preview := renderContractDoc(t, false, nil)
	metadata, ok := preview["metadata"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, metadata, "isStateOnly", "absence means false")

	stateOnly := renderContractDoc(t, true, nil)
	metadata, ok = stateOnly["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, metadata["isStateOnly"])
}

func TestOverviewJSONContract_NoBudgetHasEmptyBudgetDisplay(t *testing.T) {
	t.Parallel()

	doc := renderContractDoc(t, true, nil)

	display, ok := doc["budgetDisplay"].(map[string]any)
	require.True(t, ok, "budgetDisplay is present even with no budget")
	assert.Equal(t, []any{}, display["details"])
	assert.NotContains(t, doc, "budgets")
}

func TestOverviewJSONContract_BudgetDisplayMatchesBudget(t *testing.T) {
	t.Parallel()

	doc := renderContractDoc(t, true, configBudgetResult(t))

	display, ok := doc["budgetDisplay"].(map[string]any)
	require.True(t, ok)
	assert.NotEmpty(t, display["healthDisplay"])
	details, ok := display["details"].([]any)
	require.True(t, ok)
	require.Len(t, details, 1)
	assert.Contains(t, display["footerDisplay"], "Budget:")
	assert.Contains(t, doc, "budgets")
}

func TestOverviewJSONContract_RemovingAnyRequiredFieldIsDetected(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)
	baseline, err := json.Marshal(renderContractDoc(t, true, configBudgetResult(t)))
	require.NoError(t, err)

	for path := range inv.Required {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			var doc map[string]any
			require.NoError(t, json.Unmarshal(baseline, &doc))
			removePath(doc, path)
			assert.Contains(t, strings.Join(inventoryProblems(doc, inv), "\n"), path)
		})
	}
}

func TestOverviewJSONContract_RetypingAnyRequiredFieldIsDetected(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)
	baseline := renderContractDoc(t, true, configBudgetResult(t))

	doc := map[string]any{}
	raw, err := json.Marshal(baseline)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &doc))
	summary, ok := doc["summary"].(map[string]any)
	require.True(t, ok)
	display, ok := summary["display"].(map[string]any)
	require.True(t, ok)
	display["projected"] = 12.5

	assert.Contains(t, inventoryProblems(doc, inv), "type summary.display.projected: number")
}

func TestOverviewJSONContract_FailureEnvelopeOnStderrHasInventoryKeys(t *testing.T) {
	inv := loadFieldInventory(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv(config.HistoryEnvEnabled, "false")
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})

	root := NewRootCmd("test")
	result := axtest.Run(context.Background(), t, root, []string{
		"overview", "--pulumi-state", filepath.Join(t.TempDir(), "missing.json"),
		"--plain", "--yes", "--output", "json", "--format", "json",
	})
	require.NotZero(t, result.ExitCode)

	assert.Empty(t, result.Stdout, "a failure writes nothing to stdout")
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(result.Stderr, &envelope), string(result.Stderr))
	for key, kind := range inv.Failure {
		require.Contains(t, envelope, key)
		assert.Equal(t, kind, jsonKind(envelope[key]), key)
	}
	message, ok := envelope["message"].(string)
	require.True(t, ok)
	assert.Regexp(t, `\([a-z_]+\)$`, message, "the failure category is the trailing parenthesized token")
}

func TestOverviewJSONContract_PlainPathUsesConfigBudgetFallback(t *testing.T) {
	t.Parallel()

	cmd, _ := overviewCmd(nil)
	params := overviewParams{output: outputFormatJSON}
	cfg := newPlainOverviewPipelineConfig(cmd, params, engine.DateRange{}, nil)
	assert.True(t, cfg.wantBudget)
	assert.True(t, cfg.budgetFallback, "the JSON plain path applies the config budget fallback")

	table := newPlainOverviewPipelineConfig(cmd, overviewParams{output: outputFormatTable}, engine.DateRange{}, nil)
	assert.False(t, table.wantBudget)
	assert.False(t, table.budgetFallback)
}

//nolint:paralleltest // sets the process-wide global config
func TestOverviewJSONContract_ConfigBudgetFillsInWhenNoPluginReturnsOne(t *testing.T) {
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{Cost: config.CostConfig{Budgets: &config.BudgetsConfig{
		Global: &config.ScopedBudget{Amount: 500, Currency: "USD", Period: "monthly"},
	}}})
	pipe := newOverviewPipeline(overviewPipelineConfig{wantBudget: true, budgetFallback: true})
	rows := contractRows()

	empty := make(chan budgetFetchResult, 1)
	empty <- budgetFetchResult{}
	got, err := pipe.deliverBudget(context.Background(), nil, empty, rows)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Budgets, 1)
	assert.Equal(t, "config-global", got.Budgets[0].GetId())

	plugin := &engine.BudgetResult{Budgets: []*pbc.Budget{{Id: "plugin-budget", Name: "Plugin"}}}
	fromPlugin := make(chan budgetFetchResult, 1)
	fromPlugin <- budgetFetchResult{result: plugin}
	got, err = pipe.deliverBudget(context.Background(), nil, fromPlugin, rows)
	require.NoError(t, err)
	require.Len(t, got.Budgets, 1)
	assert.Equal(t, "plugin-budget", got.Budgets[0].GetId(), "a plugin budget takes precedence over config")
}

func resourceByName(t *testing.T, doc map[string]any, name string) map[string]any {
	t.Helper()
	resources, ok := doc["resources"].([]any)
	require.True(t, ok)
	for _, item := range resources {
		resource, isObject := item.(map[string]any)
		require.True(t, isObject)
		if urn, _ := resource["urn"].(string); strings.HasSuffix(urn, "::"+name) {
			return resource
		}
	}
	require.FailNow(t, "no resource named "+name)
	return nil
}

func TestOverviewJSONContract_LocationShape(t *testing.T) {
	t.Parallel()

	doc := renderContractDoc(t, false, nil)

	assert.Equal(t,
		map[string]any{"file": "lib/web.ts", "line": float64(12), "column": float64(5)},
		resourceByName(t, doc, "web")["location"])
	assert.Equal(t,
		map[string]any{"file": "main.go", "line": float64(14)},
		resourceByName(t, doc, "logs")["location"],
		"an unknown column is omitted, not zero")
	assert.NotContains(t, resourceByName(t, doc, "wait"), "location", "an unresolved location is omitted, not null")
	assert.NotContains(t, resourceByName(t, doc, "db"), "location")
}

func TestOverviewJSONContract_InventoryPinsOptionalLocation(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)

	assert.Equal(t, "object", inv.Optional["resources[].location"])
	assert.Equal(t, "string", inv.Optional["resources[].location.file"])
	assert.Equal(t, "number", inv.Optional["resources[].location.line"])
	assert.Equal(t, "number", inv.Optional["resources[].location.column"])
	assert.NotContains(t, inv.Required, "resources[].location", "a resource may have no location")
}

func TestOverviewJSONContract_RetypedOptionalLocationIsDetected(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)
	doc := renderContractDoc(t, false, nil)
	require.Empty(t, inventoryProblems(doc, inv))

	location, ok := resourceByName(t, doc, "web")["location"].(map[string]any)
	require.True(t, ok)
	location["line"] = "12"

	assert.Equal(t, []string{"type resources[].location.line: string"}, inventoryProblems(doc, inv))
}

func TestOverviewJSONContract_ModifiedAtShape(t *testing.T) {
	t.Parallel()

	doc := renderContractDoc(t, false, nil)

	assert.Equal(t, "2026-11-16T10:00:00Z", resourceByName(t, doc, "web")["modifiedAt"])
	assert.NotContains(t, resourceByName(t, doc, "logs"), "modifiedAt", "a preview-only create has none")
	assert.NotContains(t, resourceByName(t, doc, "db"), "modifiedAt", "a missing timestamp is omitted, not null")
}

func TestOverviewJSONContract_InventoryPinsOptionalModifiedAt(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)

	assert.Equal(t, "string", inv.Optional["resources[].modifiedAt"])
	assert.NotContains(t, inv.Required, "resources[].modifiedAt")
}

func TestOverviewJSONContract_AdditionsCarryOnlyRelativePathsAndDisplayStrings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	cmd, _ := overviewCmd(nil)
	cmd.SetContext(context.Background())
	var out bytes.Buffer
	cmd.SetOut(&out)

	rows := contractRows()
	result := engine.ComputeOverviewResult(rows, 15)
	stackCtx := engine.StackContext{StackName: "dev", TotalResources: len(rows), GeneratedAt: time.Now()}
	require.NoError(t, renderOverviewOutput(cmd, outputFormatJSON, result, stackCtx, nil))

	raw := out.String()
	assert.NotContains(t, raw, root)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	locations := 0
	for _, resource := range doc["resources"].([]any) {
		location, ok := resource.(map[string]any)["location"].(map[string]any)
		if !ok {
			continue
		}
		locations++
		file, _ := location["file"].(string)
		assert.False(t, filepath.IsAbs(file), "location %q must be project-relative", file)
		assert.False(t, strings.HasPrefix(file, "/"), file)
		assert.NotContains(t, file, "..")
		assert.NotContains(t, file, `\`)
	}
	assert.Equal(t, 2, locations)
}

func TestOverviewJSONContract_UnpricedRowsPrintThePinnedPlaceholder(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)
	unpriced, ok := inv.Placeholders["unpriced"]
	require.True(t, ok, "the inventory pins the unpriced placeholder (FR-012b)")
	require.Equal(t, "resources[].display.projected", unpriced.Path)
	assert.Equal(t, "string", inv.Required[unpriced.Path])

	doc := renderContractDoc(t, true, nil)

	display := func(name string) map[string]any {
		row, isMap := resourceByName(t, doc, name)["display"].(map[string]any)
		require.True(t, isMap)
		return row
	}
	assert.Equal(t, unpriced.Value, display("wait")["projected"], "a row with no projected cost")
	assert.NotEqual(t, unpriced.Value, display("web")["projected"], "a priced row prints a figure")

	// A failed price is explained: it prints its own marker and has an
	// errors[] entry, so it is outside the silent-gap rule.
	assert.NotEqual(t, unpriced.Value, display("db")["projected"], "a row whose pricing failed")
	errs, isSlice := doc["errors"].([]any)
	require.True(t, isSlice)
	var failedURNs []any
	for _, e := range errs {
		if entry, isMap := e.(map[string]any); isMap {
			failedURNs = append(failedURNs, entry["urn"])
		}
	}
	assert.Contains(t, failedURNs, "urn:pulumi:dev::shop::aws:rds/instance:Instance::db")
}

func TestOverviewJSONContract_CostIsNestedUnderProjectedCost(t *testing.T) {
	t.Parallel()

	inv := loadFieldInventory(t)
	assert.Equal(t, "object", inv.Optional["resources[].projectedCost"])
	assert.Equal(t, "number", inv.Optional["resources[].projectedCost.monthlyCost"])
	assert.NotContains(t, inv.Required, "resources[].projectedCost.monthlyCost", "an unpriced row has no projectedCost")

	doc := renderContractDoc(t, true, nil)
	web := resourceByName(t, doc, "web")

	projected, ok := web["projectedCost"].(map[string]any)
	require.True(t, ok, "a priced row carries projectedCost")
	assert.InDelta(t, 61.32, projected["monthlyCost"], 0.001)
	assert.NotContains(t, web, "monthlyCost", "there is no top-level monthlyCost (ISS-002)")
	assert.NotContains(t, resourceByName(t, doc, "wait"), "projectedCost", "an unpriced row omits it")
}
