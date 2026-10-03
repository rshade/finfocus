package history

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitStack(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input       string
		wantProject string
		wantStack   string
	}{
		{"dev", "", "dev"},
		{"org/dev", "", "dev"},
		{"org/app/dev", "app", "dev"},
		{" org/app/dev ", "app", "dev"},
		{"a/b/c/d", "", "a/b/c/d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			project, stack := SplitStack(tt.input)
			assert.Equal(t, tt.wantProject, project)
			assert.Equal(t, tt.wantStack, stack)
		})
	}
}

func TestCostFileNameFor(t *testing.T) {
	t.Parallel()

	t.Run("no project keeps the legacy name", func(t *testing.T) {
		t.Parallel()
		name, err := CostFileNameFor("", "dev")
		require.NoError(t, err)
		assert.Equal(t, "dev.history.db", name)
	})

	t.Run("a project is part of the name", func(t *testing.T) {
		t.Parallel()
		name, err := CostFileNameFor("app", "dev")
		require.NoError(t, err)
		assert.Equal(t, "app@dev.history.db", name)
	})

	t.Run("projects with the same stack get different names", func(t *testing.T) {
		t.Parallel()
		first, err := CostFileNameFor("app-a", "dev")
		require.NoError(t, err)
		second, err := CostFileNameFor("app", "a-dev")
		require.NoError(t, err)
		assert.NotEqual(t, first, second)
	})

	for _, bad := range [][2]string{
		{"app@x", "dev"}, {"app", "de@v"}, {"..", "dev"}, {"app", ""}, {"ap p", "dev"}, {"app/x", "dev"},
	} {
		t.Run("rejects "+bad[0]+"/"+bad[1], func(t *testing.T) {
			t.Parallel()
			_, err := CostFileNameFor(bad[0], bad[1])
			assert.Error(t, err)
		})
	}
}

func TestProjectFromURN(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "app", ProjectFromURN("urn:pulumi:dev::app::aws:ec2/instance:Instance::web"))
	assert.Empty(t, ProjectFromURN("urn:web"))
	assert.Empty(t, ProjectFromURN(""))
}

func TestProjectFromFileName(t *testing.T) {
	t.Parallel()

	project, stack, ok := ProjectFromFileName("app@dev.history.db")
	assert.True(t, ok)
	assert.Equal(t, "app", project)
	assert.Equal(t, "dev", stack)

	_, _, ok = ProjectFromFileName("dev.history.db")
	assert.False(t, ok)
}

func TestOpenCostDBFor_ProjectIdentity(t *testing.T) {
	t.Parallel()

	t.Run("a different project is an error", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "app@dev.history.db")
		db, err := OpenCostDBFor(path, "app", "dev")
		require.NoError(t, err)
		require.NoError(t, db.Close())

		_, err = OpenCostDBFor(path, "other", "dev")
		require.ErrorContains(t, err, `belongs to project "app"`)

		again, err := OpenCostDBFor(path, "app", "dev")
		require.NoError(t, err)
		require.NoError(t, again.Close())
	})

	t.Run("a database with no project is claimed by the first project", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "dev.history.db")
		legacy, err := OpenCostDB(path, "dev")
		require.NoError(t, err)
		require.NoError(t, legacy.Close())

		claimed, err := OpenCostDBFor(path, "app", "dev")
		require.NoError(t, err)
		stats, err := claimed.Stats()
		require.NoError(t, err)
		require.NoError(t, claimed.Close())
		assert.Equal(t, "app", stats.Project)

		_, err = OpenCostDBFor(path, "other", "dev")
		require.Error(t, err)
	})
}

func TestInferProject(t *testing.T) {
	t.Parallel()

	t.Run("from the stored project", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "app@dev.history.db")
		db, err := OpenCostDBFor(path, "app", "dev")
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		project, err := db.InferProject()
		require.NoError(t, err)
		assert.Equal(t, "app", project)
	})

	t.Run("from the resource URNs of a legacy database", func(t *testing.T) {
		t.Parallel()
		db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
		snapshot := ZeroSnapshot(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1)
		snapshot.Resources = []CostResource{{URN: "urn:pulumi:dev::billing::aws:ec2/instance:Instance::web"}}
		require.NoError(t, db.Put(snapshot, CostAnnotation{Version: 1}))
		project, err := db.InferProject()
		require.NoError(t, err)
		assert.Equal(t, "billing", project)
	})

	t.Run("unknown when nothing says", func(t *testing.T) {
		t.Parallel()
		db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
		project, err := db.InferProject()
		require.NoError(t, err)
		assert.Empty(t, project)
	})
}
