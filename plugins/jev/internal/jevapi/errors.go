package jevapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrMalformedResponse wraps every failure to decode or validate a 200
// response.
var ErrMalformedResponse = errors.New("jevapi: malformed response")

// Kind classifies an API failure so callers can pick a status without
// inspecting HTTP codes.
type Kind int

const (
	// KindUnavailable covers 529, other 5xx and connection failures.
	KindUnavailable Kind = iota + 1
	// KindRateLimited is HTTP 429.
	KindRateLimited
	// KindUnauthenticated is HTTP 401.
	KindUnauthenticated
	// KindPermissionDenied is HTTP 403.
	KindPermissionDenied
	// KindInvalidRequest is 422 and every other 4xx: the request was refused
	// as sent, so retrying it unchanged cannot help.
	KindInvalidRequest
)

// APIError is a failed exchange with the Jev API. Status is zero for
// connection failures. It never carries the API key or the request body.
type APIError struct {
	Kind       Kind
	Status     int
	RequestID  string
	Message    string
	RetryAfter time.Duration
}

// Error implements error.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("jev api")
	if e.Status != 0 {
		fmt.Fprintf(&b, " status %d", e.Status)
	}
	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}
	if e.RequestID != "" {
		b.WriteString(" (request id ")
		b.WriteString(e.RequestID)
		b.WriteString(")")
	}
	return b.String()
}

func (e *APIError) retryable() bool {
	return e.Status == http.StatusTooManyRequests || e.Status == statusOverloaded || e.Status == 0
}

func newStatusError(resp *http.Response, body []byte) *APIError {
	e := &APIError{
		Status:    resp.StatusCode,
		RequestID: resp.Header.Get(requestIDHeader),
		Message:   errorMessage(body),
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		e.Kind = KindUnauthenticated
	case resp.StatusCode == http.StatusForbidden:
		e.Kind = KindPermissionDenied
	case resp.StatusCode == http.StatusTooManyRequests:
		e.Kind = KindRateLimited
	case resp.StatusCode >= http.StatusInternalServerError:
		e.Kind = KindUnavailable
	default:
		e.Kind = KindInvalidRequest
	}
	if e.Status == http.StatusTooManyRequests || e.Status == statusOverloaded {
		e.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	}
	return e
}

// errorMessage extracts a short, printable description from an error body:
// FastAPI's "detail" string, the first validation message, or the raw text.
func errorMessage(body []byte) string {
	var parsed struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(body, &parsed) == nil && len(parsed.Detail) > 0 {
		var text string
		if json.Unmarshal(parsed.Detail, &text) == nil {
			return truncate(text)
		}
		var items []struct {
			Msg string `json:"msg"`
		}
		if json.Unmarshal(parsed.Detail, &items) == nil && len(items) > 0 {
			return truncate(items[0].Msg)
		}
	}
	return truncate(string(body))
}

func truncate(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxMessageRunes {
		return s
	}
	return string([]rune(s)[:maxMessageRunes]) + "..."
}
