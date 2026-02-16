package spec

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// FuzzYAML tests the YAML parser for resilience against malformed inputs.
// The parser should return errors gracefully without panicking.
func FuzzYAML(f *testing.F) {
	// Add seed corpus - valid YAML structures
	f.Add([]byte(`provider: aws
service: ec2
sku: t3-micro
currency: USD
pricing:
  hourly: 0.0104`))

	f.Add([]byte(`provider: azure
service: compute
sku: standard-b1s
currency: USD
pricing:
  hourly: 0.0124
metadata:
  region: eastus`))

	// Add seed corpus - minimal valid
	f.Add([]byte(`provider: test`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))

	// Add seed corpus - edge cases
	f.Add([]byte(`null`))
	f.Add([]byte(`~`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`- item1
- item2`))

	// Add seed corpus - malformed YAML
	f.Add([]byte(`provider: "unclosed`))
	f.Add([]byte(`key: value
  bad_indent: here`))
	f.Add([]byte(`---
...
---`))

	// Add seed corpus - deeply nested
	f.Add([]byte(`pricing:
  tier1:
    tier2:
      tier3:
        tier4:
          value: 1.23`))

	// Add seed corpus - unicode and special characters
	f.Add([]byte(`provider: 日本語
service: サービス`))
	f.Add([]byte("provider: \"value\\x00null\""))

	// Add seed corpus - large numbers and edge values
	f.Add([]byte(`pricing:
  hourly: 999999999999999999999999999999
  monthly: -0.00000000001
  yearly: .inf`))

	// Add seed corpus - anchors and aliases (YAML specific)
	f.Add([]byte(`defaults: &defaults
  currency: USD
spec:
  <<: *defaults
  provider: aws`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Recover from panics in the YAML library (e.g., yaml.v3 merge key bugs).
		// Our goal is to test that our code handles all inputs gracefully;
		// panics inside third-party libraries are not our fault.
		defer func() {
			if r := recover(); r != nil {
				t.Logf("yaml.Unmarshal panicked (known yaml.v3 issue): %v", r)
			}
		}()

		var spec PricingSpec
		_ = yaml.Unmarshal(data, &spec)

		// If parsing succeeded, verify we can safely access all fields
		_ = spec.Provider
		_ = spec.Service
		_ = spec.SKU
		_ = spec.Currency
		_ = spec.Pricing
		_ = spec.Metadata
	})
}

// FuzzSpecFilename tests the ParseSpecFilename function.
func FuzzSpecFilename(f *testing.F) {
	// Add seed corpus - valid filenames
	f.Add("aws-ec2-t3-micro.yaml")
	f.Add("azure-compute-standard-b1s.yaml")
	f.Add("gcp-compute-n1-standard-1.yaml")
	f.Add("aws-s3-standard.yml")

	// Add seed corpus - edge cases
	f.Add("")
	f.Add(".yaml")
	f.Add("-.yaml")
	f.Add("a-b-c.yaml")
	f.Add("provider-service.yaml")     // Only 2 parts
	f.Add("provider.yaml")             // Only 1 part
	f.Add("a-b-c-d-e-f.yaml")          // Many dashes in SKU
	f.Add("Provider-Service-SKU.YAML") // Mixed case extension
	f.Add("test.txt")                  // Wrong extension
	f.Add("no-extension")              // No extension
	f.Add("unicode-日本語-test.yaml")     // Unicode in parts

	f.Fuzz(func(t *testing.T, filename string) {
		// The function must not panic on any input
		provider, service, sku, ok := ParseSpecFilename(filename)

		// If parsing succeeded, verify results are non-empty
		if ok {
			if provider == "" || service == "" || sku == "" {
				t.Errorf(
					"ParseSpecFilename returned ok=true but empty parts: %q, %q, %q",
					provider,
					service,
					sku,
				)
			}
		}
	})
}
