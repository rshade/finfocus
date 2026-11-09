package cli

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/logging"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
	"github.com/rshade/finfocus/internal/registry"
)

const escOpenBudget = 5 * time.Second

type escWarnSink struct {
	mu   sync.Mutex
	hold bool
	buf  []string
	emit func(string)
}

func newESCWarnSink(cmd *cobra.Command) *escWarnSink {
	return &escWarnSink{emit: func(text string) {
		cmd.PrintErrln("Warning: " + text)
	}}
}

func (s *escWarnSink) Warn(text string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hold {
		s.buf = append(s.buf, text)
		return
	}
	s.emit(text)
}

// Hold keeps warnings until Release. Overview uses it so the TUI is not overwritten.
func (s *escWarnSink) Hold() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.hold = true
	s.mu.Unlock()
}

// Release prints each held warning once.
func (s *escWarnSink) Release() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.hold = false
	lines := s.buf
	s.buf = nil
	emit := s.emit
	s.mu.Unlock()
	for _, line := range lines {
		emit(line)
	}
}

type escWarnKey struct{}

func contextWithESCWarn(ctx context.Context, sink *escWarnSink) context.Context {
	return context.WithValue(ctx, escWarnKey{}, sink)
}

func escWarnFrom(ctx context.Context) *escWarnSink {
	sink, _ := ctx.Value(escWarnKey{}).(*escWarnSink)
	return sink
}

// HoldESCWarnings starts collecting ESC warnings on ctx.
func HoldESCWarnings(ctx context.Context) {
	if sink := escWarnFrom(ctx); sink != nil {
		sink.Hold()
	}
}

// ReleaseESCWarnings prints the warnings collected since HoldESCWarnings.
func ReleaseESCWarnings(ctx context.Context) {
	if sink := escWarnFrom(ctx); sink != nil {
		sink.Release()
	}
}

// attachESCSource stores a lazy ESC resolver on the command context.
// A missing project or stack is logged and does not fail the command.
// A malformed configured reference fails every command except config,
// which can still report or repair the value.
func attachESCSource(cmd *cobra.Command, cwd string) error {
	config.SetESCCatalogNames(embeddedCatalogNames)
	global, perPlugin, err := escSelection(config.InitializedGlobalConfig())
	if err != nil && !inConfigCommand(cmd) {
		return toValidationError(cmd.Context(), err)
	}
	sink := newESCWarnSink(cmd)
	src := esc.NewSource(esc.Options{
		Global:    global,
		PerPlugin: perPlugin,
		Stack:     loadESCStack(cmd, cwd),
		UseStack:  true,
		Getenv:    os.Getenv,
		Budget:    escOpenBudget,
		Warn:      sink.Warn,
	})
	ctx := esc.WithSource(cmd.Context(), src)
	cmd.SetContext(contextWithESCWarn(ctx, sink))
	return nil
}

func embeddedCatalogNames() []string {
	names, err := registry.ListPluginsFromRegistry()
	if err != nil {
		return nil
	}
	return names
}

func escSelection(cfg *config.Config) (*[]string, map[string]esc.PluginOptions, error) {
	if cfg == nil || cfg.ESC == nil {
		return nil, nil, nil
	}
	if err := cfg.FirstESCError(); err != nil {
		return nil, nil, err
	}
	return cfg.ESC.Environments, config.PluginOptionsForESC(cfg.ESC), nil
}

func inConfigCommand(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Name() == "config" {
			return true
		}
	}
	return false
}

func analyzerESCContext(ctx context.Context, cmd *cobra.Command) context.Context {
	opts := esc.Options{
		UseStack: false,
		Getenv:   os.Getenv,
		Budget:   escOpenBudget,
		Warn: func(text string) {
			if cmd != nil {
				cmd.PrintErrln("Warning: " + text)
			}
		},
	}
	if src, ok := esc.SourceFromContext(ctx); ok {
		opts = src.Options()
		opts.UseStack = false
		opts.Stack = pulumidetect.StackContext{}
	}
	return esc.WithSource(ctx, esc.NewSource(opts))
}

func loadESCStack(cmd *cobra.Command, cwd string) pulumidetect.StackContext {
	projectDir, err := pulumidetect.FindProject(cwd)
	if err != nil {
		return pulumidetect.StackContext{}
	}
	return loadStackContextQuietly(cmd.Context(), projectDir, stackFlagName(cmd))
}

// loadStackContextQuietly reads a stack's own settings. An unreadable file is
// an absent stack context, logged at debug, never an error for the caller.
func loadStackContextQuietly(ctx context.Context, projectDir, stack string) pulumidetect.StackContext {
	loaded, err := pulumidetect.LoadStackContext(projectDir, stack)
	if err != nil {
		logging.FromContext(ctx).Debug().
			Str("component", "esc").
			Str("operation", "stack_context").
			Err(err).
			Msg("stack context unavailable")
		return pulumidetect.StackContext{}
	}
	return loaded
}

// stackESCSource returns the ESC source for one stack: the root source's
// global and per-plugin configuration with the stack environments read from
// that stack's own Pulumi.<stack>.yaml. Each stack gets its own source because
// a source memoizes opened environments. An empty stack, or no root source,
// returns root unchanged so a run without a named stack resolves ESC as before.
// The raw --stack flag is never consulted: a list value such as "dev,tailscale"
// names no settings file.
func stackESCSource(ctx context.Context, root *esc.Source, projectDir, stack string) *esc.Source {
	stack = strings.TrimSpace(stack)
	if root == nil || stack == "" {
		return root
	}
	opts := root.Options()
	opts.Stack = loadStackContextQuietly(ctx, projectDir, stack)
	return esc.NewSource(opts)
}
