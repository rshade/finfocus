package conformance

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginname"
)

// The name checks are advisory: each one passes or warns and never fails the
// run, because core accepts a plugin under its install name, a catalog alias,
// or its reported name. A check that cannot run warns with what is missing,
// since a skipped test would block certification.

// nameCheckDeadline bounds every RPC of one name check by a single deadline,
// three quarters of the runner's test timeout. The runner reports a test that
// outlives its timeout as an error, so the RPCs must give up first for a
// stalled plugin to end as a warning.
func nameCheckDeadline(ctx *TestContext) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ctx.Timeout*nameCheckBudgetNum/nameCheckBudgetDen)
}

// The name checks' RPCs share three quarters of the runner's test timeout.
const (
	nameCheckBudgetNum = 3
	nameCheckBudgetDen = 4
)

// reportedName calls the Name RPC. On failure it returns a warning result for
// check, explaining that the check did not run.
func reportedName(
	rpcCtx context.Context, ctx *TestContext, check string,
) (pbc.CostSourceServiceClient, string, *TestResult) {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return nil, "", &TestResult{Status: StatusError, Error: errInvalidPluginClientType}
	}
	resp, err := client.Name(rpcCtx, &pbc.NameRequest{})
	if err != nil {
		return nil, "", &TestResult{
			Status: StatusWarn,
			Error:  fmt.Sprintf("Name() failed, so %s is not checked: %v", check, err),
		}
	}
	return client, resp.GetName(), nil
}

// testNameMatchesInstallName warns when the plugin reports a name other than
// the install name it is expected to have (SuiteConfig.ExpectedName, the
// install layout, or the binary name).
func testNameMatchesInstallName(ctx *TestContext) *TestResult {
	rpcCtx, cancel := nameCheckDeadline(ctx)
	defer cancel()
	_, name, failed := reportedName(rpcCtx, ctx, "the install name")
	if failed != nil {
		return failed
	}
	identity := ctx.Identity
	switch {
	case identity.Expected == "":
		return &TestResult{Status: StatusWarn, Error: "install name not checked: " + identity.Reason}
	case name != identity.Expected:
		return &TestResult{Status: StatusWarn, Error: fmt.Sprintf(
			"plugin reports %q; its install name is %q (from %s)", name, identity.Expected, identity.Source)}
	default:
		return &TestResult{Status: StatusPass, Details: fmt.Sprintf(
			"Plugin reports its install name %q (from %s)", name, identity.Source)}
	}
}

// testNameMatchesPluginInfo warns when the Name RPC and GetPluginInfo report
// different names.
func testNameMatchesPluginInfo(ctx *TestContext) *TestResult {
	rpcCtx, cancel := nameCheckDeadline(ctx)
	defer cancel()
	client, name, failed := reportedName(rpcCtx, ctx, "the plugin info name")
	if failed != nil {
		return failed
	}
	info, err := client.GetPluginInfo(rpcCtx, &pbc.GetPluginInfoRequest{})
	switch {
	case status.Code(err) == codes.Unimplemented:
		return &TestResult{Status: StatusWarn, Error: "GetPluginInfo is not implemented, so its name is not checked"}
	case err != nil:
		return &TestResult{
			Status: StatusWarn,
			Error:  fmt.Sprintf("GetPluginInfo failed, so its name is not checked: %v", err),
		}
	case info.GetName() != name:
		return &TestResult{Status: StatusWarn, Error: fmt.Sprintf(
			"Name() reports %q but GetPluginInfo reports %q", name, info.GetName())}
	default:
		return &TestResult{Status: StatusPass, Details: fmt.Sprintf("Name() and GetPluginInfo both report %q", name)}
	}
}

// testNameIsCanonicalForm warns when the reported name is not canonical:
// lowercase letters and digits in hyphen-separated groups, without the
// finfocus-plugin- binary prefix.
func testNameIsCanonicalForm(ctx *TestContext) *TestResult {
	rpcCtx, cancel := nameCheckDeadline(ctx)
	defer cancel()
	_, name, failed := reportedName(rpcCtx, ctx, "the name form")
	if failed != nil {
		return failed
	}
	if err := pluginname.Canonical(name); err != nil {
		return &TestResult{Status: StatusWarn, Error: err.Error()}
	}
	return &TestResult{Status: StatusPass, Details: fmt.Sprintf("Plugin name %q is canonical", name)}
}
