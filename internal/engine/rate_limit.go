// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

const (
	// rateLimitMaxAttempts is the initial call plus retries against one plugin.
	// A rate limit that survives these attempts falls through to the next plugin.
	rateLimitMaxAttempts = 3
	rateLimitBaseDelay   = 200 * time.Millisecond
	rateLimitMaxDelay    = 2 * time.Second
	// rateLimitJitterDivisor keeps equal jitter inside [delay/2, delay].
	rateLimitJitterDivisor = 2
)

// errBatchRateLimited is returned when BatchCost stays rate limited after
// retries. Callers must not split that batch into per-resource RPCs.
var errBatchRateLimited = errors.New("batch cost rate limited")

// retryRateLimited calls fn until it succeeds, the error is not a rate limit,
// the attempt budget is spent, or ctx ends. perAttempt is the timeout for one
// attempt; zero uses ctx as the attempt context. The wait between attempts
// uses ctx, so a per-attempt timeout does not eat the backoff.
func retryRateLimited[T any](
	ctx context.Context,
	perAttempt time.Duration,
	fn func(context.Context) (T, error),
) (T, error) {
	var zero T
	var last error
	for attempt := range rateLimitMaxAttempts {
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		attemptCtx := ctx
		cancel := func() {}
		if perAttempt > 0 {
			var cancelAttempt context.CancelFunc
			attemptCtx, cancelAttempt = context.WithTimeout(ctx, perAttempt)
			cancel = cancelAttempt
		}
		value, err := fn(attemptCtx)
		cancel()
		if err == nil {
			return value, nil
		}
		last = err
		if !isRateLimited(err) || attempt == rateLimitMaxAttempts-1 {
			return zero, err
		}
		delay := rateLimitDelay(err, attempt)
		logging.FromContext(ctx).Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("operation", "rate_limit_retry").
			Int("attempt", attempt+1).
			Dur("delay", delay).
			Err(err).
			Msg("plugin rate limited, retrying the same plugin")
		if waitErr := waitRateLimit(ctx, delay); waitErr != nil {
			return zero, waitErr
		}
	}
	return zero, last
}

func (e *Engine) projectedFromPlugin(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
	perAttempt time.Duration,
) (*CostResult, error) {
	return retryRateLimited(ctx, perAttempt, func(attemptCtx context.Context) (*CostResult, error) {
		return e.getProjectedCostFromPlugin(attemptCtx, client, resource, e.usageProfile)
	})
}

func (e *Engine) actualFromPlugin(
	ctx context.Context,
	client *pluginhost.Client,
	resource ResourceDescriptor,
	from, to time.Time,
	perAttempt time.Duration,
) (*CostResult, error) {
	return retryRateLimited(ctx, perAttempt, func(attemptCtx context.Context) (*CostResult, error) {
		return e.getActualCostFromPlugin(attemptCtx, client, resource, from, to)
	})
}

// isRateLimited reports whether err is a gRPC ResourceExhausted status.
// Per-resource batch errors use that code for a rate limit.
func isRateLimited(err error) bool {
	return err != nil && status.Code(err) == codes.ResourceExhausted
}

// rateLimitDelay returns a plugin retry hint when the status carries
// ErrorDetail.retry_after_seconds, and a bounded jittered backoff otherwise.
func rateLimitDelay(err error, attempt int) time.Duration {
	if hint, ok := retryHint(err); ok {
		return hint
	}
	return backoffDelay(attempt)
}

func retryHint(err error) (time.Duration, bool) {
	st, ok := status.FromError(err)
	if !ok {
		return 0, false
	}
	for _, detail := range st.Details() {
		info, isDetail := detail.(*pbc.ErrorDetail)
		if !isDetail || info.RetryAfterSeconds == nil || info.GetRetryAfterSeconds() < 0 {
			continue
		}
		return time.Duration(info.GetRetryAfterSeconds()) * time.Second, true
	}
	return 0, false
}

func backoffDelay(attempt int) time.Duration {
	delay := rateLimitBaseDelay
	for range attempt {
		if delay >= rateLimitMaxDelay/rateLimitJitterDivisor {
			delay = rateLimitMaxDelay
			break
		}
		delay *= rateLimitJitterDivisor
	}
	if delay > rateLimitMaxDelay {
		delay = rateLimitMaxDelay
	}
	span := int64(delay / rateLimitJitterDivisor)
	if span <= 0 {
		return delay
	}
	//nolint:gosec // G404: backoff jitter is not a secret.
	return delay/rateLimitJitterDivisor + time.Duration(rand.Int64N(span+1))
}

func waitRateLimit(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func fallbackMessage(err error) string {
	if isRateLimited(err) {
		return "plugin rate limited, falling back to next priority plugin"
	}
	return "plugin failed, falling back to next priority plugin"
}

func stoppedChainMessage(err error) string {
	if isRateLimited(err) {
		return "plugin rate limited and fallback disabled, stopping fallback chain"
	}
	return "plugin failed and fallback disabled, stopping fallback chain"
}

// notePluginFailure logs a stopped chain or a move to the next plugin.
// It returns true when the chain must stop. nextPlugin is empty for the last match.
func notePluginFailure(
	ctx context.Context,
	resourceType, pluginName, nextPlugin string,
	fallback bool,
	err error,
) bool {
	log := logging.FromContext(ctx)
	if !fallback {
		log.Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("resource_type", resourceType).
			Str("plugin", pluginName).
			Err(err).
			Msg(stoppedChainMessage(err))
		return true
	}
	if nextPlugin != "" {
		log.Info().
			Ctx(ctx).
			Str("component", "engine").
			Str("resource_type", resourceType).
			Str("failed_plugin", pluginName).
			Str("next_plugin", nextPlugin).
			Err(err).
			Msg(fallbackMessage(err))
	}
	return false
}
