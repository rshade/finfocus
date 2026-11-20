package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ErrUnexpectedStatus is returned for any response outside 2xx, including a redirect.
var ErrUnexpectedStatus = errors.New("unexpected response status")

const maxDrainBytes = 64 << 10

// NewHTTPClient returns a client that never follows redirects. Deadlines come
// from the request context.
func NewHTTPClient() *http.Client {
	return noRedirectClient(nil)
}

func noRedirectClient(base *http.Client) *http.Client {
	client := &http.Client{}
	if base != nil {
		copied := *base
		client = &copied
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

// sendJSON sends body as JSON. A configured Content-Type header wins over the
// default. The response body is drained and discarded; only the status code
// is reported. A [url.Error] is unwrapped because its text embeds the URL.
func sendJSON(
	ctx context.Context,
	client *http.Client,
	method, target string,
	headers map[string]string,
	body any,
) error {
	resp, err := requestJSON(ctx, client, method, target, headers, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: status %d", ErrUnexpectedStatus, resp.StatusCode)
	}
	return nil
}

// requestJSON sends a JSON request. The caller owns the response body.
func requestJSON(
	ctx context.Context,
	client *http.Client,
	method, target string,
	headers map[string]string,
	body any,
) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encoding request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("building request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return nil, urlErr.Err
		}
		return nil, err
	}
	return resp, nil
}
