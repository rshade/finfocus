package registry

import (
	"errors"
	"os"

	"github.com/rshade/finfocus/internal/config"
)

// ErrSkipChecksumForbidden is returned when a caller asks to skip checksum
// verification while secure plugin mode (FINFOCUS_SECURE_PLUGINS) is on.
var ErrSkipChecksumForbidden = errors.New(
	config.EnvSecurePlugins + " is on: checksum verification cannot be skipped",
)

// ErrSecurePluginsValue is returned when FINFOCUS_SECURE_PLUGINS holds a value
// that is not 1, true, 0, or false.
var ErrSecurePluginsValue = errors.New("invalid " + config.EnvSecurePlugins)

// checksumPolicy is the checksum handling an install or update asks for.
type checksumPolicy struct {
	skip          bool
	requireFile   bool
	acceptMissing bool
}

// applySecurePlugins tightens a checksum policy when secure plugin mode is on:
// checksums.txt must exist and list the archive, a missing file cannot be
// accepted, and verification cannot be skipped. With the mode off the policy
// is returned as given.
func applySecurePlugins(policy checksumPolicy) (checksumPolicy, error) {
	secure, err := config.ResolveSecurePlugins(os.Getenv)
	if err != nil {
		return policy, errors.Join(ErrSecurePluginsValue, err)
	}
	if !secure {
		return policy, nil
	}
	if policy.skip {
		return policy, ErrSkipChecksumForbidden
	}
	policy.requireFile = true
	policy.acceptMissing = false
	return policy, nil
}

// CheckSecurePlugins reports the error an install or update would return for
// the environment and the skip request, so a command can reject it before any
// network call.
func CheckSecurePlugins(skipChecksum bool) error {
	_, err := applySecurePlugins(checksumPolicy{skip: skipChecksum})
	return err
}
