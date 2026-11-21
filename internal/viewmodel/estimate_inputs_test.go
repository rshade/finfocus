// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package viewmodel

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

const (
	ec2Type      = "aws:ec2/instance:Instance"
	pulumiSecret = "4dabf18193072939515e22adb298388d"
)

func planKeys(rows []EstimatePropertyRow) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}

func detailKeys(details []EstimateDetail) []string {
	keys := make([]string, 0, len(details))
	for _, detail := range details {
		keys = append(keys, detail.Key)
	}
	return keys
}

func rowByKey(t *testing.T, rows []EstimatePropertyRow, key string) EstimatePropertyRow {
	t.Helper()
	for _, row := range rows {
		if row.Key == key {
			return row
		}
	}
	require.Failf(t, "row not found", "no row for key %q in %v", key, planKeys(rows))
	return EstimatePropertyRow{}
}

func planFor(resourceType string, props map[string]any, overrides map[string]string,
	hints []engine.PricingMetricHint,
) EstimatePropertyPlan {
	return BuildEstimatePropertyPlan(
		context.Background(),
		&engine.ResourceDescriptor{Type: resourceType, Properties: props},
		overrides,
		NewEstimateInputContext(engine.PricingDiscovery{Modes: []engine.PricingMode{{
			Plugin: "aws-public", BillingMode: "per_hour", MetricHints: hints,
		}}}, ""),
	)
}

func TestBuildEstimatePropertyPlanReviewedOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		props        map[string]any
		wantPricing  []string
		wantOther    []string
	}{
		{
			name:         "ec2 lists reviewed keys in mapping order, then the rest sorted",
			resourceType: ec2Type,
			props: map[string]any{
				"tenancy": "default", "ami": "ami-1", "instanceType": "t3.micro",
				"region": "us-east-1", "subnetId": "subnet-1", "availabilityZone": "us-east-1a",
			},
			wantPricing: []string{"instanceType", "region", "availabilityZone", "tenancy"},
			wantOther:   []string{"ami", "subnetId"},
		},
		{
			name:         "an absent sku makes no row and an absent plugin-input is a usage row",
			resourceType: ec2Type,
			props:        map[string]any{"instanceType": "t3.micro", "ami": "ami-1"},
			wantPricing:  []string{"instanceType", "tenancy"},
			wantOther:    []string{"ami"},
		},
		{
			// Pulumi names the EBS volume type `type`; core reads it as the SKU.
			name:         "ebs volume",
			resourceType: "aws:ebs/volume:Volume",
			props:        map[string]any{"size": 100, "type": "gp3", "encrypted": true},
			wantPricing:  []string{"type", "size"},
			wantOther:    []string{"encrypted"},
		},
		{
			name:         "lambda function",
			resourceType: "aws:lambda/function:Function",
			props:        map[string]any{"memorySize": 256, "handler": "main", "architectures": []any{"arm64"}},
			wantPricing:  []string{"memorySize", "architectures", "requestsPerMonth", "avgDurationMs"},
			wantOther:    []string{"handler"},
		},
		{
			name:         "dynamodb table",
			resourceType: "aws:dynamodb/table:Table",
			props:        map[string]any{"billingMode": "PROVISIONED", "readCapacity": 5, "hashKey": "id"},
			wantPricing: []string{
				"billingMode",
				"readCapacity",
				"writeCapacity",
				"storageGb",
				"readRequestsPerMonth",
				"writeRequestsPerMonth",
			},
			wantOther: []string{"hashKey"},
		},
		{
			name:         "lb load balancer",
			resourceType: "aws:lb/loadBalancer:LoadBalancer",
			props:        map[string]any{"loadBalancerType": "application", "internal": false},
			wantPricing:  []string{"loadBalancerType", "lcuPerHour", "nlcuPerHour", "capacityUnits"},
			wantOther:    []string{"internal"},
		},
		{
			name:         "alb maps like lb",
			resourceType: "aws:alb/loadBalancer:LoadBalancer",
			props:        map[string]any{"loadBalancerType": "network", "internal": false},
			wantPricing:  []string{"loadBalancerType", "lcuPerHour", "nlcuPerHour", "capacityUnits"},
			wantOther:    []string{"internal"},
		},
		{
			name:         "elasticache cluster",
			resourceType: "aws:elasticache/cluster:Cluster",
			props:        map[string]any{"nodeType": "cache.t3.micro", "numCacheNodes": 2, "port": 6379},
			wantPricing:  []string{"nodeType", "numCacheNodes", "engine"},
			wantOther:    []string{"port"},
		},
		{
			name:         "s3 bucket",
			resourceType: "aws:s3/bucket:Bucket",
			props:        map[string]any{"storageClass": "STANDARD", "acl": "private"},
			wantPricing:  []string{"storageClass"},
			wantOther:    []string{"acl"},
		},
		{
			name:         "nat gateway",
			resourceType: "aws:ec2/natGateway:NatGateway",
			props:        map[string]any{"dataProcessedGb": 10, "subnetId": "subnet-1"},
			wantPricing:  []string{"dataProcessedGb"},
			wantOther:    []string{"subnetId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan := planFor(tt.resourceType, tt.props, nil, nil)

			require.True(t, plan.Curated)
			assert.Empty(t, plan.Note)
			assert.Equal(t, append(append([]string{}, tt.wantPricing...), tt.wantOther...), planKeys(plan.Rows))
			for _, key := range tt.wantPricing {
				row := rowByKey(t, plan.Rows, key)
				assert.Equal(t, PropertyClassPricing, row.Class, key)
				assert.NotEmpty(t, row.Provenance, key)
				assert.True(t, row.Editable, key)
				assert.True(t, row.DefaultVisible, key)
			}
			for _, key := range tt.wantOther {
				row := rowByKey(t, plan.Rows, key)
				assert.Equal(t, PropertyClassOther, row.Class, key)
				assert.Empty(t, row.Provenance, key)
				assert.True(t, row.Editable, key)
				assert.False(t, row.DefaultVisible, key)
			}
		})
	}
}

func TestBuildEstimatePropertyPlanProvenance(t *testing.T) {
	t.Parallel()

	plan := planFor(ec2Type, map[string]any{
		"instanceType": "t3.micro", "region": "us-east-1", "availabilityZone": "us-east-1a", "tenancy": "default",
	}, nil, nil)

	assert.Equal(t, ProvenanceSKU, rowByKey(t, plan.Rows, "instanceType").Provenance)
	assert.Equal(t, ProvenanceRegion, rowByKey(t, plan.Rows, "region").Provenance)
	assert.Equal(t, ProvenanceRegion, rowByKey(t, plan.Rows, "availabilityZone").Provenance)
	assert.Equal(t, ProvenancePluginInput, rowByKey(t, plan.Rows, "tenancy").Provenance)
}

func TestBuildEstimatePropertyPlanFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		props        map[string]any
	}{
		{
			name:         "unknown type",
			resourceType: "aws:ec2/vpc:Vpc",
			props:        map[string]any{"cidrBlock": "10.0.0.0/16"},
		},
		{
			name:         "empty type",
			resourceType: "",
			props:        map[string]any{"name": "x"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan := planFor(tt.resourceType, tt.props, nil, nil)

			assert.False(t, plan.Curated)
			assert.Equal(t, EstimateFallbackNote, plan.Note)
			require.Len(t, plan.Rows, len(tt.props))
			for _, row := range plan.Rows {
				assert.Equal(t, PropertyClassOther, row.Class, row.Key)
				assert.True(t, row.Editable, row.Key)
				assert.True(t, row.DefaultVisible, row.Key)
			}
		})
	}
}

func TestBuildEstimatePropertyPlanZeroDeltaNeverDemotes(t *testing.T) {
	t.Parallel()

	plan := planFor(ec2Type, map[string]any{"instanceType": "t3.micro", "ami": "ami-1"}, nil, nil)

	ApplyEstimateDeltas(plan.Rows, []engine.CostDelta{{Property: "instanceType", CostChange: 0}})

	row := rowByKey(t, plan.Rows, "instanceType")
	assert.Equal(t, PropertyClassPricing, row.Class)
	assert.True(t, row.DefaultVisible)
	assert.InDelta(t, 0.0, row.CostDelta, 0)

	ApplyEstimateDeltas(plan.Rows, nil)

	assert.Equal(t, PropertyClassPricing, rowByKey(t, plan.Rows, "instanceType").Class)
	assert.False(t, rowByKey(t, plan.Rows, "ami").DefaultVisible)
}

func TestBuildEstimatePropertyPlanIdentity(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"instanceType":       "t3.micro",
		"subnetId":           "subnet-1",
		"tags":               map[string]any{"arn": "tag-value", "Name": "web"},
		"id":                 "i-123",
		"arn":                "arn:aws:ec2:us-east-1:1:instance/i-123",
		"urn":                "urn:pulumi:dev::p::aws:ec2/instance:Instance::web",
		"pulumi:cloudId":     "i-123",
		"pulumi:created":     "2026-01-01T00:00:00Z",
		"terraform:arn":      "arn:aws:ec2:us-east-1:1:instance/i-123",
		"terraform:provider": "aws",
	}
	wantIdentity := []string{
		"arn", "id", "pulumi:cloudId", "pulumi:created", "terraform:arn", "terraform:provider", "urn",
	}

	plan := planFor(ec2Type, props, nil, nil)

	assert.Equal(t, wantIdentity, detailKeys(plan.Details))
	assert.Equal(t, []string{"instanceType", "tenancy", "subnetId", "tags"}, planKeys(plan.Rows))
	for _, detail := range plan.Details {
		assert.NotContains(t, planKeys(plan.Rows), detail.Key)
	}
	assert.Equal(t, "i-123", plan.Details[1].Value)
	assert.True(t, rowByKey(t, plan.Rows, "subnetId").Editable)
	assert.True(t, rowByKey(t, plan.Rows, "tags").Editable)
}

func TestBuildEstimatePropertyPlanIdentityIsExact(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"instanceType": "t3.micro", "ID": "upper", "Arn": "mixed", "idx": "prefix", "vpcArn": "suffix",
		"pulumi": "no-colon", "terraformer:x": "near-namespace",
	}

	plan := planFor(ec2Type, props, nil, nil)

	assert.Empty(t, plan.Details)
	assert.Len(t, plan.Rows, len(props)+1)
	assert.True(t, rowByKey(t, plan.Rows, "tenancy").Usage)
}

func TestBuildEstimatePropertyPlanIdentityOnlyDescriptor(t *testing.T) {
	t.Parallel()

	plan := planFor("aws:s3/bucket:Bucket", map[string]any{"arn": "a", "id": "b"}, nil, nil)

	assert.True(t, plan.Curated)
	assert.Empty(t, plan.Note)
	require.Len(t, plan.Rows, 1)
	assert.True(t, plan.Rows[0].Usage)
	assert.Equal(t, "storageClass", plan.Rows[0].Key)
	assert.Equal(t, []string{"arn", "id"}, detailKeys(plan.Details))
}

func TestBuildEstimatePropertyPlanOverrides(t *testing.T) {
	t.Parallel()

	t.Run("an overridden other row is default visible", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type,
			map[string]any{"instanceType": "t3.micro", "ami": "ami-1", "subnetId": "subnet-1"},
			map[string]string{"ami": "ami-2"}, nil)

		ami := rowByKey(t, plan.Rows, "ami")
		assert.True(t, ami.Overridden)
		assert.True(t, ami.DefaultVisible)
		assert.Equal(t, "ami-1", ami.OriginalValue)
		assert.Equal(t, "ami-2", ami.CurrentValue)
		subnet := rowByKey(t, plan.Rows, "subnetId")
		assert.False(t, subnet.Overridden)
		assert.False(t, subnet.DefaultVisible)
	})

	t.Run("an override equal to the original is not overridden", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type,
			map[string]any{"instanceType": "t3.micro", "ami": "ami-1"},
			map[string]string{"ami": "ami-1"}, nil)

		ami := rowByKey(t, plan.Rows, "ami")
		assert.False(t, ami.Overridden)
		assert.False(t, ami.DefaultVisible)
	})

	t.Run("an override naming a property the descriptor lacks stays visible", func(t *testing.T) {
		t.Parallel()

		props := map[string]any{"instanceType": "t3.micro", "ami": "ami-1"}
		overrides := map[string]string{"ebsOptimized": "true"}

		plan := planFor(ec2Type, props, overrides, nil)

		assert.Equal(t, []string{"instanceType", "tenancy", "ami", "ebsOptimized"}, planKeys(plan.Rows))
		row := rowByKey(t, plan.Rows, "ebsOptimized")
		assert.Equal(t, PropertyClassOther, row.Class)
		assert.True(t, row.Overridden)
		assert.True(t, row.DefaultVisible)
		assert.Empty(t, plan.Details)
		require.NoError(t, RejectIdentityOverrides(overrides))
	})
}

func TestBuildEstimatePropertyPlanUsageHints(t *testing.T) {
	t.Parallel()

	props := map[string]any{"instanceType": "t3.micro", "monthlyHours": 100, "ami": "ami-1", "Monthly": 1}

	t.Run("an exact key match becomes a usage pricing row after the mapped rows", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, props, nil, []engine.PricingMetricHint{{Metric: "monthlyHours", Unit: "hour"}})

		assert.Equal(t, []string{"instanceType", "monthlyHours", "tenancy", "Monthly", "ami"}, planKeys(plan.Rows))
		usage := rowByKey(t, plan.Rows, "monthlyHours")
		assert.Equal(t, PropertyClassPricing, usage.Class)
		assert.Equal(t, ProvenanceUsage, usage.Provenance)
		assert.True(t, usage.DefaultVisible)
	})

	t.Run("a different case or prefix does not match", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, props, nil, []engine.PricingMetricHint{{Metric: "MonthlyHours"}, {Metric: "month"}})

		assert.Equal(t, PropertyClassOther, rowByKey(t, plan.Rows, "monthlyHours").Class)
		assert.Equal(t, PropertyClassOther, rowByKey(t, plan.Rows, "Monthly").Class)
	})

	t.Run("failed or empty discovery gives no usage row", func(t *testing.T) {
		t.Parallel()

		for _, hints := range [][]engine.PricingMetricHint{nil, {}} {
			plan := planFor(ec2Type, props, nil, hints)

			assert.Equal(t, PropertyClassOther, rowByKey(t, plan.Rows, "monthlyHours").Class)
			assert.False(t, rowByKey(t, plan.Rows, "monthlyHours").DefaultVisible)
		}
	})

	t.Run("a hint never reclassifies an identity key", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, map[string]any{"instanceType": "t3.micro", "arn": "a"}, nil,
			[]engine.PricingMetricHint{{Metric: "arn"}})

		assert.Equal(t, []string{"arn"}, detailKeys(plan.Details))
		assert.Equal(t, []string{"instanceType", "tenancy"}, planKeys(plan.Rows))
	})

	t.Run("a hint on an unmapped type does not make it curated", func(t *testing.T) {
		t.Parallel()

		plan := planFor("aws:ec2/vpc:Vpc", map[string]any{"cidrBlock": "10.0.0.0/16"}, nil,
			[]engine.PricingMetricHint{{Metric: "cidrBlock"}})

		assert.False(t, plan.Curated)
		assert.Equal(t, EstimateFallbackNote, plan.Note)
	})
}

func TestBuildEstimatePropertyPlanRedaction(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"instanceType":     "t3.micro",
		"password":         "hunter2",
		"apiToken":         "tok",
		"__providerRegion": "us-west-2",
		"__defaults":       []any{},
		"userData":         map[string]any{pulumiSecret: "1", "ciphertext": "AAAA"},
		"tags":             map[string]any{"secret": "s", "Name": "web"},
		"ami":              "ami-1",
		"arn":              "arn:aws:ec2:us-east-1:1:instance/i-1",
	}

	plan := planFor(ec2Type, props, nil, nil)

	all := append(planKeys(plan.Rows), detailKeys(plan.Details)...)
	for _, hidden := range []string{"password", "apiToken", "__providerRegion", "__defaults", "userData"} {
		assert.NotContains(t, all, hidden)
	}
	for _, row := range plan.Rows {
		assert.NotContains(t, row.CurrentValue, "hunter2")
		assert.NotContains(t, row.CurrentValue, "ciphertext")
		assert.NotContains(t, row.CurrentValue, "secret")
	}
}

func TestBuildEstimatePropertyPlanCoversRedactedInput(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"instanceType": "t3.micro", "ami": "ami-1", "subnetId": "s", "region": "us-east-1",
		"id": "i-1", "arn": "a", "pulumi:urn": "u", "terraform:id": "t",
		"password": "p", "__defaults": []any{},
	}
	want := make([]string, 0, len(props)+1)
	for key := range engine.RedactedProperties(context.Background(), props) {
		want = append(want, key)
	}
	want = append(want, "tenancy")
	sort.Strings(want)

	plan := planFor(ec2Type, props, nil, nil)

	got := append(planKeys(plan.Rows), detailKeys(plan.Details)...)
	sort.Strings(got)
	assert.Equal(t, want, got)
}

func TestBuildEstimatePropertyPlanNilAndEmpty(t *testing.T) {
	t.Parallel()

	t.Run("nil resource", func(t *testing.T) {
		t.Parallel()

		plan := BuildEstimatePropertyPlan(context.Background(), nil, nil, EstimateInputContext{})

		assert.Empty(t, plan.Rows)
		assert.Empty(t, plan.Details)
		assert.False(t, plan.Curated)
	})

	t.Run("no properties", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, nil, nil, nil)

		require.Len(t, plan.Rows, 1)
		assert.True(t, plan.Rows[0].Usage)
		assert.Equal(t, "tenancy", plan.Rows[0].Key)
		assert.Empty(t, plan.Details)
		assert.True(t, plan.Curated)
		assert.Empty(t, plan.Note)
	})
}

func awsContext(plugin, modeID string, hints []engine.PricingMetricHint) EstimateInputContext {
	return NewEstimateInputContext(engine.PricingDiscovery{Modes: []engine.PricingMode{{
		Plugin: plugin, BillingMode: "per_hour", MetricHints: hints,
	}}}, modeID)
}

func TestBuildEstimatePropertyPlanUsageRowsAndPluginScope(t *testing.T) {
	t.Parallel()

	t.Run("absent plugin-input is an empty usage row in mapping order", func(t *testing.T) {
		t.Parallel()

		lambda := planFor("aws:lambda/function:Function", map[string]any{"memorySize": float64(128)}, nil, nil)
		requests := rowByKey(t, lambda.Rows, "requestsPerMonth")
		assert.Equal(
			t,
			[]string{"memorySize", "architectures", "requestsPerMonth", "avgDurationMs"},
			planKeys(lambda.Rows)[:4],
		)
		assert.True(t, requests.Usage)
		assert.Empty(t, requests.OriginalValue)
		assert.Empty(t, requests.CurrentValue)
		assert.True(t, requests.Editable)
		assert.True(t, requests.DefaultVisible)
		assert.False(t, requests.Overridden)
		assert.Equal(t, PropertyClassPricing, requests.Class)
		assert.True(t, lambda.Curated)
		assert.Empty(t, lambda.Note)

		bucket := planFor("aws:s3/bucket:Bucket", map[string]any{"acl": "private"}, nil, nil)
		storage := rowByKey(t, bucket.Rows, "storageClass")
		assert.Equal(t, "storageClass", bucket.Rows[0].Key)
		assert.True(t, storage.Usage)
		assert.True(t, bucket.Curated)
	})

	t.Run("a filled usage value is an override and an empty one is not", func(t *testing.T) {
		t.Parallel()

		filled := planFor("aws:lambda/function:Function", nil, map[string]string{"requestsPerMonth": "1000000"}, nil)
		row := rowByKey(t, filled.Rows, "requestsPerMonth")
		assert.Equal(t, "1000000", row.CurrentValue)
		assert.True(t, row.Overridden)
		assert.True(t, row.Usage)

		empty := planFor("aws:lambda/function:Function", nil, map[string]string{"requestsPerMonth": ""}, nil)
		cleared := rowByKey(t, empty.Rows, "requestsPerMonth")
		assert.Empty(t, cleared.CurrentValue)
		assert.False(t, cleared.Overridden)
	})

	t.Run("an absent sku or region makes no row", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, map[string]any{"instanceType": "t3.micro"}, nil, nil)
		assert.NotContains(t, planKeys(plan.Rows), "region")
		assert.NotContains(t, planKeys(plan.Rows), "availabilityZone")
	})

	t.Run("another plugin drops plugin-input rows and keeps the sku", func(t *testing.T) {
		t.Parallel()

		mode := (&engine.PricingMode{Plugin: "civo-public", BillingMode: "per_hour"}).ID()
		input := awsContext("civo-public", mode, nil)
		plan := BuildEstimatePropertyPlan(context.Background(), &engine.ResourceDescriptor{
			Type: ec2Type, Properties: map[string]any{"instanceType": "t3.micro", "tenancy": "default", "ami": "ami-1"},
		}, nil, input)

		assert.Equal(t, PropertyClassPricing, rowByKey(t, plan.Rows, "instanceType").Class)
		assert.Equal(t, PropertyClassOther, rowByKey(t, plan.Rows, "tenancy").Class)
		assert.False(t, rowByKey(t, plan.Rows, "tenancy").Usage)
		assert.True(t, plan.Curated)
	})

	t.Run("automatic applies a discovered aws-public plugin", func(t *testing.T) {
		t.Parallel()

		input := awsContext("finfocus-plugin-aws-public", "", nil)
		plan := BuildEstimatePropertyPlan(context.Background(), &engine.ResourceDescriptor{
			Type: "aws:s3/bucket:Bucket", Properties: map[string]any{"acl": "private"},
		}, nil, input)

		assert.True(t, rowByKey(t, plan.Rows, "storageClass").Usage)
		assert.True(t, plan.Curated)
	})

	t.Run("empty discovery keeps only sku and region rows", func(t *testing.T) {
		t.Parallel()

		ec2 := BuildEstimatePropertyPlan(context.Background(), &engine.ResourceDescriptor{
			Type: ec2Type, Properties: map[string]any{"instanceType": "t3.micro", "tenancy": "default"},
		}, nil, EstimateInputContext{})
		assert.Equal(t, PropertyClassPricing, rowByKey(t, ec2.Rows, "instanceType").Class)
		assert.Equal(t, PropertyClassOther, rowByKey(t, ec2.Rows, "tenancy").Class)
		assert.True(t, ec2.Curated)

		bucket := BuildEstimatePropertyPlan(context.Background(), &engine.ResourceDescriptor{
			Type: "aws:s3/bucket:Bucket", Properties: map[string]any{"acl": "private"},
		}, nil, EstimateInputContext{})
		assert.NotContains(t, planKeys(bucket.Rows), "storageClass")
		assert.False(t, bucket.Curated)
		assert.Equal(t, EstimateFallbackNote, bucket.Note)
	})

	t.Run("an override whose usage row does not apply stays as an edited property", func(t *testing.T) {
		t.Parallel()

		plan := BuildEstimatePropertyPlan(context.Background(), &engine.ResourceDescriptor{
			Type: "aws:s3/bucket:Bucket", Properties: map[string]any{"acl": "private"},
		}, map[string]string{"storageClass": "GLACIER"}, EstimateInputContext{})

		row := rowByKey(t, plan.Rows, "storageClass")
		assert.Equal(t, PropertyClassOther, row.Class)
		assert.False(t, row.Usage)
		assert.True(t, row.Overridden)
		assert.Equal(t, "GLACIER", row.CurrentValue)
		assert.True(t, row.DefaultVisible)
	})

	t.Run("a hidden override key makes no row", func(t *testing.T) {
		t.Parallel()

		plan := planFor(
			ec2Type,
			map[string]any{"instanceType": "t3.micro"},
			map[string]string{"password": "secret"},
			nil,
		)
		assert.NotContains(t, planKeys(plan.Rows), "password")
	})

	t.Run("a later present input stays ahead of earlier usage rows", func(t *testing.T) {
		t.Parallel()

		plan := planFor("aws:lambda/function:Function", map[string]any{
			"memorySize": float64(128), "avgDurationMs": float64(50),
		}, nil, nil)

		assert.Equal(
			t,
			[]string{"memorySize", "avgDurationMs", "architectures", "requestsPerMonth"},
			planKeys(plan.Rows),
		)
		assert.False(t, rowByKey(t, plan.Rows, "avgDurationMs").Usage)
		assert.True(t, rowByKey(t, plan.Rows, "architectures").Usage)
	})

	t.Run("an override of a redacted property makes no row", func(t *testing.T) {
		t.Parallel()

		plan := planFor(ec2Type, map[string]any{
			"instanceType": "t3.micro",
			"private":      map[string]any{pulumiSecret: "1", "ciphertext": "secret-value"},
		}, map[string]string{"private": "new-secret"}, nil)

		assert.NotContains(t, planKeys(plan.Rows), "private")
		for _, row := range plan.Rows {
			assert.NotContains(t, row.CurrentValue, "new-secret")
			assert.NotContains(t, row.CurrentValue, "secret-value")
		}
	})

	t.Run("a usage key is not identity", func(t *testing.T) {
		t.Parallel()

		require.NoError(
			t,
			RejectIdentityOverrides(map[string]string{"requestsPerMonth": "1", "storageClass": "STANDARD"}),
		)
	})

	t.Run("rds lists its reviewed keys", func(t *testing.T) {
		t.Parallel()

		plan := planFor("aws:rds/instance:Instance", map[string]any{
			"instanceClass": "db.t3.micro", "engine": "mysql", "allocatedStorage": float64(20),
		}, nil, nil)
		assert.Equal(t, []string{
			"instanceClass", "engine", "allocatedStorage", "storageType", "multiAz",
		}, planKeys(plan.Rows))
		assert.False(t, rowByKey(t, plan.Rows, "engine").Usage)
		assert.True(t, rowByKey(t, plan.Rows, "storageType").Usage)
		assert.True(t, plan.Curated)
	})
}

func TestRejectIdentityOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		overrides map[string]string
		wantKey   string
	}{
		{name: "nil map", overrides: nil},
		{name: "pricing and other keys", overrides: map[string]string{"instanceType": "t3.large", "subnetId": "s"}},
		{name: "a tag named arn is not a key", overrides: map[string]string{"tags": "arn"}},
		{name: "exact arn", overrides: map[string]string{"arn": "x"}, wantKey: "arn"},
		{name: "exact urn", overrides: map[string]string{"urn": "x"}, wantKey: "urn"},
		{name: "exact id", overrides: map[string]string{"id": "x"}, wantKey: "id"},
		{name: "pulumi namespace", overrides: map[string]string{"pulumi:cloudId": "x"}, wantKey: "pulumi:cloudId"},
		{name: "terraform namespace", overrides: map[string]string{"terraform:arn": "x"}, wantKey: "terraform:arn"},
		{
			name:      "names the first key in sorted order",
			overrides: map[string]string{"urn": "u", "arn": "a", "instanceType": "t3.large"},
			wantKey:   "arn",
		},
		{name: "case differs so it is not identity", overrides: map[string]string{"ARN": "x", "Id": "y"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := RejectIdentityOverrides(tt.overrides)

			if tt.wantKey == "" {
				require.NoError(t, err)
				return
			}
			var identityErr *IdentityOverrideError
			require.ErrorAs(t, err, &identityErr)
			assert.Equal(t, tt.wantKey, identityErr.Key)
			assert.Equal(t, `identity property "`+tt.wantKey+`" cannot be overridden`, err.Error())
			for _, value := range tt.overrides {
				assert.NotContains(t, err.Error(), `"`+value+`"`)
			}
		})
	}
}

func TestEstimatePropertyRowSetCurrentValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		class       PropertyClass
		curated     bool
		value       string
		wantOver    bool
		wantVisible bool
	}{
		{name: "other row edited stays visible", class: PropertyClassOther, curated: true, value: "b", wantOver: true,
			wantVisible: true},
		{name: "other row edited back is hidden", class: PropertyClassOther, curated: true, value: "a"},
		{name: "pricing row is always visible", class: PropertyClassPricing, curated: true, value: "a",
			wantVisible: true},
		{name: "uncurated shows every row", class: PropertyClassOther, curated: false, value: "a", wantVisible: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			row := EstimatePropertyRow{Key: "k", OriginalValue: "a", CurrentValue: "z", Class: tt.class}
			row.SetCurrentValue(tt.value, tt.curated)
			assert.Equal(t, tt.value, row.CurrentValue)
			assert.Equal(t, tt.wantOver, row.Overridden)
			assert.Equal(t, tt.wantVisible, row.DefaultVisible)
		})
	}
}
