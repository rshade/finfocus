package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestEstimatePropertyPlanParityWithTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		resourceType string
		props        map[string]any
	}{
		{
			name:         "ec2 instance",
			resourceType: "aws:ec2/instance:Instance",
			props: map[string]any{
				"instanceType": "t3.micro", "tenancy": "default", "ami": "ami-1", "subnetId": "subnet-1",
				"arn": "arn:aws:ec2:us-east-1:1:instance/i-1", "pulumi:cloudId": "i-1", "terraform:address": "web",
			},
		},
		{
			name:         "ebs volume",
			resourceType: "aws:ebs/volume:Volume",
			props:        map[string]any{"type": "gp3", "size": float64(100), "encrypted": true, "id": "vol-1"},
		},
		{
			name:         "unknown type",
			resourceType: "aws:ec2/vpc:Vpc",
			props:        map[string]any{"cidrBlock": "10.0.0.0/16", "arn": "arn:aws:ec2:vpc"},
		},
		{
			name:         "credential-bearing",
			resourceType: "aws:ec2/instance:Instance",
			props: map[string]any{
				"instanceType": "t3.micro", "password": "hunter2", "apiToken": "tok", "__defaults": []any{},
				"userData": map[string]any{"4dabf18193072939515e22adb298388d": "1", "ciphertext": "AAAA"},
				"tags":     map[string]any{"secret": "s", "Name": "web"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := &engine.ResourceDescriptor{
				ID:         "urn:ec2",
				Type:       tt.resourceType,
				Provider:   "aws",
				Properties: tt.props,
			}
			s, _, _, _ := estimateServerFor(t, &ec2EstimatePlugin{}, resource)

			resp := overviewRequest(t, s, "GET", "/api/estimate/baseline?urn=urn:ec2", "")
			require.Equal(t, http.StatusOK, resp.StatusCode)
			var web struct {
				Display viewmodel.EstimateDisplay `json:"display"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&web))
			terminal := tui.NewEstimateModel(context.Background(), resource, nil)

			assert.Equal(t, planShape(web.Display), terminalShape(terminal.Properties()))
			assert.Equal(t, web.Display.Details, terminal.Details())
			for _, hidden := range []string{"password", "apiToken", "__defaults", "userData"} {
				for _, property := range web.Display.Properties {
					assert.NotEqual(t, hidden, property.Key)
				}
				for _, row := range terminal.Properties() {
					assert.NotEqual(t, hidden, row.Key)
				}
			}
		})
	}
}

func TestEstimateUsageRowParity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		resourceType string
		props        map[string]any
		plugin       string
	}{
		{
			name: "s3 usage row", resourceType: "aws:s3/bucket:Bucket", plugin: "aws-public",
			props: map[string]any{"acl": "private"},
		},
		{
			name: "ec2 priced by another plugin", resourceType: "aws:ec2/instance:Instance", plugin: "civo-public",
			props: map[string]any{"instanceType": "t3.micro", "tenancy": "default", "password": "hunter2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resource := &engine.ResourceDescriptor{
				ID:         "urn:row",
				Type:       tt.resourceType,
				Provider:   "aws",
				Properties: tt.props,
			}
			discovery := engine.PricingDiscovery{
				Modes: []engine.PricingMode{{Plugin: tt.plugin, BillingMode: "per_hour"}},
			}
			display := viewmodel.BuildEstimateDisplay(
				context.Background(), resource, &engine.EstimateResult{}, nil,
				viewmodel.NewEstimateInputContext(discovery, ""),
			)
			terminal := tui.NewEstimateModel(context.Background(), resource, nil)
			terminal.ApplyPricingDiscovery(discovery)

			assert.Equal(t, planShape(display), terminalShape(terminal.Properties()))
			for _, property := range display.Properties {
				assert.NotEqual(t, "password", property.Key)
			}
		})
	}
}

func terminalShape(rows []tui.PropertyRow) []estimatePlanShape {
	shape := make([]estimatePlanShape, 0, len(rows))
	for _, row := range rows {
		shape = append(shape, estimatePlanShape{row.Key, row.Class, row.DefaultVisible})
	}
	return shape
}
