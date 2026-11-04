package weblog_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/weblog"
)

func TestDisplayLevelLabels(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"trace": "TRACE", "debug": "DEBUG", "info": "INFO ", "warn": "WARN ",
		"error": "ERROR", "fatal": "FATAL", "panic": "PANIC",
	}
	for level, want := range tests {
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			d := weblog.Display(weblog.Entry{Level: level, Source: "core", Time: "2026-01-02T03:04:05Z"})
			assert.Equal(t, want, d.Level)
			assert.Len(t, d.Level, 5)
		})
	}
}

func TestDisplayTimeIsLocalWithMillis(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 250_000_000, time.UTC)
	d := weblog.Display(weblog.Entry{Level: "info", Time: ts.Format(time.RFC3339Nano)})
	assert.Equal(t, ts.Local().Format("15:04:05.000"), d.Time)
	assert.Regexp(t, `^\d{2}:\d{2}:\d{2}\.250$`, d.Time)
}

func TestDisplayUnparseableTimeFallsBack(t *testing.T) {
	t.Parallel()
	d := weblog.Display(weblog.Entry{Level: "info", Time: "not a time", Source: "aws"})
	assert.Equal(t, "not a time", d.Time)
	assert.Equal(t, "aws", d.Source)
}
