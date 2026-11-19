package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"slices"
	"strings"
	"unicode"
)

// SMTP TLS modes. An omitted mode uses STARTTLS.
const (
	SMTPStartTLS    = "starttls"
	SMTPImplicitTLS = "tls"
	SMTPPlaintext   = "none"
)

const (
	smtpStartTLSPort    = 587
	smtpImplicitTLSPort = 465
	smtpPlaintextPort   = 25
	maxSMTPPort         = 65535
)

// SMTPConfig is the relay and its optional credentials.
type SMTPConfig struct {
	Host     string `yaml:"host"               json:"host"`
	Port     *int   `yaml:"port,omitempty"     json:"port,omitempty"`
	TLS      string `yaml:"tls,omitempty"      json:"tls,omitempty"`
	Username string `yaml:"username,omitempty" json:"username,omitempty"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
}

// TLSMode returns the configured mode or STARTTLS.
func (c SMTPConfig) TLSMode() string {
	if c.TLS == "" {
		return SMTPStartTLS
	}
	return c.TLS
}

// RelayPort returns the configured port or the default for the TLS mode.
func (c SMTPConfig) RelayPort() int {
	if c.Port != nil {
		return *c.Port
	}
	switch c.TLSMode() {
	case SMTPImplicitTLS:
		return smtpImplicitTLSPort
	case SMTPPlaintext:
		return smtpPlaintextPort
	default:
		return smtpStartTLSPort
	}
}

// EmailConfig holds fields used only by email destinations.
type EmailConfig struct {
	SMTP    *SMTPConfig `yaml:"smtp,omitempty"    json:"smtp,omitempty"`
	From    string      `yaml:"from,omitempty"    json:"from,omitempty"`
	To      []string    `yaml:"to,omitempty"      json:"to,omitempty"`
	Cc      []string    `yaml:"cc,omitempty"      json:"cc,omitempty"`
	Subject string      `yaml:"subject,omitempty" json:"subject,omitempty"`
}

func (c *EmailConfig) firstSetField() string {
	switch {
	case c.SMTP != nil:
		return "smtp"
	case c.From != "":
		return "from"
	case len(c.To) != 0:
		return "to"
	case len(c.Cc) != 0:
		return "cc"
	case c.Subject != "":
		return "subject"
	default:
		return ""
	}
}

// MapValues applies transform to each expandable email field in a copy.
func (c *EmailConfig) MapValues(transform func(field, value string) (string, error)) (EmailConfig, error) {
	mappedConfig := c.copyValues()
	for field, value := range mappedConfig.values {
		mapped, err := transform(field, *value)
		if err != nil {
			return EmailConfig{}, &NotificationFieldError{Field: field, Err: err}
		}
		*value = mapped
	}
	return mappedConfig, nil
}

func (c *EmailConfig) copyValues() EmailConfig {
	cloned := *c
	cloned.To = slices.Clone(c.To)
	cloned.Cc = slices.Clone(c.Cc)
	if c.SMTP != nil {
		smtp := *c.SMTP
		cloned.SMTP = &smtp
	}
	return cloned
}

func (c *EmailConfig) values(yield func(string, *string) bool) {
	if c.SMTP != nil && (!yield("smtp.host", &c.SMTP.Host) ||
		!yield("smtp.username", &c.SMTP.Username) || !yield("smtp.password", &c.SMTP.Password)) {
		return
	}
	if !yield("from", &c.From) {
		return
	}
	for i := range c.To {
		if !yield(fmt.Sprintf("to[%d]", i), &c.To[i]) {
			return
		}
	}
	for i := range c.Cc {
		if !yield(fmt.Sprintf("cc[%d]", i), &c.Cc[i]) {
			return
		}
	}
	yield("subject", &c.Subject)
}

// ValidateResolved validates an email destination after expansion. Expanded
// strings are treated as literal values, even if they contain a dollar sign.
func (c *EmailConfig) ValidateResolved() error {
	return c.validate(false)
}

func (c *EmailConfig) validate(allowReferences bool) error {
	if c.SMTP == nil {
		return emailFieldError("smtp.host", "SMTP host is required")
	}
	switch c.SMTP.TLSMode() {
	case SMTPStartTLS, SMTPImplicitTLS, SMTPPlaintext:
	default:
		return emailFieldError("smtp.tls", "SMTP TLS mode must be starttls, tls, or none")
	}
	if port := c.SMTP.RelayPort(); port < 1 || port > maxSMTPPort {
		return emailFieldError("smtp.port", "SMTP port must be between 1 and 65535")
	}
	if len(c.To) == 0 {
		return emailFieldError("to", "at least one To address is required")
	}
	for field, raw := range c.values {
		if allowReferences {
			refs, err := checkNotificationValue(*raw)
			if err != nil {
				return &NotificationFieldError{Field: field, Err: err}
			}
			if len(refs) > 0 {
				continue
			}
		}
		if err := c.validateValue(field, *raw); err != nil {
			return err
		}
	}
	return nil
}

func (c *EmailConfig) validateValue(field, value string) error {
	switch field {
	case "smtp.host":
		return validateSMTPHost(value, c.SMTP.TLSMode())
	case "smtp.username", "smtp.password":
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return emailFieldError(field, "email headers must not contain CR or LF")
	}
	if field != "subject" {
		if _, err := mail.ParseAddress(value); err != nil {
			return emailFieldError(field, "a valid email address is required")
		}
	}
	return nil
}

func validateSMTPHost(host, mode string) error {
	if host == "" {
		return emailFieldError("smtp.host", "SMTP host is required")
	}
	ip, err := netip.ParseAddr(host)
	isLocalhost := strings.EqualFold(host, "localhost")
	if mode == SMTPPlaintext && !isLocalhost && !ip.Unmap().IsLoopback() {
		return emailFieldError("smtp.host", "plaintext SMTP requires a loopback host")
	}
	if err == nil && ip.Zone() == "" {
		return nil
	}
	if strings.ContainsAny(host, ":/\\?#@[]%") || strings.ContainsFunc(host, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) {
		return emailFieldError("smtp.host", "use a hostname or IP address without a scheme or port")
	}
	return nil
}

func emailFieldError(field, message string) error {
	return &NotificationFieldError{Field: field, Err: errors.New(message)}
}
