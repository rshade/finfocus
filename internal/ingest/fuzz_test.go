package ingest

import (
	"encoding/json"
	"testing"
)

// FuzzJSON tests the JSON parser for resilience against malformed inputs.
// The parser should return errors gracefully without panicking.
func FuzzJSON(f *testing.F) {
	// Add seed corpus - valid JSON structures
	f.Add([]byte(`{"steps":[]}`))
	f.Add([]byte(`{"steps":[{"op":"create","urn":"test","type":"aws:s3/bucket:Bucket"}]}`))
	f.Add([]byte(`{"steps":[{"op":"create","urn":"test","type":"test","inputs":{"key":"value"}}]}`))

	// Add seed corpus - newState structures (Pulumi v3+)
	// Create operation with newState containing type and inputs
	f.Add(
		[]byte(
			`{"steps":[{"op":"create","urn":"test","newState":{"type":"aws:ec2/instance:Instance","urn":"test","inputs":{"instanceType":"t3.micro"}}}]}`,
		),
	)
	// Update operation with oldState and newState
	f.Add(
		[]byte(
			`{"steps":[{"op":"update","urn":"test","oldState":{"type":"aws:s3/bucket:Bucket"},"newState":{"type":"aws:s3/bucket:Bucket","inputs":{"bucket":"my-bucket"}}}]}`,
		),
	)
	f.Add([]byte(`{"steps":[{"op":"create","urn":"test","newState":{"type":"malformed","inputs":null}}]}`))

	// Add seed corpus - edge cases
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`"string"`))
	f.Add([]byte(`123`))
	f.Add([]byte(`true`))

	// Add seed corpus - malformed JSON
	f.Add([]byte(`{`))
	f.Add([]byte(`{"steps":`))
	f.Add([]byte(`{"steps":[`))
	f.Add([]byte(`{"steps": null}`))
	f.Add([]byte(``))
	f.Add([]byte(`garbage`))

	// Add seed corpus - deeply nested
	f.Add(
		[]byte(
			`{"steps":[{"op":"create","urn":"test","type":"test","inputs":{"a":{"b":{"c":{"d":"value"}}}}}]}`,
		),
	)

	// Add seed corpus - unicode and special characters
	f.Add([]byte(`{"steps":[{"op":"create","urn":"bucket-日本語","type":"test"}]}`))
	f.Add([]byte(`{"steps":[{"op":"create","urn":"test\u0000null","type":"test"}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// The parser must not panic on any input
		var plan PulumiPlan
		_ = json.Unmarshal(data, &plan)

		// If parsing succeeded, exercise GetResources as well
		if len(plan.Steps) > 0 {
			_ = plan.GetResources()
		}
	})
}

// FuzzPulumiPlanParse tests the full LoadPulumiPlan flow with raw bytes.
// This fuzz target simulates what would happen if we could pass bytes directly.
func FuzzPulumiPlanParse(f *testing.F) {
	// Add seed corpus
	f.Add([]byte(`{"steps":[]}`))
	f.Add(
		[]byte(
			`{"steps":[{"op":"create","urn":"test","type":"aws:ec2:Instance","provider":"aws"}]}`,
		),
	)
	f.Add(
		[]byte(
			`{"steps":[{"op":"same","urn":"test","type":"aws:s3/bucket:Bucket","inputs":{"acl":"private"}}]}`,
		),
	)

	// Edge cases
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"steps":null}`))
	f.Add([]byte(`{"unexpected_field": true}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var plan PulumiPlan
		err := json.Unmarshal(data, &plan)

		// If parsing succeeded, verify we can safely access all fields
		if err == nil {
			for _, step := range plan.Steps {
				_ = step.Op
				_ = step.URN
				_ = step.Type
				_ = step.Provider
				_ = step.Inputs
				_ = step.Outputs

				// Exercise the provider extraction
				_ = extractProviderFromURN(step.URN)
			}

			// Exercise GetResources
			resources := plan.GetResources()
			for _, r := range resources {
				_ = r.Type
				_ = r.URN
				_ = r.Provider
				_ = r.Inputs
			}
		}
	})
}
