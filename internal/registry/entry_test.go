package registry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

func TestValidateRegistryEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   registry.RegistryEntry
		wantErr bool
	}{
		{
			name: "valid entry",
			entry: registry.RegistryEntry{
				Name:          "test-plugin",
				Repository:    "owner/repo",
				SecurityLevel: "community",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			entry: registry.RegistryEntry{
				Repository: "owner/repo",
			},
			wantErr: true,
		},
		{
			name: "missing repository",
			entry: registry.RegistryEntry{
				Name: "test-plugin",
			},
			wantErr: true,
		},
		{
			name: "invalid repository format",
			entry: registry.RegistryEntry{
				Name:       "test-plugin",
				Repository: "invalid",
			},
			wantErr: true,
		},
		{
			name: "invalid security level",
			entry: registry.RegistryEntry{
				Name:          "test-plugin",
				Repository:    "owner/repo",
				SecurityLevel: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid official security level",
			entry: registry.RegistryEntry{
				Name:          "test-plugin",
				Repository:    "owner/repo",
				SecurityLevel: "official",
			},
			wantErr: false,
		},
		{
			name: "valid experimental security level",
			entry: registry.RegistryEntry{
				Name:          "test-plugin",
				Repository:    "owner/repo",
				SecurityLevel: "experimental",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := registry.ValidateRegistryEntry(tt.entry)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRegistryEntry_TagPrefix(t *testing.T) {
	t.Parallel()

	base := registry.RegistryEntry{Name: "kubernetes", Repository: "rshade/finfocus"}
	for _, p := range []string{"", "kubernetes-", "k8s-alloc-"} {
		e := base
		e.TagPrefix = p
		require.NoError(t, registry.ValidateRegistryEntry(e), "prefix %q", p)
	}
	for _, p := range []string{"kubernetes", "Kubernetes-", "-", "kube/", "v", "vantage-", "v1-"} {
		e := base
		e.TagPrefix = p
		err := registry.ValidateRegistryEntry(e)
		require.Error(t, err, "prefix %q", p)
		assert.Contains(t, err.Error(), "tag_prefix")
	}
}

func TestParsePluginSpecifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		spec        string
		wantName    string
		wantVersion string
		wantIsURL   bool
		wantErr     bool
	}{
		{
			name:        "simple name",
			spec:        "kubecost",
			wantName:    "kubecost",
			wantVersion: "",
			wantIsURL:   false,
		},
		{
			name:        "name with version",
			spec:        "kubecost@v1.0.0",
			wantName:    "kubecost",
			wantVersion: "v1.0.0",
			wantIsURL:   false,
		},
		{
			name:        "github url",
			spec:        "github.com/owner/finfocus-plugin-test",
			wantName:    "test",
			wantVersion: "",
			wantIsURL:   true,
		},
		{
			name:        "github url with version",
			spec:        "github.com/owner/repo@v2.0.0",
			wantName:    "repo",
			wantVersion: "v2.0.0",
			wantIsURL:   true,
		},
		{
			name:    "empty spec",
			spec:    "",
			wantErr: true,
		},
		{
			name:    "invalid github url",
			spec:    "github.com/invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := registry.ParsePluginSpecifier(tt.spec)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, result.Name)
			assert.Equal(t, tt.wantVersion, result.Version)
			assert.Equal(t, tt.wantIsURL, result.IsURL)
		})
	}
}

func TestParseGitHubURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid url",
			url:       "github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:    "invalid format",
			url:     "github.com/invalid",
			wantErr: true,
		},
		{
			name:    "wrong domain",
			url:     "gitlab.com/owner/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			owner, repo, err := registry.ParseGitHubURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}
