package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// hookCost is a cost projected document for the fake finfocus.
const hookCostUnder = `{"finfocus":{"summary":{"totalMonthly":107.59,"currency":"USD"},` +
	`"diff":{"totalBefore":100,"totalAfter":107.59,"totalDelta":7.59,"currency":"USD","creates":1,"updates":0,"deletes":0},` +
	`"resources":[{"resourceType":"aws:ec2/instance:Instance","resourceId":"urn:pulumi:dev::app::aws:ec2/instance:Instance::web-server",` +
	`"monthly":7.59,"operation":"create","beforeMonthly":0,"deltaMonthly":7.59}]}}`

const hookCostOver = `{"finfocus":{"summary":{"totalMonthly":220,"currency":"USD"},` +
	`"diff":{"totalBefore":100,"totalAfter":220,"totalDelta":120,"currency":"USD","creates":2,"updates":0,"deletes":0},` +
	`"resources":[` +
	`{"resourceType":"aws:ec2/natGateway:NatGateway","resourceId":"urn:pulumi:dev::app::aws:ec2/natGateway:NatGateway::nat",` +
	`"monthly":32.85,"operation":"create","beforeMonthly":0,"deltaMonthly":32.85},` +
	`{"resourceType":"aws:rds/instance:Instance","resourceId":"urn:pulumi:dev::app::aws:rds/instance:Instance::db",` +
	`"monthly":87.15,"operation":"create","beforeMonthly":0,"deltaMonthly":87.15},` +
	`{"resourceType":"aws:s3/bucket:Bucket","resourceId":"urn:pulumi:dev::app::aws:s3/bucket:Bucket::logs",` +
	`"operation":"create","error":{"code":"NO_COST_DATA","message":"no price","resourceType":"aws:s3/bucket:Bucket"}}]}}`

type hookRepo struct {
	dir    string
	bin    string
	marker string
	script string
}

func costCheckHookScript(t *testing.T) string {
	t.Helper()
	return filepath.Join(costSkillDir(t, "cost-check"), "hooks", "cost-check-hook.sh")
}

// newHookRepo makes a git repository holding a Pulumi project and a bin
// directory with fake pulumi and finfocus. The fakes record that they ran.
func newHookRepo(t *testing.T) *hookRepo {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the cost-check hook is a POSIX shell script")
	}
	for _, tool := range []string{"git", "jq", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
	root := t.TempDir()
	repo := &hookRepo{
		dir:    filepath.Join(root, "project"),
		bin:    filepath.Join(root, "bin"),
		marker: filepath.Join(root, "calls"),
		script: costCheckHookScript(t),
	}
	require.NoError(t, os.MkdirAll(repo.dir, 0o750))
	require.NoError(t, os.MkdirAll(repo.bin, 0o750))
	repo.write(t, "Pulumi.yaml", "name: app\nruntime: nodejs\n")
	repo.write(t, "index.ts", "export const size = 1;\n")
	repo.write(t, "README.md", "# app\n")
	repo.git(t, "init", "-q", "-b", "main")
	repo.git(t, "add", ".")
	repo.git(t, "commit", "-q", "-m", "init")
	repo.fake(t, "pulumi", `echo pulumi >> "$HOOK_MARKER"; echo '{}'`)
	repo.fake(t, "finfocus",
		`echo finfocus >> "$HOOK_MARKER"; cat "$HOOK_COST"; printf '%s' "$HOOK_ERR" >&2; exit "${HOOK_EXIT:-0}"`)
	return repo
}

func (r *hookRepo) write(t *testing.T, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, name), []byte(body), 0o600))
}

func (r *hookRepo) fake(t *testing.T, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(r.bin, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
}

func (r *hookRepo) git(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.email=hook@test", "-c", "user.name=hook", "-c", "commit.gpgsign=false",
	}, args...)...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

type hookRun struct {
	out   string
	code  int
	calls string
	took  time.Duration
}

// run executes the hook with the fake tools first on PATH.
func (r *hookRepo) run(t *testing.T, trigger, stdin, cost string, env ...string) hookRun {
	t.Helper()
	costFile := filepath.Join(r.bin, "cost.json")
	require.NoError(t, os.WriteFile(costFile, []byte(cost), 0o600))
	_ = os.Remove(r.marker)
	cmd := exec.Command("sh", r.script, trigger)
	cmd.Dir = r.dir
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Env = append(os.Environ(),
		"PATH="+hookPath(t, r.bin),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"HOOK_MARKER="+r.marker,
		"HOOK_COST="+costFile,
	)
	cmd.Env = append(cmd.Env, env...)
	start := time.Now()
	out, err := cmd.CombinedOutput()
	took := time.Since(start)
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, string(out))
		code = exitErr.ExitCode()
	}
	calls, _ := os.ReadFile(r.marker)
	return hookRun{out: string(out), code: code, calls: string(calls), took: took}
}

// hookPath is the fakes plus only the directories holding git, jq, and the
// system tools, so a real finfocus or pulumi on the developer's PATH is never
// used and a missing fake really is missing.
func hookPath(t *testing.T, bin string) string {
	t.Helper()
	dirs := []string{bin}
	for _, tool := range []string{"git", "jq"} {
		path, err := exec.LookPath(tool)
		require.NoError(t, err)
		dirs = append(dirs, filepath.Dir(path))
	}
	return strings.Join(append(dirs, "/usr/bin", "/bin"), string(os.PathListSeparator))
}

func lines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_NoInfrastructureChangeDoesNothing(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "README.md", "# app\n\nmore docs\n")

	got := repo.run(t, "manual", "", hookCostOver)

	assert.Equal(t, 0, got.code)
	assert.Empty(t, got.out)
	assert.Empty(t, got.calls, "neither pulumi nor finfocus may run")
	assert.Less(t, got.took, time.Second, "the no-change path must stay cheap (11 ms measured on a real project)")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_UnderThresholdSummary(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 2;\n")

	got := repo.run(t, "manual", "", hookCostUnder)

	require.Equal(t, 0, got.code, got.out)
	assert.Equal(t, "pulumi\nfinfocus\n", got.calls)
	out := lines(got.out)
	assert.LessOrEqual(t, len(out), 5)
	assert.Equal(
		t,
		"FinFocus cost check: +7.59 USD/mo, plan total 107.59 USD/mo (1 create, 0 update, 0 delete)",
		out[0],
	)
	assert.Equal(t, "Status: OK", out[1])
	assert.Equal(t, "  +7.59 create web-server (aws:ec2/instance:Instance)", out[2])
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_OverThresholdReportsWithoutBlocking(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 3;\n")
	repo.git(t, "add", "index.ts")

	got := repo.run(t, "pre-commit", "", hookCostOver)

	assert.Equal(t, 0, got.code, got.out)
	out := lines(got.out)
	assert.LessOrEqual(t, len(out), 5)
	assert.Equal(t, "Status: over threshold: increase +120.00 > 50; increase 120% > 20%"+
		" (1 unpriced, so totals are a lower bound)", out[1])
	assert.Equal(t, "  +87.15 create db (aws:rds/instance:Instance)", out[2], "largest change first")
	assert.Equal(t, "  +32.85 create nat (aws:ec2/natGateway:NatGateway)", out[3])
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_BlockingConfigFailsGitHook(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, ".cost-check.yml", "thresholds:\n  total: 200  # monthly\nhook:\n  block: true\n")
	repo.write(t, "index.ts", "export const size = 4;\n")
	repo.git(t, "add", "index.ts")

	got := repo.run(t, "pre-commit", "", hookCostOver)

	assert.Equal(t, 1, got.code, got.out)
	assert.Contains(t, got.out, "plan total 220.00 > 200")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_ClaudeStopReportsOnceToTheUser(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 5;\n")

	first := repo.run(t, "claude-stop", `{"stop_hook_active":false}`, hookCostOver)
	require.Equal(t, 0, first.code, first.out)
	var answer map[string]string
	require.NoError(t, json.Unmarshal([]byte(first.out), &answer), first.out)
	assert.Contains(t, answer["systemMessage"], "Status: over threshold")
	assert.NotContains(t, answer, "decision", "blocking is off by default")

	again := repo.run(t, "claude-stop", `{"stop_hook_active":false}`, hookCostOver)
	assert.Equal(t, 0, again.code)
	assert.Empty(t, again.out, "an unchanged change is not checked twice")
	assert.Empty(t, again.calls)
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_ClaudeStopBlocksOnlyOnce(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, ".cost-check.yml", "hook:\n  block: true\n")
	repo.write(t, "index.ts", "export const size = 6;\n")

	got := repo.run(t, "claude-stop", `{"stop_hook_active": false}`, hookCostOver)
	require.Equal(t, 0, got.code, got.out)
	var answer map[string]string
	require.NoError(t, json.Unmarshal([]byte(got.out), &answer), got.out)
	assert.Equal(t, "block", answer["decision"])
	assert.Contains(t, answer["reason"], "increase +120.00 > 50")

	repo.write(t, "index.ts", "export const size = 7;\n")
	looping := repo.run(t, "claude-stop", `{"stop_hook_active": true}`, hookCostOver)
	require.NoError(t, json.Unmarshal([]byte(looping.out), &answer), looping.out)
	assert.NotContains(t, looping.out, `"decision"`, "a turn already continued by the hook is never blocked again")
	assert.Contains(t, answer["systemMessage"], "over threshold")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_ToolingProblemsNeverBlock(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, repo *hookRepo)
		env   []string
		want  string
	}{
		{
			name: "finfocus fails",
			env:  []string{"HOOK_EXIT=1", `HOOK_ERR={"error_code":"internal_error"}`},
			want: "finfocus cost projected exited 1",
		},
		{
			name: "preview fails",
			setup: func(t *testing.T, repo *hookRepo) {
				repo.fake(t, "pulumi", "exit 255")
			},
			want: "pulumi preview failed",
		},
		{
			name: "invalid threshold",
			setup: func(t *testing.T, repo *hookRepo) {
				repo.write(t, ".cost-check.yml", "thresholds:\n  total: lots\nhook:\n  block: true\n")
			},
			want: "thresholds.total in .cost-check.yml is not a non-negative number",
		},
	}
	for _, tc := range cases { //nolint:paralleltest // Writes fake tools and executes them (text file busy).
		t.Run(tc.name, func(t *testing.T) {
			repo := newHookRepo(t)
			if tc.setup != nil {
				tc.setup(t, repo)
			}
			repo.write(t, "index.ts", "export const size = 8;\n")

			got := repo.run(t, "manual", "", hookCostOver, tc.env...)

			assert.Equal(t, 0, got.code, got.out)
			assert.Len(t, lines(got.out), 1, got.out)
			assert.True(t, strings.HasPrefix(got.out, "FinFocus cost check skipped: "+tc.want), got.out)
		})
	}
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_BudgetExceededIsReported(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 9;\n")

	got := repo.run(t, "manual", "", hookCostUnder,
		"HOOK_EXIT=1", `HOOK_ERR={"error_code":"budget_exceeded"}`)

	assert.Equal(t, 0, got.code, got.out)
	assert.Contains(t, got.out, "Status: over threshold: budget exceeded")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_PrePushChecksPushedCommits(t *testing.T) {
	repo := newHookRepo(t)
	before := repo.head(t)
	repo.write(t, "index.ts", "export const size = 10;\n")
	repo.git(t, "commit", "-q", "-am", "grow")
	after := repo.head(t)

	got := repo.run(t, "pre-push", "refs/heads/main "+after+" refs/heads/main "+before+"\n", hookCostUnder)
	assert.Equal(t, 0, got.code, got.out)
	assert.Equal(t, "pulumi\nfinfocus\n", got.calls)

	repo.write(t, "README.md", "# app\n\nonly docs\n")
	repo.git(t, "commit", "-q", "-am", "docs")
	docs := repo.head(t)
	docsOnly := repo.run(t, "pre-push", "refs/heads/main "+docs+" refs/heads/main "+after+"\n", hookCostUnder)
	assert.Equal(t, 0, docsOnly.code)
	assert.Empty(t, docsOnly.calls, "a push that changes only docs checks nothing")
}

func (r *hookRepo) head(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = r.dir
	out, err := cmd.Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// The hook calls finfocus and reads its JSON. Pin both to the live CLI, as
// the skill text is pinned, so a renamed flag or field fails here.
func TestCostCheckHookUsesLiveFlagsAndFields(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(costCheckHookScript(t))
	require.NoError(t, err)
	script := string(body)

	projected := costSubcommand(t, "projected")
	invocation := regexp.MustCompile(`(?m)^\s*finfocus cost projected (--[^>]*)`)
	calls := invocation.FindAllStringSubmatch(script, -1)
	require.Len(t, calls, 2, "one call for Pulumi, one for Terraform")
	for _, call := range calls {
		for _, flag := range regexp.MustCompile(`--([a-z][a-z-]*)`).FindAllStringSubmatch(call[1], -1) {
			assert.Truef(t, lookupSkillFlag(projected, flag[1]), "cost projected has --%s", flag[1])
		}
	}

	contract := readCostSkill(t, "cost-check")
	pinned := map[string][]string{
		"DiffSummary": jsonTagNames(t, engine.DiffSummary{}),
		"CostResult":  jsonTagNames(t, engine.CostResult{}),
		"DiffEntry":   jsonTagNames(t, engine.DiffEntry{}),
	}
	for owner, fields := range map[string][]string{
		"DiffSummary": {"totalDelta", "totalBefore", "creates", "updates", "deletes"},
		"CostResult":  {"resourceType", "resourceId", "monthly", "currency", "error"},
		"DiffEntry":   {"operation", "deltaMonthly"},
	} {
		for _, field := range fields {
			assert.Containsf(t, script, field, "the hook reads %s", field)
			assert.Containsf(t, pinned[owner], field, "engine.%s has the JSON field %s", owner, field)
		}
	}
	for _, field := range []string{"totalMonthly", "emptyPlan", "complete", "errors", "budget_exceeded"} {
		assert.Containsf(t, script, field, "the hook reads %s", field)
		assert.Containsf(t, contract, field, "the cost-check contract documents %s", field)
	}
}

// jsonTagNames lists the JSON names of a struct's fields, including omitempty
// ones a marshalled zero value would leave out.
func jsonTagNames(t *testing.T, v any) []string {
	t.Helper()
	typ := reflect.TypeOf(v)
	require.Equal(t, reflect.Struct, typ.Kind())
	var names []string
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names = append(names, name)
		}
	}
	return names
}

const hookCostNothingPriced = `{"finfocus":{"emptyPlan":true,"summary":{"totalMonthly":0,"currency":"USD"},` +
	`"diff":{"totalBefore":0,"totalAfter":0,"totalDelta":0,"currency":"USD","creates":0,"updates":2,"deletes":0},` +
	`"stack":{"totalMonthly":173.61,"currency":"USD","resources":61,"complete":true},` +
	`"resources":[` +
	`{"resourceType":"tailscale:index/acl:Acl","resourceId":"urn:pulumi:dev::app::tailscale:index/acl:Acl::acl",` +
	`"operation":"update","error":{"code":"NO_COST_DATA","message":"no price","resourceType":"tailscale:index/acl:Acl"}},` +
	`{"resourceType":"grafana:index/folder:Folder","resourceId":"urn:pulumi:dev::app::grafana:index/folder:Folder::f",` +
	`"operation":"update","error":{"code":"NO_COST_DATA","message":"no price","resourceType":"grafana:index/folder:Folder"}}]}}`

// The old side of the update failed to price, so the diff counts it as 0 and
// reports a 60 increase that is not real.
const hookCostBaselineFailed = `{"finfocus":{"summary":{"totalMonthly":60,"currency":"USD"},` +
	`"diff":{"totalBefore":0,"totalAfter":60,"totalDelta":60,"currency":"USD","creates":0,"updates":1,"deletes":0},` +
	`"resources":[{"resourceType":"aws:ec2/instance:Instance","resourceId":"urn:pulumi:dev::app::aws:ec2/instance:Instance::web",` +
	`"monthly":60,"operation":"update","beforeMonthly":0,"deltaMonthly":60}],` +
	`"errors":[{"resourceType":"aws:ec2/instance:Instance","resourceId":"urn:pulumi:dev::app::aws:ec2/instance:Instance::web",` +
	`"pluginName":"aws-public","message":"baseline lookup failed"}]}}`

const hookCostSaving = `{"finfocus":{"summary":{"totalMonthly":87.5,"currency":"USD"},` +
	`"diff":{"totalBefore":100,"totalAfter":87.5,"totalDelta":-12.5,"currency":"USD","creates":0,"updates":0,"deletes":1},` +
	`"stack":{"totalMonthly":5000,"currency":"USD","resources":40,"complete":true},` +
	`"resources":[{"resourceType":"aws:s3/bucket:Bucket","resourceId":"urn:pulumi:dev::app::aws:s3/bucket:Bucket::old",` +
	`"monthly":0,"operation":"delete","beforeMonthly":12.5,"deltaMonthly":-12.5}]}}`

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_NothingPricedIsNotOK(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, ".cost-check.yml", "hook:\n  block: true\n")
	repo.write(t, "index.ts", "export const size = 11;\n")
	repo.git(t, "add", "index.ts")

	got := repo.run(t, "pre-commit", "", hookCostNothingPriced)

	assert.Equal(t, 0, got.code, "nothing priced is not a threshold breach: "+got.out)
	assert.Equal(t, []string{
		"FinFocus cost check: nothing in this change has a price (2 resource(s)), stack total 173.61 USD/mo",
		"Status: not priced (no installed plugin prices these resources; see finfocus plugin list)",
	}, lines(got.out))
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_FailedBaselineNeverRaisesAnIncrease(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, ".cost-check.yml", "hook:\n  block: true\n")
	repo.write(t, "index.ts", "export const size = 12;\n")
	repo.git(t, "add", "index.ts")

	got := repo.run(t, "pre-commit", "", hookCostBaselineFailed)

	assert.Equal(t, 0, got.code, "a pricing failure must never block: "+got.out)
	assert.Equal(t, []string{
		"FinFocus cost check: change unavailable, plan total 60.00 USD/mo (0 create, 1 update, 0 delete)",
		"Status: OK (increase unavailable: the current cost of 1 resource(s) failed to price)",
	}, lines(got.out))
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_SavingAndStackTotal(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 13;\n")

	got := repo.run(t, "manual", "", hookCostSaving)

	require.Equal(t, 0, got.code, got.out)
	assert.Equal(t, []string{
		"FinFocus cost check: -12.50 USD/mo, stack total 5000.00 USD/mo (0 create, 0 update, 1 delete)",
		"Status: OK",
		"  -12.50 delete old (aws:s3/bucket:Bucket)",
	}, lines(got.out))
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_OtherCurrencySkipsThresholds(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, ".cost-check.yml", "currency: EUR\nhook:\n  block: true\n")
	repo.write(t, "index.ts", "export const size = 14;\n")

	got := repo.run(t, "manual", "", hookCostOver)

	assert.Equal(t, 0, got.code, got.out)
	assert.Equal(t, "Status: OK (thresholds not applied: costs are in USD, .cost-check.yml uses EUR)"+
		" (1 unpriced, so totals are a lower bound)", lines(got.out)[1])
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_PathWithSpaces(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "my stack.ts", "export const extra = 1;\n")

	got := repo.run(t, "manual", "", hookCostUnder)

	assert.Equal(t, "pulumi\nfinfocus\n", got.calls, got.out)
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_NewBranchPushChecksEveryUnpushedCommit(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "README.md", "# app\n\ndocs\n")
	repo.git(t, "commit", "-q", "-am", "docs")
	tip := repo.head(t)
	zero := strings.Repeat("0", 40)

	got := repo.run(t, "pre-push", "refs/heads/feature "+tip+" refs/heads/feature "+zero+"\n", hookCostUnder)

	assert.Equal(t, "pulumi\nfinfocus\n", got.calls,
		"the root commit added index.ts and no remote has it, so the push is checked: "+got.out)
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_MissingFinfocusNeverBlocks(t *testing.T) {
	repo := newHookRepo(t)
	require.NoError(t, os.Remove(filepath.Join(repo.bin, "finfocus")))
	repo.write(t, "index.ts", "export const size = 15;\n")

	got := repo.run(t, "manual", "", hookCostOver)

	assert.Equal(t, 0, got.code)
	assert.Equal(t, "FinFocus cost check skipped: finfocus is not installed or not on PATH\n", got.out)
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_Lock(t *testing.T) {
	repo := newHookRepo(t)
	repo.write(t, "index.ts", "export const size = 16;\n")
	lock := filepath.Join(repo.dir, ".git", "finfocus-cost-hook.lock")
	require.NoError(t, os.MkdirAll(lock, 0o750))
	pid := filepath.Join(lock, "pid")

	require.NoError(t, os.WriteFile(pid, []byte(strconv.Itoa(os.Getpid())), 0o600))
	busy := repo.run(t, "manual", "", hookCostUnder)
	assert.Equal(t, "FinFocus cost check skipped: another cost check is running\n", busy.out)
	assert.Empty(t, busy.calls)

	require.NoError(t, os.WriteFile(pid, []byte("2147483646"), 0o600))
	reclaimed := repo.run(t, "manual", "", hookCostUnder)
	assert.Equal(t, "pulumi\nfinfocus\n", reclaimed.calls, "a dead owner's lock is reclaimed: "+reclaimed.out)
	assert.NoDirExists(t, lock, "the lock is released after the run")
}

//nolint:paralleltest // Writes fake tools and executes them (text file busy).
func TestCostCheckHook_TerraformNeedsAFreshPlan(t *testing.T) {
	repo := newHookRepo(t)
	repo.git(t, "rm", "-q", "Pulumi.yaml", "index.ts")
	repo.write(t, "main.tf", "resource \"aws_instance\" \"web\" {}\n")
	repo.write(t, ".cost-check.yml", "hook:\n  terraform_plan: plan.json\n")
	repo.write(t, "plan.json", "{}\n")
	repo.git(t, "add", "main.tf", ".cost-check.yml")
	repo.git(t, "commit", "-q", "-m", "terraform")
	repo.write(t, "main.tf", "resource \"aws_instance\" \"web\" { instance_type = \"m5.large\" }\n")
	now := time.Now()
	require.NoError(t, os.Chtimes(filepath.Join(repo.dir, "plan.json"), now.Add(-time.Hour), now.Add(-time.Hour)))

	stale := repo.run(t, "manual", "", hookCostUnder)
	assert.Contains(t, stale.out, "is older than your .tf changes")
	assert.Empty(t, stale.calls)

	require.NoError(t, os.Chtimes(filepath.Join(repo.dir, "plan.json"), now.Add(time.Minute), now.Add(time.Minute)))
	fresh := repo.run(t, "manual", "", hookCostUnder)
	assert.Equal(t, "finfocus\n", fresh.calls, "Terraform never runs pulumi: "+fresh.out)
	assert.Contains(t, fresh.out, "Status: OK")
}
