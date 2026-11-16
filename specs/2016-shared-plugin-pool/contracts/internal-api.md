# Internal API: `pluginhost.Pool`

No HTTP or protocol surface changes. Web UI Fit is `Not applicable`, so no
`http-api.md` is required.

```go
// ErrPoolClosed is returned by Acquire after Close.
var ErrPoolClosed = errors.New("plugin pool is closed")

// NewPool returns a pool whose processes live until Close or until parent ends.
func NewPool(parent context.Context) *Pool

// WithPool returns ctx carrying pool; PoolFromContext reads it back.
func WithPool(ctx context.Context, pool *Pool) context.Context
func PoolFromContext(ctx context.Context) (*Pool, bool)

// Acquire has NewClient's signature. It returns a client for binPath whose
// process is shared with every other holder that resolved the same key.
// Closing the returned client releases this hold once.
func (p *Pool) Acquire(ctx context.Context, launcher Launcher, binPath string) (*Client, error)

// Close stops every process the pool started, held or not, and waits for
// launches in progress. It is safe to call more than once.
func (p *Pool) Close()
```

Callers:

- `registry.(*Registry).openPlugin`: `pool.Acquire` when
  `PoolFromContext(ctx)` is set, otherwise `pluginhost.NewClient`.
- `cli.runWebStacks`: `NewPool`, `WithPool`, and `Close` after every stack
  has stopped.
