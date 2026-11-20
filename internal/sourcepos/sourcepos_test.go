package sourcepos_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/sourcepos"
)

const fixtureDir = "../../testdata/sourcepos"

type recordedFrame struct {
	SourcePosition string `json:"sourcePosition"`
}

type recordedResource struct {
	URN            string          `json:"urn"`
	SourcePosition string          `json:"sourcePosition"`
	StackTrace     []recordedFrame `json:"stackTrace"`
}

func (r recordedResource) frames() []string {
	frames := make([]string, len(r.StackTrace))
	for i, frame := range r.StackTrace {
		frames[i] = frame.SourcePosition
	}
	return frames
}

func (r recordedResource) name() string {
	for i := len(r.URN) - 1; i > 0; i-- {
		if r.URN[i] == ':' {
			return r.URN[i+1:]
		}
	}
	return r.URN
}

type expectedFile struct {
	Runtime string `json:"runtime"`
	State   struct {
		Root      string                         `json:"root"`
		Resources map[string]*sourcepos.Location `json:"resources"`
	} `json:"state"`
	Preview struct {
		Root      string `json:"root"`
		Resources map[string]struct {
			Op       string              `json:"op"`
			Location *sourcepos.Location `json:"location"`
		} `json:"resources"`
	} `json:"preview"`
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, into))
}

func TestResolve_RecordedFixtures(t *testing.T) {
	t.Parallel()

	for _, runtime := range []string{"nodejs", "go", "python", "yaml"} {
		t.Run(runtime, func(t *testing.T) {
			t.Parallel()
			dir := filepath.Join(fixtureDir, runtime)

			var expected expectedFile
			readJSON(t, filepath.Join(dir, "expected.json"), &expected)
			require.Equal(t, expected.Runtime, runtime)

			t.Run("state", func(t *testing.T) {
				t.Parallel()
				var state struct {
					Deployment struct {
						Resources []recordedResource `json:"resources"`
					} `json:"deployment"`
				}
				readJSON(t, filepath.Join(dir, "state.json"), &state)
				require.NotEmpty(t, state.Deployment.Resources)
				require.Len(t, expected.State.Resources, len(state.Deployment.Resources))

				resolver := sourcepos.NewResolver(filepath.Join(dir, expected.State.Root), runtime)
				for _, res := range state.Deployment.Resources {
					want, recorded := expected.State.Resources[res.name()]
					require.True(t, recorded, "no expectation for %s", res.name())
					assert.Equal(t, want, resolver.Resolve(res.SourcePosition, res.frames()), res.name())
				}
			})

			t.Run("preview", func(t *testing.T) {
				t.Parallel()
				var preview struct {
					Steps []struct {
						Op       string            `json:"op"`
						NewState *recordedResource `json:"newState"`
						OldState *recordedResource `json:"oldState"`
					} `json:"steps"`
				}
				readJSON(t, filepath.Join(dir, "preview.json"), &preview)
				require.NotEmpty(t, preview.Steps)
				require.Len(t, expected.Preview.Resources, len(preview.Steps))

				resolver := sourcepos.NewResolver(filepath.Join(dir, expected.Preview.Root), runtime)
				for _, step := range preview.Steps {
					res := step.NewState
					if res == nil {
						res = step.OldState
					}
					require.NotNil(t, res)
					want, recorded := expected.Preview.Resources[res.name()]
					require.True(t, recorded, "no expectation for %s", res.name())
					assert.Equal(t, want.Op, step.Op, res.name())
					assert.Equal(t, want.Location, resolver.Resolve(res.SourcePosition, res.frames()), res.name())
				}
			})
		})
	}
}

func TestResolve_NeverLeaksOutsidePaths(t *testing.T) {
	t.Parallel()

	for _, runtime := range []string{"nodejs", "go", "python", "yaml"} {
		dir := filepath.Join(fixtureDir, runtime)
		var state struct {
			Deployment struct {
				Resources []recordedResource `json:"resources"`
			} `json:"deployment"`
		}
		readJSON(t, filepath.Join(dir, "state.json"), &state)
		resolver := sourcepos.NewResolver(filepath.Join(dir, "program"), runtime)
		for _, res := range state.Deployment.Resources {
			loc := resolver.Resolve(res.SourcePosition, res.frames())
			if loc == nil {
				continue
			}
			assert.False(t, filepath.IsAbs(loc.File), "%s/%s: %s", runtime, res.name(), loc.File)
			assert.NotContains(t, loc.File, "..", "%s/%s", runtime, res.name())
			assert.NotContains(t, loc.File, `\`, "%s/%s", runtime, res.name())
		}
	}
}

// project lays out files under a temp root and returns that root.
func project(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	return root
}

func TestResolve_Positions(t *testing.T) {
	t.Parallel()

	root := project(t, map[string]string{
		"index.js":                        "",
		"lib/box.js":                      "",
		"src/main.go":                     "",
		"node_modules/dep/index.js":       "",
		"venv/lib/site-packages/dep/m.py": "",
		"go.mod":                          "module example.com/shop\n\ngo 1.24\n",
	})
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir.js"), 0o750))

	tests := []struct {
		name     string
		runtime  string
		position string
		frames   []string
		want     *sourcepos.Location
	}{
		{
			"line and column",
			"nodejs",
			"project:///index.js#4,1",
			nil,
			&sourcepos.Location{File: "index.js", Line: 4, Column: 1},
		},
		{"line only", "python", "project:///index.js#4", nil, &sourcepos.Location{File: "index.js", Line: 4}},
		{
			"nested path",
			"nodejs",
			"project:///lib/box.js#8,5",
			nil,
			&sourcepos.Location{File: "lib/box.js", Line: 8, Column: 5},
		},
		{
			"dot segments inside root",
			"nodejs",
			"project:///lib/../lib/box.js#8",
			nil,
			&sourcepos.Location{File: "lib/box.js", Line: 8},
		},
		{
			"windows separators",
			"nodejs",
			`project:///lib\box.js#8,5`,
			nil,
			&sourcepos.Location{File: "lib/box.js", Line: 8, Column: 5},
		},
		{"empty position", "nodejs", "", nil, nil},
		{"file scheme", "nodejs", "file:///index.js#4,1", nil, nil},
		{"http scheme", "nodejs", "https://example.com/index.js#4,1", nil, nil},
		{"project scheme with two slashes", "nodejs", "project://index.js#4,1", nil, nil},
		{"no line", "nodejs", "project:///index.js", nil, nil},
		{"zero line", "nodejs", "project:///index.js#0,1", nil, nil},
		{"negative line", "nodejs", "project:///index.js#-3,1", nil, nil},
		{"non numeric line", "nodejs", "project:///index.js#x,1", nil, nil},
		{"non numeric column", "nodejs", "project:///index.js#4,x", nil, nil},
		{"trailing text", "nodejs", "project:///index.js#4,1,9", nil, nil},
		{
			"zero column is absent",
			"nodejs",
			"project:///index.js#4,0",
			nil,
			&sourcepos.Location{File: "index.js", Line: 4},
		},
		{"empty path", "nodejs", "project:///#4,1", nil, nil},
		{"missing file", "nodejs", "project:///gone.js#4,1", nil, nil},
		{"directory is not a file", "nodejs", "project:///dir.js#4,1", nil, nil},
		{"traversal out of root", "nodejs", "project:///../index.js#4,1", nil, nil},
		{"traversal after descent", "nodejs", "project:///lib/../../index.js#4,1", nil, nil},
		{"absolute path is outside", "nodejs", "project:////etc/passwd#1", nil, nil},
		{"dependency directory", "nodejs", "project:///node_modules/dep/index.js#1,1", nil, nil},
		{"python dependency directory", "python", "project:///venv/lib/site-packages/dep/m.py#1", nil, nil},
		{
			"innermost project frame wins",
			"nodejs", "project:///lib/box.js#8,5",
			[]string{"project:///lib/box.js#8,5", "project:///index.js#4,1"},
			&sourcepos.Location{File: "lib/box.js", Line: 8, Column: 5},
		},
		{
			"skips frames outside the project",
			"nodejs", "project:///../vendor/index.js#4,5",
			[]string{"project:///../vendor/index.js#4,5", "", "project:///index.js#9,18"},
			&sourcepos.Location{File: "index.js", Line: 9, Column: 18},
		},
		{
			"skips dependency frames",
			"nodejs", "project:///node_modules/dep/index.js#4,5",
			[]string{"project:///node_modules/dep/index.js#4,5", "project:///index.js#10,16"},
			&sourcepos.Location{File: "index.js", Line: 10, Column: 16},
		},
		{
			"skips frames that name no file",
			"nodejs", "project:///gone.js#1,1",
			[]string{"project:///gone.js#1,1", "project:///index.js#2,2"},
			&sourcepos.Location{File: "index.js", Line: 2, Column: 2},
		},
		{
			"every frame outside gives nothing",
			"nodejs", "project:///../a.js#1,1",
			[]string{"project:///../a.js#1,1", "project:///../../b.js#2,2"},
			nil,
		},
		{
			"falls back to the position when the trace is empty",
			"nodejs", "project:///index.js#4,1", []string{},
			&sourcepos.Location{File: "index.js", Line: 4, Column: 1},
		},
		{
			"falls back to the position when no frame resolves",
			"nodejs", "project:///index.js#4,1",
			[]string{"project:///gone.js#1,1"},
			&sourcepos.Location{File: "index.js", Line: 4, Column: 1},
		},
		{"go path of another module", "go", "project:///../../../../srcposgo/main.go#14", nil, nil},
		{
			"go module path is stripped",
			"go", "project:///../../../../example.com/shop/src/main.go#14", nil,
			&sourcepos.Location{File: "src/main.go", Line: 14},
		},
		{
			"go module prefix needs a path boundary",
			"go", "project:///../../example.com/shop2/src/main.go#14", nil, nil,
		},
		{
			"go dependency path is dropped",
			"go", "project:///../../../../github.com/pulumi/pulumi/sdk/v3@v3.276.0/go/pulumi/run.go#104", nil, nil,
		},
		{
			"go module path with no remainder",
			"go", "project:///../../example.com/shop#14", nil, nil,
		},
		{
			"go path inside the root is kept",
			"go", "project:///src/main.go#14", nil,
			&sourcepos.Location{File: "src/main.go", Line: 14},
		},
		{
			"module paths are for go only",
			"nodejs", "project:///../../../../example.com/shop/src/main.go#14", nil, nil,
		},
		{
			"go stack trace walks to the project frame",
			"go", "project:///../../../../runtime/proc.go#302",
			[]string{"project:///../../../../runtime/proc.go#302", "project:///../../example.com/shop/src/main.go#11"},
			&sourcepos.Location{File: "src/main.go", Line: 11},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := sourcepos.NewResolver(root, tt.runtime).Resolve(tt.position, tt.frames)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolve_GoWithoutModule(t *testing.T) {
	t.Parallel()

	root := project(t, map[string]string{"main.go": ""})
	got := sourcepos.NewResolver(root, "go").Resolve("project:///../../shop/main.go#3", nil)
	assert.Nil(t, got)

	inRoot := sourcepos.NewResolver(root, "go").Resolve("project:///main.go#3", nil)
	assert.Equal(t, &sourcepos.Location{File: "main.go", Line: 3}, inRoot)
}

func TestResolve_GoModuleDirectiveForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		goMod string
	}{
		{"plain", "module shop\n\ngo 1.24\n"},
		{"quoted", "module \"shop\"\n\ngo 1.24\n"},
		{"comment", "// header\nmodule shop // trailing\n\ngo 1.24\n"},
		{"crlf", "module shop\r\n\r\ngo 1.24\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := project(t, map[string]string{"main.go": "", "go.mod": tt.goMod})
			got := sourcepos.NewResolver(root, "go").Resolve("project:///../../shop/main.go#3", nil)
			assert.Equal(t, &sourcepos.Location{File: "main.go", Line: 3}, got)
		})
	}
}

func TestResolve_SymlinkEscape(t *testing.T) {
	t.Parallel()

	outside := project(t, map[string]string{"secret.js": "", "dir/inner.js": ""})
	root := project(t, map[string]string{"real.js": "", "sub/real.js": ""})

	links := map[string]string{
		"leak.js":        filepath.Join(outside, "secret.js"),
		"leakdir":        filepath.Join(outside, "dir"),
		"inside.js":      filepath.Join(root, "real.js"),
		"sub/inside.js":  filepath.Join(root, "sub", "real.js"),
		"dangling.js":    filepath.Join(root, "absent.js"),
		"leak-parent.js": filepath.Join("..", filepath.Base(outside), "secret.js"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(name))); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	resolver := sourcepos.NewResolver(root, "nodejs")

	tests := []struct {
		name     string
		position string
		want     *sourcepos.Location
	}{
		{"file link out of the root", "project:///leak.js#1,1", nil},
		{"directory link out of the root", "project:///leakdir/inner.js#1,1", nil},
		{"relative link out of the root", "project:///leak-parent.js#1,1", nil},
		{"dangling link", "project:///dangling.js#1,1", nil},
		{
			"link that stays inside",
			"project:///inside.js#2,1",
			&sourcepos.Location{File: "inside.js", Line: 2, Column: 1},
		},
		{
			"nested link that stays inside",
			"project:///sub/inside.js#2",
			&sourcepos.Location{File: "sub/inside.js", Line: 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolver.Resolve(tt.position, nil))
		})
	}
}

func TestResolve_SymlinkedRoot(t *testing.T) {
	t.Parallel()

	realRoot := project(t, map[string]string{"index.js": ""})
	link := filepath.Join(t.TempDir(), "project")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := sourcepos.NewResolver(link, "nodejs").Resolve("project:///index.js#4,1", nil)
	assert.Equal(t, &sourcepos.Location{File: "index.js", Line: 4, Column: 1}, got)
}

func TestResolve_RelativeRoot(t *testing.T) {
	t.Parallel()

	root := project(t, map[string]string{"index.js": ""})
	rel, err := filepath.Rel(mustGetwd(t), root)
	if err != nil {
		t.Skipf("no relative path to the temp dir: %v", err)
	}
	got := sourcepos.NewResolver(rel, "nodejs").Resolve("project:///index.js#4,1", nil)
	assert.Equal(t, &sourcepos.Location{File: "index.js", Line: 4, Column: 1}, got)
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}

func TestResolve_MissingRoot(t *testing.T) {
	t.Parallel()

	resolver := sourcepos.NewResolver(filepath.Join(t.TempDir(), "absent"), "nodejs")
	assert.Nil(t, resolver.Resolve("project:///index.js#4,1", nil))
	assert.Nil(t, sourcepos.NewResolver("", "nodejs").Resolve("project:///index.js#4,1", nil))
}

func TestLocation_JSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		loc  sourcepos.Location
		want string
	}{
		{
			"with column",
			sourcepos.Location{File: "lib/box.js", Line: 8, Column: 5},
			`{"file":"lib/box.js","line":8,"column":5}`,
		},
		{"without column", sourcepos.Location{File: "main.go", Line: 14}, `{"file":"main.go","line":14}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tt.loc)
			require.NoError(t, err)
			assert.JSONEq(t, tt.want, string(data))
		})
	}
}
