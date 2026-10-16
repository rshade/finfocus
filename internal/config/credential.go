package config

import (
	"slices"
	"strings"
)

// credentialNameFragments is the only list of credential-like name fragments.
// engine.isCredentialKey delegates here so tags, plugin attributes, and the
// config page cannot grow a second rule.
func credentialNameFragments() []string {
	return []string{
		"password", "passwd", "passphrase", "secret", "token", "credential", "ciphertext",
		"privatekey", "private_key", "apikey", "api_key", "accesskey", "access_key",
		"connectionstring", "connection_string",
	}
}

// IsCredentialName reports whether key looks like it holds a credential.
// The match is a case-insensitive substring, not a configurable list.
func IsCredentialName(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range credentialNameFragments() {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// IsSecretSetting reports whether a dotted configuration key must not be shown.
// A key is secret when any dot-separated segment matches IsCredentialName.
func IsSecretSetting(key string) bool {
	return slices.ContainsFunc(strings.Split(key, "."), IsCredentialName)
}
