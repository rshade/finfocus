---
layout: default
title: Cost Calculations
parent: Guides
nav_order: 4
---

FinFocus supports two primary workflows for analyzing infrastructure costs: **Projected Costs** (pre-deployment) and
**Actual Costs** (post-deployment).

## 1. Projected Costs (Pre-Deployment)

The projected cost workflow estimates monthly spending based on the resources defined in your Pulumi plan, before they are
created in the cloud.

### Workflow

1. **Parse Plan**: FinFocus reads the JSON output from `pulumi preview --json`.
2. **Map Resources**: It identifies cloud resources (e.g., `aws:ec2/instance`) and extracts relevant properties (Type,
   Region, Size).
3. **Route to Plugin**: The system routes the resource data to the appropriate plugin (e.g., `aws-public`).
4. **Lookup Pricing**: The plugin queries its pricing database (usually public list prices) to find a match.
5. **Calculate**: Monthly costs are calculated assuming 730 hours/month of usage (on-demand rates).
6. **Report**: The Core engine aggregates these estimates into a summary table.

### Use Cases

- Budget approval before deployment.
- Comparing costs between different instance types or regions.
- CI/CD checks to prevent cost spikes.

## 2. Actual Costs (Post-Deployment)

The actual cost workflow retrieves historical spending data for resources that have already been deployed and are running.

### Workflow

1. **Parse Plan**: FinFocus parses the Pulumi plan to identify the _specific_ resource IDs (e.g.,
   `i-0123456789abcdef0`) of deployed resources.
2. **Route to Plugin**: The request is routed to a production-grade plugin (e.g., `aws-costexplorer`).
3. **Query Provider API**: The plugin uses the cloud provider's API to fetch billing data for those specific resource
   IDs over a specified time range.
4. **Return Data**: The plugin returns the exact spend, including taxes, credits, and discounts (RIs/Savings Plans).
5. **Report**: The Core engine presents the historical spend data.

### Use Cases

- Chargeback/showback reports.
- Validating that reserved instances are being applied.
- Monitoring daily spend trends.

## Key Differences

| Feature         | Projected Costs           | Actual Costs                |
| :-------------- | :------------------------ | :-------------------------- |
| **Timing**      | Before Deployment         | After Deployment            |
| **Data Source** | Public List Prices (MSRP) | Billing API (Real Invoices) |
| **Accuracy**    | Estimate                  | Precise                     |
| **Auth**        | Usually None              | Required (API Keys)         |
| **Plugin**      | `aws-public`              | `aws-costexplorer`          |
