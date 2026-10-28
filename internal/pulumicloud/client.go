package pulumicloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi-cloud-sdk/go/apiclient"

	"github.com/rshade/finfocus/internal/esc"
)

// Service errors intentionally omit response bodies and credential-bearing URLs.
const (
	httpScheme         = "http"
	maxAttempts        = 3
	retryBaseDelay     = 250 * time.Millisecond
	maxRetryWait       = 30 * time.Second
	maxUpdateListBytes = 32 * 1024 * 1024
	maxErrorBodyBytes  = 4 * 1024
	updatePageSize     = 100
)

var (
	ErrUnauthorized = errors.New("pulumi cloud token rejected")
	ErrForbidden    = errors.New("pulumi cloud permission denied")
	ErrNotFound     = errors.New("pulumi cloud update not found")
	ErrBadRequest   = errors.New("pulumi cloud request rejected")
)

// RateLimitError reports service-requested backoff after throttling.
type RateLimitError struct{ RetryAfter time.Duration }

func (e *RateLimitError) Error() string { return "Pulumi Cloud rate limited the request" }

// StackRef identifies a stack within a Pulumi Cloud backend.
type StackRef struct{ Org, Project, Stack string }

// UpdateRecord contains only the metadata needed to select snapshot observations.
type UpdateRecord struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Result  string `json:"result"`
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
}

// UpdateList holds all pages, or a conditional-request cache validation.
type UpdateList struct {
	Updates     []UpdateRecord
	ETag        string
	NotModified bool
}

// Client reads stack updates and identity-only exports from one backend.
type Client interface {
	ListUpdates(context.Context, StackRef, string) (UpdateList, error)
	ExportIdentities(context.Context, StackRef, int) ([]Identity, error)
}

type restClient struct {
	backend            string
	token              string
	http               *http.Client
	sleep              func(context.Context, time.Duration) error
	now                func() time.Time
	api                *apiclient.CloudClient
	backendErr         error
	prefixPath         string
	prefixRaw          string
	maxExportBytes     int64
	maxUpdateListBytes int64
}

// callState carries per-call results out of the shared Executor through the
// request context, so concurrent calls on one client never share it.
type callState struct {
	limit       int64
	limitErr    error
	etag        string
	notModified bool
	succeeded   bool
}

type callStateKey struct{}

func withCallState(ctx context.Context, state *callState) context.Context {
	return context.WithValue(ctx, callStateKey{}, state)
}

var (
	errExportTooLarge     = errors.New("deployment export exceeds 256 MiB limit")
	errUpdateListTooLarge = errors.New("invalid Pulumi Cloud update-list response")
)

// NewClient creates a backend-bound client that refuses all redirects.
// A custom client's transport is retained for private CAs and test servers.
func NewClient(session esc.Session, httpClient *http.Client) Client {
	return newRestClient(session, httpClient)
}

func newRestClient(session esc.Session, httpClient *http.Client) *restClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Minute}
	}
	copied := *httpClient
	copied.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	c := &restClient{
		backend:            strings.TrimRight(session.Backend, "/"),
		token:              session.Token,
		http:               &copied,
		sleep:              sleepContext,
		now:                time.Now,
		maxExportBytes:     MaxExportBytes,
		maxUpdateListBytes: maxUpdateListBytes,
	}
	backend, err := validateBackend(c.backend)
	if err != nil {
		c.backendErr = err
		return c
	}
	c.prefixPath = strings.TrimRight(backend.Path, "/")
	c.prefixRaw = strings.TrimRight(backend.EscapedPath(), "/")
	c.api = &apiclient.CloudClient{BaseURL: c.backend, Executor: c.execute}
	return c
}

func validateBackend(raw string) (*url.URL, error) {
	backend, err := url.Parse(raw)
	if err != nil || backend.User != nil || backend.RawQuery != "" || backend.Fragment != "" ||
		backend.Host == "" {
		return nil, errors.New("invalid Pulumi Cloud backend")
	}
	if backend.Scheme != "https" &&
		(backend.Scheme != httpScheme || !isLoopback(backend.Hostname())) {
		return nil, errors.New("pulumi cloud backend must use HTTPS")
	}
	return backend, nil
}

// execute is the SDK's transport seam. It owns the credential, the retry
// budget, conditional requests, and the response size bound, none of which the
// generated client provides.
func (c *restClient) execute(request *http.Request) (*http.Response, error) {
	ctx := request.Context()
	state, _ := ctx.Value(callStateKey{}).(*callState)
	if state == nil {
		state = &callState{}
	}
	request.URL.Path = c.prefixPath + request.URL.Path
	if request.URL.RawPath != "" {
		request.URL.RawPath = c.prefixRaw + request.URL.RawPath
	}
	request.Header.Set("Authorization", "token "+c.token)
	for attempt := range maxAttempts {
		response, err := c.send(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusTooManyRequests &&
			response.StatusCode != http.StatusServiceUnavailable {
			return settle(request, response, state)
		}
		_ = response.Body.Close()
		delay := retryDelay(response.Header.Get("Retry-After"), c.now(), attempt)
		// The server picks Retry-After; a wait past the cap fails fast so a
		// hostile or misconfigured backend cannot stall the command.
		if attempt == maxAttempts-1 || delay > maxRetryWait {
			return nil, &RateLimitError{RetryAfter: delay}
		}
		if sleepErr := c.sleep(ctx, delay); sleepErr != nil {
			return nil, sleepErr
		}
	}
	return nil, errors.New("pulumi cloud retry attempts exhausted")
}

func (c *restClient) send(request *http.Request) (*http.Response, error) {
	//nolint:gosec // G704: the SDK builds this URL from the validated backend, never from user input.
	response, err := c.http.Do(request)
	if err != nil {
		if ctxErr := request.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, errors.New("pulumi cloud request failed: backend unreachable")
	}
	return response, nil
}

// settle turns a non-throttled response into what the SDK should see: success
// with a bounded body, a 304 as an empty success the caller reads from state,
// a failure the SDK reduces to an APIError, or a typed error for other codes.
func settle(request *http.Request, response *http.Response, state *callState) (*http.Response, error) {
	switch {
	case response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices:
		state.succeeded = true
		state.etag = response.Header.Get("ETag")
		if state.limit > 0 {
			response.Body = boundBody(response.Body, state.limit, state.limitErr)
		}
		return response, nil
	case response.StatusCode == http.StatusNotModified:
		_ = response.Body.Close()
		state.notModified = true
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     response.Header.Clone(),
			Body:       io.NopCloser(strings.NewReader("null")),
			Request:    request,
		}, nil
	case response.StatusCode < http.StatusBadRequest:
		_ = response.Body.Close()
		return nil, serviceError(response.StatusCode)
	default:
		response.Body = &truncatedBody{
			Reader: io.LimitReader(response.Body, maxErrorBodyBytes),
			Closer: response.Body,
		}
		return response, nil
	}
}

type truncatedBody struct {
	io.Reader
	io.Closer
}

// boundedBody fails with tooLarge once more than limit bytes are available.
type boundedBody struct {
	reader   *io.LimitedReader
	closer   io.Closer
	tooLarge error
}

func boundBody(body io.ReadCloser, limit int64, tooLarge error) io.ReadCloser {
	return &boundedBody{
		reader:   &io.LimitedReader{R: body, N: limit + 1},
		closer:   body,
		tooLarge: tooLarge,
	}
}

func (b *boundedBody) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	if b.reader.N == 0 {
		return n, b.tooLarge
	}
	return n, err
}

func (b *boundedBody) Close() error { return b.closer.Close() }

// mapError reduces SDK and transport failures to the package's typed errors
// without carrying a response body, URL, or token.
func mapError(err error) error {
	if throttled, ok := errors.AsType[*RateLimitError](err); ok {
		return throttled
	}
	if apiErr, ok := errors.AsType[*apiclient.APIError](err); ok {
		return serviceError(apiErr.HTTPStatusCode())
	}
	return err
}

func serviceError(status int) error {
	switch status {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusBadRequest:
		return ErrBadRequest
	default:
		return fmt.Errorf("pulumi cloud service returned HTTP %d", status)
	}
}

// sleepContext waits without delaying cancellation of the request lifecycle.
func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func retryDelay(header string, now time.Time, attempt int) time.Duration {
	if seconds, err := strconv.ParseInt(header, 10, 64); err == nil && seconds >= 0 &&
		seconds <= int64((1<<63-1)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(header); err == nil {
		return max(0, date.Sub(now))
	}
	// Equal jitter keeps retries between 250 ms and 2 s over three attempts.
	base := retryBaseDelay * time.Duration(1<<attempt)
	//nolint:gosec // Retry jitter requires distribution, not cryptographic unpredictability.
	return base + time.Duration(rand.Int64N(int64(base)))
}

// ListUpdates reads pages from 1 until a page adds no new version.
func (c *restClient) ListUpdates(
	ctx context.Context,
	stack StackRef,
	etag string,
) (UpdateList, error) {
	if c.backendErr != nil {
		return UpdateList{}, c.backendErr
	}
	var result UpdateList
	seen := map[int]bool{}
	for page := 1; ; page++ {
		conditional := ""
		if page == 1 {
			conditional = etag
		}
		fetched, err := c.fetchUpdatePage(ctx, stack, page, conditional)
		if err != nil {
			return UpdateList{}, err
		}
		if fetched.notModified {
			return UpdateList{NotModified: true, ETag: etag}, nil
		}
		if page == 1 {
			result.ETag = fetched.etag
		}
		added := 0
		for _, update := range fetched.updates {
			if !seen[update.Version] {
				seen[update.Version] = true
				result.Updates = append(result.Updates, update)
				added++
			}
		}
		if added == 0 {
			return result, nil
		}
	}
}

type updatePage struct {
	updates     []UpdateRecord
	etag        string
	notModified bool
}

func (c *restClient) fetchUpdatePage(
	ctx context.Context,
	stack StackRef,
	page int,
	etag string,
) (updatePage, error) {
	var headers []http.Header
	if etag != "" {
		headers = append(headers, http.Header{"If-None-Match": {etag}})
	}
	state := &callState{limit: c.maxUpdateListBytes, limitErr: errUpdateListTooLarge}
	pageSize := updatePageSize
	body, err := c.api.GetStackUpdates(
		withCallState(ctx, state),
		stack.Org, stack.Project, stack.Stack,
		nil, &page, &pageSize,
		headers...,
	)
	if err != nil {
		if errors.Is(err, errUpdateListTooLarge) {
			return updatePage{}, errUpdateListTooLarge
		}
		return updatePage{}, mapError(err)
	}
	if state.notModified {
		return updatePage{notModified: true}, nil
	}
	updates, err := decodeUpdateResult(*body)
	if err != nil {
		return updatePage{}, err
	}
	return updatePage{updates: updates, etag: state.etag}, nil
}

func decodeUpdateResult(value any) ([]UpdateRecord, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("invalid Pulumi Cloud update-list response")
	}
	return decodeUpdatePage(bytes.NewReader(raw))
}

type wireUpdateInfo struct {
	Kind   string `json:"kind"`
	Result string `json:"result"`
	Start  int64  `json:"startTime"`
	End    int64  `json:"endTime"`
}

func decodeUpdatePage(body io.Reader) ([]UpdateRecord, error) {
	var page struct {
		Updates json.RawMessage `json:"updates"`
	}
	if err := json.NewDecoder(io.LimitReader(body, maxUpdateListBytes)).Decode(&page); err != nil {
		return nil, errors.New("invalid Pulumi Cloud update-list response")
	}
	// A missing field is malformed. Pulumi Cloud ends a list with JSON null.
	if len(page.Updates) == 0 {
		return nil, errors.New("invalid Pulumi Cloud update-list response: missing updates")
	}
	var updates []struct {
		wireUpdateInfo

		Version int             `json:"version"`
		Info    *wireUpdateInfo `json:"info"`
	}
	if err := json.Unmarshal(page.Updates, &updates); err != nil {
		return nil, errors.New("invalid Pulumi Cloud update-list response")
	}
	if updates == nil {
		return []UpdateRecord{}, nil
	}
	result := make([]UpdateRecord, 0, len(updates))
	for _, update := range updates {
		info := update.wireUpdateInfo
		if update.Info != nil {
			info = *update.Info
		}
		end := info.End
		if end == 0 {
			end = info.Start
		}
		result = append(
			result,
			UpdateRecord{
				Version: update.Version,
				Kind:    info.Kind,
				Result:  info.Result,
				Start:   info.Start,
				End:     end,
			},
		)
	}
	return result, nil
}

// ExportIdentities retrieves and extracts one historical deployment snapshot.
func (c *restClient) ExportIdentities(
	ctx context.Context,
	stack StackRef,
	version int,
) ([]Identity, error) {
	if c.backendErr != nil {
		return nil, c.backendErr
	}
	state := &callState{limit: c.maxExportBytes, limitErr: errExportTooLarge}
	export, err := c.api.ExportStackAtVersion(
		withCallState(ctx, state),
		stack.Org, stack.Project, stack.Stack,
		version,
	)
	if err != nil {
		if errors.Is(err, errExportTooLarge) {
			return nil, errExportTooLarge
		}
		return nil, mapError(err)
	}
	return identitiesFromDeployment(export.Deployment)
}

// probe checks that the credential is accepted. Only reachability and
// authorization matter, so a 2xx with an empty or unexpected body passes.
func (c *restClient) probe(ctx context.Context) error {
	if c.backendErr != nil {
		return c.backendErr
	}
	state := &callState{}
	if _, err := c.api.GetCurrentUser(withCallState(ctx, state)); err != nil && !state.succeeded {
		if apiErr, ok := errors.AsType[*apiclient.APIError](err); ok && apiErr.IsNoContent() {
			return nil
		}
		return mapError(err)
	}
	return nil
}
