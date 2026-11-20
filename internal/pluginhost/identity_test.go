package pluginhost_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestClientMatchesName(t *testing.T) {
	t.Parallel()

	renamed := &pluginhost.Client{
		Name:        "finfocus-plugin-aws-public",
		InstallName: "aws-public",
		Aliases:     []string{"aws-public-legacy"},
	}
	unregistered := &pluginhost.Client{Name: "recorder"}

	tests := []struct {
		name   string
		client *pluginhost.Client
		input  string
		want   bool
	}{
		{"install name", renamed, "aws-public", true},
		{"alias", renamed, "aws-public-legacy", true},
		{"reported name", renamed, "finfocus-plugin-aws-public", true},
		{"other name", renamed, "aws-ce", false},
		{"empty never matches", renamed, "", false},
		{"no install name matches reported", unregistered, "recorder", true},
		{"empty never matches empty install name", unregistered, "", false},
		{"nil client", nil, "aws-public", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, tc.client.MatchesName(tc.input))
		})
	}
}

func TestResolveName(t *testing.T) {
	t.Parallel()

	awsPublic := &pluginhost.Client{
		Name:        "finfocus-plugin-aws-public",
		InstallName: "aws-public",
		Aliases:     []string{"finfocus-plugin-aws-public"},
	}
	renamed := &pluginhost.Client{
		Name:        "aws-public",
		InstallName: "aws-public",
		Aliases:     []string{"finfocus-plugin-aws-public"},
	}
	impostor := &pluginhost.Client{Name: "aws-public", InstallName: "custom"}
	aliasOwner := &pluginhost.Client{Name: "x-reported", InstallName: "x", Aliases: []string{"shared"}}
	reportsShared := &pluginhost.Client{Name: "shared", InstallName: "y"}
	twinA := &pluginhost.Client{Name: "twin", InstallName: "a"}
	twinB := &pluginhost.Client{Name: "twin", InstallName: "b"}

	tests := []struct {
		name          string
		clients       []*pluginhost.Client
		input         string
		want          *pluginhost.Client
		wantKind      pluginhost.MatchKind
		wantAmbiguous bool
	}{
		{"install name", []*pluginhost.Client{awsPublic}, "aws-public", awsPublic, pluginhost.MatchInstall, false},
		{
			"alias before reported name on the same client",
			[]*pluginhost.Client{awsPublic}, "finfocus-plugin-aws-public",
			awsPublic, pluginhost.MatchAlias, false,
		},
		{
			"alias after a rename",
			[]*pluginhost.Client{renamed}, "finfocus-plugin-aws-public",
			renamed, pluginhost.MatchAlias, false,
		},
		{
			"install name wins over another client's reported name",
			[]*pluginhost.Client{impostor, renamed}, "aws-public",
			renamed, pluginhost.MatchInstall, true,
		},
		{
			"alias wins over another client's reported name",
			[]*pluginhost.Client{reportsShared, aliasOwner}, "shared",
			aliasOwner, pluginhost.MatchAlias, true,
		},
		{"reported name", []*pluginhost.Client{impostor}, "aws-public", impostor, pluginhost.MatchReported, false},
		{
			"same reported name twice is not ambiguous",
			[]*pluginhost.Client{twinA, twinB}, "twin",
			twinA, pluginhost.MatchReported, false,
		},
		{"no match", []*pluginhost.Client{awsPublic}, "aws-publik", nil, pluginhost.MatchNone, false},
		{"empty name", []*pluginhost.Client{awsPublic}, "", nil, pluginhost.MatchNone, false},
		{"nil slice", nil, "aws-public", nil, pluginhost.MatchNone, false},
		{
			"nil entries are skipped",
			[]*pluginhost.Client{nil, awsPublic},
			"aws-public",
			awsPublic,
			pluginhost.MatchInstall,
			false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, kind, ambiguous := pluginhost.ResolveName(tc.clients, tc.input)
			assert.Same(t, tc.want, got)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantAmbiguous, ambiguous)
		})
	}
}

func TestMatchKindString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "none", pluginhost.MatchNone.String())
	assert.Equal(t, "install", pluginhost.MatchInstall.String())
	assert.Equal(t, "alias", pluginhost.MatchAlias.String())
	assert.Equal(t, "reported", pluginhost.MatchReported.String())
	assert.Equal(t, "unknown", pluginhost.MatchKind(99).String())
}
