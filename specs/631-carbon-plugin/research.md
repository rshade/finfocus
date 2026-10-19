---
title: "Carbon Plugin Research"
description: "Pinned CCF inputs, AWS instance coverage, and integration decisions."
layout: "docs"
---

## Decision: Pin CCF Operational Inputs

Use Cloud Carbon Footprint revision
[`f584c549ee358d5980d36513267d007ecd3ee716`](https://github.com/cloud-carbon-footprint/cloud-carbon-footprint/tree/f584c549ee358d5980d36513267d007ecd3ee716).
The dataset is fixed at build time. There are no runtime provider calls.

Source paths under that revision:

- `packages/aws/src/lib/AWSInstanceTypes.ts`: instance processor groups and sizes.
- `packages/aws/src/lib/AWSMemoryEstimatesBuilder.ts`: provisioned memory model.
- `packages/aws/src/domain/AwsFootprintEstimationConstants.ts`: regional intensities,
  PUE 1.135, and memory coefficient 0.000392 kWh per GB-hour.
- `packages/core/src/memory/helpers.ts`: extra-memory allocation rules.
- `packages/core/src/FootprintEstimationConstants.ts`: processor baselines.

Retain the upstream Apache-2.0 license and NOTICE with the copied data and record
changes in `plugins/carbon/data/README.md`. CCF's complete method includes embodied
emissions; this feature implements its operational compute/memory portion only.
The issue's approximate TDP and 0.3725 W/GB examples are superseded by the pinned
CCF inputs. This avoids unsourced TDP assignments and double-counted RAM.

## Instance Coverage and Units

Primary AWS verification: [general purpose](https://docs.aws.amazon.com/ec2/latest/instancetypes/gp.html),
[compute optimized](https://docs.aws.amazon.com/ec2/latest/instancetypes/co.html),
[memory optimized](https://docs.aws.amazon.com/ec2/latest/instancetypes/mo.html), and
[T3](https://aws.amazon.com/ec2/instance-types/t3/).

- t3: nano, micro, small, medium, large, xlarge, 2xlarge. vCPU counts are
  2, 2, 2, 2, 2, 4, 8; memory GiB is 0.5, 1, 2, 4, 8, 16, 32.
- m5/r5: large, xlarge, 2xlarge, 4xlarge, 8xlarge, 12xlarge, 16xlarge,
  24xlarge, metal. vCPU counts are 2, 4, 8, 16, 32, 48, 64, 96, 96.
  Memory GiB is vCPU times 4 for m5 and times 8 for r5.
- c5: large, xlarge, 2xlarge, 4xlarge, 9xlarge, 12xlarge, 18xlarge,
  24xlarge, metal. vCPU counts are 2, 4, 8, 16, 36, 48, 72, 96, 96.
  Memory GiB is vCPU times 2.

This is 34 explicitly supported instance identifiers. Correct CCF's truncated
`t3.nano` memory value from 0 to AWS's 0.5 GiB and document that override.
CCF labels its memory coefficient GB while using the AWS GiB numeric inputs;
retain and explicitly describe that modeling convention rather than silently
converting those inputs to decimal GB.

## Operational Formula

Use a fixed 730-hour projected month and default utilization fraction 0.5.

| Processor group | Minimum W/vCPU | Maximum W/vCPU | Baseline memory per chip |
| --- | --- | --- | --- |
| Cascade Lake | 0.69 | 4.06 | 105.97 |
| Skylake | 0.61 | 4.10 | 75.1 |
| Equal average | 0.65 | 4.08 | 90.535 |

Map t3 to Cascade Lake, m5 to Skylake, and r5 to the equal average. Map
c5.12xlarge/c5.24xlarge to Cascade Lake and other supported c5 sizes to the equal
average, including c5.metal. Do not infer CPU mappings from unrelated families.

For m5/c5/r5, the largest supported family has 96 vCPUs, corresponding to one
modeled chip. Maximum family memory is 384/192/768 respectively. Additional
memory is `max(0, familyMaxMemory - baselineMemory) * instanceMemory / familyMaxMemory`.
For t3 it is zero because CCF's burstable compute model already includes RAM.

```text
wattsPerVCPU = minimum + utilization * (maximum - minimum)
monthlyKWh = (vCPUs * wattsPerVCPU / 1000 + extraMemory * 0.000392) * 730 * 1.135
monthlyGrams = monthlyKWh * regionMetricTonsPerKWh * 1000000
```

RAM is not multiplied by utilization again. Idle compute remains nonzero.
No storage, network, or embodied component is implied by this total.

## Region Coverage

Include the 38 explicit regions in the pinned AWS regional table and exclude its
`UNKNOWN` entry. Coefficients below are metric tons CO2e/kWh; multiply by 1e6
exactly once to obtain grams CO2e/kWh.

| Regions | Coefficient |
| --- | --- |
| us-east-1, us-gov-east-1 | 0.0003651277965 |
| us-east-2 | 0.0003761283186 |
| us-west-1, us-west-2, us-gov-west-1 | 0.0002986502059 |
| af-south-1 | 0.00075744 |
| ap-east-1 | 0.00067348 |
| ap-east-2 | 0.00064142 |
| ap-south-1, ap-south-2 | 0.00095182 |
| ap-northeast-1, ap-northeast-3 | 0.00043984 |
| ap-northeast-2 | 0.00047739 |
| ap-southeast-1 | 0.00049453 |
| ap-southeast-2, ap-southeast-4 | 0.00058023 |
| ap-southeast-3 | 0.00071767 |
| ap-southeast-5 | 0.00062051 |
| ap-southeast-6 | 0.00007939 |
| ap-southeast-7 | 0.00050824 |
| ca-central-1, ca-west-1 | 0.00011541 |
| cn-north-1, cn-northwest-1 | 0.0005374 |
| eu-central-1, eu-central-2 | 0.000368 |
| eu-west-1, eu-west-2 | 0.000305 |
| eu-west-3 | 0.000074 |
| eu-south-1 | 0.000297 |
| eu-south-2 | 0.000178 |
| eu-north-1 | 0.000008 |
| il-central-1 | 0.00053982 |
| me-south-1 | 0.00050549 |
| me-central-1 | 0.0003652 |
| mx-central-1 | 0.00041896 |
| sa-east-1 | 0.00006398 |

New regions without source data decline. Do not claim current all-region coverage.

## Independent Numerical Fixtures

At 730 hours and 50% utilization:

| Instance | Monthly kWh | us-east-1 grams | eu-north-1 grams |
| --- | --- | --- | --- |
| t3.medium | 3.9356125 | 1437.00152000285625 | 31.4849 |
| m5.large | 5.992639775833333333 | 2188.07935656827895125 | 47.941118206666666667 |
| c5.large | 4.605603576958333333 | 1681.6338856073144223125 | 36.844828615666666667 |
| r5.large | 8.503102776958333333 | 3104.7191803638272223125 | 68.024822215666666667 |

`t3.medium` us-east-1 at utilization 0/1: 417.4867573903035 /
2456.516282615409 grams. `m5.large` at 0/1: 1132.26139766091720125 /
3243.89731547564070125 grams. Use absolute tolerance 1e-8 grams. Tests load
expected values from a fixture, not from the production calculation.

## Decision: Enrich Outside Monetary Cache

Core uses first-success monetary selection. Keep that behavior, exclude
carbon-only clients from monetary paths, and select carbon capability separately.
Serialize monetary responses before enrichment; enrich both fresh and cached
results, including batch cache pre-checks. This avoids a second cache or a new
protocol and lets settings/data/source changes take effect immediately.

Alternatives rejected: caching the merged result adds source/settings/lifetime
coupling; routing carbon as a monetary plugin can replace a real dollar price.

## Decision: Preserve Partial Results

`GetProjectedCostWithErrors` currently discards collected rows on cancellation.
Move its cancellation return after assembling the collected result so a carbon
cancellation returns completed monetary rows with the cancellation error. Keep
regular projected-cost partial-result behavior and deterministic order.

## Existing Work Overlap

Jev rename PR #1819 remains open. Preserve its eventual Makefile, CI, release,
and catalog migration when rebasing. No scorer implementation changes belong
here. Existing SDK v0.7.6 supports carbon metadata and impact metrics, so no
finfocus-spec change or external plugin release is a prerequisite.
