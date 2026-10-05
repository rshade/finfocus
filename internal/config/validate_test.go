package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfigSource_BudgetRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		src        string
		valid      bool
		warning    bool
		path       string
		text       string
		hint       string
		suggestion string
		line       int
	}{
		{
			name: "typo suggests amount",
			src: `{
  "cost": {
    "budgets": {
      "global": {
        "ammount": 100
      }
    }
  }
}`,
			valid:      true,
			warning:    true,
			path:       "cost.budgets.global.ammount",
			text:       "Unknown field 'ammount'",
			suggestion: "Did you mean: 'amount'?",
			line:       5,
		},
		{
			name: "flat amount is not applied",
			src: `{
  "cost": {
    "budgets": {
      "ammount": 100
    }
  }
}`,
			valid:      true,
			warning:    true,
			path:       "cost.budgets.ammount",
			text:       "Unknown field 'ammount'",
			suggestion: "Did you mean: 'amount'?",
			line:       4,
		},
		{
			name: "string amount",
			src: `{
  "cost": {
    "budgets": {
      "global": {
        "amount": "one hundred"
      }
    }
  }
}`,
			path: "cost.budgets.global.amount",
			text: "must be a number, got string",
			hint: "amount: 100",
			line: 5,
		},
		{
			name: "negative amount",
			src:  `{"cost":{"budgets":{"global":{"amount":-5}}}}`,
			path: "cost.budgets.global.amount",
			text: "cannot be negative",
			hint: "greater than or equal to 0",
			line: 1,
		},
		{
			name: "weekly period",
			src: `{
  "cost": {"budgets": {"global": {"amount": 100, "currency": "USD", "period": "weekly"}}}
}`,
			path: "cost.budgets.global.period",
			text: "must be 'monthly'",
			hint: "period: monthly",
			line: 2,
		},
		{
			name: "threshold 150 stays valid",
			src: `{
  "cost": {"budgets": {"global": {
    "amount": 100,
    "currency": "USD",
    "alerts": [{"threshold": 150, "type": "actual"}]
  }}}
}`,
			valid: true,
		},
		{
			name: "threshold above 1000",
			src: `{
  "cost": {"budgets": {"global": {
    "amount": 100,
    "currency": "USD",
    "alerts": [{"threshold": 1500, "type": "actual"}]
  }}}
}`,
			path: "cost.budgets.global.alerts[0].threshold",
			text: "between 0 and 1000",
			hint: "percentage",
			line: 5,
		},
		{
			name: "predicted alert type",
			src: `{
  "cost": {"budgets": {"global": {
    "amount": 100,
    "currency": "USD",
    "alerts": [{"threshold": 80, "type": "predicted"}]
  }}}
}`,
			path: "cost.budgets.global.alerts[0].type",
			text: "actual",
			hint: "forecasted",
			line: 5,
		},
		{
			name: "short currency",
			src:  `{"cost":{"budgets":{"global":{"amount":10,"currency":"us"}}}}`,
			path: "cost.budgets.global.currency",
			text: "3 letters",
			hint: "USD",
			line: 1,
		},
		{
			name: "exit code out of range",
			src:  `{"cost":{"budgets":{"exit_code":300}}}`,
			path: "cost.budgets.exit_code",
			text: "0 and 255",
			hint: "exit_code: 2",
			line: 1,
		},
		{
			name: "empty alerts",
			src: `{
  "cost": {"budgets": {"global": {"amount": 10, "currency": "USD", "alerts": []}}}
}`,
			path: "cost.budgets.global.alerts",
			text: "non-empty",
			hint: "Omit alerts",
			line: 2,
		},
		{
			name: "exit_on_threshold must be boolean",
			src:  `{"cost":{"budgets":{"global":{"exit_on_threshold":"yes"}}}}`,
			path: "cost.budgets.global.exit_on_threshold",
			text: "must be a boolean, got string",
			hint: "true or false",
			line: 1,
		},
		{
			name: "valid global budget",
			src: `{
  // comment kept by hujson
  "cost": {"budgets": {"global": {
    "amount": 100,
    "currency": "USD",
    "period": "monthly",
    "exit_on_threshold": true,
    "exit_code": 2,
    "alerts": [{"threshold": 80, "type": "forecasted"}]
  }}}
}`,
			valid: true,
		},
		{
			name: "syntax error keeps the line",
			src:  "{\n  ,\n}",
			path: "",
			text: "syntax is invalid",
			hint: "trailing commas",
			line: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, _ := ValidateConfigSource("config.hujson", []byte(tt.src))
			assert.Equal(t, tt.valid, result.Valid)
			assert.Equal(t, "config.hujson", result.File)
			if tt.valid && !tt.warning {
				assert.Empty(t, result.Errors)
				return
			}
			if tt.warning {
				require.NotEmpty(t, result.Warnings)
				got := result.Warnings[0]
				assert.Equal(t, tt.path, got.Path)
				assert.Contains(t, got.Message, tt.text)
				assert.Equal(t, tt.suggestion, got.Suggestion)
				assert.Equal(t, tt.line, got.Line)
				return
			}
			require.NotEmpty(t, result.Errors)
			got := findError(result.Errors, tt.path)
			require.NotNil(t, got)
			assert.Contains(t, got.Message, tt.text)
			assert.Contains(t, got.Hint, tt.hint)
			assert.Equal(t, tt.line, got.Line)
		})
	}
}

func TestValidateConfig_InMemoryPeriod(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Output: OutputConfig{DefaultFormat: formatTable, Precision: defaultPrecision},
		Cost: CostConfig{
			Budgets: &BudgetsConfig{
				Global: &ScopedBudget{Amount: 25, Currency: "USD", Period: "daily"},
			},
		},
	}
	result := ValidateConfig(cfg)
	require.False(t, result.Valid)
	require.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, "monthly")
	assert.Contains(t, result.Errors[0].Hint, "period: monthly")
	assert.Equal(t, "cost.budgets.global.period", result.Errors[0].Path)
	assert.Zero(t, result.Errors[0].Line)
}

func findError(errs []ValidationError, path string) *ValidationError {
	for i := range errs {
		if errs[i].Path == path {
			return &errs[i]
		}
	}
	if path == "" && len(errs) > 0 {
		return &errs[0]
	}
	return nil
}

func TestValidateConfigSource_Notifications(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		src  string
		path string
		text string
		hint string
	}{
		{
			name: "global http url",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":80,"type":"actual","notifications":[
    {"type":"slack","url":"https://hooks.slack.com/x"},
    {"type":"webhook","url":"http://api.example.com"}
  ]}
]}}}}`,
			path: "cost.budgets.global.alerts[0].notifications[1].url",
			text: "HTTPS is required",
			hint: "https://",
		},
		{
			name: "provider unknown type",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD"},
  "providers":{"aws":{"amount":50,"alerts":[{"threshold":100,"type":"forecasted",
    "notifications":[{"type":"email","url":"https://example.com"}]}]}}}}}`,
			path: "cost.budgets.providers.aws.alerts[0].notifications[0].type",
			text: "slack, webhook",
			hint: "slack, webhook",
		},
		{
			name: "tag webhook channel",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD"},
  "tags":[{"selector":"team:platform","amount":20,"alerts":[{"threshold":80,"type":"actual",
    "notifications":[{"type":"webhook","url":"https://example.com","channel":"#x"}]}]}]}}}`,
			path: "cost.budgets.tags[0].alerts[0].notifications[0].channel",
			text: "slack",
			hint: "slack",
		},
		{
			name: "type budget references a CI variable",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD"},
  "types":{"aws:ec2/instance":{"amount":20,"alerts":[{"threshold":80,"type":"actual",
    "notifications":[{"type":"webhook","url":"https://example.com","headers":{"Authorization":"${GITHUB_TOKEN}"}}]}]}}}}}`,
			path: "cost.budgets.types.aws:ec2/instance.alerts[0].notifications[0].headers.Authorization",
			text: "GITHUB_TOKEN",
			hint: "FINFOCUS_NOTIFY_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, _ := ValidateConfigSource("config.hujson", []byte(tt.src))
			require.False(t, result.Valid)
			got := findError(result.Errors, tt.path)
			require.NotNil(t, got, "errors: %+v", result.Errors)
			assert.Contains(t, got.Message, tt.text)
			assert.Contains(t, got.Hint, tt.hint)
			assert.Positive(t, got.Line)
		})
	}
}

func TestValidateConfigSource_NotificationKeysAreKnown(t *testing.T) {
	t.Parallel()

	src := `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":80,"type":"actual","notifications":[
    {"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}","channel":"#finops"},
    {"type":"webhook","url":"https://api.example.com","method":"PUT",
     "headers":{"Authorization":"Bearer ${FINFOCUS_NOTIFY_API_TOKEN}"}}
  ]}
]}}}}`
	result, cfg := ValidateConfigSource("config.hujson", []byte(src))
	require.True(t, result.Valid, "errors: %+v", result.Errors)
	assert.Empty(t, result.Warnings)
	require.NotNil(t, cfg)
	assert.Len(t, cfg.Cost.Budgets.Global.Alerts[0].Notifications, 2)
}

func TestValidateProjectConfigSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		src   string
		valid bool
		path  string
	}{
		{
			name: "literal https destination passes",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":80,"type":"actual","notifications":[{"type":"webhook","url":"https://api.example.com/hook"}]}
]}}}}`,
			valid: true,
		},
		{
			name: "allowed variable in url is rejected",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD","alerts":[
  {"threshold":80,"type":"actual","notifications":[{"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}"}]}
]}}}}`,
			path: "cost.budgets.global.alerts[0].notifications[0].url",
		},
		{
			name: "variable in a provider header is rejected",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD"},
  "providers":{"aws":{"amount":50,"alerts":[{"threshold":80,"type":"actual","notifications":[
    {"type":"webhook","url":"https://api.example.com","headers":{"Authorization":"Bearer ${FINFOCUS_NOTIFY_TOKEN}"}}]}]}}}}}`,
			path: "cost.budgets.providers.aws.alerts[0].notifications[0].headers.Authorization",
		},
		{
			name: "variable in a tag destination is rejected",
			src: `{"cost":{"budgets":{"global":{"amount":100,"currency":"USD"},
  "tags":[{"selector":"team:x","amount":5,"alerts":[{"threshold":80,"type":"actual","notifications":[
    {"type":"slack","url":"${FINFOCUS_NOTIFY_SLACK_URL}"}]}]}],
  "types":{"aws:s3/bucket":{"amount":5,"alerts":[{"threshold":80,"type":"actual","notifications":[
    {"type":"slack","url":"https://hooks.slack.com/x"}]}]}}}}}`,
			path: "cost.budgets.tags[0].alerts[0].notifications[0].url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, _ := ValidateProjectConfigSource(".finfocus/config.hujson", []byte(tt.src))
			assert.Equal(t, tt.valid, result.Valid, "errors: %+v", result.Errors)
			if tt.valid {
				return
			}
			got := findError(result.Errors, tt.path)
			require.NotNil(t, got, "errors: %+v", result.Errors)
			assert.Contains(t, got.Message, "project config")
			assert.Contains(t, got.Hint, "global config")
			assert.Positive(t, got.Line)
		})
	}
}

func TestValidateProjectConfigSource_LegacyYAML(t *testing.T) {
	t.Parallel()

	src := `cost:
  budgets:
    global:
      amount: 100
      currency: USD
      alerts:
        - threshold: 80
          type: actual
          notifications:
            - type: slack
              url: ${FINFOCUS_NOTIFY_SLACK_URL}
`
	result, _ := ValidateProjectConfigSource(".finfocus/config.yaml", []byte(src))
	require.False(t, result.Valid)
	got := findError(result.Errors, "cost.budgets.global.alerts[0].notifications[0].url")
	require.NotNil(t, got, "errors: %+v", result.Errors)
	assert.Contains(t, got.Message, "project config")
	assert.Zero(t, got.Line, "converted YAML has no meaningful line numbers")

	valid, cfg := ValidateProjectConfigSource(".finfocus/config.yml", []byte("output:\n  default_format: json\n"))
	assert.True(t, valid.Valid, "errors: %+v", valid.Errors)
	require.NotNil(t, cfg)

	broken, cfg := ValidateProjectConfigSource(".finfocus/config.yaml", []byte("cost: [unclosed"))
	assert.False(t, broken.Valid)
	assert.Nil(t, cfg)
	assert.Contains(t, broken.Errors[0].Message, "syntax is invalid")
}
