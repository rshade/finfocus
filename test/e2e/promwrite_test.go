package e2e

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestEncodeWriteRequest_RoundTrip(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	raw := encodeWriteRequest([]rwSeries{{
		labels: map[string]string{"job": "a", "__name__": "up"},
		points: []rwPoint{{at: at, value: 4}},
	}})
	series, n := protowire.ConsumeBytes(raw[1:])
	require.Positive(t, n)
	assert.Equal(t, fieldSeries, protowire.Number(raw[0]>>3))

	labels := map[string]string{}
	var gotValue float64
	var gotTime int64
	for len(series) > 0 {
		num, typ, width := protowire.ConsumeTag(series)
		require.Positive(t, width)
		series = series[width:]
		body, consumed := protowire.ConsumeBytes(series)
		require.Positive(t, consumed)
		series = series[consumed:]
		switch int(num) {
		case int(fieldLabels):
			name, value := decodeLabel(t, body)
			labels[name] = value
		case int(fieldSamples):
			gotValue, gotTime = decodeSample(t, body)
		default:
			require.Failf(t, "unexpected wire field", "unexpected field %d type %d", num, typ)
		}
	}
	assert.Equal(t, map[string]string{"__name__": "up", "job": "a"}, labels)
	assert.InDelta(t, 4.0, gotValue, 1e-9)
	assert.Equal(t, at.UnixMilli(), gotTime)
}

func decodeLabel(t *testing.T, raw []byte) (string, string) {
	t.Helper()
	var name, value string
	for len(raw) > 0 {
		num, _, width := protowire.ConsumeTag(raw)
		require.Positive(t, width)
		raw = raw[width:]
		text, consumed := protowire.ConsumeString(raw)
		require.Positive(t, consumed)
		raw = raw[consumed:]
		switch int(num) {
		case int(fieldLabelName):
			name = text
		case int(fieldLabelValue):
			value = text
		default:
			require.Failf(t, "unexpected wire field", "unexpected label field %d", num)
		}
	}
	return name, value
}

func decodeSample(t *testing.T, raw []byte) (float64, int64) {
	t.Helper()
	var value float64
	var timestamp int64
	for len(raw) > 0 {
		num, _, width := protowire.ConsumeTag(raw)
		require.Positive(t, width)
		raw = raw[width:]
		switch int(num) {
		case int(fieldSampleValue):
			bits, consumed := protowire.ConsumeFixed64(raw)
			require.Positive(t, consumed)
			raw = raw[consumed:]
			value = math.Float64frombits(bits)
		case int(fieldSampleTime):
			ts, consumed := protowire.ConsumeVarint(raw)
			require.Positive(t, consumed)
			raw = raw[consumed:]
			timestamp = int64(ts)
		default:
			require.Failf(t, "unexpected wire field", "unexpected sample field %d", num)
		}
	}
	return value, timestamp
}
