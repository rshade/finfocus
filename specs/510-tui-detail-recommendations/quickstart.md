# Quickstart: TUI Detail View Recommendations

This feature enables viewing optimization suggestions directly within the resource
detail view for both projected and actual cost commands.

## Usage

### Interactive TUI

1. Run `cost projected` or `cost actual` in interactive mode:

   ```bash
   finfocus cost projected --pulumi-json plan.json
   finfocus cost actual --pulumi-json plan.json --from 2025-01-01
   ```

2. Select a resource row using arrow keys and press **Enter**.

3. View the **RECOMMENDATIONS** section (if available) between the sustainability
   metrics and notes sections. Recommendations are sorted by highest savings first.

4. Press **Esc** to return to the resource list.

## Example TUI Output

```text
┌──────────────────────────────────────────────────────────────────────┐
│ RESOURCE DETAIL                                                      │
│                                                                      │
│ Resource ID:   my-instance                                           │
│ Type:          aws:ec2/instance:Instance                             │
│ Provider:      aws                                                   │
│                                                                      │
│ Monthly Cost:  $15.00 USD                                            │
│ Hourly Cost:   $0.0205 USD                                           │
│                                                                      │
│ BREAKDOWN                                                            │
│ - compute: $15.0000                                                  │
│                                                                      │
│ RECOMMENDATIONS                                                      │
│ - [MIGRATE] Switch to Graviton (ARM64) instance ($8.00 USD/mo)       │
│     ⚠ Ensure application compatibility with ARM64 architecture       │
│ - [RIGHTSIZE] Switch to t3.small ($5.00 USD/mo savings)              │
│ - [TERMINATE] Resource is idle during weekends                       │
│                                                                      │
│ NOTES                                                                │
│ ...                                                                  │
└──────────────────────────────────────────────────────────────────────┘
```

## JSON Output

To see recommendations in machine-readable format:

```bash
finfocus cost projected --pulumi-json plan.json --output json
```

Recommendations appear in the `recommendations` array for each resource:

```json
{
  "resourceType": "aws:ec2/instance:Instance",
  "resourceId": "my-instance",
  "monthly": 15.00,
  "currency": "USD",
  "recommendations": [
    {
      "type": "MIGRATE",
      "description": "Switch to Graviton (ARM64) instance",
      "estimatedSavings": 8.00,
      "currency": "USD",
      "reasoning": ["Ensure application compatibility with ARM64 architecture"]
    },
    {
      "type": "RIGHTSIZE",
      "description": "Switch to t3.small",
      "estimatedSavings": 5.00,
      "currency": "USD"
    }
  ]
}
```

## Behavior Notes

- Recommendations only appear when plugins provide them for a resource.
- If no recommendations exist, the section is omitted entirely (no empty section).
- Recommendation fetch failures are logged but never block cost display.
- Dismissed/snoozed recommendations are automatically excluded.
