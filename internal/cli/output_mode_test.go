package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectOutputFormatFromArgs(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "long flag with equals",
			argv: []string{"finfocus", "overview", "--output=json"},
			want: "json",
		},
		{
			name: "long flag with separate value",
			argv: []string{"finfocus", "cost", "projected", "--output", "ndjson"},
			want: "ndjson",
		},
		{
			name: "short flag with separate value",
			argv: []string{"finfocus", "plugin", "certify", "-o", "json"},
			want: "json",
		},
		{
			name: "short flag with equals",
			argv: []string{"finfocus", "plugin", "certify", "-o=json"},
			want: "json",
		},
		{
			name: "no output flag",
			argv: []string{"finfocus", "overview"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectOutputFormatFromArgs(tt.argv)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldSuppressAuxiliaryOutput(t *testing.T) {
	assert.True(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output", "json"}))
	assert.True(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output=ndjson"}))
	assert.False(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview", "--output", "table"}))
	assert.False(t, shouldSuppressAuxiliaryOutput([]string{"finfocus", "overview"}))
}

func TestSuppressAuxOutputContextHelpers(t *testing.T) {
	base := context.Background()
	assert.False(t, suppressAuxOutputFromContext(base))

	withSuppress := contextWithSuppressAuxOutput(base, true)
	assert.True(t, suppressAuxOutputFromContext(withSuppress))

	withNoSuppress := contextWithSuppressAuxOutput(base, false)
	assert.False(t, suppressAuxOutputFromContext(withNoSuppress))
}
