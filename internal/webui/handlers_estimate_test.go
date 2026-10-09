package webui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestEstimateRoutesLoading(t *testing.T) {
	t.Parallel()
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	for _, path := range []string{"/api/estimate/resources", "/api/estimate/baseline?urn=x"} {
		assert.Equal(t, http.StatusServiceUnavailable, overviewRequest(t, s, http.MethodGet, path, "").StatusCode)
	}
	assert.Equal(
		t,
		http.StatusBadRequest,
		overviewRequest(t, s, http.MethodPost, "/api/estimate/recalculate", `{"urn":"x","overrides":{}}`).StatusCode,
	)
}

type webEstimatePlugin struct {
	proto.CostSourceClient

	rate    float64
	failure error
}

func (p *webEstimatePlugin) GetPricingSpec(
	context.Context,
	*pbc.GetPricingSpecRequest,
	...grpc.CallOption,
) (*pbc.GetPricingSpecResponse, error) {
	return &pbc.GetPricingSpecResponse{
		Spec: &pbc.PricingSpec{
			BillingMode: "hourly",
			Currency:    "USD",
			RatePerUnit: p.rate,
			Assumptions: []string{"730 hours"},
		},
	}, nil
}

func (p *webEstimatePlugin) EstimateCost(
	_ context.Context,
	request *pbc.EstimateCostRequest,
	_ ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	if p.failure != nil {
		return nil, p.failure
	}
	return &pbc.EstimateCostResponse{
		Currency:    "USD",
		CostMonthly: p.rate * request.GetAttributes().GetFields()["size"].GetNumberValue(),
	}, nil
}

func estimateHTTPFixture(t *testing.T) (*Server, *Session, *engine.Engine, *engine.ResourceDescriptor) {
	t.Helper()
	resource := &engine.ResourceDescriptor{
		ID:       "urn:real",
		Type:     "aws:ec2:Instance",
		Provider: "aws",
		Properties: map[string]any{
			"size":     float64(1),
			"password": "credential-value",
			"private":  pulumiSecretFixture("secret-value"),
			"verbatim": "old",
		},
	}
	eng := engine.New(
		[]*pluginhost.Client{
			{Name: "first", API: &webEstimatePlugin{rate: 10}},
			{Name: "second", API: &webEstimatePlugin{rate: 30}},
		},
		nil,
	)
	session := NewSession(
		context.Background(),
		SessionOptions{EstimateResources: func(context.Context) ([]engine.ResourceDescriptor, error) {
			return []engine.ResourceDescriptor{*resource}, nil
		}},
	)
	session.SetData(
		eng,
		[]engine.OverviewRow{
			{
				URN:  "urn:real",
				Type: resource.Type,
				ProjectedProperties: map[string]any{
					"size":     float64(2),
					"password": "projected-credential",
					"private":  pulumiSecretFixture("projected-secret"),
					"verbatim": "old",
				},
			},
			{URN: "synthetic", Type: "kubernetes:Pod"},
		},
		engine.DateRange{},
		"",
	)
	return newRunningServer(t, Options{Session: session}), session, eng, resource
}

func TestEstimateHTTPBaselineModesAndSourceResources(t *testing.T) {
	t.Parallel()
	s, _, _, _ := estimateHTTPFixture(t)
	resp := overviewRequest(t, s, "GET", "/api/estimate/resources", "")
	require.Equal(t, 200, resp.StatusCode)
	var resources struct {
		Resources []engine.ResourceDescriptor `json:"resources"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&resources))
	require.Len(t, resources.Resources, 1)
	assert.Equal(t, "urn:real", resources.Resources[0].ID)
	assert.InDelta(t, 2.0, resources.Resources[0].Properties["size"], 1e-9)
	assert.NotContains(t, resources.Resources[0].Properties, "password")
	assert.NotContains(t, resources.Resources[0].Properties, "private")
	for _, tc := range []struct {
		mode string
		cost float64
	}{{"", 20}, {`["first","hourly"]`, 20}, {`["second","hourly"]`, 60}} {
		resp = overviewRequest(
			t,
			s,
			"GET",
			"/api/estimate/baseline?urn=urn:real&pricingMode="+url.QueryEscape(tc.mode),
			"",
		)
		require.Equal(t, 200, resp.StatusCode)
		var got struct {
			Result  engine.EstimateResult     `json:"result"`
			Modes   []estimateMode            `json:"pricingModes"`
			Display viewmodel.EstimateDisplay `json:"display"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
		require.NotNil(t, got.Result.Baseline)
		require.NotNil(t, got.Result.Modified)
		assert.InDelta(t, tc.cost, got.Result.Baseline.Monthly, 1e-9)
		assert.InDelta(t, tc.cost, got.Result.Modified.Monthly, 1e-9)
		assert.Zero(t, got.Result.TotalChange)
		require.Len(t, got.Modes, 2)
		assert.Equal(t, `["second","hourly"]`, got.Modes[1].ID)
		assert.Equal(t, "USD 30/hourly", got.Modes[1].Rate)
		require.Len(t, got.Display.Properties, 2)
	}
	assert.Equal(t, 404, overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=synthetic", "").StatusCode)
}

func TestEstimateHTTPRecalculateParityAndRedaction(t *testing.T) {
	t.Parallel()
	s, _, eng, resource := estimateHTTPFixture(t)
	overrides := map[string]string{
		"size":     "3",
		"verbatim": "  exact\tvalue  ",
		"password": "new-credential",
		"private":  "new-secret",
	}
	body, err := json.Marshal(
		map[string]any{"urn": "urn:real", "overrides": overrides, "pricingMode": `["second","hourly"]`},
	)
	require.NoError(t, err)
	resp := overviewRequest(t, s, "POST", "/api/estimate/recalculate", string(body))
	require.Equal(t, 200, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	for _, secret := range []string{"credential-value", "projected-credential", "new-credential", "secret-value", "projected-secret", "new-secret"} {
		assert.NotContains(t, string(raw), secret)
	}
	var got engine.EstimateResult
	require.NoError(t, json.Unmarshal(raw, &got))
	require.NotNil(t, got.Baseline)
	require.NotNil(t, got.Modified)
	// The CLI calls this same request. Only the source snapshot uses preview properties.
	projected := *resource
	projected.Properties = map[string]any{"size": float64(2), "verbatim": "old"}
	want, err := eng.EstimateCost(
		context.Background(),
		&engine.EstimateRequest{Resource: &projected, PropertyOverrides: overrides, PricingMode: `["second","hourly"]`},
	)
	require.NoError(t, err)
	assert.Equal(t, want.Baseline, got.Baseline)
	assert.Equal(t, want.Modified, got.Modified)
	assert.InDelta(t, want.TotalChange, got.TotalChange, 1e-9)
	assert.Equal(t, want.Deltas, got.Deltas)
	var display struct {
		Display viewmodel.EstimateDisplay `json:"display"`
	}
	require.NoError(t, json.Unmarshal(raw, &display))
	require.Len(t, display.Display.Properties, 2)
	assert.Equal(t, "  exact\tvalue  ", display.Display.Properties[1].CurrentValue)
	assert.InDelta(t, 30.0, got.TotalChange, 1e-9)
	assert.InDelta(t, 1.0, resource.Properties["size"], 1e-9, "source maps remain immutable")
}

func TestEstimateHTTPValidationAndFailures(t *testing.T) {
	t.Parallel()
	s, session, _, _ := estimateHTTPFixture(t)
	for _, body := range []string{`{}`, `{"urn":"urn:real","overrides":{}}`, `{"urn":"urn:real","overrides":{"size":"3"},"pricingMode":"invalid"}`, `{"urn":"urn:real","overrides":{"size":3}}`} {
		assert.Equal(t, 400, overviewRequest(t, s, "POST", "/api/estimate/recalculate", body).StatusCode)
	}
	assert.Equal(t, 400, overviewRequest(t, s, "GET", "/api/estimate/baseline", "").StatusCode)
	assert.Equal(
		t,
		400,
		overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=urn:real&pricingMode=invalid", "").StatusCode,
	)
	session.SetData(
		engine.New(
			[]*pluginhost.Client{
				{Name: "bad", API: &webEstimatePlugin{failure: status.Error(codes.Unavailable, "credential-value")}},
			},
			nil,
		),
		nil,
		engine.DateRange{},
		"",
	)
	resp := overviewRequest(
		t,
		s,
		"GET",
		"/api/estimate/baseline?urn=urn:real&pricingMode="+url.QueryEscape(`["bad","hourly"]`),
		"",
	)
	require.Equal(t, 502, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "credential-value")
}

func TestEstimateQueriesJoinSessionCancellation(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	session := NewSession(
		context.Background(),
		SessionOptions{EstimateResources: func(ctx context.Context) ([]engine.ResourceDescriptor, error) {
			close(entered)
			<-ctx.Done()
			return nil, ctx.Err()
		}},
	)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	s := newRunningServer(t, Options{Session: session})
	request, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		s.BaseURL()+"/api/estimate/resources",
		nil,
	)
	require.NoError(t, err)
	request.AddCookie(&http.Cookie{Name: s.cookieName(), Value: s.token})
	finished := make(chan error, 1)
	go func() {
		response, requestErr := http.DefaultClient.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
		finished <- requestErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("estimate did not start")
	}
	session.Close()
	select {
	case err = <-finished:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("estimate did not join cancellation")
	}
}

func TestEstimateHTTPUnchangedOverridesAndSecretDeltas(t *testing.T) {
	t.Parallel()
	s, session, _, _ := estimateHTTPFixture(t)
	for _, tc := range []struct{ key, value string }{{"verbatim", "  exact\tvalue  "}, {"private", "edited-private"}, {"password", "edited-password"}} {
		body, err := json.Marshal(
			map[string]any{
				"urn":         "urn:real",
				"overrides":   map[string]string{tc.key: tc.value},
				"pricingMode": `["first","hourly"]`,
			},
		)
		require.NoError(t, err)
		resp := overviewRequest(t, s, "POST", "/api/estimate/recalculate", string(body))
		require.Equal(t, 200, resp.StatusCode)
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		var got engine.EstimateResult
		require.NoError(t, json.Unmarshal(raw, &got))
		if tc.key == "verbatim" {
			require.Len(t, got.Deltas, 1)
			assert.Equal(t, tc.value, got.Deltas[0].NewValue)
		} else {
			assert.Empty(t, got.Deltas)
			assert.NotContains(t, string(raw), tc.value)
		}
	}
	session.Ready(
		[]engine.OverviewRow{
			{
				URN:                 "urn:real",
				Type:                "aws:ec2:Instance",
				Properties:          map[string]any{"size": float64(1)},
				ProjectedProperties: map[string]any{"size": float64(4)},
			},
		},
	)
	resp := overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=urn:real", "")
	require.Equal(t, 200, resp.StatusCode)
	var got struct {
		Result engine.EstimateResult `json:"result"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.NotNil(t, got.Result.Baseline)
	assert.InDelta(
		t,
		40.0,
		got.Result.Baseline.Monthly,
		1e-9,
		"latest projected properties take precedence over original source",
	)
}

func TestEstimateHTTPSourceReadinessAndFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ready   bool
		closed  bool
		failure error
		want    int
	}{
		{name: "loading", want: 503},
		{name: "closed", ready: true, closed: true, want: 503},
		{name: "source failed", ready: true, failure: errors.New("private source failure"), want: 502},
		{name: "empty", ready: true, want: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			session := NewSession(
				context.Background(),
				SessionOptions{
					EstimateResources: func(context.Context) ([]engine.ResourceDescriptor, error) { return nil, tc.failure },
				},
			)
			if tc.ready {
				session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
			}
			if tc.closed {
				session.Close()
			}
			s := newRunningServer(t, Options{Session: session})
			resp := overviewRequest(t, s, "GET", "/api/estimate/resources", "")
			assert.Equal(t, tc.want, resp.StatusCode)
			raw, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.NotContains(t, string(raw), "private source failure")
			if tc.failure != nil {
				assert.Equal(t, 502, overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=urn:real", "").StatusCode)
			}
		})
	}
}

func TestEstimateHTTPUnavailablePriceIsNotSuccessfulZero(t *testing.T) {
	t.Parallel()
	s, session, _, _ := estimateHTTPFixture(t)
	session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
	resp := overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=urn:real", "")
	assert.Equal(t, 502, resp.StatusCode)
}

func TestEstimateShutdownDoesNotWaitForResponseWriter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, body string
		sourceErr        bool
		unpriced         bool
		want             int
	}{
		{name: "resources", path: "/api/estimate/resources", want: 200},
		{name: "baseline", path: "/api/estimate/baseline?urn=urn:real", want: 200},
		{name: "recalculate", path: "/api/estimate/recalculate", body: `{"urn":"urn:real","overrides":{"size":"3"}}`, want: 200},
		{name: "resources source failure", path: "/api/estimate/resources", sourceErr: true, want: 502},
		{name: "baseline source failure", path: "/api/estimate/baseline?urn=urn:real", sourceErr: true, want: 502},
		{name: "unknown resource", path: "/api/estimate/baseline?urn=missing", want: 404},
		{name: "invalid mode", path: "/api/estimate/recalculate", body: `{"urn":"urn:real","overrides":{"size":"3"},"pricingMode":"invalid"}`, want: 400},
		{name: "unavailable price", path: "/api/estimate/baseline?urn=urn:real", unpriced: true, want: 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, session, _, _ := estimateHTTPFixture(t)
			if tc.sourceErr {
				session.opts.EstimateResources = func(context.Context) ([]engine.ResourceDescriptor, error) { return nil, errors.New("source failed") }
			}
			if tc.unpriced {
				session.SetData(engine.New(nil, nil), nil, engine.DateRange{}, "")
			}
			writer := &blockedJSONWriter{
				ResponseRecorder: httptest.NewRecorder(),
				entered:          make(chan struct{}),
				release:          make(chan struct{}),
			}
			returned := make(chan struct{})
			go func() {
				defer close(returned)
				method := http.MethodGet
				if tc.body != "" {
					method = http.MethodPost
				}
				request := httptest.NewRequest(method, s.BaseURL()+tc.path, strings.NewReader(tc.body))
				switch {
				case strings.Contains(tc.path, "/resources"):
					s.handleEstimateResources(writer, request)
				case strings.Contains(tc.path, "/baseline"):
					s.handleEstimateBaseline(writer, request)
				default:
					s.handleEstimateRecalculate(writer, request)
				}
			}()
			<-writer.entered
			closed := make(chan struct{})
			go func() { session.Close(); close(closed) }()
			select {
			case <-closed:
			case <-time.After(time.Second):
				assert.Fail(t, "estimate session shutdown waited for socket write")
			}
			close(writer.release)
			<-returned
			<-closed
			assert.Equal(t, tc.want, writer.Code)
		})
	}
}
