package esc

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pulumi"
)

// Delivery is the merged ESC environment for one plugin, before the allowlist.
type Delivery struct {
	Values       map[string]string
	Origins      map[string]Ref
	Fingerprints map[string]string
}

// PluginOptions is the esc.plugins.<name> configuration for one plugin.
type PluginOptions struct {
	Environments *[]string
	Variables    []string
}

// Options configures one Source. Every field is a plain value.
type Options struct {
	Global     *[]string
	PerPlugin  map[string]PluginOptions
	Stack      pulumi.StackContext
	UseStack   bool
	Getenv     func(string) string
	HTTPClient *http.Client
	Budget     time.Duration
	Warn       func(text string)
}

// Source resolves ESC environments for one command. It is not safe to reuse
// across commands, web reloads, or MCP calls.
type Source struct {
	opts    Options
	once    sync.Once
	session authSession
	client  *sdkClient
	authErr *Failure

	mu            sync.Mutex
	memo          map[string]memoEntry
	flights       map[string]*flight
	warned        map[warnKey]struct{}
	charged       time.Duration
	inflight      int
	inflightSince time.Time
}

type memoEntry struct {
	vars     map[string]entry
	revision string
	fail     *Failure
}

type flight struct {
	memoEntry

	done chan struct{}
}

type warnKey struct {
	ref      string
	category Category
}

type chosen struct {
	ref       Ref
	fromStack bool
}

type opened struct {
	ref  Ref
	vars map[string]entry
	rev  string
}

type sourceContextKey struct{}

// NewSource returns a resolver that opens environments on first use.
func NewSource(opts Options) *Source {
	return &Source{
		opts:    opts,
		memo:    map[string]memoEntry{},
		flights: map[string]*flight{},
		warned:  map[warnKey]struct{}{},
	}
}

// Options returns a copy of the options used to build s.
func (s *Source) Options() Options {
	if s == nil {
		return Options{}
	}
	return copyOptions(s.opts)
}

// Fresh returns a new Source with the same options and an empty cache.
func (s *Source) Fresh() *Source {
	if s == nil {
		return NewSource(Options{})
	}
	return NewSource(s.Options())
}

// Variables returns the extra environment names plugin may receive from ESC.
func (s *Source) Variables(plugin string) []string {
	if s == nil {
		return nil
	}
	opts, ok := s.opts.PerPlugin[plugin]
	if !ok || len(opts.Variables) == 0 {
		return nil
	}
	return append([]string(nil), opts.Variables...)
}

func copyOptions(opts Options) Options {
	if opts.Global != nil {
		list := append([]string(nil), (*opts.Global)...)
		opts.Global = &list
	}
	if opts.PerPlugin == nil {
		return opts
	}
	copied := make(map[string]PluginOptions, len(opts.PerPlugin))
	for name, plugin := range opts.PerPlugin {
		plugin.Variables = append([]string(nil), plugin.Variables...)
		if plugin.Environments != nil {
			list := append([]string(nil), (*plugin.Environments)...)
			plugin.Environments = &list
		}
		copied[name] = plugin
	}
	opts.PerPlugin = copied
	return opts
}

// WithSource returns a context that carries s.
func WithSource(ctx context.Context, src *Source) context.Context {
	return context.WithValue(ctx, sourceContextKey{}, src)
}

// SourceFromContext returns the Source stored by WithSource.
func SourceFromContext(ctx context.Context) (*Source, bool) {
	if ctx == nil {
		return nil, false
	}
	src, ok := ctx.Value(sourceContextKey{}).(*Source)
	if !ok || src == nil {
		return nil, false
	}
	return src, true
}

// Delivery returns every resolved name except reserved startup names.
// A failure is reported through Options.Warn and yields an empty Delivery.
// Delivery never returns an error.
func (s *Source) Delivery(ctx context.Context, plugin string) Delivery {
	if s == nil {
		return Delivery{}
	}
	refs, unsupported := s.selection(plugin)
	if unsupported {
		s.warn(ctx, Ref{}, CategoryUnsupportedDefinition)
	}
	if len(refs) == 0 {
		return Delivery{}
	}
	client, fail := s.ensure()
	if fail != nil {
		s.warn(ctx, fail.Ref, fail.Category)
		return Delivery{}
	}
	return mergeOpened(s.openAll(ctx, client, refs))
}

func (s *Source) ensure() (*sdkClient, *Failure) {
	s.once.Do(func() {
		session, fail := discover(s.opts.Getenv, s.opts.Stack.BackendURL)
		if fail != nil {
			s.authErr = fail
			return
		}
		client, err := newSDKClient(session.BackendURL, session.Token, s.opts.HTTPClient)
		if err != nil {
			s.authErr = &Failure{Category: CategoryUnavailable}
			return
		}
		s.session = session
		s.client = client
	})
	return s.client, s.authErr
}

func (s *Source) selection(plugin string) ([]chosen, bool) {
	if opts, ok := s.opts.PerPlugin[plugin]; ok && opts.Environments != nil {
		return parseConfigured(*opts.Environments, false), false
	}
	if s.opts.Global != nil {
		return parseConfigured(*s.opts.Global, false), false
	}
	if !s.opts.UseStack {
		return nil, false
	}
	switch s.opts.Stack.Environment.Form {
	case pulumi.StackEnvImports:
		return parseConfigured(s.opts.Stack.Environment.Imports, true), false
	case pulumi.StackEnvUnsupported:
		return nil, true
	case pulumi.StackEnvAbsent:
		return nil, false
	default:
		return nil, false
	}
}

func parseConfigured(raw []string, fromStack bool) []chosen {
	out := make([]chosen, 0, len(raw))
	for _, item := range raw {
		ref, err := ParseRef(item)
		if err != nil {
			continue
		}
		out = append(out, chosen{ref: ref, fromStack: fromStack})
	}
	return out
}

func (s *Source) openAll(ctx context.Context, client *sdkClient, refs []chosen) []opened {
	results := make([]opened, len(refs))
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref chosen) {
			defer wg.Done()
			results[i] = s.openChosen(ctx, client, ref)
		}(i, ref)
	}
	wg.Wait()
	return results
}

func (s *Source) openChosen(ctx context.Context, client *sdkClient, item chosen) opened {
	ref := item.ref
	if ref.Org == "" {
		org, fail := s.session.orgFor(ref, item.fromStack, s.opts.Stack.Stack, s.opts.Getenv)
		if fail != nil {
			s.warn(ctx, ref, fail.Category)
			return opened{}
		}
		ref.Org = org
	}
	vars, revision, fail := s.openRef(ctx, client, ref)
	if fail != nil {
		s.warn(ctx, ref, fail.Category)
		return opened{}
	}
	return opened{ref: ref, vars: vars, rev: revision}
}

func (s *Source) openRef(ctx context.Context, client *sdkClient, ref Ref) (map[string]entry, string, *Failure) {
	key := ref.String()
	s.mu.Lock()
	if entry, ok := s.memo[key]; ok {
		s.mu.Unlock()
		return entry.vars, entry.revision, entry.fail
	}
	if existing, ok := s.flights[key]; ok {
		s.mu.Unlock()
		return s.waitFlight(ctx, existing)
	}
	created := &flight{done: make(chan struct{})}
	s.flights[key] = created
	s.mu.Unlock()

	vars, revision, fail := s.fetch(ctx, client, ref)
	s.mu.Lock()
	delete(s.flights, key)
	created.memoEntry = memoEntry{vars: vars, revision: revision, fail: fail}
	s.memo[key] = created.memoEntry
	close(created.done)
	s.mu.Unlock()
	return vars, revision, fail
}

func (s *Source) waitFlight(ctx context.Context, existing *flight) (map[string]entry, string, *Failure) {
	select {
	case <-existing.done:
		return existing.vars, existing.revision, existing.fail
	case <-ctx.Done():
		classified := classify(ctx, ctx.Err())
		return nil, "", &classified
	}
}

func (s *Source) fetch(ctx context.Context, client *sdkClient, ref Ref) (map[string]entry, string, *Failure) {
	callCtx, end, fail := s.startCall(ctx)
	if fail != nil {
		fail.Ref = ref
		return nil, "", fail
	}
	defer end()
	vars, revision, err := client.open(callCtx, ref)
	if err != nil {
		classified := classify(callCtx, err)
		classified.Ref = ref
		return nil, "", &classified
	}
	return vars, revision, nil
}

func (s *Source) startCall(parent context.Context) (context.Context, func(), *Failure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	remaining := s.remaining(time.Now())
	if remaining <= 0 {
		return nil, nil, &Failure{Category: CategoryTimedOut}
	}
	if s.inflight == 0 {
		s.inflightSince = time.Now()
	}
	s.inflight++
	ctx, cancel := context.WithTimeout(parent, remaining)
	return ctx, s.endCall(cancel), nil
}

func (s *Source) endCall(cancel context.CancelFunc) func() {
	return func() {
		cancel()
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.inflight == 0 {
			return
		}
		s.inflight--
		if s.inflight == 0 {
			s.charged += time.Since(s.inflightSince)
		}
	}
}

func (s *Source) remaining(now time.Time) time.Duration {
	charged := s.charged
	if s.inflight > 0 {
		charged += now.Sub(s.inflightSince)
	}
	if charged >= s.opts.Budget {
		return 0
	}
	return s.opts.Budget - charged
}

func (s *Source) warn(ctx context.Context, ref Ref, category Category) {
	if !s.markWarned(ref, category) {
		return
	}
	text := Failure{Ref: ref, Category: category}.Error()
	logging.FromContext(ctx).Warn().
		Str("component", "esc").
		Str("operation", "resolve").
		Str("ref", ref.String()).
		Str("category", string(category)).
		Msg("skipped ESC environment")
	if s.opts.Warn != nil {
		s.opts.Warn(text)
	}
}

func (s *Source) markWarned(ref Ref, category Category) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := warnKey{ref: ref.String(), category: category}
	if _, ok := s.warned[key]; ok {
		return false
	}
	s.warned[key] = struct{}{}
	return true
}

func mergeOpened(parts []opened) Delivery {
	values := map[string]string{}
	origins := map[string]Ref{}
	fingerprints := map[string]string{}
	for _, part := range parts {
		for name, item := range part.vars {
			if reservedName(name) {
				continue
			}
			values[name] = item.value
			origins[name] = part.ref
			fingerprints[name] = part.rev
		}
	}
	return Delivery{Values: values, Origins: origins, Fingerprints: fingerprints}
}

func reservedName(name string) bool {
	switch name {
	case pluginsdk.EnvPort, pluginsdk.EnvLogFile, pluginsdk.EnvTraceID:
		return true
	default:
		return false
	}
}
