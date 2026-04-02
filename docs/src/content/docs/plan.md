---
title: Documentation Architecture Plan
---
**Last Updated:** 2025-10-29
**Status:** In Development
**Audience:** Technical Content Architects, Product Managers, Stakeholders

---

## Table of Contents

1. [Overview](#overview)
2. [Documentation Goals](#documentation-goals)
3. [Directory Structure](#directory-structure)
4. [Audience-Specific Approaches](#audience-specific-approaches)
5. [Content Strategy by Audience](#content-strategy-by-audience)
6. [Plugin Documentation Model](#plugin-documentation-model)
7. [GitHub Pages Setup](#github-pages-setup)
8. [GitHub Actions Pipeline](#github-actions-pipeline)
9. [Linting & Quality Tools](#linting--quality-tools)
10. [LLM-Friendly Documentation](#llm-friendly-documentation)
11. [Maintenance & Updates](#maintenance--updates)
12. [Implementation Timeline](#implementation-timeline)

---

## Overview

FinFocus documentation serves four distinct audiences:

- **End Users**: Install, configure, and use FinFocus CLI
- **Engineers**: Extend system, build plugins, contribute code
- **Software Architects**: Design integrations, understand system design, operational concerns
- **Business Stakeholders (CEO/Product)**: Cost visibility, ROI, competitive advantage, roadmap

Each audience requires tailored content that speaks their language and addresses their specific
concerns. The documentation system is designed to be:

- **Modular**: Plugin-agnostic structure supports Vantage now, Kubecost/Flexera/Cloudability in future
- **Progressive**: Start simple, deepen understanding with detailed guides
- **LLM-Friendly**: Structured for both human and AI consumption
- **Maintainable**: Automated linting and validation prevents documentation drift
- **Scalable**: Support multiple plugins and deployment models without duplication

---

## Documentation Goals

### Primary Goals

1. **Reduce Support Load** - Comprehensive FAQ, troubleshooting, and examples answer 80% of questions
2. **Enable Self-Service** - Users can install, configure, and troubleshoot without support intervention
3. **Empower Plugin Developers** - FinOps services can build and maintain plugins independently
4. **Drive Adoption** - Clear business value and easy onboarding increase user base
5. **Maintain Quality** - Automated validation catches errors, inconsistencies, and broken links

### Secondary Goals

1. **Build Community** - Clear contribution guidelines encourage community contributions
2. **Reduce Training Time** - Well-organized guides reduce onboarding time for new team members
3. **Improve SEO** - High-quality docs improve search visibility and organic traffic
4. **Support Future Scale** - Modular structure handles growth to 10+ plugins without major refactoring

---

## Directory Structure

```text
docs/
├── _config.yml                      # Jekyll configuration for GitHub Pages
├── .gitignore                       # Ignore generated files
├── plan.md                          # THIS FILE - documentation architecture
├── llms.txt                         # LLM-friendly index of all documentation
├── README.md                        # Documentation home page
├── CNAME                            # GitHub Pages custom domain (if used)
│
├── _includes/                       # Jekyll templates (shared across pages)
│   ├── nav.html                     # Navigation sidebar
│   ├── audience-selector.html       # Audience toggle on home
│   └── breadcrumbs.html             # Breadcrumb navigation
│
├── _layouts/                        # Jekyll layouts
│   ├── default.html                 # Base layout
│   ├── guide.html                   # Guide layout with TOC
│   └── plugin.html                  # Plugin-specific layout
│
├── guides/                          # Audience-specific guides
│   ├── README.md                    # Guides index
│   ├── user-guide.md                # For end users: "How do I use this?"
│   ├── developer-guide.md           # For engineers: "How do I extend this?"
│   ├── architect-guide.md           # For architects: "How is this designed?"
│   ├── business-value.md            # For CEO/product: "What problem does this solve?"
│   └── comparison.md                # Comparison with existing solutions
│
├── getting-started/                 # Quick onboarding
│   ├── README.md                    # Getting started index
│   ├── installation.md              # Installation instructions
│   ├── quickstart.md                # First 5-minute example
│   ├── prerequisites.md             # System requirements
│   └── examples/
│       ├── README.md
│       ├── vantage-setup.md         # "How to set up with Vantage" (with screenshots)
│       ├── local-pricing.md         # "How to use without external service"
│       └── multi-plugin.md          # "How to use multiple plugins together"
│
├── architecture/                    # Deep dive into system design
│   ├── README.md                    # Architecture index
│   ├── system-overview.md           # High-level architecture with diagrams
│   ├── core-concepts.md             # Key concepts: resources, costs, providers
│   ├── plugin-protocol.md           # gRPC plugin protocol specification
│   ├── cost-calculation.md          # How costs are calculated and aggregated
│   ├── actual-vs-projected.md       # Actual costs vs projected costs
│   ├── roadmap.md                   # Planned features and timeline
│   │
│   └── diagrams/                    # Mermaid diagrams (embedded in docs)
│       ├── system-architecture.md   # High-level system diagram
│       ├── data-flow.md             # Data flow between components
│       ├── plugin-lifecycle.md      # Plugin startup/shutdown lifecycle
│       ├── cost-calculation-flow.md # Cost calculation pipeline
│       └── integration-example.md   # End-to-end integration (Pulumi → FinFocus → Vantage)
│
├── plugins/                         # Plugin-focused documentation
│   ├── README.md                    # Plugins index & overview
│   ├── plugin-development.md        # "How to build a plugin" (for FinOps engineers)
│   ├── plugin-sdk.md                # SDK reference and API documentation
│   ├── plugin-checklist.md          # Plugin completeness checklist
│   ├── plugin-examples.md           # Code examples for common patterns
│   │
│   ├── vantage/                     # Vantage plugin documentation (IN PROGRESS)
│   │   ├── README.md                # Overview
│   │   ├── setup.md                 # Setup & configuration
│   │   ├── authentication.md        # API key management
│   │   ├── features.md              # What's supported
│   │   ├── troubleshooting.md       # Common issues
│   │   └── cost-mapping.md          # How Vantage costs map to FinFocus
│   │
│   ├── kubecost/                    # Kubecost plugin documentation (PLANNED)
│   │   ├── README.md
│   │   ├── coming-soon.md           # Timeline and features
│   │   └── differences.md           # How it differs from Vantage
│   │
│   ├── flexera/                     # Flexera plugin documentation (FUTURE)
│   │   └── coming-soon.md           # Timeline and customer requirements
│   │
│   └── cloudability/                # Cloudability plugin documentation (FUTURE)
│       └── coming-soon.md           # Timeline and customer requirements
│
├── reference/                       # API & CLI reference
│   ├── README.md                    # Reference index
│   ├── cli-commands.md              # CLI command reference with examples
│   ├── cli-flags.md                 # Detailed flag documentation
│   ├── config-reference.md          # Configuration file reference
│   ├── api-reference.md             # gRPC API (Protocol Buffer reference)
│   ├── error-codes.md               # Error codes and solutions
│   └── environment-variables.md     # Environment variable reference
│
├── deployment/                      # Installation & deployment guides
│   ├── README.md                    # Deployment index
│   ├── installation.md              # Detailed installation guide
│   ├── configuration.md             # Configuration how-to
│   ├── docker.md                    # Docker setup & deployment
│   ├── kubernetes.md                # Kubernetes deployment (FUTURE)
│   ├── cicd-integration.md          # CI/CD pipeline integration examples
│   ├── security.md                  # Security best practices
│   └── troubleshooting.md           # Deployment troubleshooting
│
├── support/                         # Help & community
│   ├── README.md                    # Support index
│   ├── faq.md                       # Frequently asked questions
│   ├── troubleshooting.md           # Troubleshooting guide (by error/symptom)
│   ├── contributing.md              # How to contribute
│   ├── code-of-conduct.md           # Community code of conduct
│   └── support-channels.md          # Where to get help (GitHub, Discord, email)
│
├── blog/                            # Optional: Blog for announcements & tutorials
│   └── README.md
│
└── assets/                          # Images, logos, screenshots
    ├── logos/
    ├── screenshots/
    ├── diagrams/
    └── icons/
```

---

## Audience-Specific Approaches

### End Users

**Goal:** "I have a Pulumi infrastructure. How do I see what it costs?"

**Entry Point:** `getting-started/quickstart.md`

**Key Workflows:**

1. Install FinFocus CLI
2. Choose cost provider (Vantage, local pricing, or future Kubecost)
3. Run command to get costs
4. Understand output and troubleshoot issues

**Content Characteristics:**

- **Short and practical**: Every page answers one question
- **Copy-paste ready**: Include complete commands and configuration examples
- **Screenshots**: Visual guides for configuration steps
- **Troubleshooting emphasis**: Common errors and solutions
- **Less theory**: Focus on "how" not "why"

**Key Pages:**

- Installation.md → Quick start.md → Examples (Vantage setup)
- Reference/CLI commands → Support/FAQ
- Support/Troubleshooting

---

### Engineers (Plugin Developers)

**Goal:** "How do I build a FinFocus plugin for our cost service?"

**Entry Point:** `plugins/plugin-development.md`

**Key Workflows:**

1. Understand plugin protocol and gRPC
2. Set up development environment
3. Implement plugin interface
4. Test and validate plugin
5. Deploy and monitor

**Content Characteristics:**

- **Code-heavy**: Reference real example implementations
- **API-first**: Focus on gRPC protocol and data structures
- **Testing emphasis**: How to test plugins
- **Performance**: Timeout handling, error recovery
- **Modular and reusable**: Clear patterns for common tasks

**Key Pages:**

- Architecture/plugin-protocol.md → Plugins/plugin-development.md
- Plugins/plugin-sdk.md (reference)
- Plugins/plugin-examples.md (patterns)
- Plugins/vantage/ (real implementation example)

---

### Software Architects

**Goal:** "How does FinFocus integrate with our infrastructure? What are the design considerations?"

**Entry Point:** `guides/architect-guide.md`

**Key Workflows:**

1. Understand system architecture and design patterns
2. Plan integration with existing infrastructure
3. Design for scale, reliability, and security
4. Plan upgrade and migration strategies

**Content Characteristics:**

- **Design-focused**: Why architectural decisions were made
- **Integration patterns**: How to integrate with other systems
- **Operational concerns**: Deployment, monitoring, scaling
- **Trade-offs**: When to use which approach
- **Security & compliance**: Data handling, authentication, audit trails

**Key Pages:**

- Architecture/ (all files) - system design deep dive
- Deployment/ (all files) - operational guidance
- Architecture/diagrams/ (visual design)
- Support/troubleshooting.md (operational guide)

---

### Business Stakeholders (CEO/Product)

**Goal:** "What value does FinFocus provide? What's our competitive advantage?"

**Entry Point:** `guides/business-value.md`

**Key Workflows:**

1. Understand cost visibility benefits
2. See ROI and time-to-value
3. Understand roadmap and competitive positioning
4. Make go/no-go decisions

**Content Characteristics:**

- **Problem-focused**: Real customer problems solved
- **ROI emphasis**: Cost savings, time savings, risk reduction
- **Competitive positioning**: How we differ from existing solutions
- **Roadmap transparency**: What's coming and when
- **Case studies**: Real examples of value delivered
- **Less technical**: Focus on business outcomes

**Key Pages:**

- guides/business-value.md (primary)
- guides/comparison.md (competitive analysis)
- architecture/roadmap.md (future direction)
- getting-started/quickstart.md (time-to-value demo)

---

## Content Strategy by Audience

### 1. End User Documentation

**Homepage/Entry Point:** `getting-started/quickstart.md`

**User Journey:**

```text
README.md
  ↓
getting-started/quickstart.md (5-minute example)
  ├─→ installation.md (detailed install)
  ├─→ examples/vantage-setup.md (Vantage example)
  └─→ examples/local-pricing.md (no external service)
  ↓
reference/cli-commands.md (command reference)
  ↓
support/faq.md or support/troubleshooting.md
```

**Key Sections:**

- Installation and prerequisites
- Quick start example
- Configuration options
- Common use cases (with copy-paste examples)
- Troubleshooting guide
- FAQ

**Tone:** Friendly, helpful, practical

---

### 2. Engineer/Plugin Developer Documentation

**Homepage/Entry Point:** `plugins/plugin-development.md`

**Developer Journey:**

```text
README.md
  ↓
guides/developer-guide.md (overview)
  ↓
architecture/plugin-protocol.md (understand protocol)
  ↓
plugins/plugin-development.md (build guide)
  ├─→ plugins/plugin-sdk.md (API reference)
  ├─→ plugins/vantage/ (example implementation)
  └─→ plugins/plugin-examples.md (code patterns)
  ↓
deployment/docker.md or cicd-integration.md (deploy plugin)
  ↓
support/contributing.md (contribute back)
```

**Key Sections:**

- Plugin protocol specification
- SDK reference with examples
- Real plugin implementation walkthrough
- Testing and validation
- Deployment patterns
- Contributing guidelines

**Tone:** Technical, precise, reference-focused

---

### 3. Software Architect Documentation

**Homepage/Entry Point:** `guides/architect-guide.md`

**Architect Journey:**

```text
guides/architect-guide.md (overview)
  ↓
architecture/system-overview.md (with diagrams)
  ├─→ architecture/core-concepts.md (key concepts)
  ├─→ architecture/cost-calculation.md (calculation engine)
  └─→ architecture/diagrams/* (visual architecture)
  ↓
deployment/security.md (security & compliance)
  ↓
deployment/cicd-integration.md (operational integration)
  ↓
architecture/roadmap.md (future planning)
```

**Key Sections:**

- System architecture and design patterns
- Integration points and APIs
- Security and compliance considerations
- Deployment and operational guide
- Scaling and performance considerations
- Upgrade and migration strategies
- Roadmap and future considerations

**Tone:** Technical, analytical, strategic

---

### 4. Business/Executive Documentation

**Homepage/Entry Point:** `guides/business-value.md`

**Executive Journey:**

```text
guides/business-value.md (value proposition)
  ├─→ getting-started/quickstart.md (see it in action)
  ├─→ guides/comparison.md (competitive analysis)
  └─→ architecture/roadmap.md (future direction)
```

**Key Sections:**

- Problem statement (cost visibility challenges)
- Value proposition (time to insight, cost reduction)
- Competitive analysis
- Roadmap and vision
- Success metrics
- Case studies/examples

**Tone:** Business-focused, clear, persuasive

---

## Plugin Documentation Model

### Plugin Status Levels

All plugins follow a clear status progression:

#### 1. **IN PROGRESS** (Vantage)

- Feature complete and tested
- Production use cases in development
- Full documentation
- Support provided

**Documentation:**

- `plugins/vantage/README.md` - Overview and status
- `plugins/vantage/setup.md` - Setup guide
- `plugins/vantage/authentication.md` - API key management
- `plugins/vantage/features.md` - Supported features
- `plugins/vantage/cost-mapping.md` - How costs map to FinFocus
- `plugins/vantage/troubleshooting.md` - Common issues and fixes

#### 2. **PLANNED** (Kubecost, OpenCost)

- Committed roadmap with timeline
- Architecture design complete
- Development starting
- Partial documentation available

**Documentation:**

- `plugins/kubecost/README.md` - Overview
- `plugins/kubecost/coming-soon.md` - Timeline and features
- `plugins/kubecost/differences.md` - How it differs from Vantage

#### 3. **FUTURE** (Flexera, Cloudability, others)

- Conceptual planning phase
- Waiting for customer demand and partnership
- Architectural patterns established for similar services

**Documentation:**

- `plugins/flexera/coming-soon.md` - Timeline and requirements
- `plugins/cloudability/coming-soon.md` - Timeline and requirements

### Plugin Documentation Template

Each production plugin gets:

```text
plugins/{plugin-name}/
├── README.md                    # Plugin overview, features, status
├── setup.md                     # Installation and initial configuration
├── authentication.md            # How to authenticate with service
├── features.md                  # Detailed feature list
├── cost-mapping.md              # How service costs map to FinFocus
├── troubleshooting.md           # Common issues and solutions
├── performance.md               # Performance characteristics and limits
└── api-integration.md           # API endpoints and integration details
```

### Plugin Development Onboarding

For FinOps services building plugins:

1. **Start here:** `plugins/plugin-development.md`
2. **Reference:** `plugins/plugin-sdk.md`
3. **Example:** `plugins/vantage/` (real implementation)
4. **Validate:** `plugins/plugin-checklist.md`
5. **Deploy:** `deployment/docker.md` or `deployment/cicd-integration.md`

---

## GitHub Pages Setup

### Configuration Files

**File:** `.github/workflows/docs.yml` (see GitHub Actions section)

**File:** `docs/_config.yml`

```yaml
# Jekyll configuration
title: FinFocus Documentation
description: Cost visibility for Pulumi infrastructure
url: 'https://docs.finfocus.com'

# Theme (GitHub pages supports several)
theme: jekyll-theme-minimal
# or: jekyll-theme-primer, jekyll-theme-architect

# Exclude files from build
exclude:
  - plan.md
  - llms.txt
  - Makefile
  - node_modules
  - .gitignore

# Plugins
plugins:
  - jekyll-github-metadata
  - jekyll-include-cache

# Markdown processor
markdown: kramdown
kramdown:
  input: GFM
  hard_wrap: false

# Analytics (optional)
google_analytics_id: UA-XXXXXXXXX-X
```

### Directory Requirements

```text
docs/
├── _config.yml                  # Jekyll configuration
├── _includes/                   # Reusable template components
├── _layouts/                    # Page layouts
├── assets/                      # Static assets (CSS, images, JS)
│   ├── css/
│   ├── js/
│   ├── images/
│   └── icons/
└── [content files]
```

### Custom Domain (Optional)

Create `docs/CNAME` if using custom domain:

```text
docs.finfocus.com
```

Then configure domain in GitHub repository settings.

### Hosting

- **GitHub Pages:** Enabled in repository settings
- **Deploy from:** `main` branch, `docs/` folder
- **URL:** `https://rshade.github.io/finfocus/docs` (default)
- **Custom Domain:** `https://docs.finfocus.com` (if configured)

---

## GitHub Actions Pipeline

### Workflows to Create

#### 1. **docs-build.yml** - Build and Deploy Docs

Triggered on:

- Push to main branch (when docs/ changed)
- Pull request to main (preview check)

Actions:

1. Checkout code
2. Setup Ruby + Jekyll
3. Build docs site
4. Run markdown linting
5. Check for broken links
6. Deploy to GitHub Pages (on main push only)

#### 2. **docs-validate.yml** - Validate Doc Quality

Triggered on:

- Any push to main
- Pull request to any branch

Actions:

1. Markdown linting (markdownlint)
2. Frontmatter validation
3. Link checking (markdown-link-check)
4. Prose quality (vale)
5. Check llms.txt is up-to-date

#### 3. **llms-txt-update.yml** - Auto-update llms.txt

Triggered on:

- Any push to docs/ folder

Actions:

1. Generate llms.txt from all markdown files
2. Commit update to main branch

### Implementation Details

See `.github/workflows/` for complete YAML files.

---

## Linting & Quality Tools

### Tools to Configure

#### 1. **markdownlint**

Purpose: Enforce markdown formatting consistency

**Config File:** `docs/.markdownlintrc.json`

```json
{
  "extends": "markdownlint/style/google",
  "rules": {
    "MD003": { "style": "consistent" },
    "MD004": { "style": "consistent" },
    "MD013": { "line_length": 120, "heading_line_length": 120 },
    "MD024": { "siblings_only": true }
  }
}
```

**CI Integration:** GitHub Actions runs on all PRs

#### 2. **prettier**

Purpose: Enforce code and markdown formatting

**Config File:** `.prettierrc.yaml`

```yaml
printWidth: 120
trailingComma: es5
tabWidth: 2
useTabs: false
semi: true
singleQuote: true
arrowParens: always
```

#### 3. **markdown-link-check**

Purpose: Detect broken links in documentation

**Config File:** `docs/.markdown-link-check.json`

```json
{
  "ignorePatterns": [
    {
      "pattern": "^https://github.com/.*#L[0-9]+$",
      "description": "GitHub code line links"
    }
  ]
}
```

#### 4. **vale**

Purpose: Prose quality, tone, and style consistency

**Config File:** `docs/.vale.ini`

```ini
[*]
BasedOnStyles = Google, Microsoft
IgnoredClasses = pre,code
IgnoredScopes = code

[**/architecture/*.md]
BasedOnStyles = Microsoft
```

### Makefile Targets for Docs

```makefile
.PHONY: docs-lint
docs-lint:
    markdownlint docs/**/*.md
    markdown-link-check docs/**/*.md
    vale docs/**/*.md

.PHONY: docs-format
docs-format:
    prettier --write "docs/**/*.md"

.PHONY: docs-build
docs-build:
    bundle exec jekyll build --source docs --destination docs/_site

.PHONY: docs-serve
docs-serve:
    bundle exec jekyll serve --source docs

.PHONY: docs-validate
docs-validate: docs-lint
    ./scripts/validate-frontmatter.sh
    ./scripts/update-llms-txt.sh
```

---

## LLM-Friendly Documentation

### Purpose

`docs/llms.txt` is a machine-readable index of all documentation, designed to
help LLMs understand the documentation structure and content quickly.

**Use Cases:**

- AI-powered search within documentation
- Context injection for AI assistants helping with FinFocus
- Documentation completeness verification
- Automatic documentation generation and updates

### Structure

```text
# FinFocus Documentation Index

## Quick Navigation

### For End Users
- Getting Started: docs/getting-started/quickstart.md
- Installation: docs/getting-started/installation.md
- CLI Reference: docs/reference/cli-commands.md
- Troubleshooting: docs/support/troubleshooting.md

### For Plugin Developers
- Plugin Development Guide: docs/plugins/plugin-development.md
- Plugin SDK Reference: docs/plugins/plugin-sdk.md
- Vantage Plugin Example: docs/plugins/vantage/

### For Software Architects
- System Architecture: docs/architecture/system-overview.md
- Plugin Protocol: docs/architecture/plugin-protocol.md
- Deployment Guide: docs/deployment/

### For Business/Product
- Business Value: docs/guides/business-value.md
- Comparison: docs/guides/comparison.md
- Roadmap: docs/architecture/roadmap.md

## Complete Documentation Map

### Core Guides
- [User Guide](docs/guides/user-guide.md) - Complete guide for end users
- [Developer Guide](docs/guides/developer-guide.md) - Complete guide for plugin developers
- [Architect Guide](docs/guides/architect-guide.md) - Complete guide for software architects
- [Business Value](docs/guides/business-value.md) - Value proposition and ROI

### Getting Started (Quickest path to success)
- [Quickstart](docs/getting-started/quickstart.md) - 5-minute getting started
- [Installation](docs/getting-started/installation.md) - Detailed install guide
- [Prerequisites](docs/getting-started/prerequisites.md) - System requirements
- [Vantage Example](docs/getting-started/examples/vantage-setup.md) - Complete Vantage setup
- [Local Pricing](docs/getting-started/examples/local-pricing.md) - No external service needed
- [Multi-Plugin](docs/getting-started/examples/multi-plugin.md) - Using multiple plugins

### Architecture & Design
- [System Overview](docs/architecture/system-overview.md) - High-level architecture
- [Core Concepts](docs/architecture/core-concepts.md) - Key concepts explained
- [Plugin Protocol](docs/architecture/plugin-protocol.md) - gRPC protocol specification
- [Cost Calculation](docs/architecture/cost-calculation.md) - How costs are calculated
- [Actual vs Projected](docs/architecture/actual-vs-projected.md) - Cost types explained
- [Roadmap](docs/architecture/roadmap.md) - Planned features and timeline

### Architecture Diagrams (visual reference)
- [System Architecture Diagram](docs/architecture/diagrams/system-architecture.md)
- [Data Flow Diagram](docs/architecture/diagrams/data-flow.md)
- [Plugin Lifecycle Diagram](docs/architecture/diagrams/plugin-lifecycle.md)
- [Cost Calculation Flow](docs/architecture/diagrams/cost-calculation-flow.md)
- [Integration Example](docs/architecture/diagrams/integration-example.md)

### Plugin Documentation

#### Plugin Development
- [Plugin Development Guide](docs/plugins/plugin-development.md) - How to build plugins
- [Plugin SDK Reference](docs/plugins/plugin-sdk.md) - API reference
- [Plugin Examples](docs/plugins/plugin-examples.md) - Code examples
- [Plugin Checklist](docs/plugins/plugin-checklist.md) - Completeness checklist

#### Vantage Plugin (IN PROGRESS)
- [README](docs/plugins/vantage/README.md) - Overview and status
- [Setup Guide](docs/plugins/vantage/setup.md) - Installation and configuration
- [Authentication](docs/plugins/vantage/authentication.md) - API key management
- [Features](docs/plugins/vantage/features.md) - Supported features
- [Cost Mapping](docs/plugins/vantage/cost-mapping.md) - How costs map to FinFocus
- [Troubleshooting](docs/plugins/vantage/troubleshooting.md) - Common issues and fixes

#### Kubecost Plugin (PLANNED)
- [README](docs/plugins/kubecost/README.md) - Overview
- [Coming Soon](docs/plugins/kubecost/coming-soon.md) - Timeline and features
- [Differences](docs/plugins/kubecost/differences.md) - How it differs from Vantage

#### Future Plugins
- [Flexera Coming Soon](docs/plugins/flexera/coming-soon.md)
- [Cloudability Coming Soon](docs/plugins/cloudability/coming-soon.md)

### Reference Documentation
- [CLI Commands](docs/reference/cli-commands.md) - Complete CLI reference
- [CLI Flags](docs/reference/cli-flags.md) - Detailed flag documentation
- [Configuration](docs/reference/config-reference.md) - Configuration options
- [API Reference](docs/reference/api-reference.md) - gRPC API reference
- [Error Codes](docs/reference/error-codes.md) - Error codes and solutions
- [Environment Variables](docs/reference/environment-variables.md) - Env var reference

### Deployment & Operations
- [Installation Guide](docs/deployment/installation.md) - Detailed install procedures
- [Configuration Guide](docs/deployment/configuration.md) - How to configure FinFocus
- [Docker Deployment](docs/deployment/docker.md) - Docker setup
- [Kubernetes Deployment](docs/deployment/kubernetes.md) - K8s deployment (FUTURE)
- [CI/CD Integration](docs/deployment/cicd-integration.md) - Pipeline integration examples
- [Security Best Practices](docs/deployment/security.md) - Security and compliance
- [Troubleshooting](docs/deployment/troubleshooting.md) - Operational troubleshooting

### Support & Community
- [FAQ](docs/support/faq.md) - Frequently asked questions
- [Troubleshooting Guide](docs/support/troubleshooting.md) - Troubleshooting by symptom
- [Contributing Guide](docs/support/contributing.md) - How to contribute
- [Code of Conduct](docs/support/code-of-conduct.md) - Community guidelines
- [Support Channels](docs/support/support-channels.md) - Where to get help

## Key Concepts

### FinFocus
CLI tool for calculating cloud infrastructure costs from Pulumi definitions.

**Three Cost Types:**
1. **Projected Costs** - Estimated costs from Pulumi preview
2. **Actual Costs** - Historical costs from cloud provider APIs
3. **Cost Changes** - Difference between current and previous states

### Resource
Representation of cloud infrastructure (e.g., AWS EC2 instance, Azure VM).

**Properties:**
- Type: aws:ec2:Instance, azure:compute:VirtualMachine
- Provider: aws, azure, gcp
- Metadata: tags, name, region
- Estimated Cost: projected monthly cost
- Actual Cost: actual monthly cost from provider

### Plugin
External service that provides cost data to FinFocus.

**Types:**
- **Cloud-Native** (Vantage, Flexera): Aggregate costs from multiple clouds
- **Kubernetes** (Kubecost, OpenCost): Kubernetes-specific cost allocation
- **Local** (YAML specs): No external service needed

**Communication:** gRPC protocol buffers

### Cost Aggregation
Combining costs from multiple resources and plugins.

**Types:**
- **By Provider** (AWS, Azure, GCP)
- **By Resource Type** (EC2, RDS, Lambda)
- **By Time** (Daily, Monthly)
- **By Tag** (Environment, Team, Application)

## Documentation Maintenance

### Regular Updates
- Review and update llms.txt monthly
- Update plugin status as development progresses
- Add examples from user feedback
- Update roadmap as features ship

### Quality Standards
- All code examples tested
- Links checked monthly
- Screenshots updated when UI changes
- API reference stays in sync with code

### Automated Validation
- GitHub Actions validates on every commit
- Broken links caught automatically
- Formatting enforced with prettier
- Prose quality checked with vale

## How to Use This File

**For AI Assistants:**
- Use this as context when answering questions about FinFocus
- Reference specific docs by their paths
- Point users to the most relevant documentation

**For Documentation Maintainers:**
- Use this as a checklist for completeness
- Update when creating new documentation
- Validate that llms.txt matches actual file structure

**For Developers Contributing:**
- Understand overall documentation structure
- Find where your contribution should go
- See what related docs might need updates
```

### Automatic Generation

Script: `scripts/update-llms-txt.sh`

```bash
#!/bin/bash
# Generate llms.txt from documentation structure
# Run by GitHub Actions after changes to docs/

find docs -name "*.md" ! -name "llms.txt" | sort | \
while read file; do
  echo "- [$file](../$file)"
done > docs/llms.txt
```

---

## Maintenance & Updates

### Documentation Review Cycle

**Monthly (First Monday):**

- Review plugin status and update accordingly
- Check for broken links (automated but manual review)
- Update roadmap with progress
- Verify all code examples still work

**Quarterly (First day of quarter):**

- Update llms.txt if structure changed
- Review and update architecture diagrams
- Ensure guides match current feature set
- Add/update case studies or examples

**On Release:**

- Update version in all relevant docs
- Add new features to appropriate guides
- Update CLI reference
- Update roadmap

**When Plugin Status Changes:**

- Update plugin status (in progress → planned → released)
- Add/update plugin documentation
- Update roadmap
- Update business-value.md

### Roles & Responsibilities

**Documentation Owner:** Technical Content Architect

- Maintain documentation strategy and structure
- Ensure consistency across all guides
- Update llms.txt and plan.md
- Review major documentation changes

**Content Contributors:** Engineers, Product Managers, Plugin Developers

- Write content for their areas
- Keep examples current
- Report broken links or issues

**Content Reviewer:** Designated team member

- Review new/updated documentation
- Verify accuracy and clarity
- Check tone matches audience
- Ensure formatting consistency

---

## Implementation Timeline

### Phase 1: Foundation (Week 1-2)

**Goals:**

- Create directory structure
- Setup GitHub Pages
- Create plan.md (this file)
- Setup linting tools

**Deliverables:**

- [ ] Complete docs/ directory structure
- [ ] Jekyll \_config.yml and theme selection
- [ ] .markdownlintrc.json, .prettierrc.yaml configured
- [ ] plan.md created (COMPLETE)
- [ ] GitHub Pages enabled
- [ ] Makefile targets for docs commands

---

### Phase 2: Core Guides (Week 3-4)

**Goals:**

- Create main audience guides
- Establish tone and style for each audience
- Create getting-started section

**Deliverables:**

- [ ] guides/user-guide.md
- [ ] guides/developer-guide.md
- [ ] guides/architect-guide.md
- [ ] guides/business-value.md
- [ ] getting-started/quickstart.md
- [ ] getting-started/installation.md

---

### Phase 3: Architecture & Reference (Week 5-6)

**Goals:**

- Deep dive architecture documentation
- Complete CLI and API reference
- Create diagrams

**Deliverables:**

- [ ] architecture/system-overview.md
- [ ] architecture/plugin-protocol.md
- [ ] architecture/cost-calculation.md
- [ ] reference/cli-commands.md
- [ ] reference/api-reference.md
- [ ] architecture/diagrams/ (5 diagrams)

---

### Phase 4: Plugin Documentation (Week 7-8)

**Goals:**

- Complete Vantage plugin documentation
- Create Kubecost "coming soon" documentation
- Establish plugin documentation patterns

**Deliverables:**

- [ ] plugins/plugin-development.md
- [ ] plugins/plugin-sdk.md
- [ ] plugins/vantage/ (all files)
- [ ] plugins/kubecost/coming-soon.md
- [ ] plugins/flexera/coming-soon.md
- [ ] plugins/cloudability/coming-soon.md

---

### Phase 5: GitHub Actions & Automation (Week 9)

**Goals:**

- Setup documentation build pipeline
- Create validation workflow
- Implement llms.txt auto-generation

**Deliverables:**

- [ ] .github/workflows/docs-build.yml
- [ ] .github/workflows/docs-validate.yml
- [ ] .github/workflows/llms-txt-update.yml
- [ ] scripts/update-llms-txt.sh
- [ ] scripts/validate-frontmatter.sh

---

### Phase 6: Integration & Polish (Week 10)

**Goals:**

- Update root documentation files
- Test full documentation site
- Create support and contribution content

**Deliverables:**

- [ ] Update README.md (add docs link)
- [ ] Update CLAUDE.md (add documentation strategy)
- [ ] Update CONTRIBUTING.md (add docs section)
- [ ] support/contributing.md
- [ ] support/faq.md
- [ ] support/troubleshooting.md
- [ ] Deploy docs to GitHub Pages
- [ ] Test all links work

---

## Success Metrics

Documentation will be considered successful when:

1. **Discoverability:** Users can find answers to top 20 questions within 2 minutes
2. **Completeness:** 90%+ of audience workflows have documented guides
3. **Accuracy:** Zero broken links and 0 outdated code examples
4. **Usability:** Bounce rate < 30% from docs site (Google Analytics)
5. **Maintainability:** All changes caught by automated linting
6. **Adoption:** Plugin developers can build production plugin in < 1 week
7. **Support:** 80%+ of support questions answered by docs (tracked in Discord)

---

## Related Files

- `.github/workflows/docs-build.yml` - Build and deploy workflow
- `.github/workflows/docs-validate.yml` - Validation workflow
- `Makefile` - Documentation commands
- `CONTRIBUTING.md` - Contribution guidelines
- `README.md` - Repository README (links to docs)
- `CLAUDE.md` - Internal documentation strategy notes

---

## Appendix: Quick Reference

### Directory Purpose Quick Guide

| Directory          | Purpose                           | Audience              |
| ------------------ | --------------------------------- | --------------------- |
| `guides/`          | Audience-specific complete guides | All                   |
| `getting-started/` | Quick onboarding                  | End Users             |
| `architecture/`    | System design deep dive           | Architects            |
| `plugins/`         | Plugin-specific documentation     | Engineers             |
| `reference/`       | API and CLI reference             | Engineers, Architects |
| `deployment/`      | Installation and operations       | End Users, Architects |
| `support/`         | Help and community                | Everyone              |

### Key Files to Create First

1. `plan.md` ✓ (you are reading this)
2. `README.md` (home page)
3. `guides/user-guide.md` (primary end user guide)
4. `getting-started/quickstart.md` (5-minute onboarding)
5. `_config.yml` (Jekyll config)

### Quick Start for Creating Content

```bash
# Create new guide
cp docs/guides/user-guide.md docs/guides/my-guide.md

# Use this frontmatter template:
---
layout: guide
title: My Guide Title
description: Short description for search
---

# Lint as you write
make docs-lint

# Preview locally
make docs-serve

# Validate links
make docs-validate
```

---

**Document Version:** 1.0
**Last Updated:** 2025-10-29
**Next Review:** 2025-11-29
