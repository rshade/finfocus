package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
)

const (
	costBudgetsPath  = "cost.budgets"
	fieldAmount      = "amount"
	fieldAlerts      = "alerts"
	fieldCurrency    = "currency"
	fieldExitCode    = "exit_code"
	fieldPeriod      = "period"
	fieldThreshold   = "threshold"
	fieldType        = "type"
	maxTypoDistance  = 2
	syntaxHint       = "Fix the JSON syntax. Comments and trailing commas are allowed."
	unknownFieldText = "Unknown field"
)

// ValidationError is one problem in a configuration file.
type ValidationError struct {
	// File is set when the finding belongs to a different file than the
	// result's File, such as the project config checked by a bare config validate.
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Example string `json:"example,omitempty"`
}

// ValidationWarning is a non-fatal configuration finding.
type ValidationWarning struct {
	// File is set when the finding belongs to a different file than the result's File.
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Path       string `json:"path"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ValidationResult is the report from ValidateConfig and ValidateConfigSource.
type ValidationResult struct {
	Valid    bool                `json:"valid"`
	File     string              `json:"file,omitempty"`
	Errors   []ValidationError   `json:"errors"`
	Warnings []ValidationWarning `json:"warnings"`
}

// ValidateConfig checks cfg with Config.Validate and returns hints for known failures.
// Line numbers are zero because cfg has no source text. Use ValidateConfigSource for a file.
func ValidateConfig(cfg *Config) ValidationResult {
	result := newValidationResult("")
	if cfg == nil {
		result.Valid = true
		return result
	}
	result.File = cfg.ConfigPath()
	appendSemantic(&result, cfg, nil)
	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateConfigSource checks a HuJSON configuration document.
// path is stored on the result and is not read from disk.
// The returned Config is nil when the document cannot be decoded.
func ValidateConfigSource(path string, data []byte) (ValidationResult, *Config) {
	result, cfg, _ := validateConfigSource(path, data)
	return result, cfg
}

// ValidateProjectConfigSource checks a project config document
// ($PROJECT/.finfocus/config.hujson). It applies every ValidateConfigSource
// rule and also rejects any ${...} reference in a notification destination,
// because a project config is committed and can be changed by a pull request.
//
// A legacy YAML project file (config.yaml) is converted to JSON first, so its
// findings carry paths but no line numbers.
func ValidateProjectConfigSource(path string, data []byte) (ValidationResult, *Config) {
	fromYAML := isYAMLPath(path)
	if fromYAML {
		converted, err := yamlToJSON(data)
		if err != nil {
			result := newValidationResult(path)
			result.Errors = append(result.Errors, ValidationError{
				Message: "configuration syntax is invalid: " + err.Error(),
				Hint:    "Fix the YAML syntax, or convert the file to config.hujson.",
			})
			return result, nil
		}
		data = converted
	}
	result, cfg, lines := validateConfigSource(path, data)
	if cfg == nil {
		if fromYAML {
			clearLines(&result)
		}
		return result, nil
	}
	for _, projectErr := range projectDestinationErrors(cfg.Cost.Budgets) {
		projectErr.Line = lineForPath(lines, projectErr.Path)
		result.Errors = append(result.Errors, projectErr)
	}
	if fromYAML {
		clearLines(&result)
	}
	result.Valid = len(result.Errors) == 0
	return result, cfg
}

// clearLines removes line numbers that refer to converted text rather than the file.
func clearLines(result *ValidationResult) {
	for i := range result.Errors {
		result.Errors[i].Line = 0
	}
	for i := range result.Warnings {
		result.Warnings[i].Line = 0
	}
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func yamlToJSON(data []byte) ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return json.Marshal(doc)
}

func validateConfigSource(path string, data []byte) (ValidationResult, *Config, map[string]int) {
	result := newValidationResult(path)
	value, err := hujson.Parse(data)
	if err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Line:    lineFromSyntaxError(err),
			Message: "configuration syntax is invalid: " + err.Error(),
			Hint:    syntaxHint,
		})
		return result, nil, nil
	}

	src := &configSource{data: data, lines: map[string]int{}}
	if object, ok := value.Value.(*hujson.Object); ok {
		walkObject(object, "", configStructType(), src, &result)
	} else {
		result.Errors = append(result.Errors, ValidationError{
			Message: "configuration must be a JSON object",
			Hint:    "Start the file with '{' and a set of configuration keys.",
			Example: "{\n  \"cost\": {}\n}",
		})
		return result, nil, nil
	}

	cloned := value.Clone()
	cloned.Standardize()
	cfg := &Config{}
	if unmarshalErr := json.Unmarshal(cloned.Pack(), cfg); unmarshalErr != nil {
		if len(result.Errors) == 0 {
			result.Errors = append(result.Errors, ValidationError{
				Message: unmarshalErr.Error(),
				Hint:    "Check that each value matches the type of its field.",
			})
		}
		result.Valid = false
		return result, nil, nil
	}
	if cfg.Output.DefaultFormat == "" {
		cfg.Output.DefaultFormat = formatTable
	}
	cfg.configPath = path
	appendSemantic(&result, cfg, src.lines)
	result.Valid = len(result.Errors) == 0
	return result, cfg, src.lines
}

func newValidationResult(path string) ValidationResult {
	return ValidationResult{
		File:     path,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}
}

func appendSemantic(result *ValidationResult, cfg *Config, lines map[string]int) {
	if err := cfg.Validate(); err != nil {
		addSemanticError(result, err, lines)
	}
	if cfg.Cost.Budgets == nil {
		return
	}
	warnings, _ := cfg.Cost.Budgets.Validate()
	for _, warning := range warnings {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Path:    costBudgetsPath,
			Message: warning,
		})
	}
}

func addSemanticError(result *ValidationResult, err error, lines map[string]int) {
	path := locateConfigError(err)
	hint, example := hintFor(err)
	result.Errors = append(result.Errors, ValidationError{
		Line:    lineForPath(lines, path),
		Path:    path,
		Message: err.Error(),
		Hint:    hint,
		Example: example,
	})
}

func lineForPath(lines map[string]int, path string) int {
	if lines == nil || path == "" {
		return 0
	}
	if line := lines[path]; line != 0 {
		return line
	}
	field := path[strings.LastIndex(path, ".")+1:]
	for key, line := range lines {
		if key == field || strings.HasSuffix(key, "."+field) {
			return line
		}
	}
	return 0
}

func locateConfigError(err error) string {
	msg := err.Error()
	base := ""
	if strings.Contains(msg, "budget") {
		base = costBudgetsPath
	}
	switch {
	case strings.Contains(msg, "global budget"):
		base += ".global"
	case providerPath(msg) != "":
		base += ".providers." + providerPath(msg)
	case typePath(msg) != "":
		base += ".types." + typePath(msg)
	case tagPath(msg) != "":
		base += ".tags[" + tagPath(msg) + "]"
	}
	if idx := alertIndex(msg); idx >= 0 {
		base += ".alerts[" + strconv.Itoa(idx) + "]"
	}
	if idx := notificationIndex(msg); idx >= 0 {
		base += ".notifications[" + strconv.Itoa(idx) + "]"
	}
	field := fieldForSentinel(err)
	if field == "" {
		return base
	}
	if base == "" {
		return field
	}
	return base + "." + field
}

func providerPath(msg string) string {
	return regexpSubmatch(msg, `provider "([^"]+)" budget`)
}

func typePath(msg string) string {
	return regexpSubmatch(msg, `type "([^"]+)" budget`)
}

func tagPath(msg string) string {
	return regexpSubmatch(msg, `tag budget\[(\d+)\]`)
}

func alertIndex(msg string) int {
	return indexFromMessage(msg, `alert\[(\d+)\]`)
}

func notificationIndex(msg string) int {
	return indexFromMessage(msg, `notifications\[(\d+)\]`)
}

func indexFromMessage(msg, pattern string) int {
	found := regexpSubmatch(msg, pattern)
	if found == "" {
		return -1
	}
	idx, err := strconv.Atoi(found)
	if err != nil {
		return -1
	}
	return idx
}

func regexpSubmatch(msg, pattern string) string {
	const matchParts = 2
	match := regexp.MustCompile(pattern).FindStringSubmatch(msg)
	if len(match) < matchParts {
		return ""
	}
	return match[1]
}

func fieldForSentinel(err error) string {
	if fieldErr, ok := errors.AsType[*NotificationFieldError](err); ok {
		return fieldErr.Field
	}
	switch {
	case errors.Is(err, ErrBudgetAmountNegative):
		return fieldAmount
	case errors.Is(err, ErrUnsupportedBudgetPeriod):
		return fieldPeriod
	case errors.Is(err, ErrAlertThresholdOutOfRange):
		return fieldThreshold
	case errors.Is(err, ErrAlertTypeInvalid):
		return fieldType
	case errors.Is(err, ErrExitCodeOutOfRange):
		return fieldExitCode
	case errors.Is(err, ErrBudgetCurrencyRequired):
		return fieldCurrency
	default:
		if strings.Contains(err.Error(), "invalid currency code") {
			return fieldCurrency
		}
		return ""
	}
}

func hintFor(err error) (string, string) {
	switch {
	case errors.Is(err, ErrBudgetAmountNegative):
		return "Use a number greater than or equal to 0. Zero disables that scope.", "amount: 100"
	case errors.Is(err, ErrUnsupportedBudgetPeriod):
		return "The only supported period is monthly (period: monthly).", "period: monthly"
	case errors.Is(err, ErrAlertThresholdOutOfRange):
		return "Threshold is a percentage of the budget amount, from 0 to 1000.", "threshold: 80"
	case errors.Is(err, ErrAlertTypeInvalid):
		return "Valid values: actual, forecasted.", "type: actual"
	case errors.Is(err, ErrExitCodeOutOfRange):
		return "Exit codes use the Unix range 0 through 255 (exit_code: 2).", "exit_code: 2"
	case errors.Is(err, ErrBudgetCurrencyRequired):
		return "Use a 3-letter uppercase ISO 4217 code such as USD.", "currency: USD"
	case strings.Contains(err.Error(), "invalid currency code"):
		return "Use a 3-letter uppercase ISO 4217 code such as USD.", "currency: USD"
	case errors.Is(err, ErrNotificationTypeInvalid):
		return "Supported destination types: slack, webhook.", "type: slack"
	case errors.Is(err, ErrNotificationURLRequired), errors.Is(err, ErrNotificationHTTPSRequired):
		return "Use an https:// URL with a host, or a ${FINFOCUS_NOTIFY_*} reference in the global config.",
			"url: ${FINFOCUS_NOTIFY_SLACK_URL}"
	case errors.Is(err, ErrNotificationMethodInvalid):
		return "Use POST (the default) or PUT.", "method: POST"
	case errors.Is(err, ErrNotificationFieldNotAllowed):
		return "channel applies to slack destinations; method and headers apply to webhook destinations.", ""
	case errors.Is(err, ErrNotificationHeaderNameEmpty):
		return "Give every header a name.", "headers:\n  Authorization: Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"
	case errors.Is(err, ErrNotificationVariableNotAllowed), errors.Is(err, ErrNotificationReferenceMalformed):
		return "Only ${FINFOCUS_NOTIFY_*} variables expand. Store the secret in a variable with that prefix.",
			"url: ${FINFOCUS_NOTIFY_SLACK_URL}"
	case errors.Is(err, ErrNotificationProjectVariable):
		return "Move this destination to the global config (~/.finfocus/config.hujson), " +
				"or use only literal values here. A project config is committed, so it must not read secrets.",
			""
	case errors.Is(err, ErrGlobalBudgetRequired):
		return "Add cost.budgets.global when provider, tag, or type budgets are set.", "global:\n  amount: 100\n  currency: USD"
	default:
		return "", ""
	}
}

func lineFromSyntaxError(err error) int {
	found := regexpSubmatch(err.Error(), `line (\d+)`)
	if found == "" {
		return 0
	}
	line, convErr := strconv.Atoi(found)
	if convErr != nil {
		return 0
	}
	return line
}

type configSource struct {
	data  []byte
	lines map[string]int
}

func (s *configSource) lineAt(offset int) int {
	if s == nil || offset < 0 {
		return 0
	}
	if offset > len(s.data) {
		offset = len(s.data)
	}
	return bytes.Count(s.data[:offset], []byte("\n")) + 1
}

func didYouMean(name string, known []string) string {
	best := ""
	bestDistance := maxTypoDistance + 1
	for _, candidate := range known {
		distance := levenshtein(name, candidate)
		if distance < bestDistance {
			bestDistance = distance
			best = candidate
		}
	}
	if best == "" || bestDistance > maxTypoDistance {
		return ""
	}
	return "Did you mean: '" + best + "'?"
}

func levenshtein(left, right string) int {
	if left == right {
		return 0
	}
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if len(leftRunes) == 0 {
		return len(rightRunes)
	}
	previous := make([]int, len(rightRunes)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, leftRune := range leftRunes {
		current := make([]int, len(rightRunes)+1)
		current[0] = i + 1
		for j, rightRune := range rightRunes {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(rightRunes)]
}
