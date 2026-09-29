package plugin

import (
	"github.com/rshade/finfocus/internal/proto"
)

// ResponseScenario represents a pre-configured response scenario for testing.
type ResponseScenario string

const (
	// ScenarioSuccess represents a successful cost calculation scenario.
	ScenarioSuccess ResponseScenario = "success"

	// ScenarioPartialData represents a scenario with some missing cost data.
	ScenarioPartialData ResponseScenario = "partial"

	// ScenarioHighCost represents a scenario with expensive resources.
	ScenarioHighCost ResponseScenario = "high_cost"

	// ScenarioZeroCost represents a scenario with zero-cost resources.
	ScenarioZeroCost ResponseScenario = "zero_cost"

	// ScenarioMultiCurrency represents a scenario with mixed currencies.
	ScenarioMultiCurrency ResponseScenario = "multi_currency"
)

// Shared fixture constants for mock plugin responses.
const (
	// currencyUSD is the ISO 4217 currency code used for mock cost data.
	currencyUSD = "USD"

	// resourceTypeEC2Instance is the Pulumi type token for AWS EC2 instances.
	resourceTypeEC2Instance = "aws:ec2/instance:Instance"
	// resourceTypeS3Bucket is the Pulumi type token for AWS S3 buckets.
	resourceTypeS3Bucket = "aws:s3/bucket:Bucket"
	// resourceTypeRDSInstance is the Pulumi type token for AWS RDS instances.
	resourceTypeRDSInstance = "aws:rds/instance:Instance"
	// resourceTypeLambdaFunction is the Pulumi type token for AWS Lambda functions.
	resourceTypeLambdaFunction = "aws:lambda/function:Function"

	// breakdownKeyCompute is the cost breakdown key for compute charges.
	breakdownKeyCompute = "compute"
	// breakdownKeyStorage is the cost breakdown key for storage charges.
	breakdownKeyStorage = "storage"
	// breakdownKeyRequests is the cost breakdown key for request charges.
	breakdownKeyRequests = "requests"
	// breakdownKeyTotal is the cost breakdown key for total-cost rollup entries.
	breakdownKeyTotal = "total"

	// ec2MicroMonthlyCost is the t3.micro on-demand fixture price shared by the
	// success, partial-data, and multi-currency scenarios.
	ec2MicroMonthlyCost = 7.30
	// ec2MicroHourlyCost is the t3.micro on-demand hourly fixture price shared by
	// the success, partial-data, and multi-currency scenarios.
	ec2MicroHourlyCost = 0.01
)

// ConfigureScenario applies a pre-defined response scenario to the mock plugin.
// This is a convenience method for common testing scenarios.
func (m *MockPlugin) ConfigureScenario(scenario ResponseScenario) {
	m.Reset()

	switch scenario {
	case ScenarioSuccess:
		m.configureSuccessScenario()
	case ScenarioPartialData:
		m.configurePartialDataScenario()
	case ScenarioHighCost:
		m.configureHighCostScenario()
	case ScenarioZeroCost:
		m.configureZeroCostScenario()
	case ScenarioMultiCurrency:
		m.configureMultiCurrencyScenario()
	}
}

// configureSuccessScenario sets up typical successful responses for common AWS resources.
//
//nolint:mnd // One-off fixture prices (S3, RDS, Lambda) are intentionally literal data.
func (m *MockPlugin) configureSuccessScenario() {
	// EC2 t3.micro instance
	m.SetProjectedCostResponse(resourceTypeEC2Instance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: ec2MicroMonthlyCost,
		HourlyCost:  ec2MicroHourlyCost,
		Notes:       "t3.micro on-demand pricing",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: ec2MicroMonthlyCost,
		},
	})

	// S3 bucket (standard storage)
	m.SetProjectedCostResponse(resourceTypeS3Bucket, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 2.30,
		HourlyCost:  0.00315,
		Notes:       "Standard storage, 100GB",
		CostBreakdown: map[string]float64{
			breakdownKeyStorage: 2.30,
		},
	})

	// RDS db.t3.micro instance
	m.SetProjectedCostResponse(resourceTypeRDSInstance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 12.41,
		HourlyCost:  0.017,
		Notes:       "db.t3.micro single-AZ",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: 12.41,
		},
	})

	// Lambda function
	m.SetProjectedCostResponse(resourceTypeLambdaFunction, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 0.20,
		HourlyCost:  0.000274,
		Notes:       "128MB, 1M requests/month",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute:  0.17,
			breakdownKeyRequests: 0.03,
		},
	})
}

// configurePartialDataScenario simulates a scenario where some resources have no cost data.
func (m *MockPlugin) configurePartialDataScenario() {
	// Only configure some resources
	m.SetProjectedCostResponse(resourceTypeEC2Instance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: ec2MicroMonthlyCost,
		HourlyCost:  ec2MicroHourlyCost,
		Notes:       "t3.micro on-demand pricing",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: ec2MicroMonthlyCost,
		},
	})
	// aws:s3/bucket:Bucket intentionally not configured to simulate missing data
}

// configureHighCostScenario simulates expensive resources for testing cost warnings.
//
//nolint:mnd // One-off fixture prices are intentionally literal data.
func (m *MockPlugin) configureHighCostScenario() {
	// High-end EC2 instance
	m.SetProjectedCostResponse(resourceTypeEC2Instance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 2500.00,
		HourlyCost:  3.424,
		Notes:       "p3.8xlarge GPU instance",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: 2500.00,
		},
	})

	// Large RDS instance
	m.SetProjectedCostResponse(resourceTypeRDSInstance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 1200.00,
		HourlyCost:  1.644,
		Notes:       "db.r5.4xlarge multi-AZ",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: 1200.00,
		},
	})
}

// configureZeroCostScenario simulates free-tier or zero-cost resources.
func (m *MockPlugin) configureZeroCostScenario() {
	m.SetProjectedCostResponse(resourceTypeS3Bucket, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 0,
		HourlyCost:  0,
		Notes:       "Free tier eligible",
		CostBreakdown: map[string]float64{
			breakdownKeyStorage: 0,
		},
	})

	m.SetProjectedCostResponse(resourceTypeLambdaFunction, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: 0,
		HourlyCost:  0,
		Notes:       "Within free tier limits",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute:  0,
			breakdownKeyRequests: 0,
		},
	})
}

// configureMultiCurrencyScenario simulates mixed currency responses for testing aggregation.
//
//nolint:mnd // The EUR fixture price is intentionally literal data.
func (m *MockPlugin) configureMultiCurrencyScenario() {
	m.SetProjectedCostResponse(resourceTypeEC2Instance, &proto.CostResult{
		Currency:    currencyUSD,
		MonthlyCost: ec2MicroMonthlyCost,
		HourlyCost:  ec2MicroHourlyCost,
		Notes:       "US region pricing",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: ec2MicroMonthlyCost,
		},
	})

	m.SetProjectedCostResponse(resourceTypeRDSInstance, &proto.CostResult{
		Currency:    "EUR",
		MonthlyCost: 11.50,
		HourlyCost:  0.0158,
		Notes:       "EU region pricing",
		CostBreakdown: map[string]float64{
			breakdownKeyCompute: 11.50,
		},
	})
}

// ConfigureActualCostScenario sets up actual cost responses for testing historical data.
func (m *MockPlugin) ConfigureActualCostScenario(resourceID string, totalCost float64, breakdown map[string]float64) {
	m.SetActualCostResponse(resourceID, &proto.ActualCostResult{
		Currency:      currencyUSD,
		TotalCost:     totalCost,
		CostBreakdown: breakdown,
	})
}

// QuickResponse creates a simple cost response with the given values (convenience method for tests).
func QuickResponse(currency string, monthly, hourly float64) *proto.CostResult {
	return &proto.CostResult{
		Currency:    currency,
		MonthlyCost: monthly,
		HourlyCost:  hourly,
		CostBreakdown: map[string]float64{
			breakdownKeyTotal: monthly,
		},
	}
}

// QuickActualResponse creates a simple actual cost response (convenience method for tests).
func QuickActualResponse(currency string, total float64) *proto.ActualCostResult {
	return &proto.ActualCostResult{
		Currency:  currency,
		TotalCost: total,
		CostBreakdown: map[string]float64{
			breakdownKeyTotal: total,
		},
	}
}
