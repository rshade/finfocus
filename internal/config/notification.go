package config

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// NotificationType names a budget alert destination type.
type NotificationType string

// Supported notification destination types.
const (
	// NotificationTypeSlack posts to a Slack incoming webhook.
	NotificationTypeSlack NotificationType = "slack"
	// NotificationTypeWebhook sends the budget.threshold.exceeded JSON event to an HTTPS endpoint.
	NotificationTypeWebhook NotificationType = "webhook"
)

// NotificationVariablePrefix is the only environment variable prefix that a
// destination's ${NAME} references may expand. Any other name is refused so a
// committed configuration cannot read unrelated CI secrets.
const NotificationVariablePrefix = "FINFOCUS_NOTIFY_"

const (
	notificationFieldType    = "type"
	notificationFieldURL     = "url"
	notificationFieldChannel = "channel"
	notificationFieldMethod  = "method"
	notificationFieldHeaders = "headers"
	referenceOpen            = "${"
	referenceClose           = "}"
)

// Notification validation errors.
var (
	ErrNotificationTypeInvalid     = errors.New("notification type must be one of: slack, webhook")
	ErrNotificationURLRequired     = errors.New("notification url is required")
	ErrNotificationHTTPSRequired   = errors.New("HTTPS is required: the url must start with https:// and name a host")
	ErrNotificationMethodInvalid   = errors.New("notification method must be POST or PUT")
	ErrNotificationFieldNotAllowed = errors.New("field is not valid for this destination type")
	ErrNotificationHeaderNameEmpty = errors.New("notification header names must not be empty")
	// ErrNotificationVariableNotAllowed is returned for a ${NAME} reference whose
	// name does not start with NotificationVariablePrefix.
	ErrNotificationVariableNotAllowed = errors.New("not a " + NotificationVariablePrefix + " variable")
	// ErrNotificationReferenceMalformed is returned for "${" without a closing
	// brace or without a variable name made of letters, digits, and underscores.
	ErrNotificationReferenceMalformed = errors.New("malformed variable reference")
	// ErrNotificationProjectVariable is returned when a project config
	// destination uses a variable reference.
	ErrNotificationProjectVariable = errors.New(
		"project config destinations cannot use variables; move this destination to the global config")
)

// NotificationFieldError reports which destination field failed validation.
// Field is a path relative to the destination, such as "url" or
// "headers.Authorization". The message never contains the field's value.
type NotificationFieldError struct {
	Field string
	Err   error
}

// Error returns the field name followed by the underlying error.
func (e *NotificationFieldError) Error() string {
	return e.Field + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *NotificationFieldError) Unwrap() error {
	return e.Err
}

type destinationSource int

const (
	destinationSourceGlobal destinationSource = iota
	destinationSourceProject
)

// NotificationDestination is one place a budget alert is delivered.
// URL and every header value are secrets: they are never logged and are
// masked by config get and config list.
type NotificationDestination struct {
	// Type is "slack" or "webhook".
	Type NotificationType `yaml:"type" json:"type"`
	// URL is the HTTPS endpoint. It may contain ${FINFOCUS_NOTIFY_*} references.
	URL string `yaml:"url" json:"url"`
	// Channel optionally overrides the Slack channel. Slack destinations only.
	Channel string `yaml:"channel,omitempty" json:"channel,omitempty"`
	// Method is POST (default) or PUT. Webhook destinations only.
	Method string `yaml:"method,omitempty" json:"method,omitempty"`
	// Headers are extra request headers. Values may contain ${FINFOCUS_NOTIFY_*}
	// references. Webhook destinations only.
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	source destinationSource
}

// FromProject reports whether the destination was loaded from a project
// config overlay. Such destinations never expand variables.
func (d NotificationDestination) FromProject() bool {
	return d.source == destinationSourceProject
}

// AsProject returns a copy of d marked as loaded from a project config.
// The mark only removes the ability to expand variables.
func (d NotificationDestination) AsProject() NotificationDestination {
	d.source = destinationSourceProject
	return d
}

// Validate checks the destination type, required fields, the HTTPS rule for a
// literal URL, and that every ${NAME} reference names a FINFOCUS_NOTIFY_
// variable. A URL that contains a reference is checked for HTTPS after
// expansion at send time instead.
func (d NotificationDestination) Validate() error {
	switch d.Type {
	case NotificationTypeSlack:
		if d.Method != "" {
			return fieldNotAllowed(notificationFieldMethod, NotificationTypeWebhook)
		}
		if len(d.Headers) > 0 {
			return fieldNotAllowed(notificationFieldHeaders, NotificationTypeWebhook)
		}
	case NotificationTypeWebhook:
		if d.Channel != "" {
			return fieldNotAllowed(notificationFieldChannel, NotificationTypeSlack)
		}
		if err := validateNotificationMethod(d.Method); err != nil {
			return err
		}
	default:
		return &NotificationFieldError{
			Field: notificationFieldType,
			Err:   fmt.Errorf("%w (got %q)", ErrNotificationTypeInvalid, d.Type),
		}
	}
	if err := validateNotificationURL(d.URL); err != nil {
		return err
	}
	return validateNotificationHeaders(d.Headers)
}

func fieldNotAllowed(field string, owner NotificationType) error {
	return &NotificationFieldError{
		Field: field,
		Err:   fmt.Errorf("%w: %s is only valid for %s destinations", ErrNotificationFieldNotAllowed, field, owner),
	}
}

func validateNotificationMethod(method string) error {
	switch strings.ToUpper(method) {
	case "", "POST", "PUT":
		return nil
	default:
		return &NotificationFieldError{Field: notificationFieldMethod, Err: ErrNotificationMethodInvalid}
	}
}

func validateNotificationURL(raw string) error {
	if raw == "" {
		return &NotificationFieldError{Field: notificationFieldURL, Err: ErrNotificationURLRequired}
	}
	refs, err := checkNotificationValue(raw)
	if err != nil {
		return &NotificationFieldError{Field: notificationFieldURL, Err: err}
	}
	if len(refs) > 0 {
		return nil
	}
	if !IsHTTPSURL(raw) {
		return &NotificationFieldError{Field: notificationFieldURL, Err: ErrNotificationHTTPSRequired}
	}
	return nil
}

func validateNotificationHeaders(headers map[string]string) error {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return &NotificationFieldError{Field: notificationFieldHeaders, Err: ErrNotificationHeaderNameEmpty}
		}
		if _, err := checkNotificationValue(headers[name]); err != nil {
			return &NotificationFieldError{Field: notificationFieldHeaders + "." + name, Err: err}
		}
	}
	return nil
}

func checkNotificationValue(value string) ([]NotificationReference, error) {
	refs, err := FindNotificationReferences(value)
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		if !IsNotificationVariable(ref.Name) {
			return nil, fmt.Errorf("variable %s is %w", ref.Name, ErrNotificationVariableNotAllowed)
		}
	}
	return refs, nil
}

// IsHTTPSURL reports whether raw parses as an https URL with a host.
func IsHTTPSURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

// NotificationReference is one ${NAME} occurrence in a destination value.
// Start and End are byte offsets of the whole reference, so
// value[Start:End] is "${NAME}".
type NotificationReference struct {
	Start int
	End   int
	Name  string
}

// FindNotificationReferences returns every ${NAME} occurrence in value, in
// order. A "${" without a closing brace, or with a name that is not made of
// letters, digits, and underscores, returns ErrNotificationReferenceMalformed;
// the error does not quote the text, which may hold a secret. A "$" that is
// not followed by "{" is literal.
func FindNotificationReferences(value string) ([]NotificationReference, error) {
	var refs []NotificationReference
	offset := 0
	for {
		idx := strings.Index(value[offset:], referenceOpen)
		if idx < 0 {
			return refs, nil
		}
		start := offset + idx
		nameStart := start + len(referenceOpen)
		closeIdx := strings.Index(value[nameStart:], referenceClose)
		if closeIdx < 0 {
			return nil, ErrNotificationReferenceMalformed
		}
		name := value[nameStart : nameStart+closeIdx]
		if !isVariableName(name) {
			return nil, ErrNotificationReferenceMalformed
		}
		end := nameStart + closeIdx + len(referenceClose)
		refs = append(refs, NotificationReference{Start: start, End: end, Name: name})
		offset = end
	}
}

// IsNotificationVariable reports whether name may be expanded in a destination:
// it starts with NotificationVariablePrefix and has at least one more character.
func IsNotificationVariable(name string) bool {
	return len(name) > len(NotificationVariablePrefix) &&
		strings.HasPrefix(name, NotificationVariablePrefix) && isVariableName(name)
}

func isVariableName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// hasNotificationReference reports whether value contains any "${".
func hasNotificationReference(value string) bool {
	return strings.Contains(value, referenceOpen)
}

// DestinationHasReference reports whether the URL or any header value of d
// contains a "${" reference.
func DestinationHasReference(d NotificationDestination) bool {
	if hasNotificationReference(d.URL) {
		return true
	}
	for _, value := range d.Headers {
		if hasNotificationReference(value) {
			return true
		}
	}
	return false
}

// forEachDestination calls fn with a pointer to every notification destination
// in budgets and its configuration path, such as
// "cost.budgets.providers.aws.alerts[0].notifications[1]". Provider and type
// scopes are visited in sorted key order.
func forEachDestination(budgets *BudgetsConfig, fn func(path string, dest *NotificationDestination)) {
	if budgets == nil {
		return
	}
	visitScope := func(path string, scope *ScopedBudget) {
		if scope == nil {
			return
		}
		for i := range scope.Alerts {
			alert := &scope.Alerts[i]
			for j := range alert.Notifications {
				fn(fmt.Sprintf("%s.alerts[%d].notifications[%d]", path, i, j), &alert.Notifications[j])
			}
		}
	}
	visitScope(costBudgetsPath+".global", budgets.Global)
	for _, name := range sortedScopeKeys(budgets.Providers) {
		visitScope(costBudgetsPath+".providers."+name, budgets.Providers[name])
	}
	for i := range budgets.Tags {
		visitScope(fmt.Sprintf("%s.tags[%d]", costBudgetsPath, i), &budgets.Tags[i].ScopedBudget)
	}
	for _, name := range sortedScopeKeys(budgets.Types) {
		visitScope(costBudgetsPath+".types."+name, budgets.Types[name])
	}
}

func sortedScopeKeys(scopes map[string]*ScopedBudget) []string {
	keys := make([]string, 0, len(scopes))
	for key := range scopes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// markProjectDestinations marks every destination in budgets as loaded from a
// project config.
func markProjectDestinations(budgets *BudgetsConfig) {
	forEachDestination(budgets, func(_ string, dest *NotificationDestination) {
		*dest = dest.AsProject()
	})
}

// projectDestinationErrors reports every project config destination that uses
// a variable reference, with the path of the offending field.
func projectDestinationErrors(budgets *BudgetsConfig) []ValidationError {
	var errs []ValidationError
	forEachDestination(budgets, func(path string, dest *NotificationDestination) {
		fields := make([]string, 0, 1+len(dest.Headers))
		if hasNotificationReference(dest.URL) {
			fields = append(fields, notificationFieldURL)
		}
		names := make([]string, 0, len(dest.Headers))
		for name := range dest.Headers {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if hasNotificationReference(dest.Headers[name]) {
				fields = append(fields, notificationFieldHeaders+"."+name)
			}
		}
		for _, field := range fields {
			hint, example := hintFor(ErrNotificationProjectVariable)
			errs = append(errs, ValidationError{
				Path:    path + "." + field,
				Message: ErrNotificationProjectVariable.Error(),
				Hint:    hint,
				Example: example,
			})
		}
	})
	return errs
}
