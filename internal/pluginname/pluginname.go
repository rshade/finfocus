// Package pluginname holds the rules for plugin names: which names are valid,
// which are canonical, and how a plugin binary's file name maps to the name
// its install directory and catalog entry use. It depends only on the
// standard library so the CLI, the registry, and the conformance suite can
// share it.
package pluginname

import (
	"errors"
	"fmt"
	"strings"
)

// BinaryPrefix is the file name prefix of every plugin binary
// (finfocus-plugin-<name>). It is not part of the plugin's name.
const BinaryPrefix = "finfocus-plugin-"

const (
	minLength = 2
	maxLength = 50
	exeSuffix = ".exe"
)

// Valid reports whether name satisfies the plugin naming rules: 2 to 50
// characters, only lowercase letters, digits, or hyphens, and no leading or
// trailing hyphen.
func Valid(name string) bool {
	if len(name) < minLength || len(name) > maxLength {
		return false
	}
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit && r != '-' {
			return false
		}
	}
	return name[0] != '-' && name[len(name)-1] != '-'
}

// Canonical returns nil when name is the form a plugin should report from
// its Name call: a valid name made of hyphen-separated groups of lowercase
// letters and digits, without the binary prefix. The error says which rule
// failed.
func Canonical(name string) error {
	switch {
	case name == "":
		return errors.New("plugin name is empty")
	case strings.HasPrefix(name, BinaryPrefix):
		return fmt.Errorf("plugin name %q must not start with %q, which belongs to the binary file name",
			name, BinaryPrefix)
	case !Valid(name):
		return fmt.Errorf(
			"plugin name %q must be %d-%d lowercase letters, digits, or hyphens, not starting or ending with a hyphen",
			name, minLength, maxLength)
	case strings.Contains(name, "--"):
		return fmt.Errorf("plugin name %q must not contain consecutive hyphens", name)
	default:
		return nil
	}
}

// FromBinary returns the plugin name a binary's file name implies: the base
// name without a .exe extension, without BinaryPrefix, and without a
// -<region> suffix when region is set. Both / and \ separate directories, so
// Windows paths work on any platform. It returns false when the file name
// does not start with BinaryPrefix or nothing follows it.
func FromBinary(path, region string) (string, bool) {
	base := path[strings.LastIndexAny(path, `/\`)+1:]
	base = strings.TrimSuffix(base, exeSuffix)
	name, found := strings.CutPrefix(base, BinaryPrefix)
	if !found || name == "" {
		return "", false
	}
	if region != "" {
		if trimmed, cut := strings.CutSuffix(name, "-"+region); cut && trimmed != "" {
			name = trimmed
		}
	}
	return name, true
}
