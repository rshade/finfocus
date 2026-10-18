//go:build unix

package webui

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Sends SIGTERM to the whole test process; must not overlap other tests.
func TestGracefulShutdownOnSIGTERM(t *testing.T) {
	// Hold the signal so a missing server handler cannot kill this process.
	caught := make(chan os.Signal, 1)
	signal.Notify(caught, syscall.SIGTERM)
	t.Cleanup(func() { signal.Stop(caught) })

	s, err := New(Options{})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- s.Run(context.Background())
	}()

	require.Eventually(t, func() bool {
		resp, getErr := noRedirectClient().Get(s.URL())
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
		require.FailNow(t, "Run did not return after SIGTERM")
	}

	_, getErr := noRedirectClient().Get(s.URL())
	require.Error(t, getErr)
}

//nolint:paralleltest // Sends os.Interrupt to the whole test process; must not overlap other tests.
func TestGracefulShutdownOnInterrupt(t *testing.T) {
	s, err := New(Options{})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- s.Run(context.Background())
	}()

	// Wait until the server answers.
	require.Eventually(t, func() bool {
		resp, getErr := noRedirectClient().Get(s.URL())
		if getErr != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusSeeOther
	}, 10*time.Second, 25*time.Millisecond)

	proc, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	require.NoError(t, proc.Signal(os.Interrupt))

	select {
	case runErr := <-done:
		require.NoError(t, runErr)
	case <-time.After(15 * time.Second):
		require.FailNow(t, "Run did not return after interrupt")
	}

	// The listener is closed: new connections fail.
	_, getErr := noRedirectClient().Get(s.URL())
	require.Error(t, getErr)
}
