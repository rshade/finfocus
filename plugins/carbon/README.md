---
title: "Carbon Estimation Plugin"
description: "Offline AWS EC2 operational emissions alongside monetary pricing."
layout: "docs"
---

The carbon plugin estimates monthly operational emissions for supported AWS EC2
instances. Core adds its carbon measurement to the existing monetary result,
without replacing the price or creating another resource row. It requires no AWS
credentials, provider requests, or prior local state.

## Install from Source

Run from the repository root:

```bash
make build-carbon
make install-carbon
finfocus plugin list
```

The installed version defaults to `v0.0.0` until the first carbon release.
The module releases independently under `carbon-vX.Y.Z` tags. Catalog installation
will work once the first release has downloadable assets.

`CARBON_INSTALL_DIR` can select another plugin home, for example:

```bash
make install-carbon CARBON_INSTALL_DIR=/tmp/carbon-home/plugins/carbon/v0.0.0
FINFOCUS_HOME=/tmp/carbon-home finfocus plugin list
```

## Estimate Carbon

Install a monetary pricing plugin or configure local pricing specifications too,
then use the normal projected-cost command:

```bash
finfocus cost projected --pulumi-json plan.json
```

The response includes `carbon_footprint` in `gCO2e` for a modeled 730-hour month.
For example, t3.medium in us-east-1 at 50% CPU utilization is approximately
1437.00152 grams CO2e/month. Existing output displays carbon equivalencies above
its normal threshold. The unit is `gCO2e`, because the value is already a monthly
total; `/month` is not part of the protocol's mass-unit string.

A carbon-only installation retains `NO_COST_DATA` for missing monetary prices.
Zero dollar transport fields from the plugin do not mean the resource is free.
Carbon failures preserve the monetary estimate and add an explanatory note.
If monetary pricing already supplies carbon, that measurement takes precedence.

## Utilization

The default is a 0.5 fraction. Set a finite fraction in [0, 1]:

```bash
FINFOCUS_CARBON_UTILIZATION=0.25 finfocus cost projected --pulumi-json plan.json
```

The plugin also advertises the optional `utilization` setting for FinFocus plugin
settings configuration. Zero utilization still includes idle power; provisioned
memory power is not scaled by CPU utilization. Invalid configuration stops startup
with an actionable error.

Monetary cache hits receive fresh carbon. The enrichment itself is not persisted
in that cache, so updated plugin settings or data take effect on the next query.
The plugin advertises carbon capability only and uses `GetProjectedCost` as its
transport, following Kubernetes and Jev's nested-module/BasePlugin/Info/Serve
conventions. It does not provide monetary pricing or actual emissions.

## Coverage

There are 34 supported identifiers:

- t3: nano, micro, small, medium, large, xlarge, 2xlarge.
- m5 and r5: large, xlarge, 2xlarge, 4xlarge, 8xlarge, 12xlarge, 16xlarge,
  24xlarge, metal.
- c5: large, xlarge, 2xlarge, 4xlarge, 9xlarge, 12xlarge, 18xlarge,
  24xlarge, metal.

The pinned snapshot has 38 explicit regional coefficients, listed in
[data/aws.json](data/aws.json). A size or region outside the dataset is declined;
there is no global-average fallback or extrapolated instance specification.

## Methodology and Limits

See [data provenance and formula](data/README.md) for the pinned
[Cloud Carbon Footprint](https://www.cloudcarbonfootprint.org/docs/methodology/)
model, AWS instance references, memory convention, and upstream attribution.
These are operational compute and memory estimates, not measurements or total
lifecycle emissions. Storage, networking, embodied emissions, historical billing,
other clouds, and Kubernetes workload carbon allocation are excluded.

## Development

```bash
make test-carbon
make lint-carbon
make check-plugin-boundaries
```

The plugin is a separate Go module pinned to the root finfocus-spec version. It
must not import core packages. Numerical tests use independent reference values;
conformance and installation tests exercise the built binary.
