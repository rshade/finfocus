package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixtureResolver_ResolvePlanFixture_Offline(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	fixturesDir := filepath.Join(tmpDir, "fixtures", "plans", "aws")
	err := os.MkdirAll(fixturesDir, 0755)
	require.NoError(t, err)

	planContent := `{"resources": []}`
	err = os.WriteFile(filepath.Join(fixturesDir, "simple.json"), []byte(planContent), 0644)
	require.NoError(t, err)

	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	source, err := resolver.ResolvePlanFixture(context.Background(), "aws")
	require.NoError(t, err)
	require.NotNil(t, source)

	assert.Equal(t, "plan", source.Type)
	assert.Equal(t, "aws", source.Provider)
	assert.Equal(t, "local", source.Version)
	assert.Contains(t, source.Origin, "simple.json")
}

func TestFixtureResolver_ResolvePlanFixture_OfflineNotFound(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	source, err := resolver.ResolvePlanFixture(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Nil(t, source)
	assert.Contains(t, err.Error(), "local plan fixture not found")
}

func TestFixtureResolver_ResolveStateFixture_Offline(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "fixtures", "state")
	err := os.MkdirAll(stateDir, 0755)
	require.NoError(t, err)

	stateContent := `{"resources": []}`
	err = os.WriteFile(filepath.Join(stateDir, "valid-state.json"), []byte(stateContent), 0644)
	require.NoError(t, err)

	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	source, err := resolver.ResolveStateFixture(context.Background())
	require.NoError(t, err)
	require.NotNil(t, source)

	assert.Equal(t, "state", source.Type)
	assert.Equal(t, "local", source.Version)
}

func TestFixtureResolver_ResolveStateFixture_OfflineNotFound(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	source, err := resolver.ResolveStateFixture(context.Background())
	require.Error(t, err)
	assert.Nil(t, source)
	assert.Contains(t, err.Error(), "local state fixture not found")
}

func TestFixtureResolver_DownloadFixture_Offline(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	source := &FixtureSource{
		ID:      "test-fixture",
		Type:    "plan",
		Origin:  filepath.Join(tmpDir, "test.json"),
		Version: "local",
	}

	path, err := resolver.DownloadFixture(context.Background(), source)
	require.NoError(t, err)
	assert.Equal(t, source.Origin, path)
}

func TestFixtureResolver_DownloadFixture_Offline_FileExists(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	tmpDir := t.TempDir()
	resolver := NewFixtureResolver(logger, true, "latest", tmpDir)

	// Create a real fixture file with JSON content
	fixtureContent := []byte(`{"resources": [{"type": "aws:ec2/instance:Instance"}]}`)
	fixtureFilePath := filepath.Join(tmpDir, "test-fixture.json")
	err := os.WriteFile(fixtureFilePath, fixtureContent, 0644)
	require.NoError(t, err)

	source := &FixtureSource{
		ID:      "test-fixture",
		Type:    "plan",
		Origin:  fixtureFilePath,
		Version: "local",
	}

	path, err := resolver.DownloadFixture(context.Background(), source)
	require.NoError(t, err)
	assert.Equal(t, source.Origin, path)

	// Read the file back and verify contents
	readContent, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(readContent), "aws:ec2/instance:Instance")
}

func TestInitialization(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "FixtureSource fields",
			test: func(t *testing.T) {
				source := &FixtureSource{
					ID:       "plan-aws-v1.0",
					Type:     "plan",
					Provider: "aws",
					Version:  "v1.0",
					Origin:   "https://example.com/plan.json",
					Checksum: "abc123",
				}

				assert.Equal(t, "plan-aws-v1.0", source.ID)
				assert.Equal(t, "plan", source.Type)
				assert.Equal(t, "aws", source.Provider)
				assert.Equal(t, "v1.0", source.Version)
				assert.Equal(t, "https://example.com/plan.json", source.Origin)
				assert.Equal(t, "abc123", source.Checksum)
			},
		},
		{
			name: "NewFixtureResolver parameters",
			test: func(t *testing.T) {
				logger := zerolog.New(zerolog.NewTestWriter(t))
				resolver := NewFixtureResolver(logger, true, "v1.0", "/test/path")

				assert.Equal(t, true, resolver.offlineMode)
				assert.Equal(t, "v1.0", resolver.fixtureVersion)
				assert.Equal(t, "/test/path", resolver.localBasePath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestFixtureResolver_ResolveRemotePlanFixture_ExplicitVersion(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	resolver := NewFixtureResolver(logger, false, "main", "/test/path")

	source, err := resolver.ResolvePlanFixture(context.Background(), "aws")
	require.NoError(t, err)
	require.NotNil(t, source)

	assert.Equal(t, "plan", source.Type)
	assert.Equal(t, "aws", source.Provider)
	assert.Equal(t, "main", source.Version)
	assert.Contains(t, source.Origin, "aws")
	assert.Contains(t, source.Origin, "simple.json")
}

func TestFixtureResolver_ResolveRemotePlanFixture_LatestFallback(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))

	// Use stubbed releaseTagFetcher to avoid network calls
	stubbedFetcher := func(_ context.Context) (string, error) {
		return "v1.2.3", nil
	}

	resolver := NewFixtureResolver(logger, false, "latest", "/test/path", WithReleaseTagFetcher(stubbedFetcher))

	source, err := resolver.ResolvePlanFixture(context.Background(), "aws")
	require.NoError(t, err)
	require.NotNil(t, source)

	assert.Equal(t, "plan", source.Type)
	assert.Equal(t, "aws", source.Provider)
	assert.Equal(t, "v1.2.3", source.Version)
	assert.Contains(t, source.Origin, "aws")
	assert.Contains(t, source.Origin, "simple.json")
}
