package cli

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/webui"
)

//nolint:paralleltest // Sends SIGTERM to the whole test process; must not overlap other tests.
func TestServeWebShutsDownOnSIGTERM(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM delivery is unix-only")
	}
	// Hold the signal so a missing handler cannot kill this process.
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(caught) })

	srv, err := webui.New(webui.Options{})
	require.NoError(t, err)
	require.NoError(t, srv.Listen())

	done := make(chan error, 1)
	go func() {
		done <- serveWebUntilInterrupt(context.Background(), srv)
	}()

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	require.Eventually(t, func() bool {
		resp, getErr := client.Get(srv.URL())
		if getErr != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusSeeOther
	}, 10*time.Second, 25*time.Millisecond)

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-time.After(15 * time.Second):
		require.FailNow(t, "serveWebUntilInterrupt did not return after SIGTERM")
	}

	_, getErr := client.Get(srv.URL())
	require.Error(t, getErr)
}
