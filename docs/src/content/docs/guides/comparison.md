---
title: Competitive Comparison
---

FinFocus is designed specifically for the [Pulumi](https://www.pulumi.com/) ecosystem,
offering unique advantages over generic cost estimation tools such as
[Infracost](https://www.infracost.io/).

## Feature Comparison

| Feature                 | FinFocus         | Infracost                | Cloud Provider Calculators |
| :---------------------- | :--------------- | :----------------------- | :------------------------- |
| **Pulumi Native**       | ✅ First-class   | ⚠️ Terraform translation | ❌ None                    |
| **Plugin Architecture** | ✅ Extensible    | ❌ Monolithic            | ❌ N/A                     |
| **Actual Costs**        | ✅ Vantage, etc. | ❌ Estimate only         | ✅ Historical              |
| **Local Specs**         | ✅ Air-gapped    | ⚠️ Requires API          | ❌ Online only             |
| **Open Source**         | ✅ Apache 2.0    | ✅ Apache 2.0            | ❌ Proprietary             |

## Why FinFocus?

### 1. Built for Pulumi

Unlike tools that treat Pulumi as a second-class citizen (often by converting to
Terraform HCL first), FinFocus understands Pulumi's resource model natively.
This leads to higher accuracy and better support for Pulumi-specific features
like ComponentResources.

### 2. Extensible Plugin System

FinFocus uses a gRPC-based plugin system. This means you can write your own
pricing source or cost policy engine in any language, without waiting for the
core team to implement it.

### 3. Unified View (Projected vs. Actual)

FinFocus aims to close the loop between what you _think_ you will spend
(Projected) and what you _actually_ spend (Actual). By integrating with
providers like [Vantage](../plugins/vantage.md) or
[AWS Cost Explorer](https://docs.aws.amazon.com/cost-management/latest/userguide/ce-what-is.html)
(via plugins), you get a complete
financial picture.
