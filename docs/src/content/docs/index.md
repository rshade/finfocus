---
title: FinFocus Documentation
description: Cost visibility for Pulumi infrastructure. Calculate projected and actual costs.
template: splash
hero:
  tagline: Cost visibility for Pulumi infrastructure. Calculate projected and actual costs.
  image:
    file: ../../assets/logo-readme.png
  actions:
    - text: Get Started
      link: /finfocus/getting-started/quickstart/
      icon: right-arrow
    - text: Read the Docs
      link: /finfocus/user-guide/
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';

## Discover FinFocus

<CardGrid stagger>
  <Card title="End User" icon="rocket">
    **See costs in 5 minutes**<br/>
    [Quickstart](/finfocus/getting-started/quickstart/) |
    [Install](/finfocus/installation/) |
    [CLI Commands](/finfocus/commands/) |
    [FAQ](/finfocus/support/faq/)
  </Card>
  <Card title="Engineer/Developer" icon="puzzle">
    **Build a plugin**<br/>
    [Developer Guide](/finfocus/guides/developer-guide/) | [Plugin Dev](/finfocus/plugins/plugin-development/) | [SDK Reference](/finfocus/plugins/plugin-sdk/)
  </Card>
  <Card title="Software Architect" icon="setting">
    **Integrate with our system**<br/>
    [Architecture](/finfocus/architecture/system-overview/) | [Deployment](/finfocus/deployment/deployment/) | [Security](/finfocus/deployment/security/)
  </Card>
  <Card title="Business/CEO" icon="approve-check-circle">
    **Understand the value**<br/>
    [Value Prop](/finfocus/guides/business-value/) | [Roadmap](/finfocus/architecture/roadmap/)
  </Card>
</CardGrid>

## How It Works

1. You define infrastructure with Pulumi
2. FinFocus reads your Pulumi definitions
3. Plugins fetch pricing and cost data
4. FinFocus calculates and displays results
