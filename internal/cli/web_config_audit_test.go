package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/logging"
)

type captureAudit struct {
	entries []logging.AuditEntry
}

func (c *captureAudit) Log(_ context.Context, entry logging.AuditEntry) {
	c.entries = append(c.entries, entry)
}

func (c *captureAudit) Enabled() bool { return true }

func (c *captureAudit) Close() error { return nil }

func TestLogConfigSaveRecordsKeysOnly(t *testing.T) {
	t.Parallel()
	captured := &captureAudit{}
	audit := &auditContext{logger: captured, traceID: "trace", start: time.Now()}
	audit.logConfigSave(context.Background(), "project", []string{"output.precision", "plugins.aws.region"})
	require.Len(t, captured.entries, 1)
	assert.Equal(t, "config save", captured.entries[0].Command)
	assert.Equal(t, "project", captured.entries[0].Parameters["tier"])
	assert.Equal(t, "output.precision,plugins.aws.region", captured.entries[0].Parameters["keys"])
	assert.NotContains(t, captured.entries[0].Parameters, "value")
	assert.Empty(t, captured.entries[0].Error)
}

func TestLogConfigSaveIgnoresNilLogger(t *testing.T) {
	t.Parallel()
	var audit *auditContext
	audit.logConfigSave(context.Background(), "global", []string{"output.precision"})
}
