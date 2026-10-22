package registry

import (
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awsCEReleaseV010() *GitHubRelease {
	names := []string{
		"checksums.txt",
		"finfocus-plugin-aws-ce_0.1.0_Darwin_arm64.tar.gz",
		"finfocus-plugin-aws-ce_0.1.0_Darwin_x86_64.tar.gz",
		"finfocus-plugin-aws-ce_0.1.0_Linux_arm64.tar.gz",
		"finfocus-plugin-aws-ce_0.1.0_Linux_x86_64.tar.gz",
		"finfocus-plugin-aws-ce_0.1.0_Windows_arm64.zip",
		"finfocus-plugin-aws-ce_0.1.0_Windows_x86_64.zip",
	}
	release := &GitHubRelease{TagName: "v0.1.0"}
	for _, name := range names {
		release.Assets = append(release.Assets, ReleaseAsset{Name: name})
	}
	return release
}

func expectedAWSCEAsset() string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	osName := strings.ToUpper(runtime.GOOS[:1]) + runtime.GOOS[1:]
	return "finfocus-plugin-aws-ce_0.1.0_" + osName + "_" + arch + ext
}

func TestFindPlatformAsset_RepositoryPrefixedAssets(t *testing.T) {
	t.Parallel()
	release := awsCEReleaseV010()

	_, err := FindPlatformAssetWithHints(release, "aws-ce", nil)
	require.Error(t, err, "the short plugin name alone cannot match repository-prefixed assets")

	asset, err := FindPlatformAssetWithHints(release, "aws-ce", hintsForRepository("finfocus-plugin-aws-ce"))
	require.NoError(t, err)
	assert.Equal(t, expectedAWSCEAsset(), asset.Name)
}

func TestHintsForRepository(t *testing.T) {
	t.Parallel()

	assert.Nil(t, hintsForRepository(""))
	hints := hintsForRepository("finfocus-plugin-aws-ce")
	require.NotNil(t, hints)
	assert.Equal(t, "finfocus-plugin-aws-ce", hints.AssetPrefix)
}

func TestResolvePluginSource_URLPluginGetsRepositoryPrefix(t *testing.T) {
	t.Parallel()
	installer := NewInstallerWithClient(&GitHubClient{}, t.TempDir())

	owner, repo, hints, err := installer.resolvePluginSource(
		"not-in-catalog",
		"github.com/rshade/finfocus-plugin-aws-ce",
	)
	require.NoError(t, err)
	assert.Equal(t, "rshade", owner)
	assert.Equal(t, "finfocus-plugin-aws-ce", repo)
	require.NotNil(t, hints)
	assert.Equal(t, "finfocus-plugin-aws-ce", hints.AssetPrefix)
}
