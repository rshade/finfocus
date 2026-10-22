---
title: "Carbon Estimation Plugin"
description: "Estimate operational AWS EC2 carbon alongside monetary costs."
layout: "docs"
---

## Overview

The carbon plugin estimates operational emissions for [Amazon EC2](https://aws.amazon.com/ec2/) t3, m5, c5, and r5
instances using a pinned [Cloud Carbon Footprint](https://www.cloudcarbonfootprint.org/) dataset. It works offline without
AWS credentials. Core adds carbon to the monetary result while retaining its
amount, currency, source, and resource count.

## Install and Run

From the FinFocus repository root:

```bash
make install-carbon
finfocus plugin list
finfocus cost projected --pulumi-json plan.json
```

The plugin releases independently under `carbon-vX.Y.Z`; catalog installation
requires the first release's downloadable assets. Until then, build from source.
Install a monetary pricing plugin or configure local pricing specifications for
dollar estimates. A carbon-only installation still reports `NO_COST_DATA` for
missing monetary pricing.

## Configure Utilization

```bash
FINFOCUS_CARBON_UTILIZATION=0.25 finfocus cost projected --pulumi-json plan.json
```

The default is 0.5. Values must be finite fractions in [0, 1]. The plugin advertises
`utilization` in its settings metadata too. Zero utilization retains idle compute
power, and provisioned memory is not scaled by CPU utilization.

## Interpret Results

The canonical `carbon_footprint` metric contains a monthly total in `gCO2e`, based
on 730 hours. At the default utilization, t3.medium in us-east-1 estimates
1437.00152 grams CO2e/month. Existing output converts qualifying totals into
[carbon equivalencies](../guides/greenops-equivalencies.md). A monetary source's existing carbon takes precedence;
carbon failures do not discard successful monetary prices.

Carbon enrichment runs again on monetary cache hits, so changed assumptions take
effect on the next query. It never turns missing pricing into a free-resource
claim or duplicates resource rows.

## Coverage and Limitations

There are 34 supported sizes and 38 regions in the pinned snapshot. Unknown
sizes/regions are declined explicitly. These are operational compute and memory
estimates, not measurements or total lifecycle emissions. They exclude storage,
networking, embodied emissions, actual billing data, other clouds, and Kubernetes
workload allocation.

See the [plugin README](https://github.com/rshade/finfocus/tree/main/plugins/carbon)
for exact supported sizes, installation paths, and development commands, and
[data provenance](https://github.com/rshade/finfocus/tree/main/plugins/carbon/data)
for the formula, unit conventions, coefficient revision, and source attribution.
The underlying approach is described by
[Cloud Carbon Footprint](https://www.cloudcarbonfootprint.org/docs/methodology/).
