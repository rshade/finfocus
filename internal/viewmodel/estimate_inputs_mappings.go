// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package viewmodel

import "github.com/rshade/finfocus-spec/sdk/go/pluginsdk/mapping"

const (
	awsProvider     = "aws"
	awsPublicPlugin = "aws-public"
)

// InputSpec names one reviewed pricing input and why it is trusted.
type InputSpec struct {
	Key        string
	Provenance InputProvenance
}

// InputMapping lists the reviewed pricing inputs of one resource type, in
// display order. Provider is the normalized billing cloud and ResourceSegment
// the lowercased module/resource segment of the type token. Plugin names the
// plugin whose plugin-input keys the list uses.
type InputMapping struct {
	Provider        string
	ResourceSegment string
	Plugin          string
	Inputs          []InputSpec
}

func pluginInputs(keys ...string) []InputSpec {
	specs := make([]InputSpec, 0, len(keys))
	for _, key := range keys {
		specs = append(specs, InputSpec{Key: key, Provenance: ProvenancePluginInput})
	}
	return specs
}

func loadBalancerInputs() []InputSpec {
	return pluginInputs("loadBalancerType", "lcuPerHour", "nlcuPerHour", "capacityUnits")
}

// reviewedInputMappings is the reviewed AWS list from research R10. A sku or
// region key must be one the shared extraction reads; the provenance test
// proves it. A plugin-input key is a name the aws-public plugin documents.
func reviewedInputMappings() []InputMapping {
	return []InputMapping{
		{
			Provider: awsProvider, ResourceSegment: "ec2/instance", Plugin: awsPublicPlugin,
			Inputs: append(
				[]InputSpec{
					{Key: mapping.AWSKeyInstanceType, Provenance: ProvenanceSKU},
					{Key: mapping.AWSKeyRegion, Provenance: ProvenanceRegion},
					{Key: mapping.AWSKeyAvailabilityZone, Provenance: ProvenanceRegion},
				},
				pluginInputs("tenancy")...,
			),
		},
		{
			Provider: awsProvider, ResourceSegment: "ebs/volume", Plugin: awsPublicPlugin,
			Inputs: append(
				// Pulumi names the EBS volume type `type`; ExtractAWSSKU reads it.
				[]InputSpec{{Key: mapping.AWSKeyType, Provenance: ProvenanceSKU}},
				pluginInputs("size")...,
			),
		},
		{
			Provider: awsProvider, ResourceSegment: "rds/instance", Plugin: awsPublicPlugin,
			Inputs: append(
				[]InputSpec{
					{Key: mapping.AWSKeyInstanceClass, Provenance: ProvenanceSKU},
					{Key: mapping.AWSKeyRegion, Provenance: ProvenanceRegion},
					{Key: mapping.AWSKeyAvailabilityZone, Provenance: ProvenanceRegion},
				},
				pluginInputs("engine", "storageType", "allocatedStorage", "multiAz")...,
			),
		},
		{
			Provider: awsProvider, ResourceSegment: "lambda/function", Plugin: awsPublicPlugin,
			Inputs: pluginInputs("memorySize", "architectures", "requestsPerMonth", "avgDurationMs"),
		},
		{
			Provider: awsProvider, ResourceSegment: "dynamodb/table", Plugin: awsPublicPlugin,
			Inputs: pluginInputs(
				"billingMode", "readCapacity", "writeCapacity", "storageGb",
				"readRequestsPerMonth", "writeRequestsPerMonth",
			),
		},
		{
			Provider:        awsProvider,
			ResourceSegment: "lb/loadbalancer",
			Plugin:          awsPublicPlugin,
			Inputs:          loadBalancerInputs(),
		},
		{
			Provider:        awsProvider,
			ResourceSegment: "alb/loadbalancer",
			Plugin:          awsPublicPlugin,
			Inputs:          loadBalancerInputs(),
		},
		{
			Provider: awsProvider, ResourceSegment: "elasticache/cluster", Plugin: awsPublicPlugin,
			Inputs: pluginInputs("nodeType", "numCacheNodes", "engine"),
		},
		{
			Provider:        awsProvider,
			ResourceSegment: "s3/bucket",
			Plugin:          awsPublicPlugin,
			Inputs:          pluginInputs("storageClass"),
		},
		{
			Provider:        awsProvider,
			ResourceSegment: "ec2/natgateway",
			Plugin:          awsPublicPlugin,
			Inputs:          pluginInputs("dataProcessedGb"),
		},
	}
}
