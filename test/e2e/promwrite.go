package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/golang/snappy"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	fieldSeries      protowire.Number = 1
	fieldLabels      protowire.Number = 1
	fieldSamples     protowire.Number = 2
	fieldLabelName   protowire.Number = 1
	fieldLabelValue  protowire.Number = 2
	fieldSampleValue protowire.Number = 1
	fieldSampleTime  protowire.Number = 2

	httpStatusClass   = 100
	httpSuccessClass  = 2
	writeBodyLimit    = 4096
	queryBodyLimit    = 1 << 20
	vectorSampleWidth = 2
)

// rwSeries is one Prometheus time series for the remote-write receiver.
type rwSeries struct {
	labels map[string]string
	points []rwPoint
}

// rwPoint is one sample. at is the sample timestamp.
type rwPoint struct {
	at    time.Time
	value float64
}

// promVector is one instant-query sample.
type promVector struct {
	metric map[string]string
	value  float64
}

// remoteWrite posts series to Prometheus /api/v1/write.
// The body is a hand-encoded WriteRequest compressed with snappy.
func remoteWrite(ctx context.Context, base string, series []rwSeries) error {
	compressed := snappy.Encode(nil, encodeWriteRequest(series))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/write", bytes.NewReader(compressed))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, writeBodyLimit))
	if err != nil {
		return err
	}
	if resp.StatusCode/httpStatusClass != httpSuccessClass {
		return fmt.Errorf("remote write: %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	return nil
}

// queryInstant evaluates an instant query at at and returns the vector.
func queryInstant(ctx context.Context, base, expr string, at time.Time) ([]promVector, error) {
	endpoint, err := url.Parse(base + "/api/v1/query")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("query", expr)
	query.Set("time", strconv.FormatInt(at.Unix(), 10))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, queryBodyLimit))
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode/httpStatusClass != httpSuccessClass {
		return nil, fmt.Errorf("query: %s: %s", resp.Status, bytes.TrimSpace(body))
	}
	var parsed struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []any             `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err = json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Status != "success" || parsed.Data.ResultType != "vector" {
		return nil, fmt.Errorf("query status %q type %q: %s", parsed.Status, parsed.Data.ResultType, parsed.Error)
	}
	out := make([]promVector, 0, len(parsed.Data.Result))
	for _, sample := range parsed.Data.Result {
		if len(sample.Value) != vectorSampleWidth {
			return nil, fmt.Errorf("query sample has %d fields", len(sample.Value))
		}
		text, ok := sample.Value[1].(string)
		if !ok {
			return nil, fmt.Errorf("query sample value is %T", sample.Value[1])
		}
		value, parseErr := strconv.ParseFloat(text, 64)
		if parseErr != nil {
			return nil, parseErr
		}
		out = append(out, promVector{metric: sample.Metric, value: value})
	}
	return out, nil
}

// encodeWriteRequest builds a Prometheus WriteRequest.
// Field 1 is a repeated TimeSeries. Labels are sorted, including __name__.
func encodeWriteRequest(series []rwSeries) []byte {
	var out []byte
	for _, item := range series {
		raw := encodeSeries(item)
		out = protowire.AppendTag(out, fieldSeries, protowire.BytesType)
		out = protowire.AppendBytes(out, raw)
	}
	return out
}

func encodeSeries(series rwSeries) []byte {
	var out []byte
	names := make([]string, 0, len(series.labels))
	for name := range series.labels {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		raw := encodeLabel(name, series.labels[name])
		out = protowire.AppendTag(out, fieldLabels, protowire.BytesType)
		out = protowire.AppendBytes(out, raw)
	}
	for _, point := range series.points {
		raw := encodeSample(point.value, point.at.UnixMilli())
		out = protowire.AppendTag(out, fieldSamples, protowire.BytesType)
		out = protowire.AppendBytes(out, raw)
	}
	return out
}

func encodeLabel(name, value string) []byte {
	var out []byte
	out = protowire.AppendTag(out, fieldLabelName, protowire.BytesType)
	out = protowire.AppendString(out, name)
	out = protowire.AppendTag(out, fieldLabelValue, protowire.BytesType)
	out = protowire.AppendString(out, value)
	return out
}

func millis(timestamp int64) uint64 {
	if timestamp < 0 {
		return 0
	}
	return uint64(timestamp)
}

func encodeSample(value float64, timestamp int64) []byte {
	var out []byte
	out = protowire.AppendTag(out, fieldSampleValue, protowire.Fixed64Type)
	out = protowire.AppendFixed64(out, math.Float64bits(value))
	out = protowire.AppendTag(out, fieldSampleTime, protowire.VarintType)
	out = protowire.AppendVarint(out, millis(timestamp))
	return out
}
