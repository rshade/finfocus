package conformance

import (
	"fmt"
	"path/filepath"

	"github.com/Masterminds/semver/v3"

	"github.com/rshade/finfocus/internal/pluginname"
	"github.com/rshade/finfocus/internal/registry"
)

// Sources of an expected plugin name, in the order ResolveExpectedName tries them.
const (
	// IdentitySourceFlag means the name came from --expected-name.
	IdentitySourceFlag = "flag"
	// IdentitySourceInstallLayout means the binary sits in <plugins>/<name>/<version>/.
	IdentitySourceInstallLayout = "install-layout"
	// IdentitySourceBinaryName means the name came from finfocus-plugin-<name>[-<region>].
	IdentitySourceBinaryName = "binary-name"
)

// PluginIdentity is the name a plugin under test is expected to report.
type PluginIdentity struct {
	// Expected is the expected install name; empty when it cannot be derived.
	Expected string
	// Source says where Expected came from (an IdentitySource* value).
	Source string
	// Reason explains why Expected is empty.
	Reason string
}

// ResolveExpectedName derives the install name a plugin binary should report:
// the explicit name when given, else the plugin directory when the binary is
// in an install layout (<plugins>/<name>/<version>/), else the binary name
// without the finfocus-plugin- prefix and the region suffix that a
// plugin.metadata.json beside it declares. It never guesses: when none
// applies, Expected is empty and Reason says how to supply the name.
func ResolveExpectedName(binPath, explicit string) PluginIdentity {
	if explicit != "" {
		return PluginIdentity{Expected: explicit, Source: IdentitySourceFlag}
	}

	versionDir := filepath.Dir(binPath)
	if _, err := semver.NewVersion(filepath.Base(versionDir)); err == nil {
		name := filepath.Base(filepath.Dir(versionDir))
		if pluginname.Valid(name) {
			return PluginIdentity{Expected: name, Source: IdentitySourceInstallLayout}
		}
	}

	region := ""
	if meta, err := registry.ReadPluginMetadata(versionDir); err == nil {
		region = meta["region"]
	}
	if name, ok := pluginname.FromBinary(binPath, region); ok {
		return PluginIdentity{Expected: name, Source: IdentitySourceBinaryName}
	}

	return PluginIdentity{Reason: fmt.Sprintf(
		"%s is not under <plugins>/<name>/<version>/ and is not named %s<name>; pass --expected-name to check it",
		filepath.Base(binPath), pluginname.BinaryPrefix)}
}
