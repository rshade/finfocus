---
title: CI/CD Integration
description: Integrating FinFocus into CI/CD pipelines
---

You can run FinFocus in your CI/CD pipeline to enforce cost policies
and visibility.

## GitHub Actions

The [finfocus-action](https://github.com/rshade/finfocus-action) GitHub
Action installs FinFocus and its plugins, runs the analysis, and posts the
result as a pull request comment. It also supports analyzer mode, budgets,
recommendations, and sustainability metrics. To see the comment it produces,
open [the live demo pull request](https://github.com/rshade/finfocus-demo/pull/1)
in [finfocus-demo](https://github.com/rshade/finfocus-demo).

To run the CLI directly instead:

```yaml
name: Cost Estimate
on: [pull_request]

jobs:
  estimate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install FinFocus
        run: |
          curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/v0.3.1/scripts/install.sh \
            -o install.sh
          sha256sum --check <(echo "$(curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/v0.3.1/scripts/install.sh.sha256)")
          sh install.sh

      - name: Pulumi Preview
        run: pulumi preview --json > plan.json
        env:
          PULUMI_ACCESS_TOKEN: ${{ secrets.PULUMI_ACCESS_TOKEN }}

      - name: Estimate Cost
        run: finfocus cost projected --pulumi-json plan.json
```

## GitLab CI

```yaml
estimate_cost:
  stage: test
  script:
    - curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/v0.3.1/scripts/install.sh -o install.sh
    - curl -fsSL https://raw.githubusercontent.com/rshade/finfocus/v0.3.1/scripts/install.sh.sha256 -o install.sh.sha256
    - sha256sum -c install.sh.sha256
    - sh install.sh
    - pulumi preview --json > plan.json
    - finfocus cost projected --pulumi-json plan.json
```
