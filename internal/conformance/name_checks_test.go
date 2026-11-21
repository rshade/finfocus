package conformance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// namedClient answers only Name and GetPluginInfo; any other call panics on
// the nil embedded interface, which the name checks never make.
type namedClient struct {
	pbc.CostSourceServiceClient

	name     string
	infoName string
	infoErr  error
}

func (c namedClient) Name(context.Context, *pbc.NameRequest, ...grpc.CallOption) (*pbc.NameResponse, error) {
	return &pbc.NameResponse{Name: c.name}, nil
}

func (c namedClient) GetPluginInfo(
	context.Context, *pbc.GetPluginInfoRequest, ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	if c.infoErr != nil {
		return nil, c.infoErr
	}
	return &pbc.GetPluginInfoResponse{Name: c.infoName}, nil
}

func writeBinary(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(
		t,
		os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755),
	)
	return path
}

func TestResolveExpectedName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	layout := writeBinary(t, filepath.Join(root, "plugins", "aws-public", "v0.2.5", "finfocus-plugin-aws-public"))
	bareVersion := writeBinary(t, filepath.Join(root, "plugins", "aws-ce", "0.3.0", "finfocus-plugin-aws-ce"))
	regionDir := filepath.Join(root, "build")
	region := writeBinary(t, filepath.Join(regionDir, "finfocus-plugin-aws-public-us-east-1"))
	require.NoError(t, os.WriteFile(filepath.Join(regionDir, "plugin.metadata.json"),
		[]byte(`{"region":"us-east-1"}`), 0o600))
	plain := writeBinary(t, filepath.Join(root, "bin", "my-plugin"))

	tests := []struct {
		name       string
		path       string
		explicit   string
		want       string
		wantSource string
	}{
		{"explicit name wins", layout, "custom", "custom", IdentitySourceFlag},
		{"install layout", layout, "", "aws-public", IdentitySourceInstallLayout},
		{"version directory without v", bareVersion, "", "aws-ce", IdentitySourceInstallLayout},
		{"binary name with region metadata", region, "", "aws-public", IdentitySourceBinaryName},
		{"unresolved", plain, "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveExpectedName(tc.path, tc.explicit)
			assert.Equal(t, tc.want, got.Expected)
			assert.Equal(t, tc.wantSource, got.Source)
			if tc.want == "" {
				assert.Contains(t, got.Reason, "--expected-name")
			} else {
				assert.Empty(t, got.Reason)
			}
		})
	}
}

func nameCheckContext(client namedClient, identity PluginIdentity) *TestContext {
	return &TestContext{PluginClient: client, Timeout: 5 * time.Second, Identity: identity}
}

func TestNameMatchesInstallName(t *testing.T) {
	t.Parallel()

	resolved := PluginIdentity{Expected: "aws-public", Source: IdentitySourceInstallLayout}
	tests := []struct {
		name        string
		client      namedClient
		identity    PluginIdentity
		wantStatus  Status
		wantMessage string
	}{
		{"matches", namedClient{name: "aws-public"}, resolved, StatusPass, ""},
		{
			"differs", namedClient{name: "finfocus-plugin-aws-public"}, resolved, StatusWarn,
			`plugin reports "finfocus-plugin-aws-public"; its install name is "aws-public" (from install-layout)`,
		},
		{
			"unresolved", namedClient{name: "aws-public"},
			PluginIdentity{Reason: "pass --expected-name to check it"}, StatusWarn,
			"install name not checked: pass --expected-name to check it",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := testNameMatchesInstallName(nameCheckContext(tc.client, tc.identity))
			assert.Equal(t, tc.wantStatus, result.Status)
			assert.Equal(t, tc.wantMessage, result.Error)
		})
	}
}

func TestNameMatchesPluginInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		client      namedClient
		wantStatus  Status
		wantMessage string
	}{
		{"matches", namedClient{name: "aws-public", infoName: "aws-public"}, StatusPass, ""},
		{
			"differs", namedClient{name: "aws-public", infoName: "finfocus-plugin-aws-public"}, StatusWarn,
			`Name() reports "aws-public" but GetPluginInfo reports "finfocus-plugin-aws-public"`,
		},
		{
			"not implemented", namedClient{name: "aws-public", infoErr: status.Error(codes.Unimplemented, "no")},
			StatusWarn, "GetPluginInfo is not implemented, so its name is not checked",
		},
		{
			"call fails", namedClient{name: "aws-public", infoErr: status.Error(codes.Internal, "boom")},
			StatusWarn, "GetPluginInfo failed, so its name is not checked: rpc error: code = Internal desc = boom",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := testNameMatchesPluginInfo(nameCheckContext(tc.client, PluginIdentity{}))
			assert.Equal(t, tc.wantStatus, result.Status)
			assert.Equal(t, tc.wantMessage, result.Error)
		})
	}
}

func TestNameIsCanonicalForm(t *testing.T) {
	t.Parallel()

	pass := testNameIsCanonicalForm(nameCheckContext(namedClient{name: "aws-public"}, PluginIdentity{}))
	assert.Equal(t, StatusPass, pass.Status)

	warn := testNameIsCanonicalForm(nameCheckContext(namedClient{name: "finfocus-plugin-aws-public"}, PluginIdentity{}))
	assert.Equal(t, StatusWarn, warn.Status)
	assert.Contains(t, warn.Error, `must not start with "finfocus-plugin-"`)
}

func TestSuiteRegistersNameChecks(t *testing.T) {
	t.Parallel()

	suite, err := NewSuite(SuiteConfig{PluginPath: "/plugins/aws-public/v0.2.5/finfocus-plugin-aws-public"})
	require.NoError(t, err)
	byName := map[string]TestCase{}
	for _, tc := range suite.GetTestCases() {
		byName[tc.Name] = tc
	}
	for _, name := range []string{"Name_MatchesInstallName", "Name_MatchesPluginInfo", "Name_IsCanonicalForm"} {
		tc, ok := byName[name]
		require.True(t, ok, name)
		assert.Equal(t, CategoryProtocol, tc.Category)
	}
}

// stalledInfoClient answers Name and never answers GetPluginInfo until the
// call's context ends.
type stalledInfoClient struct {
	namedClient
}

func (stalledInfoClient) GetPluginInfo(
	ctx context.Context, _ *pbc.GetPluginInfoRequest, _ ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	<-ctx.Done()
	return nil, status.FromContextError(ctx.Err()).Err()
}

func TestNameMatchesPluginInfo_StalledCallWarnsWithinTheTestDeadline(t *testing.T) {
	t.Parallel()

	tc := TestCase{
		Name: "Name_MatchesPluginInfo", Category: CategoryProtocol,
		Timeout: 200 * time.Millisecond, TestFunc: testNameMatchesPluginInfo,
	}
	client := stalledInfoClient{namedClient{name: "aws-public"}}
	result := NewRunner(zerolog.Nop(), VerbosityQuiet).RunTest(context.Background(), tc, client)

	assert.Equal(t, StatusWarn, result.Status, "an advisory check must not time out into an error: %s", result.Error)
	assert.Contains(t, result.Error, "GetPluginInfo failed, so its name is not checked")
}
