# AWS Example Plugin

This is a reference implementation of a FinFocus plugin for AWS cost calculations. It demonstrates best practices and patterns for plugin development.

## Features

- **Projected Cost Calculation**: Estimates costs for AWS resources based on instance types, regions, and configurations
- **Multiple Resource Types**: Supports EC2 instances, S3 buckets, and RDS instances
- **Regional Pricing**: Applies regional pricing multipliers
- **Extensible Design**: Easy to add new resource types and pricing logic

## Supported Resources

### EC2 Instances (`aws:ec2:Instance`)

- Instance types: t3.micro, t3.small, t3.medium, t3.large, t3.xlarge, t3.2xlarge, m5.large, m5.xlarge, c5.large, c5.xlarge
- Regional pricing support
- Properties: `instanceType`, `region`

### S3 Buckets (`aws:s3:Bucket`)

- Storage classes: STANDARD, STANDARD_IA, ONEZONE_IA, GLACIER, DEEP_ARCHIVE
- Pricing per GB per month
- Properties: `storageClass`, `region`

### RDS Instances (`aws:rds:Instance`)

- Instance classes: db.t3.micro, db.t3.small, db.t3.medium, db.m5.large, db.m5.xlarge
- Engine support: mysql, postgres, oracle, sqlserver
- Properties: `instanceClass`, `engine`, `region`

## Building and Testing

### Prerequisites

- Go 1.25.6+
- FinFocus Core development environment

### Build the Plugin

```bash
# From the examples/plugins/aws-example directory
go build -o bin/finfocus-plugin-aws-example main.go
```

### Test the Plugin

```bash
# Start the plugin manually for testing
./bin/finfocus-plugin-aws-example

# In another terminal, test with FinFocus
finfocus cost projected --pulumi-json test-plan.json
```

### Install for Local Testing

```bash
# Create plugin directory structure
mkdir -p ~/.finfocus/plugins/aws-example/1.0.0

# Copy binary and manifest
cp bin/finfocus-plugin-aws-example ~/.finfocus/plugins/aws-example/1.0.0/
cp manifest.yaml ~/.finfocus/plugins/aws-example/1.0.0/plugin.manifest.json

# Verify installation
finfocus plugin list
finfocus plugin validate
```

## Example Usage

With a Pulumi plan containing AWS resources:

```json
{
  "resources": [
    {
      "type": "aws:ec2:Instance",
      "properties": {
        "instanceType": "t3.medium",
        "region": "us-east-1"
      }
    },
    {
      "type": "aws:s3:Bucket",
      "properties": {
        "storageClass": "STANDARD",
        "region": "us-east-1"
      }
    }
  ]
}
```

Calculate projected costs:

```bash
finfocus cost projected --pulumi-json plan.json
```

## Implementation Details

### Pricing Logic

The plugin uses simplified pricing data for demonstration. In a production plugin, you would:

1. **Use AWS Pricing API**: Query real-time pricing data
2. **Cache Pricing Data**: Store pricing information locally for performance
3. **Handle Spot Pricing**: Support spot instance pricing
4. **Calculate Data Transfer**: Include data transfer costs
5. **Support Reserved Instances**: Handle reserved instance pricing

### Error Handling

The plugin demonstrates proper error handling patterns:

```go
if !p.Matcher().Supports(req.Resource) {
    return nil, pluginsdk.NotSupportedError(req.Resource)
}
```

### Regional Pricing

Shows how to apply regional multipliers:

```go
regionalMultiplier := map[string]float64{
    "us-east-1":      1.0,
    "us-west-2":      1.0,
    "eu-west-1":      1.1,
    "ap-southeast-1": 1.2,
}
```

## Extending the Plugin

### Adding New Resource Types

1. Register the resource type in the constructor:

```go
base.Matcher().AddResourceType("aws:lambda:Function")
```

2. Add case in `GetProjectedCost`:

```go
case "aws:lambda:Function":
    unitPrice = p.calculateLambdaCost(req.Resource)
    billingDetail = "Lambda function cost per execution"
```

3. Implement the calculation function:

```go
func (p *AWSExamplePlugin) calculateLambdaCost(resource *pbc.ResourceDescriptor) float64 {
    // Implementation here
}
```

### Adding Actual Cost Support

To implement actual cost retrieval:

1. Set up AWS credentials and Cost Explorer client
2. Implement `GetActualCost` method with proper API calls
3. Handle AWS API errors and rate limiting

## Configuration

The plugin supports configuration through:

- **Environment Variables**: AWS credentials, regions
- **Resource Tags**: Instance types, storage classes, etc.
- **Manifest Metadata**: Plugin-specific configuration

## Testing

Use the FinFocus SDK testing utilities:

```go
func TestAWSPlugin(t *testing.T) {
    plugin := NewAWSExamplePlugin()
    testPlugin := pluginsdk.NewTestPlugin(t, plugin)

    // Test plugin name
    testPlugin.TestName("aws-example")

    // Test supported resource
    resource := pluginsdk.CreateTestResource("aws", "aws:ec2:Instance", map[string]string{
        "instanceType": "t3.micro",
        "region": "us-east-1",
    })
    testPlugin.TestProjectedCost(resource, false)
}
```

## Contributing

This example plugin serves as a template for AWS plugin development. Contributions to improve the pricing accuracy, add new resource types, or enhance the implementation are welcome.

## License

Same as FinFocus Core project.
