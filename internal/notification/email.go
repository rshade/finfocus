package notification

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
)

type emailSender struct {
	rootCAs *x509.CertPool
}

func (s *emailSender) Send(ctx context.Context, dest Resolved, event BudgetAlertEvent) error {
	settings := dest.Email.SMTP
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(settings.Host, strconv.Itoa(settings.RelayPort())))
	if err != nil {
		return smtpFailure(ctx, "SMTP connection failed", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if deadlineErr := conn.SetDeadline(deadline); deadlineErr != nil {
			return smtpFailure(ctx, "setting SMTP deadline failed", deadlineErr)
		}
	}

	client, err := s.connect(ctx, conn, *settings)
	if err != nil {
		return err
	}
	defer client.Close()
	if sendErr := sendEmail(ctx, client, dest.Email, event); sendErr != nil {
		return sendErr
	}
	// DATA has been accepted. A disconnect during QUIT cannot undo delivery.
	_ = client.Quit()
	return nil
}

func (s *emailSender) connect(ctx context.Context, conn net.Conn, settings config.SMTPConfig) (*smtp.Client, error) {
	mode := settings.TLSMode()
	host := settings.Host
	tlsConfig := &tls.Config{ServerName: host, RootCAs: s.rootCAs, MinVersion: tls.VersionTLS12}
	switch mode {
	case config.SMTPPlaintext:
		// Resolve localhost normally (IPv4 or IPv6) but verify the peer before
		// smtp.NewClient reads a greeting or PlainAuth permits cleartext auth.
		if peer, ok := conn.RemoteAddr().(*net.TCPAddr); !ok || !peer.IP.IsLoopback() {
			return nil, errors.New("plaintext SMTP requires a loopback peer")
		}
		host = "localhost"
	case config.SMTPImplicitTLS:
		secure := tls.Client(conn, tlsConfig)
		if err := secure.HandshakeContext(ctx); err != nil {
			return nil, smtpFailure(ctx, "SMTP TLS handshake failed", err)
		}
		conn = secure
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, smtpFailure(ctx, "SMTP greeting failed", err)
	}
	if mode == config.SMTPStartTLS {
		if tlsErr := requireStartTLS(ctx, client, tlsConfig); tlsErr != nil {
			return nil, tlsErr
		}
	}
	if settings.Username != "" {
		auth := smtp.PlainAuth("", settings.Username, settings.Password, host)
		if authErr := client.Auth(auth); authErr != nil {
			return nil, smtpFailure(ctx, "SMTP authentication failed", authErr)
		}
	}
	return client, nil
}

func requireStartTLS(ctx context.Context, client *smtp.Client, tlsConfig *tls.Config) error {
	// Ask for EHLO now so a failed or timed-out greeting remains distinct from
	// a successful greeting that did not advertise the required extension.
	if err := client.Hello("localhost"); err != nil {
		return smtpFailure(ctx, "SMTP greeting failed", err)
	}
	if ok, _ := client.Extension("STARTTLS"); !ok {
		return errors.New("SMTP server does not offer STARTTLS")
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return smtpFailure(ctx, "SMTP STARTTLS failed", err)
	}
	return nil
}

// smtpFailure reports the stage and reply status only. A relay's response text
// can include credentials, auth blobs, addresses, or other sensitive data.
func smtpFailure(ctx context.Context, stage string, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if reply, ok := errors.AsType[*textproto.Error](err); ok {
		return fmt.Errorf("%s (SMTP %d)", stage, reply.Code)
	}
	if _, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return fmt.Errorf("%s: certificate verification failed", stage)
	}
	return errors.New(stage)
}

func sendEmail(ctx context.Context, client *smtp.Client, email config.EmailConfig, event BudgetAlertEvent) error {
	from, fromErr := mail.ParseAddress(email.From)
	if fromErr != nil {
		return errors.New("invalid sender address")
	}
	to, toErr := parseEmailAddresses(email.To)
	if toErr != nil {
		return toErr
	}
	cc, ccErr := parseEmailAddresses(email.Cc)
	if ccErr != nil {
		return ccErr
	}
	headers, headerErr := emailHeaders(from, to, cc, email.Subject, event)
	if headerErr != nil {
		return headerErr
	}
	if err := client.Mail(emailMailbox(from)); err != nil {
		return smtpFailure(ctx, "SMTP sender rejected", err)
	}
	for _, recipient := range append(to, cc...) {
		if err := client.Rcpt(emailMailbox(recipient)); err != nil {
			return smtpFailure(ctx, "SMTP recipient rejected", err)
		}
	}

	data, dataErr := client.Data()
	if dataErr != nil {
		return smtpFailure(ctx, "SMTP DATA rejected", dataErr)
	}
	if err := writeEmail(data, headers, event); err != nil {
		return smtpFailure(ctx, "writing SMTP message failed", err)
	}
	if err := data.Close(); err != nil {
		return smtpFailure(ctx, "SMTP message rejected", err)
	}
	return nil
}

func parseEmailAddresses(values []string) ([]*mail.Address, error) {
	addresses := make([]*mail.Address, len(values))
	for i, value := range values {
		address, err := mail.ParseAddress(value)
		if err != nil {
			return nil, errors.New("invalid recipient address")
		}
		addresses[i] = address
	}
	return addresses, nil
}

// emailMailbox preserves quoting for local parts that need it in SMTP.
func emailMailbox(address *mail.Address) string {
	wire := (&mail.Address{Address: address.Address}).String()
	return wire[1 : len(wire)-1]
}

func emailHeaders(
	from *mail.Address, to, cc []*mail.Address, subject string, event BudgetAlertEvent,
) (string, error) {
	threshold := emailPercent(event.ThresholdPercent)
	if subject == "" {
		subject = "[FinFocus] Budget Alert: " + threshold + "% threshold exceeded"
	} else {
		subject = strings.ReplaceAll(subject, "{{threshold}}", threshold)
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", errors.New("subject: email headers must not contain CR or LF")
	}
	subject = mime.QEncoding.Encode("UTF-8", subject)
	subject = strings.ReplaceAll(subject, "?= =?", "?=\r\n =?")

	var headers strings.Builder
	fmt.Fprintf(&headers, "From: %s\r\nTo: %s\r\n", from, addressHeader(to))
	if len(cc) != 0 {
		fmt.Fprintf(&headers, "Cc: %s\r\n", addressHeader(cc))
	}
	fmt.Fprintf(&headers, "Subject: %s\r\nDate: %s\r\nMessage-ID: <%s@finfocus.local>\r\n",
		subject, event.Timestamp.Format(time.RFC1123Z), rand.Text())
	headers.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n")
	headers.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	const maxHeaderLine = 998 // RFC 5322, excluding CRLF
	for line := range strings.SplitSeq(headers.String(), "\r\n") {
		if len(line) > maxHeaderLine {
			return "", errors.New("email header line exceeds 998 bytes")
		}
	}
	return headers.String(), nil
}

func addressHeader(addresses []*mail.Address) string {
	names := make([]string, len(addresses))
	for i, address := range addresses {
		names[i] = address.String()
	}
	return strings.Join(names, ",\r\n ")
}

func writeEmail(writer io.Writer, headers string, event BudgetAlertEvent) error {
	if _, err := io.WriteString(writer, headers); err != nil {
		return err
	}
	body := quotedprintable.NewWriter(writer)
	spendLabel := "Current Spend"
	if event.AlertType == config.AlertTypeForecasted {
		spendLabel = "Forecasted Spend"
	}
	period := event.Period
	if period == config.DefaultBudgetPeriod {
		period = "month"
	}
	_, err := fmt.Fprintf(body, "Budget Alert\n============\n\n"+
		"Your configured budget threshold has been exceeded.\n\n"+
		"Budget: %s/%s\nScope: %s\n%s: %s (%s%%)\n"+
		"Threshold: %s%% (%s)\nStatus: EXCEEDED\n\n---\nThis alert was generated by FinFocus %s.\n",
		emailMoney(event.Amount, event.Currency), period, event.BudgetName(), spendLabel,
		emailMoney(event.Spend, event.Currency), emailPercent(event.SpendPercent),
		emailPercent(event.ThresholdPercent), event.AlertType, event.Version)
	if err != nil {
		return err
	}
	return body.Close()
}

func emailMoney(amount float64, currency string) string {
	if currency == "USD" {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%s %.2f", currency, amount)
}

func emailPercent(percent float64) string {
	return strconv.FormatFloat(percent, 'f', -1, 64)
}
