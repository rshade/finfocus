package cli

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/esc"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

func rootESCSource(warn func(string)) *esc.Source {
	global := []string{"global/shared"}
	return esc.NewSource(esc.Options{
		Global:    &global,
		PerPlugin: map[string]esc.PluginOptions{"aws-public": {Variables: []string{"AWS_REGION"}}},
		UseStack:  true,
		Budget:    7 * time.Second,
		Warn:      warn,
	})
}

func TestStackESCSource_EachStackResolvesItsOwnEnvironments(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{
		"dev":       "environment:\n  - proj/a\n  - proj/b\n  - proj/c\n  - proj/d\n  - proj/e\n  - proj/f\n",
		"tailscale": "environment:\n  - proj/x\n  - proj/y\n  - proj/z\n  - proj/p\n  - proj/q\n",
	})
	var warnings []string
	root := rootESCSource(func(text string) { warnings = append(warnings, text) })

	dev := stackESCSource(context.Background(), root, project, "dev")
	tailscale := stackESCSource(context.Background(), root, project, "tailscale")

	assert.Equal(t, "dev", dev.Options().Stack.Stack)
	assert.Equal(t, []string{"proj/a", "proj/b", "proj/c", "proj/d", "proj/e", "proj/f"},
		dev.Options().Stack.Environment.Imports)
	assert.Equal(t, "tailscale", tailscale.Options().Stack.Stack)
	assert.Equal(t, []string{"proj/x", "proj/y", "proj/z", "proj/p", "proj/q"},
		tailscale.Options().Stack.Environment.Imports)
	assert.NotSame(t, dev, tailscale)
	assert.NotSame(t, root, dev)
	assert.Empty(t, warnings)
}

func TestStackESCSource_KeepsSharedOptions(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{"dev": "environment:\n  - proj/a\n"})
	root := rootESCSource(nil)

	got := stackESCSource(context.Background(), root, project, "dev").Options()

	require.NotNil(t, got.Global)
	assert.Equal(t, []string{"global/shared"}, *got.Global)
	assert.Equal(t, root.Options().PerPlugin, got.PerPlugin)
	assert.Equal(t, 7*time.Second, got.Budget)
	assert.True(t, got.UseStack)
}

func TestStackESCSource_AbsentSettingsGiveNoStackEnvironment(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{"plain": "config:\n  demo:key: value\n"})
	root := rootESCSource(nil)

	missing := stackESCSource(context.Background(), root, project, "nofile")
	noKey := stackESCSource(context.Background(), root, project, "plain")

	assert.Equal(t, pulumidetect.StackEnvAbsent, missing.Options().Stack.Environment.Form)
	assert.Empty(t, missing.Options().Stack.Environment.Imports)
	assert.Equal(t, pulumidetect.StackEnvAbsent, noKey.Options().Stack.Environment.Form)
	assert.Equal(t, []string{"global/shared"}, *noKey.Options().Global)
}

func TestStackESCSource_EmptyStackKeepsRoot(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{"dev": "environment:\n  - proj/a\n"})
	root := rootESCSource(nil)

	assert.Same(t, root, stackESCSource(context.Background(), root, project, ""))
	assert.Same(t, root, stackESCSource(context.Background(), root, project, "  "))
	assert.Nil(t, stackESCSource(context.Background(), nil, project, "dev"))
}

func TestStackESCSource_ListValueIsNotAStackName(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{
		"dev":       "environment:\n  - proj/dev\n",
		"tailscale": "environment:\n  - proj/ts\n",
	})
	root := rootESCSource(nil)

	list := stackESCSource(context.Background(), root, project, "dev,tailscale")

	assert.Empty(t, list.Options().Stack.Environment.Imports,
		"a raw list value names no settings file and must not borrow a member's environments")
}

func TestStackESCSource_BadSettingsDoNotAffectAnotherStack(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{
		"bad":  "environment: [unterminated\n",
		"good": "environment:\n  - proj/ok\n",
	})
	root := rootESCSource(nil)

	bad := stackESCSource(context.Background(), root, project, "bad")
	good := stackESCSource(context.Background(), root, project, "good")

	assert.Empty(t, bad.Options().Stack.Environment.Imports)
	assert.Equal(t, []string{"proj/ok"}, good.Options().Stack.Environment.Imports)
}

func TestStackESCSource_FreshOnOneStackLeavesOthers(t *testing.T) {
	t.Parallel()
	project := escProject(t, map[string]string{
		"dev":  "environment:\n  - proj/dev\n",
		"prod": "environment:\n  - proj/prod\n",
	})
	root := rootESCSource(nil)
	dev := stackESCSource(context.Background(), root, project, "dev")
	prod := stackESCSource(context.Background(), root, project, "prod")

	refreshed := dev.Fresh()

	assert.NotSame(t, dev, refreshed)
	assert.Equal(t, []string{"proj/dev"}, refreshed.Options().Stack.Environment.Imports)
	assert.Equal(t, []string{"proj/prod"}, prod.Options().Stack.Environment.Imports)
}

func webDataWithRootESC(t *testing.T, stack string) (*webData, context.Context) {
	t.Helper()
	root := rootESCSource(nil)
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(esc.WithSource(context.Background(), root))
	return newWebData(cmd, overviewParams{stack: stack, cfg: config.New()}, engine.DateRange{}, nil), cmd.Context()
}

// Not parallel: t.Chdir changes the process working directory.
func TestWebData_BindStackESCScopesEachStack(t *testing.T) {
	project := escProject(t, map[string]string{
		"dev":       "environment:\n  - proj/dev\n",
		"tailscale": "environment:\n  - proj/ts\n",
	})
	t.Chdir(project)
	dev, devCtx := webDataWithRootESC(t, "dev")
	tailscale, tailscaleCtx := webDataWithRootESC(t, "tailscale")

	devCtx = dev.bindStackESC(devCtx)
	tailscaleCtx = tailscale.bindStackESC(tailscaleCtx)

	assert.Equal(t, []string{"proj/dev"}, dev.ESCSource().Options().Stack.Environment.Imports)
	assert.Equal(t, []string{"proj/ts"}, tailscale.ESCSource().Options().Stack.Environment.Imports)
	fromDevCtx, ok := esc.SourceFromContext(devCtx)
	require.True(t, ok)
	assert.Same(t, dev.ESCSource(), fromDevCtx)
	fromTailscaleCtx, ok := esc.SourceFromContext(tailscaleCtx)
	require.True(t, ok)
	assert.Same(t, tailscale.ESCSource(), fromTailscaleCtx)
	refreshed := dev.freshESC(devCtx)
	freshSource, ok := esc.SourceFromContext(refreshed)
	require.True(t, ok)
	assert.Equal(t, []string{"proj/dev"}, freshSource.Options().Stack.Environment.Imports,
		"a reload keeps the stack's own environments")
}

// Not parallel: t.Chdir changes the process working directory.
func TestWebData_BindStackESCWithoutStackKeepsRoot(t *testing.T) {
	t.Chdir(escProject(t, map[string]string{"dev": "environment:\n  - proj/dev\n"}))
	data, ctx := webDataWithRootESC(t, "")
	before := data.ESCSource()

	after := data.bindStackESC(ctx)

	assert.Same(t, before, data.ESCSource())
	fromCtx, ok := esc.SourceFromContext(after)
	require.True(t, ok)
	assert.Same(t, before, fromCtx)
}

// A single-stack dashboard pins the workspace stack it named (#2088) by its
// `pulumi stack ls` name, which can drop the organization the workspace
// selection carries. The root source already read that selection, so ESC keeps
// it and resolves the stack's environments in the right organization.
// Not parallel: t.Chdir changes the process working directory.
func TestWebData_BindStackESCKeepsRootForAPinnedWorkspaceStack(t *testing.T) {
	t.Chdir(escProject(t, map[string]string{"dev": "environment:\n  - proj/dev\n"}))
	data, ctx := webDataWithRootESC(t, "dev")
	data.params.workspaceStack = true
	before := data.ESCSource()

	after := data.bindStackESC(ctx)

	assert.Same(t, before, data.ESCSource())
	fromCtx, ok := esc.SourceFromContext(after)
	require.True(t, ok)
	assert.Same(t, before, fromCtx)
}

// Not parallel: t.Chdir changes the process working directory.
func TestWebData_BindStackESCOutsideProjectKeepsRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	data, ctx := webDataWithRootESC(t, "dev")
	before := data.ESCSource()

	data.bindStackESC(ctx)

	assert.Same(t, before, data.ESCSource())
}

// Not parallel: t.Chdir changes the process working directory.
func TestWebData_BindStackESCMatchesTheFlagBuiltSourceForOneStack(t *testing.T) {
	project := escProject(t, map[string]string{"dev": "environment:\n  - proj/dev\n  - proj/shared\n"})
	t.Chdir(project)
	flagStack, err := pulumidetect.LoadStackContext(project, "dev")
	require.NoError(t, err)
	global := []string{"global/shared"}
	root := esc.NewSource(esc.Options{Global: &global, Stack: flagStack, UseStack: true})
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(esc.WithSource(context.Background(), root))
	data := newWebData(cmd, overviewParams{stack: "dev", cfg: config.New()}, engine.DateRange{}, nil)

	data.bindStackESC(cmd.Context())

	scoped := data.ESCSource().Options()
	assert.Equal(t, root.Options().Stack, scoped.Stack,
		"a single named stack resolves exactly as the flag-built source did")
	assert.Equal(t, root.Options().Global, scoped.Global)
	assert.Equal(t, root.Options().UseStack, scoped.UseStack)
}
