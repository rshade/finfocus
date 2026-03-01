// Package main provides an example AWS cost calculation plugin for FinFocus.
// This demonstrates how to implement a plugin that calculates projected costs for AWS resources
// including EC2 instances, S3 buckets, and RDS databases with region and engine-specific pricing.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	defaultRegion           = "us-east-1"
	defaultInstanceType     = "t3.micro"
	defaultRootVolumeType   = "gp2"
	defaultRootVolumeSizeGB = 8.0
	defaultS3StorageClass   = "STANDARD"
	defaultRDSInstanceClass = "db.t3.micro"
	defaultRDSEngine        = "mysql"

	priceT3Micro   = 0.0104
	priceT3Small   = 0.0208
	priceT3Medium  = 0.0416
	priceT3Large   = 0.0832
	priceT3XLarge  = 0.1664
	priceT32XLarge = 0.3328
	priceM5Large   = 0.096
	priceM5XLarge  = 0.192
	priceC5Large   = 0.085
	priceC5XLarge  = 0.17

	priceS3Standard    = 0.023
	priceS3StandardIA  = 0.0125
	priceS3OneZoneIA   = 0.01
	priceS3Glacier     = 0.004
	priceS3DeepArchive = 0.00099

	priceEBSGP2      = 0.10
	priceEBSGP3      = 0.08
	priceEBSIO1      = 0.125
	priceEBSIO2      = 0.125
	priceEBSST1      = 0.045
	priceEBSSC1      = 0.015
	priceEBSIOPSUnit = 0.065

	kvPairParts = 2

	priceRDST3Micro  = 0.017
	priceRDST3Small  = 0.034
	priceRDST3Medium = 0.068
	priceRDSM5Large  = 0.192
	priceRDSM5XLarge = 0.384

	multiplierNeutral      = 1.0
	multiplierOracle       = 2.0
	multiplierSQLServer    = 1.5
	multiplierRegionUS     = 1.0
	multiplierRegionEUWest = 1.1
	multiplierRegionAPSE   = 1.2
)

// AWSExamplePlugin demonstrates a basic AWS cost calculation plugin.
type AWSExamplePlugin struct {
	*pluginsdk.BasePlugin

	ec2Prices            map[string]float64
	ec2RegionMultipliers map[string]float64
	ebsVolumePrices      map[string]float64
	s3Prices             map[string]float64
	rdsPrices            map[string]float64
	rdsEngineMultipliers map[string]float64
}

// NewAWSExamplePlugin creates and returns an AWSExamplePlugin preconfigured with supported provider and resource types and populated pricing and multiplier tables for EC2, EBS, S3, and RDS.
// The plugin registers the "aws" provider and adds support for the resource types "aws:ec2:Instance", "aws:s3:Bucket", and "aws:rds:Instance".
// It returns a pointer to an AWSExamplePlugin with in-memory price maps and region/engine multipliers initialized.
func NewAWSExamplePlugin() *AWSExamplePlugin {
	base := pluginsdk.NewBasePlugin("aws-example")

	// Configure supported AWS provider
	base.Matcher().AddProvider("aws")

	// Add supported resource types
	base.Matcher().AddResourceType("aws:ec2:Instance")
	base.Matcher().AddResourceType("aws:s3:Bucket")
	base.Matcher().AddResourceType("aws:rds:Instance")

	return &AWSExamplePlugin{
		BasePlugin: base,
		ec2Prices: map[string]float64{
			"t3.micro":   priceT3Micro,
			"t3.small":   priceT3Small,
			"t3.medium":  priceT3Medium,
			"t3.large":   priceT3Large,
			"t3.xlarge":  priceT3XLarge,
			"t3.2xlarge": priceT32XLarge,
			"m5.large":   priceM5Large,
			"m5.xlarge":  priceM5XLarge,
			"c5.large":   priceC5Large,
			"c5.xlarge":  priceC5XLarge,
		},
		ec2RegionMultipliers: map[string]float64{
			"us-east-1":      multiplierRegionUS,
			"us-west-2":      multiplierRegionUS,
			"eu-west-1":      multiplierRegionEUWest,
			"ap-southeast-1": multiplierRegionAPSE,
		},
		ebsVolumePrices: map[string]float64{
			"gp2": priceEBSGP2,
			"gp3": priceEBSGP3,
			"io1": priceEBSIO1,
			"io2": priceEBSIO2,
			"st1": priceEBSST1,
			"sc1": priceEBSSC1,
		},
		s3Prices: map[string]float64{
			"STANDARD":     priceS3Standard,
			"STANDARD_IA":  priceS3StandardIA,
			"ONEZONE_IA":   priceS3OneZoneIA,
			"GLACIER":      priceS3Glacier,
			"DEEP_ARCHIVE": priceS3DeepArchive,
		},
		rdsPrices: map[string]float64{
			"db.t3.micro":  priceRDST3Micro,
			"db.t3.small":  priceRDST3Small,
			"db.t3.medium": priceRDST3Medium,
			"db.m5.large":  priceRDSM5Large,
			"db.m5.xlarge": priceRDSM5XLarge,
		},
		rdsEngineMultipliers: map[string]float64{
			"mysql":     multiplierNeutral,
			"postgres":  multiplierNeutral,
			"oracle":    multiplierOracle,
			"sqlserver": multiplierSQLServer,
		},
	}
}

// GetProjectedCost calculates projected costs for AWS resources.
func (p *AWSExamplePlugin) GetProjectedCost(
	_ context.Context,
	req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	if req == nil {
		return nil, errors.New("GetProjectedCostRequest cannot be nil")
	}
	resource := req.GetResource()
	if resource == nil {
		return nil, errors.New("resource cannot be nil")
	}
	if !p.Matcher().Supports(resource) {
		return nil, pluginsdk.NotSupportedError(resource)
	}

	var (
		unitPrice     float64
		billingDetail string
	)

	switch resource.GetResourceType() {
	case "aws:ec2:Instance":
		unitPrice, billingDetail = p.calculateEC2Cost(resource)
	case "aws:s3:Bucket":
		unitPrice = p.calculateS3Cost(resource)
		billingDetail = "S3 storage monthly cost per GB"
	case "aws:rds:Instance":
		unitPrice = p.calculateRDSCost(resource)
		billingDetail = "RDS instance hourly cost"
	default:
		return nil, pluginsdk.NotSupportedError(resource)
	}

	return p.Calculator().CreateProjectedCostResponse("USD", unitPrice, billingDetail), nil
}

// GetActualCost retrieves actual historical costs (placeholder implementation).
func (p *AWSExamplePlugin) GetActualCost(
	_ context.Context,
	req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
	if req == nil {
		return nil, errors.New("GetActualCostRequest cannot be nil")
	}
	// In a real implementation, this would call AWS Cost Explorer API
	return nil, pluginsdk.NoDataError(req.GetResourceId())
}

// calculateEC2Cost calculates EC2 instance hourly cost including root EBS storage.
func (p *AWSExamplePlugin) calculateEC2Cost(resource *pbc.ResourceDescriptor) (float64, string) {
	tags := resource.GetTags()
	if tags == nil {
		tags = map[string]string{}
	}

	instanceType := tags["instanceType"]
	if instanceType == "" {
		instanceType = defaultInstanceType
	}

	computeHourly, hasPrice := p.ec2Prices[instanceType]
	if !hasPrice {
		computeHourly = priceT3Micro
	}

	region := tags["region"]
	if region == "" {
		region = defaultRegion
	}

	if multiplier, hasMultiplier := p.ec2RegionMultipliers[region]; hasMultiplier {
		computeHourly *= multiplier
	}

	rootMonthly := p.calculateRootVolumeMonthlyCost(tags, region)
	rootHourly := rootMonthly / float64(pluginsdk.HoursPerMonth)
	totalHourly := computeHourly + rootHourly

	billingDetail := fmt.Sprintf(
		"EC2 instance hourly cost (compute=$%.4f/hr, rootVolume=$%.2f/mo)",
		computeHourly,
		rootMonthly,
	)

	return totalHourly, billingDetail
}

// calculateRootVolumeMonthlyCost calculates monthly EBS root volume cost from tags.
func (p *AWSExamplePlugin) calculateRootVolumeMonthlyCost(tags map[string]string, region string) float64 {
	volumeType := defaultRootVolumeType
	volumeSizeGB := defaultRootVolumeSizeGB
	iops := 0.0

	volumeType, volumeSizeGB, iops = applyRootBlockDefaults(tags, volumeType, volumeSizeGB, iops)

	if volumeTypeTag := strings.ToLower(tags["rootVolumeType"]); volumeTypeTag != "" {
		volumeType = volumeTypeTag
	}
	if volumeSizeTag := parsePositiveFloat(tags["rootVolumeSize"]); volumeSizeTag > 0 {
		volumeSizeGB = volumeSizeTag
	}
	if iopsTag := parsePositiveFloat(tags["rootVolumeIops"]); iopsTag > 0 {
		iops = iopsTag
	}

	perGBMonthly, hasPrice := p.ebsVolumePrices[volumeType]
	if !hasPrice {
		perGBMonthly = priceEBSGP2
	}

	monthly := perGBMonthly * volumeSizeGB
	if volumeType == "io1" || volumeType == "io2" {
		monthly += iops * priceEBSIOPSUnit
	}

	if multiplier, hasMultiplier := p.ec2RegionMultipliers[region]; hasMultiplier {
		monthly *= multiplier
	}

	return monthly
}

func applyRootBlockDefaults(
	tags map[string]string, volumeType string, volumeSizeGB, iops float64,
) (string, float64, float64) {
	rootBlock := tags["rootBlockDevice"]
	if rootBlock == "" {
		return volumeType, volumeSizeGB, iops
	}
	rootFields, ok := parseRootBlockDevice(rootBlock)
	if !ok {
		return volumeType, volumeSizeGB, iops
	}
	if parsedType := strings.ToLower(rootFields["volumeType"]); parsedType != "" {
		volumeType = parsedType
	}
	if parsedSize := parsePositiveFloat(rootFields["volumeSize"]); parsedSize > 0 {
		volumeSizeGB = parsedSize
	}
	iops = parsePositiveFloat(rootFields["iops"])
	return volumeType, volumeSizeGB, iops
}

func parseRootBlockDevice(raw string) (map[string]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, false
	}

	// Try JSON format first (e.g., {"volumeType":"gp3","volumeSize":"100"}).
	if strings.HasPrefix(trimmed, "{") {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &jsonMap); err == nil && len(jsonMap) > 0 {
			fields := make(map[string]string, len(jsonMap))
			for k, v := range jsonMap {
				fields[k] = fmt.Sprintf("%v", v)
			}
			return fields, true
		}
	}

	// Fall back to Go-style map[key:val ...] format.
	if !strings.HasPrefix(trimmed, "map[") || !strings.HasSuffix(trimmed, "]") {
		return nil, false
	}

	body := strings.TrimSuffix(strings.TrimPrefix(trimmed, "map["), "]")
	if body == "" {
		return nil, false
	}

	fields := make(map[string]string)
	for _, token := range strings.Fields(body) {
		parts := strings.SplitN(token, ":", kvPairParts)
		if len(parts) != kvPairParts {
			continue
		}
		fields[parts[0]] = parts[1]
	}

	if len(fields) == 0 {
		return nil, false
	}
	return fields, true
}

func parsePositiveFloat(raw string) float64 {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

// calculateS3Cost calculates S3 storage cost (per GB monthly).
func (p *AWSExamplePlugin) calculateS3Cost(resource *pbc.ResourceDescriptor) float64 {
	tags := resource.GetTags()
	if tags == nil {
		tags = map[string]string{}
	}

	storageClass := tags["storageClass"]
	if storageClass == "" {
		storageClass = defaultS3StorageClass
	}

	price, hasPrice := p.s3Prices[storageClass]
	if !hasPrice {
		price = priceS3Standard
	}

	return price
}

// calculateRDSCost calculates RDS instance cost based on instance class and engine.
func (p *AWSExamplePlugin) calculateRDSCost(resource *pbc.ResourceDescriptor) float64 {
	tags := resource.GetTags()
	if tags == nil {
		tags = map[string]string{}
	}

	instanceClass := tags["instanceClass"]
	if instanceClass == "" {
		instanceClass = defaultRDSInstanceClass
	}

	price, hasPrice := p.rdsPrices[instanceClass]
	if !hasPrice {
		price = priceRDST3Micro
	}

	engine := tags["engine"]
	if engine == "" {
		engine = defaultRDSEngine
	}

	if multiplier, hasMultiplier := p.rdsEngineMultipliers[engine]; hasMultiplier {
		price *= multiplier
	}

	return price
}

// main starts the AWS example plugin, sets up signal-based graceful shutdown, and serves it on an OS-assigned port.
// It registers handlers for SIGINT and SIGTERM to cancel the server context and exits on serve errors.
func main() {
	// Create the plugin implementation
	plugin := NewAWSExamplePlugin()

	// Set up context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Start serving the plugin
	config := pluginsdk.ServeConfig{
		Plugin: plugin,
		Port:   0, // Let the system choose a port
	}

	log.Printf("Starting %s plugin...", plugin.Name())
	if err := pluginsdk.Serve(ctx, config); err != nil {
		cancel()
		log.Printf("Failed to serve plugin: %v", err)
		os.Exit(1)
	}
}
