package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/webui"
)

// promptingStack is a running stack whose loader asks for a passphrase.
type promptingStack struct {
	stack    *webStack
	received chan string
	origin   string
	cookie   *http.Cookie
}

// post submits a passphrase to this stack's session and returns the status, or
// 0 with the error. It reports instead of failing, so it can run in Eventually.
func (p *promptingStack) post(passphrase string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, p.origin+"/api/passphrase",
		strings.NewReader(`{"passphrase":"`+passphrase+`"}`))
	if err != nil {
		return 0, err
	}
	req.AddCookie(p.cookie)
	req.Header.Set("Origin", p.origin)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

// accepted reports whether the stack accepted a passphrase.
func (p *promptingStack) accepted(passphrase string) bool {
	status, err := p.post(passphrase)
	return err == nil && status == http.StatusAccepted
}

// newPromptingStack builds a stack whose loader asks for a passphrase and
// reports what its own channel received.
func newPromptingStack(t *testing.T, cmd *cobra.Command, name string) *promptingStack {
	t.Helper()
	got := make(chan string, 4)
	stack := newWebStack(context.Background(), cmd, overviewParams{stack: name, cfg: config.New()},
		engine.DateRange{}, nil, nil)
	stack.pipe.cfg.openPlugins = fakeOpenPlugins
	stack.pipe.cfg.newEngine = fakeNewEngine
	stack.pipe.cfg.loader = func(ctx context.Context, p *OverviewPipeline) (*OverviewPipelineData, error) {
		p.OnPassphraseRequired()
		select {
		case pw := <-p.cfg.passphrases:
			got <- pw
		case <-ctx.Done():
		}
		return nil, errors.New("stop after passphrase")
	}
	srv, err := webui.New(webui.Options{Session: stack.session})
	require.NoError(t, err)
	require.NoError(t, srv.Listen())
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, srv.Shutdown(ctx))
	})
	stack.start()
	t.Cleanup(stack.stop)
	origin, cookie := webHTTP(t, srv.URL())
	return &promptingStack{stack: stack, received: got, origin: origin, cookie: cookie}
}

// Not parallel: Builds web stacks that read process-wide config and the web pipeline hook.
func TestWebStacks_PassphraseReachesOnlyItsOwnStack(t *testing.T) {
	unsetPassphraseEnv(t)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())

	dev := newPromptingStack(t, cmd, "dev")
	prod := newPromptingStack(t, cmd, "prod")

	require.Eventually(t, func() bool {
		return prod.accepted("prod-secret")
	}, 10*time.Second, 20*time.Millisecond, "prod must prompt and accept its passphrase")
	select {
	case pw := <-prod.received:
		assert.Equal(t, "prod-secret", pw)
	case <-time.After(5 * time.Second):
		require.Fail(t, "prod pipeline never received its passphrase")
	}
	assert.Never(t, func() bool { return len(dev.received) > 0 }, 300*time.Millisecond, 20*time.Millisecond,
		"dev must not receive prod's passphrase")

	require.Eventually(t, func() bool {
		return dev.accepted("dev-secret")
	}, 10*time.Second, 20*time.Millisecond, "dev must still be waiting for its own passphrase")
	select {
	case pw := <-dev.received:
		assert.Equal(t, "dev-secret", pw)
	case <-time.After(5 * time.Second):
		require.Fail(t, "dev pipeline never received its passphrase")
	}
	assert.Empty(t, prod.received, "prod must not receive dev's passphrase")
}

// Not parallel: Builds web stacks that read process-wide config and the web pipeline hook.
func TestWebStacks_EachStackOwnsItsPassphraseChannel(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)
	config.SetGlobalConfig(&config.Config{})
	cmd := &cobra.Command{Use: "test"}
	cmd.SetContext(context.Background())
	build := func(name string) *webStack {
		return newWebStack(context.Background(), cmd, overviewParams{stack: name, cfg: config.New()},
			engine.DateRange{}, nil, nil)
	}

	dev, prod := build("dev"), build("prod")
	t.Cleanup(dev.cancel)
	t.Cleanup(prod.cancel)

	assert.NotSame(t, dev.session, prod.session)
	assert.NotSame(t, dev.data, prod.data)
	assert.NotSame(t, dev.pipe, prod.pipe)
	assert.NotEqual(t, dev.pipe.cfg.passphrases, prod.pipe.cfg.passphrases)
	require.NoError(t, dev.pipeCtx.Err())
	prod.cancel()
	assert.NoError(t, dev.pipeCtx.Err(), "stopping one stack must not cancel another")
}
