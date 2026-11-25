package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
)

// ErrPoolClosed is returned by Acquire after the pool is closed.
var ErrPoolClosed = errors.New("plugin pool is closed")

// poolProbeTimeout bounds the Name call that checks a running process before
// it is handed to another holder.
const poolProbeTimeout = 5 * time.Second

// poolLearnWait bounds how long a request waits for a binary's first launch to
// report its advertised settings before it launches on its own. A cold
// aws-public start takes several seconds (see pluginProbeTimeout in cli).
const poolLearnWait = 30 * time.Second

type poolContextKey struct{}

// Pool shares plugin processes between holders that would launch the same
// binary with the same environment, so several web stacks do not each start a
// copy of every plugin (specs/2016-shared-plugin-pool/). A process lives until
// its last holder releases it or the pool closes; no holder's context can end
// it.
//
// A process is found by one of two keys. Before a binary has answered
// GetPluginInfo, the pool cannot tell which ESC values the plugin will be
// allowed, so it keys on the launch environment plus everything the holder's
// ESC source offers (an offer key). Once a launch has reported the binary's
// advertised settings, a request computes the exact environment its process
// would receive and keys on that (a final key). Every running process is also
// registered under the final key of what it actually received, so a stack
// whose ESC environments hold values the plugin never sees still shares it.
//
// Holders whose declared credentials differ can share a process of a plugin
// that reads credentials from each call (specs/2023-per-request-credentials/).
// That process starts without any holder's values, and each holder's client
// attaches its own.
type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	entries map[poolKey]*poolEntry
	live    map[*poolEntry]struct{}
	closed  bool
	// advertised holds each binary's GetPluginInfo settings, learned from its
	// first launch that answered.
	advertised map[binaryID]advertisement
	// learning is closed when the launch that will teach a binary's settings
	// ends. Requests with another offer wait on it instead of launching a
	// second process whose environment may turn out the same.
	learning map[binaryID]chan struct{}
	// learnWaiters counts requests waiting on learning.
	learnWaiters int
	// closing tracks processes stopped outside Close, so Close returns only
	// after they are gone.
	closing sync.WaitGroup
}

// binaryID names one plugin file: its path plus its size and modification
// time, so a reinstall at the same path is a different binary.
type binaryID struct {
	path  string
	stamp string
}

// advertisement is what a binary's GetPluginInfo answer says about its launches.
type advertisement struct {
	settings    []*pbc.PluginSetting
	credentials bool
	// unproven means a process did not prove that it holds its port. The binary
	// keeps its credentials in the environment from then on.
	unproven bool
}

type keyKind int

const (
	offerKey keyKind = iota
	finalKey
)

type poolKey struct {
	binary binaryID
	kind   keyKind
	env    string
	// channel separates a process that reads credentials from calls from one
	// with the same environment that does not. Only final keys set it.
	channel bool
}

// poolEntry is one shared process. holds counts requesters still waiting for
// the launch plus holders that have not released. keys lists every key that
// maps to it; final is the final key of what the process actually received.
type poolEntry struct {
	binary   binaryID
	learning chan struct{}
	keys     []poolKey
	final    poolKey
	state    launchState
	ready    chan struct{}
	client   *Client
	err      error
	holds    int
	closed   bool
	stop     func()
}

// NewPool returns an empty pool. Its processes stop on Close or when parent ends.
func NewPool(parent context.Context) *Pool {
	ctx, cancel := context.WithCancel(parent)
	return &Pool{
		ctx:        ctx,
		cancel:     cancel,
		entries:    map[poolKey]*poolEntry{},
		live:       map[*poolEntry]struct{}{},
		advertised: map[binaryID]advertisement{},
		learning:   map[binaryID]chan struct{}{},
	}
}

// WithPool returns ctx carrying pool. Plugin opens under that context share
// processes through it.
func WithPool(ctx context.Context, pool *Pool) context.Context {
	return context.WithValue(ctx, poolContextKey{}, pool)
}

// PoolFromContext returns the pool attached by WithPool.
func PoolFromContext(ctx context.Context) (*Pool, bool) {
	pool, ok := ctx.Value(poolContextKey{}).(*Pool)
	return pool, ok && pool != nil
}

// Acquire returns a client for binPath, launching the process only when no
// running process has the same key. It has NewClient's signature. Closing the
// returned client releases this hold once; the process stops with the last.
func (p *Pool) Acquire(ctx context.Context, launcher Launcher, binPath string) (*Client, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	in := resolveLaunchInputs(ctx)
	in.credentials = resolveCredentialPlan(ctx, in.installName, in.sources.escOffer)
	bin := binaryID{path: binPath, stamp: binaryStamp(binPath)}
	predict, learn := true, true
	for {
		key, launchIn := p.keyFor(ctx, bin, in, predict)
		entry, launched, learning, err := p.hold(ctx, launcher, launchIn, key, learn)
		if err != nil {
			return nil, err
		}
		if learning != nil {
			if learn, err = p.waitToLearn(ctx, learning); err != nil {
				return nil, err
			}
			continue
		}
		client, next, err := p.take(ctx, entry, key, in, launched)
		if err != nil {
			return nil, err
		}
		if client != nil {
			logPoolAcquire(ctx, in.installName, binPath, launched)
			return client, nil
		}
		if next == retryWithoutPrediction {
			predict = false
		}
	}
}

type retryKind int

const (
	retrySameKey retryKind = iota
	retryWithoutPrediction
)

// take waits for the held entry and hands its process to the caller, or gives
// the hold back and says how to retry. A process found under a predicted key
// that it did not end with is never handed out: the binary advertised other
// settings than the pool learned, so the offer key, which needs no
// prediction, is used next. A running process that does not answer is
// evicted and the same key launches a replacement.
func (p *Pool) take(
	ctx context.Context, entry *poolEntry, key poolKey, in launchInputs, launched bool,
) (*Client, retryKind, error) {
	select {
	case <-entry.ready:
	case <-ctx.Done():
		_ = p.release(entry)
		return nil, retrySameKey, ctx.Err()
	}
	if entry.err != nil {
		_ = p.release(entry)
		return nil, retrySameKey, entry.err
	}
	if key.kind == finalKey && entry.final != key {
		_ = p.release(entry)
		return nil, retryWithoutPrediction, nil
	}
	if launched || responds(ctx, entry.client) {
		return p.holderClient(ctx, entry, in), retrySameKey, nil
	}
	_ = p.release(entry)
	if err := ctx.Err(); err != nil {
		return nil, retrySameKey, err
	}
	p.evict(entry)
	return nil, retrySameKey, nil
}

// keyFor returns the key to look up and the inputs to launch with. With the
// binary's advertised settings known, it resolves the exact environment the
// process would receive, launches with it directly (no restart), and keys on
// it. Otherwise it keys on the launch environment and the whole ESC offer.
func (p *Pool) keyFor(ctx context.Context, bin binaryID, in launchInputs, predict bool) (poolKey, launchInputs) {
	p.mu.Lock()
	learned, known := p.advertised[bin]
	p.mu.Unlock()
	if learned.unproven {
		in.credentials = credentialPlan{mapping: in.credentials.mapping}
	}
	if !predict || !in.named || !known {
		return poolKey{binary: bin, kind: offerKey, env: environmentKey(in)}, in
	}
	in.start = in.state(ctx, learned.settings, learned.credentials && in.credentials.usable())
	return finalKeyFor(bin, in.start), in
}

// Close stops every process the pool started, held or not, and waits for
// launches in progress. Later Acquire calls fail with ErrPoolClosed.
func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	var running, launching []*poolEntry
	for entry := range p.live {
		switch {
		case !isReady(entry):
			launching = append(launching, entry)
		case entry.err == nil && !entry.closed:
			entry.closed = true
			running = append(running, entry)
		}
	}
	p.entries = map[poolKey]*poolEntry{}
	p.live = map[*poolEntry]struct{}{}
	p.mu.Unlock()

	for _, entry := range running {
		_ = closeEntry(entry)
	}
	p.cancel()
	for _, entry := range launching {
		<-entry.ready
	}
	p.closing.Wait()
}

// hold takes a hold on the entry for key, starting a launch when there is
// none. launched is true when the entry was still launching, so the caller
// waits for a fresh process instead of probing an old one. When the binary's
// settings are unknown and another launch is learning them, hold takes
// nothing and returns that launch's channel to wait on (only when learn).
func (p *Pool) hold(
	ctx context.Context, launcher Launcher, in launchInputs, key poolKey, learn bool,
) (*poolEntry, bool, chan struct{}, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, false, nil, ErrPoolClosed
	}
	entry, ok := p.entries[key]
	if !ok {
		learning, wait := p.learningLocked(key)
		if wait && learn {
			p.learnWaiters++
			return nil, false, learning, nil
		}
		entry = p.startLocked(ctx, launcher, in, key)
		if learning == nil && key.kind == offerKey && !p.knownLocked(key.binary) {
			entry.learning = make(chan struct{})
			p.learning[key.binary] = entry.learning
		}
	}
	entry.holds++
	return entry, !isReady(entry), nil, nil
}

// learningLocked returns the channel of a launch that is learning the
// binary's settings, and whether an offer-key request should wait on it.
func (p *Pool) learningLocked(key poolKey) (chan struct{}, bool) {
	learning := p.learning[key.binary]
	return learning, learning != nil && key.kind == offerKey && !p.knownLocked(key.binary)
}

func (p *Pool) knownLocked(bin binaryID) bool {
	_, known := p.advertised[bin]
	return known
}

// waitToLearn waits for a binary's first launch to end. It reports false when
// the wait timed out, so the caller stops waiting and launches on its own.
func (p *Pool) waitToLearn(ctx context.Context, learning chan struct{}) (bool, error) {
	defer func() {
		p.mu.Lock()
		p.learnWaiters--
		p.mu.Unlock()
	}()
	timer := time.NewTimer(poolLearnWait)
	defer timer.Stop()
	select {
	case <-learning:
		return true, nil
	case <-timer.C:
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// startLocked launches under a context that keeps the requester's values
// (install name, log writers, ESC source) but is cancelled only by the pool,
// because [os/exec.CommandContext] kills the plugin when that context ends.
func (p *Pool) startLocked(
	ctx context.Context, launcher Launcher, in launchInputs, key poolKey,
) *poolEntry {
	launchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stopAfter := context.AfterFunc(p.ctx, cancel)
	entry := &poolEntry{
		binary: key.binary,
		keys:   []poolKey{key},
		ready:  make(chan struct{}),
		stop: func() {
			stopAfter()
			cancel()
		},
	}
	p.entries[key] = entry
	p.live[entry] = struct{}{}
	go p.launch(launchCtx, launcher, key.binary, in, entry)
	return entry
}

func (p *Pool) launch(ctx context.Context, launcher Launcher, bin binaryID, in launchInputs, entry *poolEntry) {
	client, outcome, err := launchClient(ctx, launcher, bin.path, in)

	p.mu.Lock()
	entry.client, entry.err = client, err
	if err == nil {
		p.registerLocked(entry, bin, outcome)
	}
	p.stopLearningLocked(entry)
	idle := false
	switch {
	case err != nil:
		p.forgetLocked(entry)
	case p.closed:
		entry.err = ErrPoolClosed
		entry.closed = true
		idle = true
	case entry.holds == 0:
		entry.closed = true
		p.forgetLocked(entry)
		p.closing.Add(1)
		idle = true
	}
	if !idle {
		close(entry.ready)
	}
	p.mu.Unlock()

	if err != nil {
		entry.stop()
	}
	if idle {
		_ = closeEntry(entry)
		close(entry.ready)
		if entry.err == nil {
			p.closing.Done()
		}
	}
}

// release gives back one hold and stops the process when it was the last. A
// launch every requester has given up on leaves the lookup, so a launch that
// never answers cannot block later requests; it is closed when it finishes.
func (p *Pool) release(entry *poolEntry) error {
	p.mu.Lock()
	entry.holds--
	ready := isReady(entry)
	if entry.holds == 0 && !ready {
		// Nobody wants this launch anymore. Cancelling its context kills the
		// process, as a cancelled requester did before the pool, so a plugin
		// that binds but never answers Name cannot outlive its requesters.
		p.unmapLocked(entry)
		p.stopLearningLocked(entry)
		entry.stop()
	}
	last := entry.holds == 0 && ready && entry.err == nil && !entry.closed
	if last {
		entry.closed = true
		p.forgetLocked(entry)
		p.closing.Add(1)
	}
	p.mu.Unlock()
	if !last {
		return nil
	}
	defer p.closing.Done()
	return closeEntry(entry)
}

// evict stops handing out a process that no longer answers. Its holders keep
// it until they release, and Close still stops it.
func (p *Pool) evict(entry *poolEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.unmapLocked(entry)
}

// registerLocked records what a launch learned. The binary's advertised
// settings let later requests predict their environment. The entry is mapped
// under the final key of what it actually received, and a predicted key that
// turned out wrong stops pointing at it.
func (p *Pool) registerLocked(entry *poolEntry, bin binaryID, outcome launchOutcome) {
	if outcome.info != nil {
		p.advertised[bin] = advertisement{
			settings:    outcome.info.GetSettings(),
			credentials: advertisesCredentials(outcome.info),
			unproven:    outcome.state.unproven || p.advertised[bin].unproven,
		}
	}
	entry.state = outcome.state
	entry.final = finalKeyFor(bin, outcome.state)
	keys := entry.keys[:0]
	for _, key := range entry.keys {
		if key.kind == finalKey && key != entry.final {
			if p.entries[key] == entry {
				delete(p.entries, key)
			}
			continue
		}
		keys = append(keys, key)
	}
	entry.keys = keys
	if _, taken := p.entries[entry.final]; !taken {
		p.entries[entry.final] = entry
		entry.keys = append(entry.keys, entry.final)
	}
}

// stopLearningLocked releases the requests waiting for this entry's launch to
// report the binary's settings: the launch ended, or nobody wants it anymore.
func (p *Pool) stopLearningLocked(entry *poolEntry) {
	if entry.learning == nil {
		return
	}
	if p.learning[entry.binary] == entry.learning {
		delete(p.learning, entry.binary)
	}
	close(entry.learning)
	entry.learning = nil
}

func (p *Pool) unmapLocked(entry *poolEntry) {
	for _, key := range entry.keys {
		if p.entries[key] == entry {
			delete(p.entries, key)
		}
	}
}

func (p *Pool) forgetLocked(entry *poolEntry) {
	p.unmapLocked(entry)
	delete(p.live, entry)
}

// holderClient gives each holder its own Client so per-holder writes, such as
// the registry's metadata merge, never race. Conn and API are shared, unless
// the process reads credentials from calls: then they attach this holder's,
// and the digest covers them, so cost cache keys still tell holders apart.
func (p *Pool) holderClient(ctx context.Context, entry *poolEntry, in launchInputs) *Client {
	held := *entry.client
	if entry.state.channel {
		full := in.state(ctx, entry.state.settings, false).delivered
		held.DeliveredDigest = in.credentials.digest(full.Digest)
		if in.credentials.set.Len() > 0 {
			held.Conn = newCredentialConn(entry.client.Conn, in.credentials.set)
			held.API = proto.NewCostSourceClient(held.Conn)
		}
	}
	held.Metadata = cloneMetadata(entry.client.Metadata)
	var once sync.Once
	var closeErr error
	held.Close = func() error {
		once.Do(func() { closeErr = p.release(entry) })
		return closeErr
	}
	return &held
}

func closeEntry(entry *poolEntry) error {
	err := entry.client.Close()
	entry.stop()
	return err
}

func isReady(entry *poolEntry) bool {
	select {
	case <-entry.ready:
		return true
	default:
		return false
	}
}

func responds(ctx context.Context, client *Client) bool {
	probeCtx, cancel := context.WithTimeout(ctx, poolProbeTimeout)
	defer cancel()
	_, err := client.API.Name(probeCtx, &proto.Empty{})
	return err == nil
}

func cloneMetadata(metadata *proto.PluginMetadata) *proto.PluginMetadata {
	if metadata == nil {
		return nil
	}
	clone := *metadata
	clone.SupportedProviders = slices.Clone(metadata.SupportedProviders)
	clone.Metadata = maps.Clone(metadata.Metadata)
	clone.Capabilities = slices.Clone(metadata.Capabilities)
	return &clone
}

// binaryStamp identifies the file at binPath, so a reinstall of the same
// version at the same path launches the new binary.
func binaryStamp(binPath string) string {
	info, err := os.Stat(binPath)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(info.Size(), 10) + "/" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// finalKeyFor returns the key of a process that received state.
func finalKeyFor(bin binaryID, state launchState) poolKey {
	return poolKey{binary: bin, kind: finalKey, env: deliveryKey(state.delivered), channel: state.channel}
}

// deliveryKey hashes the exact environment a process receives and its
// delivered digest, which carries each ESC value's reference and revision.
func deliveryKey(delivered Delivered) string {
	sum := sha256.New()
	write := func(parts ...string) {
		for _, part := range parts {
			_, _ = sum.Write([]byte(part))
			_, _ = sum.Write([]byte{0})
		}
	}
	for _, pair := range slices.Sorted(slices.Values(delivered.Pairs)) {
		write(pair)
	}
	write("overrides")
	for _, name := range slices.Sorted(maps.Keys(delivered.Overrides)) {
		write(name)
	}
	write("digest", delivered.Digest)
	return hex.EncodeToString(sum.Sum(nil))
}

// environmentKey hashes everything that can decide a process's final
// environment and its delivered digest before the binary's advertised
// settings are known: the launch pairs, the names that replace an inherited
// value, the parent environment and the plugin's config values (a descriptor
// can pick up a config value the launch pairs did not show), and the whole ESC
// offer (value, reference, and revision), including names allowed only after
// the plugin advertises them, and the declared credential map. Values are hashed, never kept or logged.
func environmentKey(in launchInputs) string {
	sum := sha256.New()
	write := func(parts ...string) {
		for _, part := range parts {
			_, _ = sum.Write([]byte(part))
			_, _ = sum.Write([]byte{0})
		}
	}
	write("launch")
	for _, pair := range slices.Sorted(slices.Values(in.start.delivered.Pairs)) {
		write(pair)
	}
	write("overrides")
	for _, name := range slices.Sorted(maps.Keys(in.start.delivered.Overrides)) {
		write(name)
	}
	write("parent")
	for _, entry := range slices.Sorted(slices.Values(in.sources.parent)) {
		write(entry)
	}
	for _, tier := range []struct {
		name   string
		values map[string]string
	}{{"project", in.sources.project}, {"global", in.sources.global}} {
		write(tier.name)
		for _, key := range slices.Sorted(maps.Keys(tier.values)) {
			write(key, tier.values[key])
		}
	}
	offer := in.sources.escOffer
	write("esc")
	for _, name := range slices.Sorted(maps.Keys(offer.Values)) {
		write(name, offer.Values[name], offer.Origins[name].String(), offer.Fingerprints[name])
	}
	write("credentials")
	write(in.credentials.mapping...)
	return hex.EncodeToString(sum.Sum(nil))
}

func logPoolAcquire(ctx context.Context, plugin, binPath string, launched bool) {
	logging.FromContext(ctx).Debug().
		Str("component", "pluginhost").
		Str("operation", "pool_acquire").
		Str("plugin", plugin).
		Str("plugin_path", binPath).
		Bool("reused_running", !launched).
		Msg("plugin process acquired from pool")
}
