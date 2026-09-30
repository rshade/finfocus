// Package jevapi is a minimal client for the TypeSafe AI Jev (System One) API.
// It covers the two paths the scorer needs, POST /v1/systemone and
// GET /v1/models, and depends only on the standard library.
package jevapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	systemOnePath = "/v1/systemone"
	modelsPath    = "/v1/models"

	requestIDHeader = "X-Typesafe-Request-Id"

	statusOverloaded = 529

	defaultTimeout     = 60 * time.Second
	defaultMaxRetries  = 4
	defaultBaseBackoff = 500 * time.Millisecond
	defaultMaxBackoff  = 8 * time.Second

	maxResponseBytes = 16 << 20
	maxErrorBody     = 4 << 10
	maxMessageRunes  = 300

	maxBackoffShift = 20
)

// Config configures a Client.
type Config struct {
	// BaseURL is the API origin, for example https://api.typesafe.ai. Plain
	// http is accepted only for loopback hosts.
	BaseURL string
	// APIKey is sent as a bearer token. It is never included in errors.
	APIKey string
	// HTTPClient overrides the transport. Nil uses a client with no overall
	// timeout; Timeout bounds each attempt instead.
	HTTPClient *http.Client
	// Timeout bounds each attempt. Zero means 60 seconds.
	Timeout time.Duration
	// MaxRetries is the number of retries after the first attempt for 429,
	// 529 and connection failures. Zero means 4; a negative value disables
	// retries.
	MaxRetries int
	// BaseBackoff and MaxBackoff shape exponential backoff with full jitter
	// when the server sends no Retry-After. Zero means 500ms and 8s.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// Sleep waits between attempts and must return early with the context's
	// error when it is done. Nil uses a timer. It exists so tests need not
	// wait.
	Sleep func(ctx context.Context, d time.Duration) error
}

// Client calls the Jev API.
type Client struct {
	base       *url.URL
	apiKey     string
	http       *http.Client
	timeout    time.Duration
	maxRetries int
	baseWait   time.Duration
	maxWait    time.Duration
	sleep      func(ctx context.Context, d time.Duration) error
}

// New validates cfg and builds a Client.
func New(cfg Config) (*Client, error) {
	base, err := parseBaseURL(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	if cfg.APIKey == "" {
		return nil, errors.New("jevapi: api key is empty")
	}
	c := &Client{
		base:       base,
		apiKey:     cfg.APIKey,
		http:       cfg.HTTPClient,
		timeout:    cfg.Timeout,
		maxRetries: cfg.MaxRetries,
		baseWait:   cfg.BaseBackoff,
		maxWait:    cfg.MaxBackoff,
		sleep:      cfg.Sleep,
	}
	if c.http == nil {
		c.http = &http.Client{}
	}
	if c.timeout <= 0 {
		c.timeout = defaultTimeout
	}
	switch {
	case c.maxRetries == 0:
		c.maxRetries = defaultMaxRetries
	case c.maxRetries < 0:
		c.maxRetries = 0
	}
	if c.baseWait <= 0 {
		c.baseWait = defaultBaseBackoff
	}
	if c.maxWait <= 0 {
		c.maxWait = defaultMaxBackoff
	}
	if c.sleep == nil {
		c.sleep = sleepTimer
	}
	return c, nil
}

func parseBaseURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("jevapi: base url is empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("jevapi: base url %q is not a valid URL", raw)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopback(u.Hostname()) {
			return nil, errors.New("jevapi: base url must use https unless the host is loopback")
		}
	default:
		return nil, fmt.Errorf("jevapi: base url scheme %q is not supported", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// SystemOne asks the questions in req about req.State.
func (c *Client) SystemOne(ctx context.Context, req Request) (*Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("jevapi: encode request: %w", err)
	}
	payload, header, err := c.do(ctx, http.MethodPost, systemOnePath, body)
	if err != nil {
		return nil, err
	}
	var resp Response
	if err = json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
	}
	resp.RequestID = header.Get(requestIDHeader)
	if err = resp.validate(); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Models lists the models and aliases available to the key.
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	payload, _, err := c.do(ctx, http.MethodGet, modelsPath, nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Models []Model `json:"models"`
	}
	if err = json.Unmarshal(payload, &list); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformedResponse, err)
	}
	return list.Models, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, http.Header, error) {
	var last *APIError
	for attempt := 0; ; attempt++ {
		payload, header, apiErr, err := c.attempt(ctx, method, path, body)
		if err != nil {
			return nil, nil, err
		}
		if apiErr == nil {
			return payload, header, nil
		}
		last = apiErr
		if !apiErr.retryable() || attempt >= c.maxRetries {
			return nil, nil, last
		}
		if err = c.sleep(ctx, c.delay(apiErr, attempt)); err != nil {
			return nil, nil, err
		}
	}
}

// attempt performs one HTTP exchange. A non-nil *APIError describes a failed
// attempt the caller may retry; a non-nil error is final.
func (c *Client) attempt(
	ctx context.Context, method, path string, body []byte,
) ([]byte, http.Header, *APIError, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, nil, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	target := c.base.String() + path
	req, err := http.NewRequestWithContext(attemptCtx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("jevapi: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		apiErr, final := c.classifyTransportError(ctx, attemptCtx, err)
		return nil, nil, apiErr, final
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		if readErr != nil {
			apiErr, final := c.classifyTransportError(ctx, attemptCtx, readErr)
			return nil, nil, apiErr, final
		}
		return payload, resp.Header, nil, nil
	}
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	return nil, nil, newStatusError(resp, errBody), nil
}

// classifyTransportError separates the caller's cancellation and deadline
// and a per-attempt timeout (both final errors) from connection failures,
// which come back as a retryable *APIError.
func (c *Client) classifyTransportError(ctx, attemptCtx context.Context, err error) (*APIError, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	if attemptCtx.Err() != nil {
		return nil, fmt.Errorf("jevapi: attempt exceeded %s: %w", c.timeout, context.DeadlineExceeded)
	}
	return &APIError{Kind: KindUnavailable, Message: "connection failed: " + truncate(err.Error())}, nil
}

func (c *Client) delay(e *APIError, attempt int) time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	ceiling := c.baseWait << min(attempt, maxBackoffShift)
	if ceiling <= 0 || ceiling > c.maxWait {
		ceiling = c.maxWait
	}
	return rand.N(ceiling + 1) //nolint:gosec // Jitter, not a secret.
}

func sleepTimer(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if secs, err := strconv.ParseFloat(value, 64); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs * float64(time.Second))
	}
	if at, err := http.ParseTime(value); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
