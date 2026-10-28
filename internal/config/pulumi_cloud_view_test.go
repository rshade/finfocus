package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPulumiCloudSettingView(t *testing.T) {
	t.Parallel()
	const key = "cost.history.pulumi_cloud.enabled"
	for _, tc := range []struct{ name, document, want, source string }{
		{"default", `{}`, "false", SourceDefault},
		{"boolean false", `{"cost":{"history":{"pulumi_cloud":{"enabled":false}}}}`, "false", SourceGlobal},
		{"boolean true", `{"cost":{"history":{"pulumi_cloud":{"enabled":true}}}}`, "true", SourceGlobal},
		{"auto", `{"cost":{"history":{"pulumi_cloud":{"enabled":"auto"}}}}`, "auto", SourceGlobal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.hujson")
			require.NoError(t, os.WriteFile(path, []byte(tc.document), 0o600))
			page, err := buildPage(path, "", nil)
			require.NoError(t, err)
			var row *SettingView
			for i := range page.Settings {
				if page.Settings[i].Key == key {
					row = &page.Settings[i]
					break
				}
			}
			require.NotNil(t, row)
			assert.Equal(t, KindEnum, row.Kind)
			assert.Equal(t, []string{"false", "auto", "true"}, row.Enum)
			assert.False(t, row.Secret)
			assert.False(t, row.ReadOnly)
			assert.Equal(t, tc.want, row.Effective.Value)
			assert.Equal(t, tc.source, row.Effective.Source)
			assert.Equal(t, tc.want, row.Tiers[tc.source].Value)
		})
	}
}

func TestPulumiCloudSettingProjectPatch(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.hujson")
	require.NoError(t, os.WriteFile(path, []byte("{\n // preserve history comment\n}\n"), 0o600))
	version, err := FileVersion(path)
	require.NoError(t, err)
	_, err = PatchFile(
		path,
		TierProject,
		version,
		[]Change{{Key: "cost.history.pulumi_cloud.enabled", Value: strPtr("auto")}},
		false,
	)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "// preserve history comment")
	doc, err := decodeDocument(data)
	require.NoError(t, err)
	assert.Equal(t, "auto", storedValue(doc, "cost.history.pulumi_cloud.enabled"))
	version, err = FileVersion(path)
	require.NoError(t, err)
	_, err = PatchFile(
		path,
		TierProject,
		version,
		[]Change{{Key: "cost.history.pulumi_cloud.enabled", Value: strPtr("maybe")}},
		false,
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "false")
	assert.Contains(t, err.Error(), "auto")
	assert.Contains(t, err.Error(), "true")
}
