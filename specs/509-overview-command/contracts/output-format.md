# Output Format Contract

**Component**: JSON/NDJSON/Table Output Schemas  
**Version**: v1.0.0  
**Date**: 2026-02-11

---

## JSON Format (`--output json`)

### Schema

Single JSON object with metadata and array of overview rows.

```json
{
  "metadata": {
    "stackName": "my-stack-dev",
    "region": "us-east-1",
    "timeWindow": {
      "start": "2026-02-01T00:00:00Z",
      "end": "2026-02-11T23:59:59Z"
    },
    "hasChanges": true,
    "totalResources": 150,
    "pendingChanges": 10,
    "generatedAt": "2026-02-11T12:34:56Z"
  },
  "resources": [
    {
      "urn": "urn:pulumi:dev::my-stack::aws:ec2/instance:Instance::web-1",
      "type": "aws:ec2/instance:Instance",
      "resourceId": "i-0123456789abcdef0",
      "status": "updating",
      "actualCost": {
        "mtdCost": 42.50,
        "currency": "USD",
        "period": {
          "start": "2026-02-01T00:00:00Z",
          "end": "2026-02-11T23:59:59Z"
        },
        "breakdown": {
          "compute": 30.00,
          "storage": 10.00,
          "network": 2.50
        }
      },
      "projectedCost": {
        "monthlyCost": 100.00,
        "currency": "USD",
        "breakdown": {
          "compute": 70.00,
          "storage": 25.00,
          "network": 5.00
        }
      },
      "recommendations": [
        {
          "resourceId": "i-0123456789abcdef0",
          "type": "RIGHTSIZE",
          "description": "Instance is underutilized (5% CPU avg). Consider downsizing to t3.medium.",
          "estimatedSavings": 50.00,
          "currency": "USD"
        }
      ],
      "costDrift": {
        "extrapolatedMonthly": 115.91,
        "projected": 100.00,
        "delta": 15.91,
        "percentDrift": 15.91,
        "isWarning": true
      },
      "error": null
    }
  ],
  "summary": {
    "totalActualMTD": 47.70,
    "projectedMonthly": 312.00,
    "projectedDelta": 264.30,
    "potentialSavings": 150.00,
    "currency": "USD"
  },
  "errors": []
}
```

### Field Descriptions

#### metadata

| Field | Type | Description |
|-------|------|-------------|
| `stackName` | string | Pulumi stack identifier |
| `region` | string | Primary cloud region (may be empty) |
| `timeWindow.start` | string (RFC3339) | Start of actual cost period |
| `timeWindow.end` | string (RFC3339) | End of actual cost period |
| `hasChanges` | boolean | Whether pending changes exist |
| `totalResources` | integer | Total resource count |
| `pendingChanges` | integer | Count of resources with changes |
| `generatedAt` | string (RFC3339) | Timestamp of report generation |

#### resources[]

| Field | Type | Description |
|-------|------|-------------|
| `urn` | string | Pulumi URN (canonical identifier) |
| `type` | string | Pulumi resource type |
| `resourceId` | string | Cloud provider resource ID |
| `status` | string | Enum: `active`, `creating`, `updating`, `deleting`, `replacing` |
| `actualCost` | object \| null | Actual cost data (null if unavailable) |
| `projectedCost` | object \| null | Projected cost data (null if no changes) |
| `recommendations` | array | Cost optimization recommendations |
| `costDrift` | object \| null | Cost drift warning (null if drift <10%) |
| `error` | object \| null | Error state (null if no error) |

#### summary

| Field | Type | Description |
|-------|------|-------------|
| `totalActualMTD` | float | Sum of all actual MTD costs |
| `projectedMonthly` | float | Sum of all projected monthly costs |
| `projectedDelta` | float | Net change in monthly spend |
| `potentialSavings` | float | Sum of recommendation savings |
| `currency` | string | Currency code (assumes all same currency) |

---

## NDJSON Format (`--output ndjson`)

### Schema

Newline-delimited JSON, one resource per line. No metadata or summary wrapper.

```json
{"urn":"urn:pulumi:dev::my-stack::aws:ec2/instance:Instance::web-1","type":"aws:ec2/instance:Instance","resourceId":"i-0123456789abcdef0","status":"updating","actualCost":{"mtdCost":42.50,"currency":"USD","period":{"start":"2026-02-01T00:00:00Z","end":"2026-02-11T23:59:59Z"},"breakdown":{"compute":30.00,"storage":10.00,"network":2.50}},"projectedCost":{"monthlyCost":100.00,"currency":"USD","breakdown":{"compute":70.00,"storage":25.00,"network":5.00}},"recommendations":[{"resourceId":"i-0123456789abcdef0","type":"RIGHTSIZE","description":"Instance is underutilized (5% CPU avg). Consider downsizing to t3.medium.","estimatedSavings":50.00,"currency":"USD"}],"costDrift":{"extrapolatedMonthly":115.91,"projected":100.00,"delta":15.91,"percentDrift":15.91,"isWarning":true},"error":null}
{"urn":"urn:pulumi:dev::my-stack::aws:s3/bucket:Bucket::logs","type":"aws:s3/bucket:Bucket","resourceId":"finfocus-logs-bucket","status":"active","actualCost":{"mtdCost":5.20,"currency":"USD","period":{"start":"2026-02-01T00:00:00Z","end":"2026-02-11T23:59:59Z"},"breakdown":{"storage":5.00,"requests":0.20}},"projectedCost":null,"recommendations":[{"resourceId":"finfocus-logs-bucket","type":"DELETE_UNUSED","description":"Bucket has no objects. Consider deletion if unused.","estimatedSavings":12.00,"currency":"USD"}],"costDrift":null,"error":null}
```

**Use Case**: Streaming processing, log aggregation, line-by-line parsing.

---

## Table Format (Plain Mode)

### ASCII Table

```
RESOURCE                          TYPE              STATUS    ACTUAL(MTD)  PROJECTED   DELTA     DRIFT%   RECS
aws:ec2/instance:Instance-web-1   aws:ec2/instance  updating  $42.50       $100.00    +$57.50   +15% ⚠️  3
aws:s3/bucket:Bucket-logs         aws:s3/bucket     active    $5.20        $12.00     +$6.80    +8%      1
aws:rds/instance:Database-main    aws:rds/instance  creating  -            $200.00    +$200.00  N/A      0

SUMMARY
-------
Total Actual (MTD):        $47.70
Projected Monthly:         $312.00
Projected Delta:           +$264.30
Potential Savings (Recs):  $150.00
```

### Column Specifications

| Column | Width | Alignment | Truncation |
|--------|-------|-----------|------------|
| RESOURCE | 30 chars | Left | Ellipsis (...) |
| TYPE | 20 chars | Left | Ellipsis (...) |
| STATUS | 10 chars | Left | Icon + text |
| ACTUAL(MTD) | 12 chars | Right | Currency format |
| PROJECTED | 12 chars | Right | Currency format |
| DELTA | 12 chars | Right | Currency format with +/- |
| DRIFT% | 8 chars | Right | Percent format + warning icon |
| RECS | 4 chars | Right | Count |

### Status Icons

| Status | Icon | Color (ANSI) |
|--------|------|--------------|
| active | ✓ | Green (32) |
| creating | + | Blue (34) |
| updating | ~ | Yellow (33) |
| deleting | - | Red (31) |
| replacing | ↻ | Magenta (35) |

### Error Indicator

If `error` is non-nil, display `❌ ERROR` in place of costs.

---

## Error Format

### Errors Array (JSON)

```json
{
  "errors": [
    {
      "urn": "urn:pulumi:dev::my-stack::aws:ec2/instance:Instance::web-2",
      "errorType": "rate_limit",
      "message": "rate limit exceeded (429)",
      "retryable": true
    },
    {
      "urn": "urn:pulumi:dev::my-stack::aws:rds/instance:Database::db-1",
      "errorType": "auth",
      "message": "authentication failed (check credentials)",
      "retryable": false
    }
  ]
}
```

### Error Types

| errorType | Description | retryable |
|-----------|-------------|-----------|
| `auth` | Authentication failure | false |
| `network` | Network/connectivity issue | true |
| `rate_limit` | API rate limit exceeded | true |
| `unknown` | Unclassified error | depends |

---

## Currency Formatting

### Rules

1. Always include currency code (USD, EUR, etc.)
2. Format with 2 decimal places
3. Use comma as thousands separator (US format)
4. Sign prefix for deltas (+/-)

### Examples

```
$1,234.56       (positive)
-$500.00        (negative)
+$123.45        (delta positive)
$0.00           (zero)
```

---

## Date Formatting

### Input Formats (Accepted)

1. **YYYY-MM-DD**: `2026-02-11`
2. **RFC3339**: `2026-02-11T12:34:56Z`

### Output Format (JSON)

Always RFC3339: `2026-02-11T12:34:56Z`

---

## Schema Validation

### JSON Schema (for automated validation)

See `testdata/overview/schema.json` for complete JSON Schema definition.

**Key Constraints**:
- `metadata.totalResources` >= 0
- `metadata.pendingChanges` >= 0 (if `hasChanges` is true)
- `actualCost.mtdCost` >= 0.0
- `projectedCost.monthlyCost` >= 0.0
- `costDrift.percentDrift` > 10.0 or < -10.0

---

## Testing Requirements

### Unit Tests

- JSON serialization/deserialization
- NDJSON line-by-line parsing
- Table column alignment and truncation
- Currency formatting edge cases (zero, negative, large numbers)

### Integration Tests

- Full output generation from fixture data
- Schema validation against JSON Schema
- Golden file comparison for table output

**Test Coverage Target**: 80% (output layer)

---

## References

- **Data Model**: `../data-model.md`
- **Existing Output**: `internal/engine/render.go`

---

**Contract Version**: v1.0.0  
**Last Updated**: 2026-02-11
