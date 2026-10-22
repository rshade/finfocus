package ingest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

func TestParseTerraformPlanFixture(t *testing.T) {
	t.Parallel()

	content := readPlanFixture(t, "show.json")
	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(content)
	resp, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, resp))

	byID := resourcesByID(t, resp)
	aws := byID["aws_s3_bucket.site"]
	require.Equal(t, "aws_s3_bucket", aws.GetResource().GetResourceType())
	require.Equal(t, "aws", aws.GetResource().GetProvider())
	assert.Equal(t, "create", parser.Operations()["aws_s3_bucket.site"])
	assert.Contains(t, aws.GetUnknownProperties(), "region")
	assert.NotContains(t, attributeKeys(aws), "region")
	timeouts, ok := aws.GetResource().GetAttributes().GetFields()["timeouts"]
	require.True(t, ok, "null attribute stays present")
	assert.Equal(t, structpb.NullValue_NULL_VALUE, timeouts.GetNullValue())

	fresh := byID["terraform_data.fresh_list"]
	assert.Equal(t, "custom", fresh.GetResource().GetProvider())
	assert.Equal(t, "terraform_data", fresh.GetResource().GetResourceType())
	assert.Contains(t, fresh.GetUnknownProperties(), "input")
	assert.NotContains(t, fresh.GetUnknownProperties(), "input.1")
	for _, path := range fresh.GetUnknownProperties() {
		assert.NotContains(t, path, "[")
		assert.NotContains(t, attributeKeys(fresh), path)
	}

	secret := byID["terraform_data.item"]
	assert.NotContains(t, attributeKeys(secret), "input")
	assert.NotContains(t, secret.GetUnknownProperties(), "input")
	assert.NotContains(t, parser.Before()["terraform_data.item"], "input")
	assert.Equal(t, "update", parser.Operations()["terraform_data.item"])

	cloudflare := byID["cloudflare_zone.site"]
	assert.Equal(t, "custom", cloudflare.GetResource().GetProvider())
	assert.Equal(t, "cloudflare_zone", cloudflare.GetResource().GetResourceType())
	assert.Equal(t, "create", parser.Operations()["cloudflare_zone.site"])

	assert.Equal(t, "update", parser.Operations()["terraform_data.holder"])
	assert.Equal(t, "delete", parser.Operations()["terraform_data.gone"])
	assert.Equal(t, "replace", parser.Operations()["terraform_data.swap"])
	assert.Equal(t, "same", parser.Operations()["terraform_data.stable"])

	_, keptNull := byID["null_resource.marker"]
	assert.False(t, keptNull)
	_, keptData := byID["data.http.local"]
	assert.False(t, keptData)

	assertSkipped(t, resp, "data.http.local", pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_INFO)
	assertSkipped(
		t,
		resp,
		"data.external.echo",
		pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_INFO,
	)
	for _, address := range []string{
		"random_pet.name",
		"random_id.fresh",
		"null_resource.marker",
		"helm_release.app",
		"time_static.stamp",
		"tls_private_key.key",
		"local_file.plain",
		"archive_file.zip",
		"module.net.local_file.mod",
	} {
		assertSkipped(t, resp, address, pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_INFO)
		_, kept := byID[address]
		assert.False(t, kept, address)
	}
	assert.Equal(t, 1, parser.SkippedProviders()["helm"])
	assert.GreaterOrEqual(t, parser.SkippedProviders()["local"], 1)

	for _, resource := range resp.GetResources() {
		assert.NotContains(t, attributeKeys(resource), ingest.PropertyTerraformUnknown)
	}
}

func TestParseTerraformPlanDriftOnly(t *testing.T) {
	t.Parallel()

	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(readPlanFixture(t, "drift.json"))
	resp, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, resp))
	assert.Empty(t, resp.GetResources())
	assert.Empty(t, parser.Operations())
}

func TestParseTerraformPlanRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	content := make([]byte, ingest.TerraformPlanMaxBytes+1)
	parser := &ingest.TerraformPlanParser{}
	_, err := parser.Parse(context.Background(), ingest.TerraformPlanRequest(content))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32")
}

func TestAdaptTerraformPlanCopiesUnknownsOutsideTheResponse(t *testing.T) {
	t.Parallel()

	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(readPlanFixture(t, "show.json"))
	resp, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)

	descriptors := ingest.AdaptTerraformPlan(resp, parser.Operations(), parser.Before())
	var fresh engine.ResourceDescriptor
	for _, descriptor := range descriptors {
		if descriptor.ID == "terraform_data.fresh_list" {
			fresh = descriptor
		}
	}
	require.Equal(t, "terraform_data.fresh_list", fresh.ID)
	assert.Equal(t, "create", fresh.Operation)
	assert.Equal(t, "custom", fresh.Provider)
	assert.Contains(t, fresh.Properties[ingest.PropertyTerraformUnknown], "input")
}

func readPlanFixture(t *testing.T, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "terraform", "plan", name))
	require.NoError(t, err)
	return content
}

func resourcesByID(t *testing.T, resp *pbc.ParseResponse) map[string]*pbc.ParsedResource {
	t.Helper()
	out := make(map[string]*pbc.ParsedResource, len(resp.GetResources()))
	for _, resource := range resp.GetResources() {
		out[resource.GetSourceId()] = resource
	}
	return out
}

func attributeKeys(resource *pbc.ParsedResource) []string {
	fields := resource.GetResource().GetAttributes().GetFields()
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	return keys
}

func assertSkipped(
	t *testing.T,
	resp *pbc.ParseResponse,
	sourceID string,
	severity pbc.ParseWarningSeverity,
) {
	t.Helper()
	for _, warning := range resp.GetWarnings() {
		if warning.GetSourceId() == sourceID {
			assert.Equal(t, "resource_skipped", warning.GetCode())
			assert.Equal(t, severity, warning.GetSeverity())
			return
		}
	}
	t.Fatalf("missing resource_skipped warning for %s", sourceID)
}

func TestTerraformPlanInvalidDocuments(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, content, message string }{
		{"invalid JSON", `{`, "decoding"},
		{"missing version", `{}`, "format_version"},
		{"unsupported version", `{"format_version":"2.0"}`, "major version 1"},
		{"duplicate address", `{"format_version":"1","resource_changes":[{"address":"aws_instance.a","mode":"managed","type":"aws_instance","change":{"actions":["create"],"after":{}}},{"address":"aws_instance.a","mode":"managed","type":"aws_instance","change":{"actions":["create"],"after":{}}}]}`, "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parser := &ingest.TerraformPlanParser{}
			_, err := parser.Parse(
				context.Background(),
				ingest.TerraformPlanRequest([]byte(tc.content)),
			)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestTerraformPlanCancelledAndWrongFile(t *testing.T) {
	t.Parallel()
	parser := &ingest.TerraformPlanParser{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := parser.Parse(ctx, ingest.TerraformPlanRequest([]byte(`{"format_version":"1"}`)))
	require.ErrorIs(t, err, context.Canceled)
	req := ingest.TerraformPlanRequest([]byte(`{"format_version":"1"}`))
	req.GetFiles().Files[0].Path = "wrong.json"
	_, err = parser.Parse(context.Background(), req)
	require.ErrorContains(t, err, "plan.json")
}

func TestTerraformPlanNestedValueMarkers(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`{"format_version":"1.2","resource_changes":[{"address":"aws_instance.nested","mode":"managed","type":"aws_instance","module_address":"module.compute","change":{"actions":["update"],"before":{"id":"old","arn":"old-arn","secret":"before"},"before_sensitive":{"secret":true},"after":{"nullable":null,"settings":[{"secret":"hidden","known":"yes","pending":null},{"known":"also yes"}],"id":"new","arn":"new-arn"},"after_sensitive":{"settings":[{"secret":true},false]},"after_unknown":{"settings":[{"pending":true},false]}}}]}`,
	)
	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(raw)
	response, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, response))
	resources := ingest.AdaptTerraformPlan(response, parser.Operations(), parser.Before())
	require.Len(t, resources, 1)
	resource := resources[0]
	assert.Equal(t, "update", resource.Operation)
	assert.Equal(t, "old", resource.OldProperties["id"])
	assert.NotContains(t, resource.OldProperties, "secret")
	assert.Equal(t, "module.compute", resource.Properties[ingest.PropertyTerraformModule])
	assert.Contains(t, resource.Properties, "nullable")
	assert.Nil(t, resource.Properties["nullable"])
	assert.Equal(t, []string{"settings"}, response.GetResources()[0].GetUnknownProperties())
	assert.NotContains(t, resource.Properties, "settings")
}

func TestTerraformPlanAdapterAllActions(t *testing.T) {
	t.Parallel()
	parser := &ingest.TerraformPlanParser{}
	response, err := parser.Parse(
		context.Background(),
		ingest.TerraformPlanRequest(readPlanFixture(t, "show.json")),
	)
	require.NoError(t, err)
	resources := ingest.AdaptTerraformPlan(response, parser.Operations(), parser.Before())
	require.Len(t, resources, len(response.GetResources()))
	for _, resource := range resources {
		assert.Equal(t, parser.Operations()[resource.ID], resource.Operation)
		assert.Equal(t, parser.Before()[resource.ID], resource.OldProperties)
		assert.NotContains(t, parser.SkippedProviders(), resource.Provider)
	}
	assert.Nil(t, ingest.AdaptTerraformPlan(nil, nil, nil))
}

func TestTerraformPlanSkipsUnrecognizedManagedAction(t *testing.T) {
	t.Parallel()
	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(
		[]byte(
			`{"format_version":"1.2","resource_changes":[{"address":"aws_instance.unsupported","mode":"managed","type":"aws_instance","change":{"actions":["read"]}}]}`,
		),
	)
	response, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, response))
	assert.Empty(t, response.GetResources())
	assertSkipped(
		t,
		response,
		"aws_instance.unsupported",
		pbc.ParseWarningSeverity_PARSE_WARNING_SEVERITY_WARNING,
	)
	assert.Empty(t, parser.SkippedProviders())
}

func TestTerraformPlanSensitivityTransitions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		beforeSensitive any
		afterSensitive  any
		unknown         any
	}{
		{
			name:            "sensitive before",
			beforeSensitive: map[string]any{"secret": true},
			afterSensitive:  map[string]any{"secret": false},
		},
		{
			name:            "sensitive after",
			beforeSensitive: map[string]any{"secret": false},
			afterSensitive:  map[string]any{"secret": true},
		},
		{
			name:            "unknown previously sensitive",
			beforeSensitive: map[string]any{"secret": true},
			unknown:         map[string]any{"secret": true},
		},
		{
			name:            "unknown descendant of sensitive object",
			beforeSensitive: map[string]any{"secret": true},
			unknown:         map[string]any{"secret": map[string]any{"nested": true}},
		},
		{
			name:            "entire resource sensitive",
			beforeSensitive: true,
			unknown:         map[string]any{"secret": map[string]any{"nested": true}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(map[string]any{
				"format_version": "1.2",
				"resource_changes": []any{map[string]any{
					"address": "aws_instance.sensitive", "mode": "managed", "type": "aws_instance",
					"change": map[string]any{
						"actions": []string{"update"},
						"before": map[string]any{
							"secret": map[string]any{"nested": "old-secret"},
							"known":  "old-public",
						},
						"after": map[string]any{
							"secret": map[string]any{"nested": "new-secret"},
							"known":  "new-public",
						},
						"before_sensitive": tt.beforeSensitive,
						"after_sensitive":  tt.afterSensitive,
						"after_unknown":    tt.unknown,
					},
				}},
			})
			require.NoError(t, err)
			parser := &ingest.TerraformPlanParser{}
			req := ingest.TerraformPlanRequest(raw)
			response, err := parser.Parse(context.Background(), req)
			require.NoError(t, err)
			require.NoError(t, pluginsdk.ValidateParseResponse(req, response))
			require.Len(t, response.GetResources(), 1)
			attrs := response.GetResources()[0].GetResource().GetAttributes().AsMap()
			assert.NotContains(t, attrs, "secret")
			assert.NotContains(t, parser.Before()["aws_instance.sensitive"], "secret")
			assert.Empty(t, response.GetResources()[0].GetUnknownProperties())
			if rootSensitive, ok := tt.beforeSensitive.(bool); ok && rootSensitive {
				assert.NotContains(t, attrs, "known")
				assert.NotContains(t, parser.Before()["aws_instance.sensitive"], "known")
			} else {
				assert.Equal(t, "new-public", attrs["known"])
				assert.Equal(t, "old-public", parser.Before()["aws_instance.sensitive"]["known"])
			}
		})
	}
}

func TestTerraformPlanSensitivityListUnionPreservesPositions(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`{"format_version":"1.2","resource_changes":[{"address":"aws_instance.list","mode":"managed","type":"aws_instance","change":{"actions":["update"],"before":{"settings":[{"token":"old-0","known":"zero"},{"token":"old-1","known":"one"},{"token":"old-2","known":"two"}]},"after":{"settings":[{"token":"new-0","known":"zero"},{"token":"new-1","known":"one"},{"token":"new-2","known":"two"}]},"before_sensitive":{"settings":[true,false,{"token":true}]},"after_sensitive":{"settings":[false,{"token":true},false]}}}]}`,
	)
	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(raw)
	response, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, response))
	require.Len(t, response.GetResources(), 1)
	want := []any{map[string]any{"known": "one"}, map[string]any{"known": "two"}}
	assert.Equal(t, want, response.GetResources()[0].GetResource().GetAttributes().AsMap()["settings"])
	assert.Equal(t, want, parser.Before()["aws_instance.list"]["settings"])
}

func TestTerraformPlanSensitiveListUnknownDoesNotExposeNames(t *testing.T) {
	t.Parallel()
	raw := []byte(
		`{"format_version":"1.2","resource_changes":[{"address":"aws_instance.list","mode":"managed","type":"aws_instance","change":{"actions":["update"],"before":{"settings":[{"token":"old"}]},"after":{"settings":[{"token":null}]},"before_sensitive":{"settings":[{"token":true}]},"after_sensitive":{},"after_unknown":{"settings":[{"token":true}]}}}]}`,
	)
	parser := &ingest.TerraformPlanParser{}
	req := ingest.TerraformPlanRequest(raw)
	response, err := parser.Parse(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateParseResponse(req, response))
	require.Len(t, response.GetResources(), 1)
	assert.Empty(t, response.GetResources()[0].GetUnknownProperties())
	assert.NotContains(t, fmt.Sprint(response.GetResources()[0].GetResource().GetAttributes().AsMap()), "token")
	assert.NotContains(t, fmt.Sprint(parser.Before()), "token")
}
