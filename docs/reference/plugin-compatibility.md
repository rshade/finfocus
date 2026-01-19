---
layout: default
title: Plugin Compatibility
parent: Reference
nav_order: 3
---

This matrix outlines the capabilities of the official FinFocus plugins, helping you choose the right plugin for your use
case.

## Official Plugins

| Plugin Name              | Provider | Projected Costs |  Actual Costs   | Auth Required | E2E Support | Notes                                                                              |
| :----------------------- | :------- | :-------------: | :-------------: | :-----------: | :---------: | :--------------------------------------------------------------------------------- |
| **aws-public**           | AWS      |       ✅        | ⚠️ (Fallback\*) |      ❌       |     ✅      | Uses embedded list prices. \*Actual costs are estimated by `runtime × list price`. |
| **aws-costexplorer**     | AWS      |       ❌        |       ✅        |      ✅       |     ✅      | Queries real billing data. Includes discounts/RIs.                                 |
| **azure-retail**         | Azure    |       ✅        |       ❌        |      ❌       |     ❌      | Uses Azure Retail Prices API.                                                      |
| **google-cloud-billing** | GCP      |       ❌        |       ✅        |      ✅       |     ❌      | Queries Google Cloud Billing API.                                                  |

## Feature Definitions

- **Projected Costs**: Can estimate monthly costs from a Pulumi plan JSON _before_ deployment.
- **Actual Costs**: Can retrieve historical spending data for deployed resources.
- **Auth Required**: Requires cloud provider credentials (API keys, service accounts) to function.
- **E2E Support**: Included in the standard FinFocus End-to-End test suite for regression testing.

## Compatibility by Resource Type

Different plugins support different subsets of cloud resources. Use the CLI to inspect specific support:

```bash
finfocus plugin inspect aws-public aws:ec2/instance:Instance
```
