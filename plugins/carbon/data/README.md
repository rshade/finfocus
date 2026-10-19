---
title: "Carbon Data Provenance"
description: "Pinned Cloud Carbon Footprint model, units, and attribution."
layout: "docs"
---

## Source Snapshot

`aws.json` derives from Cloud Carbon Footprint revision
[`f584c549ee358d5980d36513267d007ecd3ee716`](https://github.com/cloud-carbon-footprint/cloud-carbon-footprint/tree/f584c549ee358d5980d36513267d007ecd3ee716).
Retained upstream [LICENSE](LICENSE) and [NOTICE](NOTICE) apply to the source data.
Cloud Carbon Footprint is copyright Thoughtworks, Inc.; changes here convert the
selected instance/region model into JSON and precompute extra-memory allocation.

Within that revision, see:

- `packages/aws/src/lib/AWSInstanceTypes.ts`: processor/instance mapping.
- `packages/aws/src/lib/AWSMemoryEstimatesBuilder.ts`: family memory allocation.
- `packages/aws/src/domain/AwsFootprintEstimationConstants.ts`: regions and PUE.
- `packages/core/src/memory/helpers.ts`: memory correction.
- `packages/core/src/FootprintEstimationConstants.ts`: processor baselines.

AWS instance capacities are verified against
[general purpose](https://docs.aws.amazon.com/ec2/latest/instancetypes/gp.html),
[compute optimized](https://docs.aws.amazon.com/ec2/latest/instancetypes/co.html),
[memory optimized](https://docs.aws.amazon.com/ec2/latest/instancetypes/mo.html),
and [T3](https://aws.amazon.com/ec2/instance-types/t3/) documentation.
`t3.nano` uses AWS's 0.5 GiB, correcting CCF's truncated zero entry.

## Formula

```text
wattsPerVCPU = minWatts + utilization * (maxWatts - minWatts)
kWh = (vCPUs * wattsPerVCPU / 1000 + extraMemory * 0.000392) * 730 * 1.135
gCO2e = kWh * regionMetricTonsPerKWh * 1000000
```

CPU utilization defaults to 0.5, the modeled month is 730 hours, and PUE is 1.135.
The memory coefficient is 0.000392 kWh per GB-hour. AWS provisioned capacities
are in GiB; CCF uses those nominal numeric capacities as GB in its memory model.
We preserve that modeling convention without silently converting to decimal GB.

The selected processor groups are Cascade Lake (0.69–4.06 W/vCPU, 105.97 baseline
memory), Skylake (0.61–4.10 W/vCPU, 75.1 baseline), or their equal average
(0.65–4.08 W/vCPU, 90.535 baseline). t3 uses Cascade Lake, m5 uses Skylake, r5
uses the average. c5.12xlarge/c5.24xlarge use Cascade Lake; other supported c5
sizes use the average.

For m5/c5/r5, maximum family memory is 384/192/768 respectively, with 96 vCPUs
and one modeled physical chip. `extraMemory` is:

```text
max(0, maximumFamilyMemory - baselineMemory) * instanceMemory / maximumFamilyMemory
```

For t3 it is zero because the compute model includes memory. This avoids adding
memory power twice. Memory power is independent of CPU utilization.

## Coverage and Updating

The data file contains all 34 documented instance sizes and the snapshot's 38
explicit regions. Regional values remain metric tons CO2e/kWh; the calculation
converts to grams exactly once. `UNKNOWN` is deliberately excluded. Unsupported
regions decline even if the upstream method has a global fallback.

To update, pin a reviewed upstream revision, check every copied coefficient and
family mapping, retain provenance and notices, and update the independent
numerical fixtures. Explain changes to methodology or coverage in the plugin
release. Do not automatically fetch data at runtime.
