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

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
)

// ErrPoolClosed is returned by Acquire after the pool is closed.
var ErrPoolClosed = errors.New("plugin pool is closed")

// poolProbeTimeout bounds the Name call that checks a running process before
// it is handed to another holder.
const poolProbeTimeout = 5 * time.Second

type poolContextKey struct{}

// Pool shares plugin processes between holders that would launch the same
// binary with the same environment, so several web stacks do not each start a
// copy of every plugin (specs/2016-shared-plugin-pool/). A process lives until
// its last holder releases it or the pool closes; no holder's context can end
// it. The key covers the binary path, the launch environment, and everything
// the holder's ESC source offers the plugin, so no value reaches a process
// another holder with a different environment uses.
type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	entries map[poolKey]*poolEntry
	live    map[*poolEntry]struct{}
	closed  bool
	// closing tracks processes stopped outside Close, so Close returns only
	// after they are gone.
	closing sync.WaitGroup
}

type poolKey struct {
	path   string
	binary string
	env    string
}

// poolEntry is one shared process. holds counts requesters still waiting for
// the launch plus holders that have not released.
type poolEntry struct {
	key    poolKey
	ready  chan struct{}
	client *Client
	err    error
	holds  int
	closed bool
	stop   func()
}

// NewPool returns an empty pool. Its processes stop on Close or when parent ends.
func NewPool(parent context.Context) *Pool {
	ctx, cancel := context.WithCancel(parent)
	return &Pool{
		ctx:     ctx,
		cancel:  cancel,
		entries: map[poolKey]*poolEntry{},
		live:    map[*poolEntry]struct{}{},
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
	key := poolKey{path: binPath, binary: binaryStamp(binPath), env: environmentKey(in)}
	for {
		entry, launched, err := p.hold(ctx, launcher, binPath, in, key)
		if err != nil {
			return nil, err
		}
		select {
		case <-entry.ready:
		case <-ctx.Done():
			_ = p.release(entry)
			return nil, ctx.Err()
		}
		if entry.err != nil {
			_ = p.release(entry)
			return nil, entry.err
		}
		if launched || responds(ctx, entry.client) {
			logPoolAcquire(ctx, in.installName, binPath, launched)
			return p.holderClient(entry), nil
		}
		_ = p.release(entry)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		p.evict(entry)
	}
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
// waits for a fresh process instead of probing an old one.
func (p *Pool) hold(
	ctx context.Context, launcher Launcher, binPath string, in launchInputs, key poolKey,
) (*poolEntry, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, false, ErrPoolClosed
	}
	entry, ok := p.entries[key]
	if !ok {
		entry = p.startLocked(ctx, launcher, binPath, in, key)
	}
	entry.holds++
	return entry, !isReady(entry), nil
}

// startLocked launches under a context that keeps the requester's values
// (install name, log writers, ESC source) but is cancelled only by the pool,
// because [os/exec.CommandContext] kills the plugin when that context ends.
func (p *Pool) startLocked(
	ctx context.Context, launcher Launcher, binPath string, in launchInputs, key poolKey,
) *poolEntry {
	launchCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stopAfter := context.AfterFunc(p.ctx, cancel)
	entry := &poolEntry{
		key:   key,
		ready: make(chan struct{}),
		stop: func() {
			stopAfter()
			cancel()
		},
	}
	p.entries[key] = entry
	p.live[entry] = struct{}{}
	go p.launch(launchCtx, launcher, binPath, in, entry)
	return entry
}

func (p *Pool) launch(ctx context.Context, launcher Launcher, binPath string, in launchInputs, entry *poolEntry) {
	client, err := launchClient(ctx, launcher, binPath, in)

	p.mu.Lock()
	entry.client, entry.err = client, err
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
	if entry.holds == 0 && !ready && p.entries[entry.key] == entry {
		delete(p.entries, entry.key)
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
	if p.entries[entry.key] == entry {
		delete(p.entries, entry.key)
	}
}

func (p *Pool) forgetLocked(entry *poolEntry) {
	if p.entries[entry.key] == entry {
		delete(p.entries, entry.key)
	}
	delete(p.live, entry)
}

// holderClient gives each holder its own Client so per-holder writes, such as
// the registry's metadata merge, never race. Conn and API are shared.
func (p *Pool) holderClient(entry *poolEntry) *Client {
	held := *entry.client
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

// environmentKey hashes everything that decides a process's final
// environment and its delivered digest: the launch pairs, the names that
// replace an inherited value, and the whole ESC offer (value, reference, and
// revision), including names allowed only after the plugin advertises them.
// Values are hashed, never kept or logged.
func environmentKey(in launchInputs) string {
	sum := sha256.New()
	write := func(parts ...string) {
		for _, part := range parts {
			_, _ = sum.Write([]byte(part))
			_, _ = sum.Write([]byte{0})
		}
	}
	write("launch")
	for _, pair := range slices.Sorted(slices.Values(in.delivered.Pairs)) {
		write(pair)
	}
	write("overrides")
	for _, name := range slices.Sorted(maps.Keys(in.delivered.Overrides)) {
		write(name)
	}
	write("esc")
	for _, name := range slices.Sorted(maps.Keys(in.escOffer.Values)) {
		write(name, in.escOffer.Values[name], in.escOffer.Origins[name].String(), in.escOffer.Fingerprints[name])
	}
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
