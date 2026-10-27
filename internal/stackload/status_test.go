package stackload

import (
	"context"
	"errors"
	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"testing"
)

func TestClassifyRedacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ raw, want string }{{"missing PULUMI_CONFIG_PASSPHRASE secret=token https://private.example", ReasonPassphraseRequired}, {"decrypt failed: incorrect passphrase", ReasonPassphraseRequired}, {"dial tcp private-backend: connection refused token=abc", ReasonBackendUnreachable}, {"please login https://secret.example", ReasonBackendUnreachable}, {"unexpected secret=abc", ReasonReadFailed}} {
		t.Run(tc.want+tc.raw[:3], func(t *testing.T) {
			t.Parallel()
			err := classifiedError("state", errors.New(tc.raw))
			assert.Equal(t, tc.want, err.Reason)
			assert.NotContains(t, err.Error(), tc.raw)
			assert.NotContains(t, err.Error(), "https://")
			assert.NotContains(t, err.Error(), "secret=")
		})
	}
}

func TestClassifyPreservesContextSentinelsWithoutBackendCauses(t *testing.T) {
	t.Parallel()
	for _, sentinel := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			t.Parallel()
			backend := fmt.Errorf("private-backend https://secret.example token=abc: %w", sentinel)
			err := classifiedError("plan", backend)
			require.ErrorIs(t, err, sentinel)
			require.NotErrorIs(t, err, backend)
			require.NoError(t, errors.Unwrap(err))
			assert.NotContains(t, err.Error(), "private-backend")
			assert.NotContains(t, err.Error(), "https://")
			assert.NotContains(t, err.Error(), "token=")
		})
	}
	backend := errors.New("context canceled https://secret.example token=abc")
	err := classifiedError("state", backend)
	require.NotErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	require.NotErrorIs(t, err, backend)
}
