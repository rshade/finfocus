package notification_test

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/notification"
)

func TestRedactor(t *testing.T) {
	t.Parallel()

	secretURL := "https://hooks.slack.com/services/T000/B000/XXXX"
	r := &notification.Redactor{}
	r.Add(secretURL)
	r.Add("")
	r.Add("XXXX")
	r.Add("token-123")

	urlErr := &url.Error{Op: "Post", URL: secretURL, Err: errors.New("connection refused")}
	got := r.Redact(urlErr.Error())
	assert.NotContains(t, got, "hooks.slack.com/services")
	assert.NotContains(t, got, "XXXX")
	assert.Contains(t, got, notification.RedactedText)
	assert.Contains(t, got, "connection refused")

	assert.Equal(t, "auth "+notification.RedactedText+" failed", r.Redact("auth token-123 failed"))
	assert.Equal(t, "nothing secret", r.Redact("nothing secret"))
}

func TestRedactorLongestFirst(t *testing.T) {
	t.Parallel()

	r := &notification.Redactor{}
	r.Add("abc")
	r.Add("abcdef")
	assert.Equal(t, notification.RedactedText+" and "+notification.RedactedText, r.Redact("abcdef and abc"))
}

func TestRedactorZeroValue(t *testing.T) {
	t.Parallel()

	var r notification.Redactor
	assert.Equal(t, "text", r.Redact("text"))
}
