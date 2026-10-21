package esc

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverBackendAndTokenOrder(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
		"current": "https://api.pulumi.com",
		"accounts": map[string]any{
			"https://api.pulumi.com": map[string]string{
				"accessToken": "from-account",
				"username":    "org-from-account",
			},
			"https://saved.example": map[string]string{
				"accessToken": "saved-for-url",
				"username":    "saved-user",
			},
		},
		"accessTokens": map[string]string{
			"https://api.pulumi.com": "from-access-tokens",
		},
	})

	t.Run("environment token and backend win", func(t *testing.T) {
		t.Parallel()
		session, fail := discover(getenv(map[string]string{
			"PULUMI_ACCESS_TOKEN": "from-env",
			"PULUMI_BACKEND_URL":  "https://api.pulumi.com",
			"PULUMI_HOME":         home,
		}), "")
		require.Nil(t, fail)
		assert.Equal(t, "https://api.pulumi.com", session.BackendURL)
		assert.Equal(t, "from-env", session.Token)
	})

	t.Run("account token then accessTokens", func(t *testing.T) {
		t.Parallel()
		session, fail := discover(getenv(map[string]string{
			"PULUMI_BACKEND_URL": "https://api.pulumi.com",
			"PULUMI_HOME":        home,
		}), "")
		require.Nil(t, fail)
		assert.Equal(t, "from-account", session.Token)
	})

	t.Run("project file backend is used before credentials current", func(t *testing.T) {
		t.Parallel()
		session, fail := discover(getenv(map[string]string{"PULUMI_HOME": home}), "https://saved.example")
		require.Nil(t, fail)
		assert.Equal(t, "https://saved.example", session.BackendURL)
		assert.Equal(t, "saved-for-url", session.Token)
	})

	t.Run("credentials current when nothing else is set", func(t *testing.T) {
		t.Parallel()
		session, fail := discover(getenv(map[string]string{"PULUMI_HOME": home}), "")
		require.Nil(t, fail)
		assert.Equal(t, "https://api.pulumi.com", session.BackendURL)
		assert.Equal(t, "from-account", session.Token)
	})
}

func TestDiscoverSelfManagedAndCleartext(t *testing.T) {
	t.Parallel()
	schemes := []string{"file:///tmp/state", "s3://bucket", "gs://bucket", "azblob://container"}
	for _, backend := range schemes {
		t.Run(backend, func(t *testing.T) {
			t.Parallel()
			_, fail := discover(getenv(map[string]string{
				"PULUMI_ACCESS_TOKEN": "tok",
				"PULUMI_BACKEND_URL":  backend,
			}), "")
			require.NotNil(t, fail)
			assert.Equal(t, CategoryUnsupportedBackend, fail.Category)
		})
	}

	t.Run("http non-loopback", func(t *testing.T) {
		t.Parallel()
		_, fail := discover(getenv(map[string]string{
			"PULUMI_ACCESS_TOKEN": "tok",
			"PULUMI_BACKEND_URL":  "http://example.com",
		}), "")
		require.NotNil(t, fail)
		assert.Equal(t, CategoryUnsupportedBackend, fail.Category)
	})

	t.Run("http loopback", func(t *testing.T) {
		t.Parallel()
		session, fail := discover(getenv(map[string]string{
			"PULUMI_ACCESS_TOKEN": "tok",
			"PULUMI_BACKEND_URL":  "http://127.0.0.1:9",
		}), "")
		require.Nil(t, fail)
		assert.Equal(t, "http://127.0.0.1:9", session.BackendURL)
	})
}

func TestDiscoverUnusableCredentials(t *testing.T) {
	t.Parallel()

	t.Run("encrypted envelope", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{"encryptedkey": "x"})
		_, fail := discover(getenv(map[string]string{"PULUMI_HOME": home}), "")
		require.NotNil(t, fail)
		assert.Equal(t, CategoryNotLoggedIn, fail.Category)
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		_, fail := discover(getenv(map[string]string{"PULUMI_HOME": t.TempDir()}), "")
		require.NotNil(t, fail)
		assert.Equal(t, CategoryNotLoggedIn, fail.Category)
	})

	t.Run("empty token", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
			"current": "https://api.pulumi.com",
			"accounts": map[string]any{
				"https://api.pulumi.com": map[string]string{"accessToken": ""},
			},
		})
		_, fail := discover(getenv(map[string]string{"PULUMI_HOME": home}), "")
		require.NotNil(t, fail)
		assert.Equal(t, CategoryNotLoggedIn, fail.Category)
	})

	t.Run("credentials path then home", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeJSON(t, filepath.Join(dir, "credentials.json"), map[string]any{
			"current":      "https://api.pulumi.com",
			"accessTokens": map[string]string{"https://api.pulumi.com": "from-path"},
		})
		session, fail := discover(getenv(map[string]string{
			"PULUMI_CREDENTIALS_PATH": dir,
			"PULUMI_HOME":             t.TempDir(),
		}), "")
		require.Nil(t, fail)
		assert.Equal(t, "from-path", session.Token)
	})
}

func TestDiscoverOrgOrder(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
		"current": "https://api.pulumi.com",
		"accounts": map[string]any{
			"https://api.pulumi.com": map[string]string{
				"accessToken": "tok",
				"username":    "from-user",
			},
		},
	})
	writeJSON(t, filepath.Join(home, "config.json"), map[string]any{
		"backends": map[string]any{
			"https://api.pulumi.com": map[string]string{"defaultOrg": "from-config"},
		},
	})
	base := getenv(map[string]string{"PULUMI_HOME": home, "PULUMI_BACKEND_URL": "https://api.pulumi.com"})
	session, fail := discover(base, "")
	require.Nil(t, fail)

	t.Run("stack import uses the stack org", func(t *testing.T) {
		t.Parallel()
		org, orgFail := session.orgFor(Ref{Project: "proj", Name: "env"}, true, "orgA/demo/dev", base)
		require.Nil(t, orgFail)
		assert.Equal(t, "orgA", org)
	})
	t.Run("configured reference uses the default org", func(t *testing.T) {
		t.Parallel()
		org, orgFail := session.orgFor(
			Ref{Project: "proj", Name: "env"},
			false,
			"orgA/demo/dev",
			getenv(map[string]string{
				"PULUMI_DEFAULT_ORGANIZATION": "orgB",
				"PULUMI_HOME":                 home,
			}),
		)
		require.Nil(t, orgFail)
		assert.Equal(t, "orgB", org)
	})
	t.Run("default org chain", func(t *testing.T) {
		t.Parallel()
		org, orgFail := session.orgFor(Ref{Project: "proj", Name: "env"}, false, "dev", base)
		require.Nil(t, orgFail)
		assert.Equal(t, "from-config", org)
	})
}

func TestDiscoverTokenTrust(t *testing.T) {
	t.Parallel()
	t.Run("project url does not receive an environment token", func(t *testing.T) {
		t.Parallel()
		_, fail := discover(getenv(map[string]string{
			"PULUMI_ACCESS_TOKEN": "from-env",
		}), "https://evil.example")
		require.NotNil(t, fail)
		assert.Equal(t, CategoryUnsupportedBackend, fail.Category)
	})

	t.Run("project url uses the token saved for that url", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
			"current": "https://api.pulumi.com",
			"accounts": map[string]any{
				"https://pulumi.example": map[string]string{"accessToken": "saved-token"},
			},
		})
		session, fail := discover(getenv(map[string]string{
			"PULUMI_ACCESS_TOKEN": "from-env",
			"PULUMI_HOME":         home,
		}), "https://pulumi.example")
		require.Nil(t, fail)
		assert.Equal(t, "saved-token", session.Token)
		assert.Equal(t, "https://pulumi.example", session.BackendURL)
	})
}

func TestDiscoverReadOnly(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
		"current":      "https://api.pulumi.com",
		"accessTokens": map[string]string{"https://api.pulumi.com": "tok"},
	})
	before := snapshotTree(t, home)
	_, fail := discover(getenv(map[string]string{"PULUMI_HOME": home}), "")
	require.Nil(t, fail)
	assert.Equal(t, before, snapshotTree(t, home))
}

func getenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func writeJSON(t *testing.T, path string, body any) {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
}

func snapshotTree(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = sha256.Sum256(data)
		return nil
	})
	require.NoError(t, err)
	return out
}
