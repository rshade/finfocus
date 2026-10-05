package cli

import (
	"context"
	"net/http"

	"github.com/rshade/finfocus/internal/pluginskill"
)

// SkillInstallerFunc adapts a function to the plugin skill installer.
type SkillInstallerFunc func(ctx context.Context, dir, version string) pluginskill.Result

// Install calls f.
func (f SkillInstallerFunc) Install(ctx context.Context, dir, version string) pluginskill.Result {
	return f(ctx, dir, version)
}

// SetPluginSkillsForTest replaces the plugin skill installer. Call it only
// from TestMain, before any test runs.
func SetPluginSkillsForTest(f SkillInstallerFunc) {
	pluginSkills = f
}

// IsNoAssetError exposes isNoAssetError to tests.
func IsNoAssetError(err error) bool { return isNoAssetError(err) }

// SetNotificationClientForTest routes budget notifications through client and
// returns a function that restores the previous client.
func SetNotificationClientForTest(client *http.Client) func() {
	prev := notificationClient
	notificationClient = client
	return func() { notificationClient = prev }
}
