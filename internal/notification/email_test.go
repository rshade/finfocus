package notification

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

const smtpTestUsername = "secret-username"
const smtpTestPassword = "secret-password"

type relayOptions struct {
	mode, reject, stall string
	auth                bool
}

type relayMessage struct {
	recipients []string
	data       string
	auth       string
}

func testRelay(t *testing.T, opts relayOptions) (int, *x509.CertPool, <-chan relayMessage) {
	t.Helper()
	cert := httptest.NewUnstartedServer(nil)
	cert.StartTLS()
	t.Cleanup(cert.Close)
	roots := x509.NewCertPool()
	roots.AddCert(cert.Certificate())

	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	result := make(chan relayMessage, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		<-done
	})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if opts.mode == config.SMTPImplicitTLS {
			conn = tls.Server(conn, cert.TLS)
		}
		message, ok := converseWithRelay(conn, cert.TLS, opts)
		if ok {
			result <- message
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, roots, result
}

func converseWithRelay(conn net.Conn, tlsConfig *tls.Config, opts relayOptions) (relayMessage, bool) {
	var message relayMessage
	proto := textproto.NewConn(conn)
	if opts.stall != "greeting" {
		_ = proto.PrintfLine("220 relay.example.com ready")
	}
	for {
		line, err := proto.ReadLine()
		if err != nil {
			return message, false
		}
		verb, arg, _ := strings.Cut(line, " ")
		if verb == opts.stall {
			continue
		}
		if verb == opts.reject {
			_ = proto.PrintfLine("550 refused " + smtpTestUsername + " " + smtpTestPassword)
			continue
		}
		switch verb {
		case "*":
			_ = proto.PrintfLine("501 canceled")
		case "EHLO":
			advertiseRelayCapabilities(proto, opts)
		case "STARTTLS":
			_ = proto.PrintfLine("220 ready for TLS")
			conn = tls.Server(conn, tlsConfig)
			proto = textproto.NewConn(conn)
			opts.mode = config.SMTPImplicitTLS
		case "AUTH":
			message.auth = arg
			_ = proto.PrintfLine("235 authenticated")
		case "RCPT":
			message.recipients = append(message.recipients, arg)
			_ = proto.PrintfLine("250 accepted")
		case "MAIL":
			_ = proto.PrintfLine("250 accepted")
		case "DATA":
			_ = proto.PrintfLine("354 end with dot")
			data, readErr := proto.ReadDotBytes()
			if readErr != nil {
				return message, false
			}
			message.data = string(data)
			_ = proto.PrintfLine("250 queued")
		case "QUIT":
			_ = proto.PrintfLine("221 bye")
			return message, message.data != ""
		}
	}
}

func advertiseRelayCapabilities(proto *textproto.Conn, opts relayOptions) {
	_ = proto.PrintfLine("250-relay.example.com")
	if opts.mode == config.SMTPStartTLS {
		_ = proto.PrintfLine("250-STARTTLS")
	}
	if opts.auth {
		_ = proto.PrintfLine("250-AUTH PLAIN")
	}
	_ = proto.PrintfLine("250 8BITMIME")
}

func emailDelivery(port int, mode string) Delivery {
	return Delivery{
		Destination: config.NotificationDestination{
			Type: "email",
			EmailConfig: config.EmailConfig{
				SMTP: &config.SMTPConfig{Host: "127.0.0.1", Port: &port, TLS: mode},
				From: "FinOps <alerts@example.com>",
				To:   []string{"owner@example.com", "Finance <finance@example.com>"},
				Cc:   []string{`"copy alerts"@example.com`},
			},
		},
		Event: NewBudgetAlertEvent(BudgetAlertInput{
			Amount: 100, Currency: "USD", Threshold: 80, AlertType: config.AlertTypeActual,
			CurrentSpend: 85, CurrentPercent: 85, ForecastedSpend: 105, ForecastPercent: 105,
		}),
	}
}

func TestEmailDeliversMessage(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{config.SMTPStartTLS, config.SMTPImplicitTLS, config.SMTPPlaintext} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			port, roots, messages := testRelay(t, relayOptions{mode: mode, auth: true})
			delivery := emailDelivery(port, mode)
			delivery.Destination.SMTP.Username = smtpTestUsername
			delivery.Destination.SMTP.Password = smtpTestPassword
			if mode == config.SMTPImplicitTLS {
				delivery.Event.Spend = 105
				delivery.Event.SpendPercent = 105
				delivery.Event.AlertType = config.AlertTypeForecasted
				delivery.Destination.Subject = "Prévision à {{threshold}} %"
			}
			dispatcher := NewDispatcher(nil, nil)
			dispatcher.register("email", &emailSender{rootCAs: roots})
			got := dispatcher.Deliver(t.Context(), []Delivery{delivery}, Options{Timeout: 2 * time.Second})
			require.Equal(t, OutcomeSent, got[0].Outcome, got[0].Reason)
			select {
			case msg := <-messages:
				assert.Equal(
					t,
					[]string{"TO:<owner@example.com>", "TO:<finance@example.com>", `TO:<"copy alerts"@example.com>`},
					msg.recipients,
				)
				assert.Equal(t, "PLAIN AHNlY3JldC11c2VybmFtZQBzZWNyZXQtcGFzc3dvcmQ=", msg.auth)
				assertEmailMessage(t, mode, msg.data)
			case <-time.After(time.Second):
				t.Fatal("relay did not record the message")
			}
		})
	}
}

func assertEmailMessage(t *testing.T, mode, data string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(data))
	require.NoError(t, err)
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	require.NoError(t, err)
	assert.Contains(t, msg.Header.Get("From"), "alerts@example.com")
	assert.Contains(t, msg.Header.Get("To"), "finance@example.com")
	assert.Contains(t, msg.Header.Get("Cc"), `"copy alerts"@example.com`)
	assert.NotEmpty(t, msg.Header.Get("Message-ID"))
	_, err = msg.Header.Date()
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", msg.Header.Get("Content-Type"))
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	require.NoError(t, err)
	for _, part := range []string{"$100.00/month", "global", "EXCEEDED", "FinFocus"} {
		assert.Contains(t, string(body), part)
	}
	if mode == config.SMTPImplicitTLS {
		assert.Equal(t, "Prévision à 80 %", subject)
		assert.Contains(t, string(body), "Forecasted Spend: $105.00 (105%)")
		assert.Contains(t, string(body), "80% (forecasted)")
	} else {
		assert.Equal(t, "[FinFocus] Budget Alert: 80% threshold exceeded", subject)
		assert.Contains(t, string(body), "Current Spend: $85.00 (85%)")
	}
}

func TestEmailDeliveryFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, mode, want string
		opts             relayOptions
	}{
		{"missing TLS", config.SMTPStartTLS, "does not offer STARTTLS", relayOptions{}},
		{"untrusted certificate", config.SMTPImplicitTLS, "certificate", relayOptions{mode: config.SMTPImplicitTLS}},
		{"auth rejected", config.SMTPPlaintext, "authentication failed (SMTP 550)", relayOptions{auth: true, reject: "AUTH"}},
		{"recipient rejected", config.SMTPPlaintext, "recipient rejected (SMTP 550)", relayOptions{auth: true, reject: "RCPT"}},
		{"data rejected", config.SMTPPlaintext, "DATA rejected (SMTP 550)", relayOptions{auth: true, reject: "DATA"}},
		{"stalled greeting", config.SMTPPlaintext, "timed out", relayOptions{stall: "greeting"}},
		{"stalled delivery", config.SMTPPlaintext, "timed out", relayOptions{auth: true, stall: "RCPT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			port, _, messages := testRelay(t, tc.opts)
			delivery := emailDelivery(port, tc.mode)
			if tc.opts.auth {
				delivery.Destination.SMTP.Username = smtpTestUsername
				delivery.Destination.SMTP.Password = smtpTestPassword
			}
			timeout := 2 * time.Second
			if tc.opts.stall != "" {
				timeout = 100 * time.Millisecond
			}
			result := NewDispatcher(nil, nil).Deliver(t.Context(), []Delivery{delivery}, Options{Timeout: timeout})[0]
			assert.Equal(t, OutcomeFailed, result.Outcome)
			assert.Contains(t, result.Reason, tc.want)
			assert.NotContains(t, result.Reason, smtpTestUsername)
			assert.NotContains(t, result.Reason, smtpTestPassword)
			assert.Empty(t, messages)
		})
	}
}

func TestEmailExpansionAndPreview(t *testing.T) {
	t.Parallel()
	lookup := func(name string) (string, bool) {
		value, ok := map[string]string{
			"FINFOCUS_NOTIFY_USER":   smtpTestUsername,
			"FINFOCUS_NOTIFY_PASS":   smtpTestPassword,
			"FINFOCUS_NOTIFY_INJECT": "ok\r\nBcc: unwanted@example.com",
			"FINFOCUS_NOTIFY_TO":     "other@example.com",
		}[name]
		return value, ok
	}
	delivery := emailDelivery(1, config.SMTPPlaintext)
	delivery.Destination.SMTP.Username = "${FINFOCUS_NOTIFY_USER}"
	delivery.Destination.SMTP.Password = "${FINFOCUS_NOTIFY_PASS}"
	delivery.Destination.To = []string{
		"${FINFOCUS_NOTIFY_TO}",
		fmt.Sprintf("%s@example.com", smtpTestUsername),
	}
	dispatcher := NewDispatcher(nil, lookup)
	preview := dispatcher.Deliver(t.Context(), []Delivery{delivery}, Options{DryRun: true})[0]
	require.Equal(t, OutcomeDryRun, preview.Outcome, preview.Reason)
	assert.Contains(t, preview.RecipientSummary, "To: other@example.com, [REDACTED]")
	assert.Contains(t, preview.RecipientSummary, `Cc: "copy alerts"@example.com`)
	assert.NotContains(t, preview.RecipientSummary, smtpTestUsername)
	assert.NotContains(t, preview.RecipientSummary, smtpTestPassword)

	delivery.Destination.Subject = "${FINFOCUS_NOTIFY_INJECT}"
	injected := dispatcher.Deliver(t.Context(), []Delivery{delivery}, Options{})[0]
	assert.Equal(t, OutcomeFailed, injected.Outcome)
	assert.Contains(t, injected.Reason, "CR or LF")

	delivery.Destination.SMTP.Password = "${FINFOCUS_NOTIFY_MISSING}"
	missing := dispatcher.Deliver(t.Context(), []Delivery{delivery}, Options{})[0]
	assert.Equal(t, OutcomeSkipped, missing.Outcome)
	assert.Contains(t, missing.Reason, "FINFOCUS_NOTIFY_MISSING")

	delivery.Destination = delivery.Destination.AsProject()
	noLookup := NewDispatcher(nil, func(string) (string, bool) {
		t.Error("project destination performed a lookup")
		return "", false
	})
	project := noLookup.Deliver(t.Context(), []Delivery{delivery}, Options{})[0]
	assert.Equal(t, OutcomeSkipped, project.Outcome)
	assert.Contains(t, project.Reason, "project config cannot use variables")
}

func TestEmailCanceledBeforeDial(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result := NewDispatcher(nil, nil).Deliver(ctx, []Delivery{emailDelivery(1, config.SMTPPlaintext)}, Options{})[0]
	assert.Equal(t, OutcomeFailed, result.Outcome)
	assert.Equal(t, "canceled", result.Reason)
}

func TestEmailConnectionRefused(t *testing.T) {
	t.Parallel()
	var lc net.ListenConfig
	listener, err := lc.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	dispatcher := NewDispatcher(nil, nil)
	result := dispatcher.Deliver(t.Context(), []Delivery{emailDelivery(port, config.SMTPPlaintext)}, Options{})[0]
	assert.Equal(t, OutcomeFailed, result.Outcome)
	assert.Equal(t, "SMTP connection failed", result.Reason)
}
