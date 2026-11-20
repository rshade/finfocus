package clidocs

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// committedDir is OutputDir seen from this package's directory.
const committedDir = "../../" + OutputDir

func renderProduction(t *testing.T) (*cobra.Command, map[string][]byte) {
	t.Helper()
	root, err := NewRoot()
	require.NoError(t, err)
	pages, err := Render(root)
	require.NoError(t, err)
	return root, pages
}

// TestCommittedReferenceIsCurrent fails when a command or flag changed without
// `make docs-cli`, the same check CI makes with git diff.
//
//nolint:paralleltest // builds the root command, which shares process-wide config state
func TestCommittedReferenceIsCurrent(t *testing.T) {
	_, pages := renderProduction(t)

	committed, err := filepath.Glob(filepath.Join(committedDir, "*.md"))
	require.NoError(t, err)
	names := make([]string, 0, len(committed))
	for _, path := range committed {
		names = append(names, filepath.Base(path))
	}
	want := make([]string, 0, len(pages))
	for name := range pages {
		want = append(want, name)
	}
	require.ElementsMatch(t, want, names, "page set differs; run make docs-cli")

	for name, page := range pages {
		got, readErr := os.ReadFile(filepath.Join(committedDir, name))
		require.NoError(t, readErr)
		assert.Equal(t, string(page), string(got), "%s is stale; run make docs-cli", name)
	}
}

// TestEveryVisibleCommandAndFlagIsDocumented walks the production tree: each
// command that --help lists has a page titled with its path, and each flag
// that its --help lists appears on that page.
//
//nolint:paralleltest // builds the root command, which shares process-wide config state
func TestEveryVisibleCommandAndFlagIsDocumented(t *testing.T) {
	root, pages := renderProduction(t)

	documented := 0
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		page, ok := pages[FileName(c)]
		require.True(t, ok, "no page for %q", c.CommandPath())
		documented++
		assert.Contains(t, string(page), "\ntitle: "+c.CommandPath()+"\n")
		visit := func(f *pflag.Flag) {
			if !f.Hidden {
				listed := regexp.MustCompile(`--` + regexp.QuoteMeta(f.Name) + `[ \[]`)
				assert.Regexp(t, listed, string(page), "%s: flag --%s", c.CommandPath(), f.Name)
			}
		}
		c.LocalFlags().VisitAll(visit)
		c.InheritedFlags().VisitAll(visit)
		for _, child := range c.Commands() {
			if Documented(child) {
				walk(child)
			}
		}
	}
	walk(root)
	assert.Len(t, pages, documented, "a page exists for a command --help does not list")

	for _, axFlag := range []string{"format", "dry-run", "yes", "strict", "idempotency-key"} {
		assert.NotNil(t, root.PersistentFlags().Lookup(axFlag), "ax.Execute flag --%s is not mounted", axFlag)
	}
	for _, name := range []string{"finfocus_completion.md", "finfocus___schema.md", "finfocus_mcp-server.md"} {
		assert.Contains(t, pages, name)
	}
	assert.NotContains(t, pages, "finfocus_help.md")
}

//nolint:paralleltest // builds the root command, which shares process-wide config state
func TestPageLinksResolve(t *testing.T) {
	_, pages := renderProduction(t)

	link := regexp.MustCompile(`\]\(\./([^)]+)\)`)
	for name, page := range pages {
		for _, m := range link.FindAllStringSubmatch(string(page), -1) {
			assert.Contains(t, pages, m[1], "%s links to a missing page", name)
		}
	}
}

// TestOverviewLinksEveryPage keeps the hand-written command overview in step
// with the generated pages: it must link each page, and only pages that exist.
//
//nolint:paralleltest // builds the root command, which shares process-wide config state
func TestOverviewLinksEveryPage(t *testing.T) {
	_, pages := renderProduction(t)

	overview, err := os.ReadFile("../../docs/src/content/docs/reference/cli-commands.md")
	require.NoError(t, err)

	linked := make(map[string]bool)
	for _, m := range regexp.MustCompile(`\]\(\./cli/([^)#]+)`).FindAllStringSubmatch(string(overview), -1) {
		linked[m[1]] = true
		assert.Contains(t, pages, m[1], "cli-commands.md links to a missing page")
	}
	for name := range pages {
		assert.True(t, linked[name], "cli-commands.md does not link %s", name)
	}
}

func TestCommandsSkipsHiddenAndDeprecated(t *testing.T) {
	t.Parallel()

	run := func(*cobra.Command, []string) {}
	root := &cobra.Command{Use: "app"}
	beta := &cobra.Command{Use: "beta", Short: "b", Run: run}
	alpha := &cobra.Command{Use: "alpha", Short: "a"}
	alpha.AddCommand(&cobra.Command{Use: "leaf", Short: "l", Run: run})
	root.AddCommand(
		beta,
		alpha,
		&cobra.Command{Use: "secret", Short: "s", Hidden: true, Run: run},
		&cobra.Command{Use: "old", Short: "o", Deprecated: "use beta", Run: run},
	)

	var paths []string
	for _, c := range Commands(root) {
		paths = append(paths, c.CommandPath())
	}
	assert.Equal(t, []string{"app", "app alpha", "app alpha leaf", "app beta"}, paths)
}

func TestRenderPage(t *testing.T) {
	t.Parallel()

	root := &cobra.Command{Use: "app", Short: "App root"}
	child := &cobra.Command{
		Use:     "run <name>",
		Short:   "Run a thing",
		Long:    "Runs <name>.\n  # not a heading\n",
		Example: "  app run demo",
		Run:     func(*cobra.Command, []string) {},
	}
	child.Flags().String("mode", "", "run mode")
	root.AddCommand(child)

	pages, err := Render(root)
	require.NoError(t, err)
	require.Len(t, pages, 2)

	want := "---\n" +
		"title: app run\n" +
		"description: Run a thing\n" +
		"editUrl: false\n" +
		"sidebar:\n" +
		"  label: run\n" +
		"  order: 2\n" +
		"---\n\n" +
		generatedNotice + "\n\n" +
		"Run a thing\n\n" +
		"## Synopsis\n\n" +
		"```text\nRuns <name>.\n  # not a heading\n```\n\n" +
		"```text\napp run <name> [flags]\n```\n\n" +
		"## Examples\n\n" +
		"```bash\n  app run demo\n```\n\n" +
		"## Options\n\n" +
		"```text\n  -h, --help[=true|false]   help for run\n      --mode string         run mode\n```\n\n" +
		"## See also\n\n" +
		"- [`app`](./app.md) -\n  App root\n"
	assert.Equal(t, want, string(pages["app_run.md"]))
	assert.Equal(t, "Runs <name>.\n  # not a heading\n", child.Long, "Render must not change the command")
	assert.Contains(t, string(pages["app.md"]), "- [`app run`](./app_run.md) -\n  Run a thing\n")
}

func TestToStarlight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "drops the command heading and promotes sections",
			in:   "## app\n\nShort\n\n### Synopsis\n\ntext\n",
			want: "Short\n\n## Synopsis\n\ntext\n",
		},
		{
			name: "labels bare fences by section",
			in:   "### Examples\n\n```\napp\n```\n\n### Options\n\n```\n-h\n```\n",
			want: "## Examples\n\n```bash\napp\n```\n\n## Options\n\n```text\n-h\n```\n",
		},
		{
			name: "leaves fence content alone",
			in:   "```\n### not a heading\n* not a list\n\n\nkept\n```\n",
			want: "```text\n### not a heading\n* not a list\n\n\nkept\n```\n",
		},
		{
			name: "rewrites the see-also list",
			in:   "### SEE ALSO\n\n* [app](./app.md)\t - App\n\n\n",
			want: "## See also\n\n- [`app`](./app.md) -\n  App\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, toStarlight(tt.in))
		})
	}
}

func TestWithAliases(t *testing.T) {
	t.Parallel()

	body := "Short\n\n## Synopsis\n"
	assert.Equal(t, body, withAliases(body, nil))
	assert.Equal(t, "Short\n\nAliases: `ov`, `o`\n\n## Synopsis\n", withAliases(body, []string{"ov", "o"}))
}

func TestFencedLong(t *testing.T) {
	t.Parallel()

	assert.Empty(t, fencedLong(" \n"))
	assert.Equal(t, "```\nline\n```", fencedLong("line\n\n"))
}

func TestWriteRemovesStalePages(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app_gone.md"), []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("keep"), 0o600))

	require.NoError(t, Write(dir, map[string][]byte{"app.md": []byte("new\n")}))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Equal(t, []string{"app.md", "notes.txt"}, names)
	got, err := os.ReadFile(filepath.Join(dir, "app.md"))
	require.NoError(t, err)
	assert.Equal(t, "new\n", string(got))
}
