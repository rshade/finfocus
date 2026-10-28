package esc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverSession(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	writeJSON(t, filepath.Join(home, "credentials.json"), map[string]any{
		"current": "https://saved.example",
		"accounts": map[string]any{
			"https://saved.example": map[string]string{"accessToken": "saved", "username": "default-org"},
		},
	})
	for _, tc := range []struct {
		name, envBackend, project, token, stack, wantBackend, wantToken, wantOrg string
		fail                                                                     bool
	}{
		{name: "saved", stack: "stack", wantBackend: "https://saved.example", wantToken: "saved", wantOrg: "default-org"},
		{name: "explicit org", stack: "org/stack", wantBackend: "https://saved.example", wantToken: "saved", wantOrg: "org"},
		{name: "environment backend", envBackend: "https://override.example", project: "https://saved.example", token: "env", stack: "org/stack", wantBackend: "https://override.example", wantToken: "env", wantOrg: "org"},
		{name: "project token override", project: "https://saved.example", token: "env", stack: "org/stack", wantBackend: "https://saved.example", wantToken: "saved", wantOrg: "org"},
		{name: "new project with environment token remains untrusted", project: "https://new.example", token: "env", stack: "org/stack", fail: true},
		{name: "unmatched project", project: "https://new.example", stack: "org/stack", fail: true},
		{name: "insecure", envBackend: "http://remote.example", token: "env", stack: "org/stack", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			session, fail := DiscoverSession(
				getenv(
					map[string]string{
						"PULUMI_HOME":         home,
						"PULUMI_BACKEND_URL":  tc.envBackend,
						"PULUMI_ACCESS_TOKEN": tc.token,
					},
				),
				tc.project,
				tc.stack,
			)
			if tc.fail {
				require.NotNil(t, fail)
				assert.Empty(t, session.Token)
				return
			}
			require.Nil(t, fail)
			assert.Equal(t, tc.wantBackend, session.Backend)
			assert.Equal(t, tc.wantToken, session.Token)
			assert.Equal(t, tc.wantOrg, session.Org)
		})
	}
}
