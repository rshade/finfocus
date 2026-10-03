package cli

import (
	"context"

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
