package esc

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc/esctest"
)

func TestFailureErrorText(t *testing.T) {
	t.Parallel()
	ref := Ref{Org: "acme", Project: "billing", Name: "prod"}
	assert.Equal(t,
		"ESC environment acme/billing/prod skipped (not found or no access); plugins use shell and config values",
		Failure{Ref: ref, Category: CategoryNotFound}.Error(),
	)
	assert.Equal(t,
		"ESC environment acme/billing/prod skipped (access_denied); plugins use shell and config values",
		Failure{Ref: ref, Category: CategoryAccessDenied}.Error(),
	)
	assert.Equal(t,
		"ESC skipped (not logged in): set PULUMI_ACCESS_TOKEN or run `pulumi login`",
		Failure{Category: CategoryNotLoggedIn}.Error(),
	)
	assert.Equal(t,
		"ESC skipped (unsupported_backend); plugins use shell and config values",
		Failure{Category: CategoryUnsupportedBackend}.Error(),
	)
}

func TestClassify(t *testing.T) {
	t.Parallel()

	t.Run("deadline", func(t *testing.T) {
		t.Parallel()
		got := classify(context.Background(), context.DeadlineExceeded)
		assert.Equal(t, CategoryTimedOut, got.Category)
	})
	t.Run("url error", func(t *testing.T) {
		t.Parallel()
		err := &url.Error{Op: "Get", URL: "https://api.pulumi.com", Err: errors.New("dial tcp")}
		got := classify(context.Background(), err)
		assert.Equal(t, CategoryUnavailable, got.Category)
		assert.NotContains(t, got.Error(), "dial tcp")
		assert.NotContains(t, got.Error(), "api.pulumi.com")
	})
	t.Run("panic", func(t *testing.T) {
		t.Parallel()
		got := classify(context.Background(), panicError{})
		assert.Equal(t, CategoryUnavailable, got.Category)
		assert.NotContains(t, got.Error(), "esc call failed")
	})

	statuses := []struct {
		name     string
		status   int
		message  string
		omit     bool
		category Category
	}{
		{
			name:     "401",
			status:   http.StatusUnauthorized,
			message:  "fixture-body-do-not-leak",
			category: CategoryNotLoggedIn,
		},
		{
			name:     "403",
			status:   http.StatusForbidden,
			message:  "fixture-body-do-not-leak",
			category: CategoryAccessDenied,
		},
		{name: "404", status: http.StatusNotFound, message: "fixture-body-do-not-leak", category: CategoryNotFound},
		{
			name:     "500",
			status:   http.StatusInternalServerError,
			message:  "fixture-body-do-not-leak",
			category: CategoryUnavailable,
		},
		{
			name:     "500 without message",
			status:   http.StatusInternalServerError,
			omit:     true,
			category: CategoryUnavailable,
		},
	}
	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := esctest.New("tok", map[string]*esctest.Environment{
				"acme/billing/prod": {
					Status:      tt.status,
					Message:     tt.message,
					OmitMessage: tt.omit,
				},
			})
			t.Cleanup(srv.Close)
			client, err := newSDKClient(srv.URL, "tok", nil)
			require.NoError(t, err)
			_, _, openErr := client.open(context.Background(), Ref{Org: "acme", Project: "billing", Name: "prod"})
			require.Error(t, openErr)
			got := classify(context.Background(), openErr)
			assert.Equal(t, tt.category, got.Category)
			got.Ref = Ref{Org: "acme", Project: "billing", Name: "prod"}
			assert.NotContains(t, got.Error(), "fixture-body-do-not-leak")
			assert.NotContains(t, got.Error(), "required property")
		})
	}
}
