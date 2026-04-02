---
title: CI/CD Integration
description: Integrating FinFocus into CI/CD pipelines
---

You can run FinFocus in your CI/CD pipeline to enforce cost policies
and visibility.

## GitHub Actions

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
