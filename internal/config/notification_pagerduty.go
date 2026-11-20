package config

import (
	"errors"
	"net/netip"
	"net/url"
	"strings"
)

const (
	// PagerDutyDefaultURL is the US Events API v2 trigger endpoint.
	PagerDutyDefaultURL = "https://events.pagerduty.com/v2/enqueue"
	// PagerDutyDefaultSeverity is used when severity is omitted.
	PagerDutyDefaultSeverity = "warning"
	// PagerDutyPerRun lets PagerDuty assign a fresh event key.
	PagerDutyPerRun = "per-run"
)

const (
	notificationFieldRoutingKey = "routing_key"
	notificationFieldSeverity   = "severity"
	notificationFieldDedupKey   = "dedup_key"
	pagerDutySeverityError      = "error"
)

func (d NotificationDestination) firstPagerDutyField() string {
	switch {
	case d.RoutingKey != "":
		return notificationFieldRoutingKey
	case d.Severity != "":
		return notificationFieldSeverity
	case d.DedupKey != "":
		return notificationFieldDedupKey
	default:
		return ""
	}
}

func (d NotificationDestination) validatePagerDuty() error {
	if field := d.EmailConfig.firstSetField(); field != "" {
		return fieldNotAllowed(field, NotificationTypeEmail)
	}
	switch {
	case d.Channel != "":
		return fieldNotAllowed(notificationFieldChannel, NotificationTypeSlack)
	case d.Method != "":
		return fieldNotAllowed(notificationFieldMethod, NotificationTypeWebhook)
	case len(d.Headers) != 0:
		return fieldNotAllowed(notificationFieldHeaders, NotificationTypeWebhook)
	}
	if strings.TrimSpace(d.RoutingKey) == "" {
		return &NotificationFieldError{
			Field: notificationFieldRoutingKey,
			Err:   errors.New("PagerDuty routing key is required"),
		}
	}
	if _, err := checkNotificationValue(d.RoutingKey); err != nil {
		return &NotificationFieldError{Field: notificationFieldRoutingKey, Err: err}
	}

	switch d.Severity {
	case "", "critical", pagerDutySeverityError, PagerDutyDefaultSeverity, "info":
	default:
		return &NotificationFieldError{
			Field: notificationFieldSeverity,
			Err:   errors.New("PagerDuty severity must be critical, error, warning, or info"),
		}
	}
	switch d.DedupKey {
	case "", "per-threshold", PagerDutyPerRun:
	default:
		return &NotificationFieldError{
			Field: notificationFieldDedupKey,
			Err:   errors.New("PagerDuty dedup_key must be per-threshold or per-run"),
		}
	}

	if d.URL == "" {
		return nil
	}
	if err := validateNotificationURL(d.URL); err != nil {
		return err
	}
	if !HasNotificationReference(d.URL) && !IsPagerDutyEndpoint(d.URL) {
		return &NotificationFieldError{Field: notificationFieldURL, Err: ErrPagerDutyEndpoint}
	}
	return nil
}

// ErrPagerDutyEndpoint identifies a URL outside the Events API or local test endpoints.
var ErrPagerDutyEndpoint = errors.New(
	"use an HTTPS US or EU PagerDuty /v2/enqueue endpoint or an HTTPS loopback test endpoint",
)

// IsPagerDutyEndpoint reports whether raw is an allowed PagerDuty target.
func IsPagerDutyEndpoint(raw string) bool {
	target, err := url.Parse(raw)
	if err != nil || target.Scheme != "https" || target.User != nil ||
		target.EscapedPath() != "/v2/enqueue" || target.ForceQuery ||
		target.RawQuery != "" || strings.Contains(raw, "#") {
		return false
	}

	host := strings.ToLower(target.Hostname())
	switch host {
	case "events.pagerduty.com", "events.eu.pagerduty.com":
		return target.Host == target.Hostname() || target.Port() == "443"
	default:
		ip, parseErr := netip.ParseAddr(host)
		return parseErr == nil && ip.Zone() == "" && ip.Unmap().IsLoopback()
	}
}
