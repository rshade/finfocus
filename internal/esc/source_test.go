package esc

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/internal/pulumi"
)

func TestSourceSingleflightAndMerge(t *testing.T) {
	t.Parallel()
	srv := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/one": {Variables: map[string]esctest.Variable{
			"FOO": {Value: "from-one"},
			"BAR": {Value: "from-one"},
		}, ETag: "e1"},
		"acme/billing/two": {Variables: map[string]esctest.Variable{
			"BAR":                  {Value: "from-two"},
			"FINFOCUS_PLUGIN_PORT": {Value: "9999"},
			"UNRELATED":            {Value: "secret-value-do-not-log"},
		}, ETag: "e2"},
	})
	t.Cleanup(srv.Close)

	var (
		mu       sync.Mutex
		warnings []string
	)
	var logs bytes.Buffer
	logger := zerolog.New(&logs).Level(zerolog.DebugLevel)
	src := NewSource(Options{
		UseStack: true,
		Stack: pulumi.StackContext{
			Environment: pulumi.StackEnvironment{
				Form:    pulumi.StackEnvImports,
				Imports: []string{"acme/billing/one", "acme/billing/two"},
			},
		},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
		Warn: func(text string) {
			mu.Lock()
			warnings = append(warnings, text)
			mu.Unlock()
		},
	})
	ctx := logger.WithContext(context.Background())

	var wg sync.WaitGroup
	got := make([]Delivery, 8)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = src.Delivery(ctx, "recorder")
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 1, srv.Opens("acme", "billing", "one"))
	assert.Equal(t, 1, srv.Opens("acme", "billing", "two"))
	assert.Empty(t, warnings)
	for _, delivery := range got {
		assert.Equal(t, "from-one", delivery.Values["FOO"])
		assert.Equal(t, "from-two", delivery.Values["BAR"])
		assert.NotContains(t, delivery.Values, "FINFOCUS_PLUGIN_PORT")
		assert.Equal(t, "acme/billing/one", delivery.Origins["FOO"].String())
		assert.Equal(t, "acme/billing/two", delivery.Origins["BAR"].String())
		assert.Equal(t, "e2", delivery.Fingerprints["BAR"])
	}
	assert.NotContains(t, logs.String(), "secret-value-do-not-log")
}

func TestSourceNoSelection(t *testing.T) {
	t.Parallel()
	srv := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/one": {ETag: "e"},
	})
	t.Cleanup(srv.Close)
	var warnings []string
	src := NewSource(Options{
		UseStack: true,
		Stack: pulumi.StackContext{
			Environment: pulumi.StackEnvironment{Form: pulumi.StackEnvAbsent},
		},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
		Warn:   func(text string) { warnings = append(warnings, text) },
	})
	got := src.Delivery(context.Background(), "recorder")
	assert.Empty(t, got.Values)
	assert.Empty(t, warnings)
	assert.Equal(t, 0, srv.Opens("acme", "billing", "one"))
}

func TestSourceBudget(t *testing.T) {
	t.Parallel()
	t.Run("overlapping opens share the budget", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/slow1": {
				Delay:     80 * time.Millisecond,
				ETag:      "e",
				Variables: map[string]esctest.Variable{"A": {Value: "1"}},
			},
			"acme/billing/slow2": {
				Delay:     80 * time.Millisecond,
				ETag:      "e",
				Variables: map[string]esctest.Variable{"B": {Value: "2"}},
			},
		})
		t.Cleanup(srv.Close)
		src := NewSource(Options{
			UseStack: true,
			Stack: pulumi.StackContext{Environment: pulumi.StackEnvironment{
				Form:    pulumi.StackEnvImports,
				Imports: []string{"acme/billing/slow1", "acme/billing/slow2"},
			}},
			Getenv: testGetenv(srv.URL),
			Budget: 400 * time.Millisecond,
		})
		start := time.Now()
		got := src.Delivery(context.Background(), "recorder")
		// Open is a POST plus a GET, so one reference takes about 240ms.
		// Sequential opens would take about 480ms.
		assert.Less(t, time.Since(start), 400*time.Millisecond)
		assert.Equal(t, "1", got.Values["A"])
		assert.Equal(t, "2", got.Values["B"])
	})

	t.Run("idle time is not charged", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/a": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "1"}}},
			"acme/billing/b": {ETag: "e", Variables: map[string]esctest.Variable{"B": {Value: "2"}}},
		})
		t.Cleanup(srv.Close)
		src := NewSource(Options{
			PerPlugin: map[string]PluginOptions{
				"first":  {Environments: ptrList("acme/billing/a")},
				"second": {Environments: ptrList("acme/billing/b")},
			},
			Getenv: testGetenv(srv.URL),
			Budget: 40 * time.Millisecond,
		})
		first := src.Delivery(context.Background(), "first")
		assert.Equal(t, "1", first.Values["A"])
		time.Sleep(80 * time.Millisecond)
		second := src.Delivery(context.Background(), "second")
		assert.Equal(t, "2", second.Values["B"])
	})

	t.Run("exhausted budget fails at once", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/slow": {Delay: 200 * time.Millisecond, ETag: "e"},
			"acme/billing/fast": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "1"}}},
		})
		t.Cleanup(srv.Close)
		var warnings []string
		src := NewSource(Options{
			PerPlugin: map[string]PluginOptions{
				"slow": {Environments: ptrList("acme/billing/slow")},
				"fast": {Environments: ptrList("acme/billing/fast")},
			},
			Getenv: testGetenv(srv.URL),
			Budget: 50 * time.Millisecond,
			Warn:   func(text string) { warnings = append(warnings, text) },
		})
		_ = src.Delivery(context.Background(), "slow")
		start := time.Now()
		got := src.Delivery(context.Background(), "fast")
		assert.Less(t, time.Since(start), 30*time.Millisecond)
		assert.Empty(t, got.Values)
		assert.Equal(t, 0, srv.Opens("acme", "billing", "fast"))
		assert.Contains(t, warnings[len(warnings)-1], "timed_out")
	})
}

func TestSourceFailuresAndStackRules(t *testing.T) {
	t.Parallel()
	t.Run("one warning and a healthy sibling", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/bad":  {Status: 404, Message: "missing"},
			"acme/billing/good": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "kept"}}},
		})
		t.Cleanup(srv.Close)
		var (
			mu       sync.Mutex
			warnings []string
		)
		src := NewSource(Options{
			UseStack: true,
			Stack: pulumi.StackContext{Environment: pulumi.StackEnvironment{
				Form:    pulumi.StackEnvImports,
				Imports: []string{"acme/billing/bad", "acme/billing/good"},
			}},
			Getenv: testGetenv(srv.URL),
			Budget: time.Second,
			Warn: func(text string) {
				mu.Lock()
				warnings = append(warnings, text)
				mu.Unlock()
			},
		})
		var wg sync.WaitGroup
		for range 5 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				got := src.Delivery(context.Background(), "recorder")
				assert.Equal(t, "kept", got.Values["A"])
			}()
		}
		wg.Wait()
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "not found or no access")
	})

	t.Run("use stack false ignores imports", func(t *testing.T) {
		t.Parallel()
		srv := esctest.New("tok", map[string]*esctest.Environment{
			"acme/billing/one": {ETag: "e"},
		})
		t.Cleanup(srv.Close)
		src := NewSource(Options{
			UseStack: false,
			Stack: pulumi.StackContext{Environment: pulumi.StackEnvironment{
				Form:    pulumi.StackEnvImports,
				Imports: []string{"acme/billing/one"},
			}},
			Getenv: testGetenv(srv.URL),
			Budget: time.Second,
		})
		assert.Empty(t, src.Delivery(context.Background(), "recorder").Values)
		assert.Equal(t, 0, srv.Opens("acme", "billing", "one"))
	})

	t.Run("unsupported definition", func(t *testing.T) {
		t.Parallel()
		var warnings []string
		src := NewSource(Options{
			UseStack: true,
			Stack:    pulumi.StackContext{Environment: pulumi.StackEnvironment{Form: pulumi.StackEnvUnsupported}},
			Getenv:   func(string) string { return "" },
			Budget:   time.Second,
			Warn:     func(text string) { warnings = append(warnings, text) },
		})
		assert.Empty(t, src.Delivery(context.Background(), "recorder").Values)
		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "unsupported_definition")
	})
}

func TestSourceSelectionOrder(t *testing.T) {
	t.Parallel()
	srv := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/plugin": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "plugin"}}},
		"acme/billing/global": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "global"}}},
		"acme/billing/stack":  {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "stack"}}},
	})
	t.Cleanup(srv.Close)
	pluginList := []string{"acme/billing/plugin"}
	globalList := []string{"acme/billing/global"}
	empty := []string{}
	src := NewSource(Options{
		Global: &globalList,
		PerPlugin: map[string]PluginOptions{
			"recorder": {Environments: &pluginList},
			"quiet":    {Environments: &empty},
		},
		UseStack: true,
		Stack: pulumi.StackContext{Environment: pulumi.StackEnvironment{
			Form: pulumi.StackEnvImports, Imports: []string{"acme/billing/stack"},
		}},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
	})
	assert.Equal(t, "plugin", src.Delivery(context.Background(), "recorder").Values["A"])
	assert.Equal(t, "global", src.Delivery(context.Background(), "other").Values["A"])
	assert.Empty(t, src.Delivery(context.Background(), "quiet").Values)
	assert.Equal(t, 0, srv.Opens("acme", "billing", "stack"))

	disabled := NewSource(Options{
		Global: &empty,
		PerPlugin: map[string]PluginOptions{
			"recorder": {Environments: &pluginList},
		},
		UseStack: true,
		Stack: pulumi.StackContext{Environment: pulumi.StackEnvironment{
			Form: pulumi.StackEnvImports, Imports: []string{"acme/billing/stack"},
		}},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
	})
	assert.Equal(t, "plugin", disabled.Delivery(context.Background(), "recorder").Values["A"])
	assert.Empty(t, disabled.Delivery(context.Background(), "other").Values)
}

func TestSourceOptionsFreshAndContext(t *testing.T) {
	t.Parallel()
	assert.Empty(t, (*Source)(nil).Options().PerPlugin)
	assert.NotNil(t, (*Source)(nil).Fresh())
	assert.Nil(t, (*Source)(nil).Variables("recorder"))
	assert.Empty(t, (*Source)(nil).Delivery(context.Background(), "recorder").Values)
	_, ok := SourceFromContext(nil) //nolint:staticcheck // SA1012: nil is the context SourceFromContext rejects.
	assert.False(t, ok)

	global := []string{"billing/global"}
	pluginEnvs := []string{"billing/plugin"}
	src := NewSource(Options{
		Global: &global,
		PerPlugin: map[string]PluginOptions{
			"recorder": {Environments: &pluginEnvs, Variables: []string{"FOO"}},
		},
		Budget: time.Second,
	})
	copied := src.Options()
	(*copied.Global)[0] = "changed"
	copied.PerPlugin["recorder"].Variables[0] = "changed"
	assert.Equal(t, []string{"billing/global"}, *src.Options().Global)
	assert.Equal(t, []string{"FOO"}, src.Variables("recorder"))
	assert.Nil(t, src.Variables("other"))
	fresh := src.Fresh()
	assert.Equal(t, []string{"FOO"}, fresh.Variables("recorder"))
	assert.NotSame(t, src, fresh)

	ctx := WithSource(context.Background(), src)
	got, ok := SourceFromContext(ctx)
	require.True(t, ok)
	assert.Same(t, src, got)
	_, ok = SourceFromContext(context.Background())
	assert.False(t, ok)
}

func TestSourceOrgFromStackAndMissingOrg(t *testing.T) {
	t.Parallel()
	srv := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/plugin": {ETag: "e", Variables: map[string]esctest.Variable{"A": {Value: "from-stack"}}},
	})
	t.Cleanup(srv.Close)
	src := NewSource(Options{
		UseStack: true,
		Stack: pulumi.StackContext{
			Stack: "acme/demo/dev",
			Environment: pulumi.StackEnvironment{
				Form: pulumi.StackEnvImports, Imports: []string{"billing/plugin"},
			},
		},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
	})
	assert.Equal(t, "from-stack", src.Delivery(context.Background(), "recorder").Values["A"])
	assert.Equal(t, "from-stack", src.Delivery(context.Background(), "recorder").Values["A"])
	assert.Equal(t, 1, srv.Opens("acme", "billing", "plugin"))

	var warnings []string
	missing := NewSource(Options{
		UseStack: true,
		Stack: pulumi.StackContext{
			Stack: "dev",
			Environment: pulumi.StackEnvironment{
				Form: pulumi.StackEnvImports, Imports: []string{"billing/plugin"},
			},
		},
		Getenv: testGetenv(srv.URL),
		Budget: time.Second,
		Warn:   func(text string) { warnings = append(warnings, text) },
	})
	assert.Empty(t, missing.Delivery(context.Background(), "recorder").Values)
	require.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "not_logged_in")
}

func TestSourceAuthFailureAndBadRef(t *testing.T) {
	t.Parallel()
	var warnings []string
	list := []string{"a//b", "billing/ok"}
	home := t.TempDir()
	src := NewSource(Options{
		Global: &list,
		Getenv: func(key string) string {
			if key == "PULUMI_HOME" || key == "HOME" {
				return home
			}
			return ""
		},
		Budget: time.Second,
		Warn:   func(text string) { warnings = append(warnings, text) },
	})
	assert.Empty(t, src.Delivery(context.Background(), "recorder").Values)
	require.NotEmpty(t, warnings)
	assert.Contains(t, warnings[0], "not logged in")

	only := []string{"billing/only"}
	plain := NewSource(Options{Global: &only, Budget: time.Second})
	assert.Nil(t, plain.Options().PerPlugin)
	assert.Equal(t, []string{"billing/only"}, *plain.Options().Global)
}

func testGetenv(backend string) func(string) string {
	return func(key string) string {
		switch key {
		case "PULUMI_ACCESS_TOKEN":
			return "tok"
		case "PULUMI_BACKEND_URL":
			return backend
		default:
			return ""
		}
	}
}

func ptrList(values ...string) *[]string {
	return &values
}
